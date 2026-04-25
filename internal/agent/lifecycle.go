package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/Lichas/maxclaw/internal/session"
)

// AgentLifecycle manages the complete verification→reflection→adaptation→persistence→evolution cycle
type AgentLifecycle struct {
	// Core components
	ErrorClassifier   *ErrorClassifier
	ContextCompressor *ContextCompressor
	InsightsEngine    *InsightsEngine
	AdaptationManager *AdaptationManager
	CheckpointManager *CheckpointManager
	EvolutionTracker  *EvolutionTracker
	FeedbackDetector  *FeedbackDetector // NEW: Detect user feedback sentiment
	FeedbackLearner   *FeedbackLearner  // NEW: Learn from user feedback

	// Configuration
	Enabled           bool
	EnableCompression bool
	EnableFallbacks   bool
	EnableCheckpoints bool
	EnableEvolution   bool
	EnableFeedback    bool // NEW: Enable feedback learning
}

// NewAgentLifecycle creates a new agent lifecycle manager
func NewAgentLifecycle(workspace string, config RuntimeConfig) *AgentLifecycle {
	return &AgentLifecycle{
		ErrorClassifier:   NewErrorClassifier(),
		ContextCompressor: nil, // Initialized with model info
		InsightsEngine:    NewInsightsEngine(),
		AdaptationManager: NewAdaptationManager(config, 3),
		CheckpointManager: NewCheckpointManager(true, 50, workspace),
		EvolutionTracker:  NewEvolutionTracker(workspace),
		FeedbackLearner:   NewFeedbackLearner(workspace),
		Enabled:           true,
		EnableCompression: true,
		EnableFallbacks:   true,
		EnableCheckpoints: true,
		EnableEvolution:   true,
		EnableFeedback:    true,
	}
}

// InitializeFeedback initializes the feedback detector with LLM provider
func (al *AgentLifecycle) InitializeFeedback(llmProvider providers.LLMProvider, llmModel string) {
	al.FeedbackDetector = NewFeedbackDetector(llmProvider, llmModel)
}

// InitializeCompression initializes the context compressor with model information
func (al *AgentLifecycle) InitializeCompression(model, provider string, configContextLength int) {
	al.ContextCompressor = NewContextCompressor(
		model,
		0.50,  // threshold percent
		3,     // protect first N
		20,    // protect last N
		0.20,  // summary target ratio
		false, // quiet mode
		"",    // summary model override
		configContextLength,
		provider,
	)
}

// AddFallbackProvider adds a fallback provider
func (al *AgentLifecycle) AddFallbackProvider(fp FallbackProvider) {
	if al.AdaptationManager != nil {
		al.AdaptationManager.AddFallbackProvider(fp)
	}
}

// StartSession marks the beginning of a new session
func (al *AgentLifecycle) StartSession(sessionKey, model, provider string) {
	if !al.Enabled {
		return
	}

	if al.EvolutionTracker != nil && al.EnableEvolution {
		al.EvolutionTracker.StartSession(model, provider)
	}

	if al.AdaptationManager != nil {
		al.AdaptationManager.RestorePrimaryRuntime()
	}
}

// EndSession marks the end of a session
func (al *AgentLifecycle) EndSession(sess *session.Session, history []session.Message) {
	if !al.Enabled {
		return
	}

	// Persist session
	if al.CheckpointManager != nil && al.EnableCheckpoints {
		persistence := NewSessionPersistence(al.CheckpointManager.Workspace)
		if err := persistence.PersistSession(sess, history); err != nil {
			fmt.Printf("[Lifecycle] Warning: failed to persist session: %v\n", err)
		}
	}

	// End evolution tracking
	if al.EvolutionTracker != nil && al.EnableEvolution {
		al.EvolutionTracker.EndSession()
	}
}

// BeforeAPICall performs pre-API call lifecycle checks
func (al *AgentLifecycle) BeforeAPICall(ctx context.Context, messages []CompressorMessage) error {
	if !al.Enabled {
		return nil
	}

	// Check if compression is needed
	if al.EnableCompression && al.ContextCompressor != nil {
		if al.ContextCompressor.ShouldCompressPreflight(messages) {
			// Compression will be handled by the caller
			return fmt.Errorf("compression needed")
		}
	}

	return nil
}

// HandleAPIError handles an API error according to the lifecycle cycle
func (al *AgentLifecycle) HandleAPIError(ctx context.Context, err error, provider, model string, approxTokens, numMessages int) (*AdaptationAction, *RuntimeConfig, error) {
	if !al.Enabled || err == nil {
		return nil, nil, nil
	}

	// 1. Verification: Classify the error
	contextLength := 128000
	if al.ContextCompressor != nil {
		contextLength = al.ContextCompressor.ContextLength
	}

	classifiedErr := al.ErrorClassifier.ClassifyError(
		err,
		provider,
		model,
		approxTokens,
		contextLength,
		numMessages,
	)

	if classifiedErr == nil {
		return nil, nil, err
	}

	// Log error for evolution tracking
	if al.EvolutionTracker != nil && al.EnableEvolution {
		al.EvolutionTracker.RecordError(
			classifiedErr.Reason,
			classifiedErr.Message,
			0, // retry count - will be updated
			false,
			0,
		)
	}

	// 2. Reflection: Log the error details
	fmt.Printf("[Lifecycle] Error classified: %s (retryable: %v)\n", classifiedErr.Reason, classifiedErr.Retryable)

	// 3. Adaptation: Decide what to do
	if al.AdaptationManager != nil {
		action := al.AdaptationManager.HandleError(ctx, classifiedErr)

		switch action {
		case ActionCompress:
			return &action, nil, fmt.Errorf("context compression required")

		case ActionFallback:
			if al.EnableFallbacks {
				newRuntime, ok := al.AdaptationManager.TryActivateFallback()
				if ok {
					fmt.Printf("[Lifecycle] Activated fallback: %s\n", newRuntime.Model)
					return &action, &newRuntime, nil
				}
			}

		case ActionAdjustContext:
			newLength := al.AdaptationManager.StepDownContextLength()
			fmt.Printf("[Lifecycle] Adjusted context length to %d\n", newLength)
			if al.ContextCompressor != nil {
				al.ContextCompressor.ContextLength = newLength
				al.ContextCompressor.ThresholdTokens = int(float64(newLength) * al.ContextCompressor.ThresholdPercent)
			}
			return &action, nil, fmt.Errorf("context adjusted, retry needed")

		case ActionRetry:
			backoff := al.AdaptationManager.GetRetryBackoff()
			fmt.Printf("[Lifecycle] Retrying after %v...\n", backoff)
			time.Sleep(backoff)
			return &action, nil, nil

		case ActionReduceOutput:
			if al.AdaptationManager != nil {
				// Reduce max tokens
				newMaxTokens := al.AdaptationManager.GetEffectiveMaxTokens() / 2
				if newMaxTokens < 1000 {
					newMaxTokens = 1000
				}
				al.AdaptationManager.SetEphemeralMaxTokens(newMaxTokens)
				return &action, nil, fmt.Errorf("output reduced, retry needed")
			}

		case ActionAbort:
			return &action, nil, err
		}
	}

	return nil, nil, err
}

// RecordSuccess records a successful API call
func (al *AgentLifecycle) RecordSuccess(model, provider string, tokens int, latency time.Duration) {
	if !al.Enabled {
		return
	}

	if al.EvolutionTracker != nil && al.EnableEvolution {
		al.EvolutionTracker.RecordAPICall(model, provider, tokens, latency)
	}

	if al.AdaptationManager != nil {
		al.AdaptationManager.ResetRetryCount()
	}
}

// SaveCheckpoint saves a checkpoint
func (al *AgentLifecycle) SaveCheckpoint(sessionKey string, messages []session.Message, systemPrompt string, iteration int) {
	if !al.Enabled || !al.EnableCheckpoints || al.CheckpointManager == nil {
		return
	}

	_, err := al.CheckpointManager.Save(sessionKey, messages, systemPrompt, iteration, nil)
	if err != nil {
		fmt.Printf("[Lifecycle] Warning: failed to save checkpoint: %v\n", err)
	}
}

// LoadLatestCheckpoint loads the most recent checkpoint
func (al *AgentLifecycle) LoadLatestCheckpoint(sessionKey string) (*Checkpoint, error) {
	if !al.Enabled || !al.EnableCheckpoints || al.CheckpointManager == nil {
		return nil, fmt.Errorf("checkpoints disabled")
	}

	return al.CheckpointManager.LoadLatest(sessionKey)
}

// CompressContext compresses the context if needed
func (al *AgentLifecycle) CompressContext(ctx context.Context, messages []CompressorMessage, systemPrompt string) ([]CompressorMessage, error) {
	if !al.Enabled || !al.EnableCompression || al.ContextCompressor == nil {
		return messages, nil
	}

	result, err := al.ContextCompressor.Compress(ctx, messages, systemPrompt)
	if err != nil {
		return messages, err
	}

	fmt.Printf("[Lifecycle] Context compressed: %d → %d messages (summary: %d chars)\n",
		len(messages), len(result.Messages), len(result.Summary))

	return result.Messages, nil
}

// GetInsights generates insights for the sessions
func (al *AgentLifecycle) GetInsights(days int, source string) *InsightsReport {
	if al.InsightsEngine == nil {
		return nil
	}

	return al.InsightsEngine.Generate(days, source)
}

// GetStats returns lifecycle statistics
func (al *AgentLifecycle) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":            al.Enabled,
		"enable_compression": al.EnableCompression,
		"enable_fallbacks":   al.EnableFallbacks,
		"enable_checkpoints": al.EnableCheckpoints,
		"enable_evolution":   al.EnableEvolution,
	}

	if al.AdaptationManager != nil {
		stats["adaptation"] = al.AdaptationManager.GetStats()
	}

	if al.EvolutionTracker != nil && al.EnableEvolution {
		stats["evolution"] = al.EvolutionTracker.GetStats()
	}

	return stats
}

// ShouldCompress checks if context compression is needed
func (al *AgentLifecycle) ShouldCompress(promptTokens int) bool {
	if !al.Enabled || !al.EnableCompression || al.ContextCompressor == nil {
		return false
	}
	return al.ContextCompressor.ShouldCompress(promptTokens)
}

// GetCompressionStatus returns the current compression status
func (al *AgentLifecycle) GetCompressionStatus() map[string]interface{} {
	if al.ContextCompressor == nil {
		return map[string]interface{}{"enabled": false}
	}
	return al.ContextCompressor.GetStatus()
}

// RestorePrimaryRuntime restores the primary runtime after fallback
func (al *AgentLifecycle) RestorePrimaryRuntime() RuntimeConfig {
	if al.AdaptationManager == nil {
		return RuntimeConfig{}
	}
	return al.AdaptationManager.RestorePrimaryRuntime()
}

// IsFallbackActive returns true if a fallback is currently active
func (al *AgentLifecycle) IsFallbackActive() bool {
	if al.AdaptationManager == nil {
		return false
	}
	return al.AdaptationManager.IsFallbackActive()
}

// RecordToolExecution records a tool execution for evolution tracking
func (al *AgentLifecycle) RecordToolExecution(toolName string, success bool, executionTime time.Duration) {
	if !al.Enabled || !al.EnableEvolution || al.EvolutionTracker == nil {
		return
	}
	al.EvolutionTracker.RecordToolCall(toolName, success, executionTime)
}

// GetRecoveryRecommendation gets a recommendation for handling an error
func (al *AgentLifecycle) GetRecoveryRecommendation(errorReason ErrorReason) *RecoveryRecommendation {
	if !al.Enabled || !al.EnableEvolution || al.EvolutionTracker == nil {
		return nil
	}
	return al.EvolutionTracker.GetRecoveryRecommendation(errorReason)
}

// ConvertSessionMessagesToCompressor converts session messages to compressor format
func ConvertSessionMessagesToCompressor(messages []session.Message) []CompressorMessage {
	result := make([]CompressorMessage, len(messages))
	for i, msg := range messages {
		result[i] = CompressorMessage{
			Role:    msg.Role,
			Content: msg.Content,
			// Note: ToolCallID not available in session.Message, would need to extract from timeline
		}
		// Note: ToolCalls and Reasoning would need to be extracted from metadata
	}
	return result
}

// ConvertCompressorMessagesToSession converts compressor messages back to session format
func ConvertCompressorMessagesToSession(messages []CompressorMessage) []session.Message {
	result := make([]session.Message, len(messages))
	for i, msg := range messages {
		result[i] = session.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: time.Now(),
		}
	}
	return result
}

// ==================== Feedback Learning Methods ====================

// DetectUserFeedback analyzes user message for feedback sentiment
func (al *AgentLifecycle) DetectUserFeedback(ctx context.Context, userMsg, agentOutput string) *FeedbackResult {
	if !al.Enabled || !al.EnableFeedback || al.FeedbackDetector == nil {
		return &FeedbackResult{Type: FeedbackNeutral, Confidence: 0.5}
	}
	return al.FeedbackDetector.Detect(ctx, userMsg, agentOutput)
}

// LearnFromFeedback records user feedback and extracts lesson
func (al *AgentLifecycle) LearnFromFeedback(
	result *FeedbackResult,
	taskContext string,
	agentOutput string,
	userFeedback string,
) *FeedbackLesson {
	if !al.Enabled || !al.EnableFeedback || al.FeedbackLearner == nil {
		return nil
	}
	return al.FeedbackLearner.RecordFeedback(result, taskContext, agentOutput, userFeedback)
}

// GetFeedbackLessons retrieves relevant lessons for a task
func (al *AgentLifecycle) GetFeedbackLessons(taskType string, maxResults int) []*FeedbackLesson {
	if !al.Enabled || !al.EnableFeedback || al.FeedbackLearner == nil {
		return nil
	}
	return al.FeedbackLearner.GetRelevantLessons(taskType, maxResults)
}

// BuildFeedbackEnhancedPrompt generates system prompt with learned lessons
func (al *AgentLifecycle) BuildFeedbackEnhancedPrompt(taskType string) string {
	if !al.Enabled || !al.EnableFeedback || al.FeedbackLearner == nil {
		return ""
	}
	return al.FeedbackLearner.BuildSystemPromptEnhancement(taskType)
}

// GetFeedbackStats returns feedback detection and learning statistics
func (al *AgentLifecycle) GetFeedbackStats() map[string]interface{} {
	if al.FeedbackDetector == nil && al.FeedbackLearner == nil {
		return map[string]interface{}{"enabled": false}
	}

	stats := map[string]interface{}{
		"enabled": al.EnableFeedback,
	}

	if al.FeedbackDetector != nil {
		stats["detector"] = al.FeedbackDetector.GetStats()
	}

	if al.FeedbackLearner != nil {
		stats["learner"] = al.FeedbackLearner.GetStats()
	}

	return stats
}
