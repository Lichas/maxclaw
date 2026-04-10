package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/providers"
)

// RuntimeConfig holds the primary runtime configuration
type RuntimeConfig struct {
	Provider           providers.LLMProvider
	Model              string
	BaseURL            string
	APIKey             string
	APIMode            string
	MaxIterations      int
	MaxTokens          int
	Temperature        float64
	UsePromptCaching   bool
	ContextLength      int
}

// FallbackProvider represents a fallback provider in the chain
type FallbackProvider struct {
	Provider  providers.LLMProvider
	Model     string
	BaseURL   string
	APIKey    string
	APIMode   string
	Priority  int
}

// AdaptationManager manages runtime adaptation including fallback and parameter adjustment
type AdaptationManager struct {
	mu sync.RWMutex

	// Primary runtime
	PrimaryRuntime RuntimeConfig

	// Fallback chain
	FallbackChain     []FallbackProvider
	FallbackIndex     int
	FallbackActivated bool

	// Context adaptation
	ContextLength        int
	OriginalContextLength int
	CompressionThreshold float64

	// Retry state
	RetryCount      int
	MaxRetries      int
	LastError       *ClassifiedError
	LastRetryAt     time.Time

	// Adaptive parameters
	AdaptiveMaxTokens   bool
	EphemeralMaxTokens  int

	// Callbacks
	OnFallbackActivated func(from, to string)
	OnContextAdjusted   func(old, new int)
}

// NewAdaptationManager creates a new adaptation manager
func NewAdaptationManager(primary RuntimeConfig, maxRetries int) *AdaptationManager {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &AdaptationManager{
		PrimaryRuntime:        primary,
		FallbackChain:         make([]FallbackProvider, 0),
		FallbackIndex:         0,
		FallbackActivated:     false,
		ContextLength:         primary.ContextLength,
		OriginalContextLength: primary.ContextLength,
		CompressionThreshold:  0.50,
		MaxRetries:            maxRetries,
		RetryCount:            0,
		AdaptiveMaxTokens:     true,
		EphemeralMaxTokens:    0,
	}
}

// AddFallbackProvider adds a fallback provider to the chain
func (am *AdaptationManager) AddFallbackProvider(fp FallbackProvider) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	// Insert in priority order
	inserted := false
	for i, existing := range am.FallbackChain {
		if fp.Priority < existing.Priority {
			am.FallbackChain = append(am.FallbackChain[:i], append([]FallbackProvider{fp}, am.FallbackChain[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		am.FallbackChain = append(am.FallbackChain, fp)
	}
}

// ShouldFallback checks if fallback should be attempted
func (am *AdaptationManager) ShouldFallback(classifiedErr *ClassifiedError) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if classifiedErr == nil {
		return false
	}

	// Check if fallback is recommended for this error
	if !classifiedErr.ShouldFallback {
		return false
	}

	// Check if we have fallback providers available
	if am.FallbackIndex >= len(am.FallbackChain) {
		return false // No more fallbacks available
	}

	// Check retry count
	if am.RetryCount >= am.MaxRetries {
		return true // Max retries exceeded, try fallback
	}

	// Auth and billing errors should fallback immediately
	if classifiedErr.IsAuth() || classifiedErr.Reason == ErrorReasonBilling {
		return true
	}

	// Model not found should fallback immediately
	if classifiedErr.Reason == ErrorReasonModelNotFound {
		return true
	}

	return false
}

// TryActivateFallback attempts to activate the next fallback provider
func (am *AdaptationManager) TryActivateFallback() (RuntimeConfig, bool) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.FallbackIndex >= len(am.FallbackChain) {
		return RuntimeConfig{}, false
	}

	fallback := am.FallbackChain[am.FallbackIndex]
	am.FallbackIndex++
	am.FallbackActivated = true
	am.RetryCount = 0 // Reset retry count for new provider

	newRuntime := RuntimeConfig{
		Provider:     fallback.Provider,
		Model:        fallback.Model,
		BaseURL:      fallback.BaseURL,
		APIKey:       fallback.APIKey,
		APIMode:      fallback.APIMode,
		MaxIterations: am.PrimaryRuntime.MaxIterations,
		MaxTokens:    am.PrimaryRuntime.MaxTokens,
		Temperature:  am.PrimaryRuntime.Temperature,
	}

	// Trigger callback
	if am.OnFallbackActivated != nil {
		from := fmt.Sprintf("%s (%s)", am.PrimaryRuntime.Model, am.getProviderName(am.PrimaryRuntime.Provider))
		to := fmt.Sprintf("%s (%s)", newRuntime.Model, am.getProviderName(newRuntime.Provider))
		go am.OnFallbackActivated(from, to)
	}

	return newRuntime, true
}

// RestorePrimaryRuntime restores the primary runtime configuration
func (am *AdaptationManager) RestorePrimaryRuntime() RuntimeConfig {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.FallbackActivated {
		return am.PrimaryRuntime
	}

	am.FallbackActivated = false
	am.FallbackIndex = 0
	am.RetryCount = 0

	// Reset context length to original
	if am.ContextLength != am.OriginalContextLength {
		am.ContextLength = am.OriginalContextLength
	}

	return am.PrimaryRuntime
}

// IsFallbackActive returns true if a fallback provider is currently active
func (am *AdaptationManager) IsFallbackActive() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.FallbackActivated
}

// RecordRetry records a retry attempt
func (am *AdaptationManager) RecordRetry(classifiedErr *ClassifiedError) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.RetryCount++
	am.LastError = classifiedErr
	am.LastRetryAt = time.Now()
}

// ShouldRetry checks if another retry should be attempted
func (am *AdaptationManager) ShouldRetry(classifiedErr *ClassifiedError) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if classifiedErr == nil {
		return false
	}

	if !classifiedErr.Retryable {
		return false
	}

	if am.RetryCount >= am.MaxRetries {
		return false
	}

	return true
}

// ResetRetryCount resets the retry counter
func (am *AdaptationManager) ResetRetryCount() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.RetryCount = 0
}

// AdjustContextLength adjusts the context length (e.g., after context overflow error)
func (am *AdaptationManager) AdjustContextLength(newLength int, persistable bool) {
	am.mu.Lock()
	defer am.mu.Unlock()

	oldLength := am.ContextLength
	am.ContextLength = newLength
	
	if !persistable {
		// Don't update original - this is a temporary adjustment
	} else {
		am.OriginalContextLength = newLength
	}

	// Trigger callback
	if am.OnContextAdjusted != nil && oldLength != newLength {
		go am.OnContextAdjusted(oldLength, newLength)
	}
}

// StepDownContextLength steps down to the next lower context tier
func (am *AdaptationManager) StepDownContextLength() int {
	am.mu.Lock()
	defer am.mu.Unlock()

	newLength := GetNextProbeTier(am.ContextLength)
	if newLength < am.ContextLength {
		oldLength := am.ContextLength
		am.ContextLength = newLength
		
		if am.OnContextAdjusted != nil {
			go am.OnContextAdjusted(oldLength, newLength)
		}
	}
	return am.ContextLength
}

// GetContextLength returns the current context length
func (am *AdaptationManager) GetContextLength() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.ContextLength
}

// GetThresholdTokens returns the current compression threshold in tokens
func (am *AdaptationManager) GetThresholdTokens() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return int(float64(am.ContextLength) * am.CompressionThreshold)
}

// SetEphemeralMaxTokens sets a temporary max tokens limit for the next request
func (am *AdaptationManager) SetEphemeralMaxTokens(tokens int) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.EphemeralMaxTokens = tokens
}

// GetEffectiveMaxTokens returns the effective max tokens (including ephemeral)
func (am *AdaptationManager) GetEffectiveMaxTokens() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if am.EphemeralMaxTokens > 0 {
		return am.EphemeralMaxTokens
	}
	return am.PrimaryRuntime.MaxTokens
}

// ClearEphemeralMaxTokens clears the ephemeral max tokens setting
func (am *AdaptationManager) ClearEphemeralMaxTokens() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.EphemeralMaxTokens = 0
}

// GetRetryBackoff returns the backoff duration for the current retry count
func (am *AdaptationManager) GetRetryBackoff() time.Duration {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	// Jittered exponential: 1s, 2s, 4s, 8s with randomness
	baseDelay := time.Duration(1<<am.RetryCount) * time.Second
	if baseDelay > 30*time.Second {
		baseDelay = 30 * time.Second
	}
	
	// Add jitter (±25%)
	jitter := time.Duration(float64(baseDelay) * 0.25 * (float64(time.Now().UnixNano()%100) / 100.0))
	if time.Now().UnixNano()%2 == 0 {
		baseDelay += jitter
	} else {
		baseDelay -= jitter
	}
	
	return baseDelay
}

// HandleError is the main error handling entry point that decides what action to take
func (am *AdaptationManager) HandleError(ctx context.Context, classifiedErr *ClassifiedError) AdaptationAction {
	if classifiedErr == nil {
		return ActionContinue
	}

	// Check for compression need
	if classifiedErr.ShouldCompress {
		return ActionCompress
	}

	// Check if we should fallback
	if am.ShouldFallback(classifiedErr) {
		return ActionFallback
	}

	// Check if we should retry
	if am.ShouldRetry(classifiedErr) {
		am.RecordRetry(classifiedErr)
		return ActionRetry
	}

	// Check for context-specific errors
	if classifiedErr.Reason == ErrorReasonContextOverflow || 
	   classifiedErr.Reason == ErrorReasonLongContextTier {
		// Step down context length
		newLength := am.StepDownContextLength()
		if newLength > 0 {
			return ActionAdjustContext
		}
	}

	// Check for output token issues
	if classifiedErr.Reason == ErrorReasonPayloadTooLarge {
		return ActionReduceOutput
	}

	return ActionAbort
}

// AdaptationAction represents the recommended action after error classification
type AdaptationAction int

const (
	ActionContinue AdaptationAction = iota
	ActionRetry
	ActionFallback
	ActionCompress
	ActionAdjustContext
	ActionReduceOutput
	ActionRotateCredential
	ActionAbort
)

func (a AdaptationAction) String() string {
	switch a {
	case ActionContinue:
		return "continue"
	case ActionRetry:
		return "retry"
	case ActionFallback:
		return "fallback"
	case ActionCompress:
		return "compress"
	case ActionAdjustContext:
		return "adjust_context"
	case ActionReduceOutput:
		return "reduce_output"
	case ActionRotateCredential:
		return "rotate_credential"
	case ActionAbort:
		return "abort"
	default:
		return "unknown"
	}
}

// Helper function
func (am *AdaptationManager) getProviderName(provider providers.LLMProvider) string {
	if provider == nil {
		return "unknown"
	}
	// This would typically call a method on the provider to get its name
	return "provider"
}

// GetStats returns current adaptation statistics
func (am *AdaptationManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return map[string]interface{}{
		"fallback_activated":     am.FallbackActivated,
		"fallback_index":         am.FallbackIndex,
		"fallback_count":         len(am.FallbackChain),
		"retry_count":            am.RetryCount,
		"max_retries":            am.MaxRetries,
		"context_length":         am.ContextLength,
		"original_context_length": am.OriginalContextLength,
		"threshold_tokens":       int(float64(am.ContextLength) * am.CompressionThreshold),
		"ephemeral_max_tokens":   am.EphemeralMaxTokens,
	}
}

// ModelSwitchRequest represents a request to switch models
type ModelSwitchRequest struct {
	NewModel   string
	NewProvider providers.LLMProvider
	BaseURL    string
	APIKey     string
	APIMode    string
}

// SwitchModel switches the primary model at runtime
func (am *AdaptationManager) SwitchModel(ctx context.Context, req ModelSwitchRequest) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Update primary runtime
	am.PrimaryRuntime.Provider = req.NewProvider
	am.PrimaryRuntime.Model = req.NewModel
	if req.BaseURL != "" {
		am.PrimaryRuntime.BaseURL = req.BaseURL
	}
	if req.APIKey != "" {
		am.PrimaryRuntime.APIKey = req.APIKey
	}
	if req.APIMode != "" {
		am.PrimaryRuntime.APIMode = req.APIMode
	}

	// Reset fallback state
	am.FallbackActivated = false
	am.FallbackIndex = 0
	am.RetryCount = 0

	// Update context length for new model
	newContextLength := getModelContextLength(req.NewModel, req.BaseURL, 0)
	am.ContextLength = newContextLength
	am.OriginalContextLength = newContextLength

	return nil
}
