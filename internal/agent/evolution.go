package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RecoveryStats tracks recovery strategy effectiveness
type RecoveryStats struct {
	Attempts   int     `json:"attempts"`
	Successes  int     `json:"successes"`
	Failures   int     `json:"failures"`
	AvgRetries float64 `json:"avg_retries"`
	AvgLatency float64 `json:"avg_latency_ms"` // Average recovery time in ms
}

// ModelStats tracks model performance
type ModelStats struct {
	Model            string    `json:"model"`
	Provider         string    `json:"provider"`
	TotalCalls       int       `json:"total_calls"`
	SuccessCalls     int       `json:"success_calls"`
	FailedCalls      int       `json:"failed_calls"`
	TotalTokens      int       `json:"total_tokens"`
	AvgLatencyMs     float64   `json:"avg_latency_ms"`
	LastUsed         time.Time `json:"last_used"`
	ErrorPatterns    map[string]int `json:"error_patterns"`
}

// ToolStats tracks tool usage efficiency
type ToolStats struct {
	ToolName         string  `json:"tool_name"`
	TotalCalls       int     `json:"total_calls"`
	SuccessCalls     int     `json:"success_calls"`
	FailedCalls      int     `json:"failed_calls"`
	AvgExecutionTime float64 `json:"avg_execution_time_ms"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
}

// ErrorPattern tracks recurring error patterns
type ErrorPattern struct {
	Pattern      string    `json:"pattern"`
	ErrorReason  ErrorReason `json:"error_reason"`
	Count        int       `json:"count"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	RecoveryRate float64   `json:"recovery_rate"`
}

// EvolutionState represents the persisted evolution state
type EvolutionState struct {
	Version           string                    `json:"version"`
	LastUpdated       time.Time                 `json:"last_updated"`
	ErrorPatterns     map[string]*ErrorPattern  `json:"error_patterns"`
	RecoveryStrategies map[ErrorReason]*RecoveryStats `json:"recovery_strategies"`
	ModelPerformance  map[string]*ModelStats    `json:"model_performance"`
	ToolEfficiency    map[string]*ToolStats     `json:"tool_efficiency"`
	SessionCount      int                       `json:"session_count"`
	TotalAPICalls     int                       `json:"total_api_calls"`
	LearnedParameters map[string]interface{}    `json:"learned_parameters"`
}

// EvolutionTracker tracks learning and evolution of the agent
type EvolutionTracker struct {
	mu sync.RWMutex

	workspace string
	state     *EvolutionState

	// In-memory tracking for current session
	sessionStart    time.Time
	sessionAPICalls int
	currentModel    string
	currentProvider string

	// Pending updates (batched for persistence)
	pendingUpdates bool
}

// NewEvolutionTracker creates a new evolution tracker
func NewEvolutionTracker(workspace string) *EvolutionTracker {
	et := &EvolutionTracker{
		workspace: workspace,
		state: &EvolutionState{
			Version:            "1.0",
			LastUpdated:        time.Now(),
			ErrorPatterns:      make(map[string]*ErrorPattern),
			RecoveryStrategies: make(map[ErrorReason]*RecoveryStats),
			ModelPerformance:   make(map[string]*ModelStats),
			ToolEfficiency:     make(map[string]*ToolStats),
			LearnedParameters:  make(map[string]interface{}),
		},
		sessionStart: time.Now(),
	}

	// Load persisted state
	et.load()

	return et
}

// StartSession marks the beginning of a new session
func (et *EvolutionTracker) StartSession(model, provider string) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.sessionStart = time.Now()
	et.sessionAPICalls = 0
	et.currentModel = model
	et.currentProvider = provider
	et.state.SessionCount++
	et.pendingUpdates = true
}

// EndSession marks the end of the current session
func (et *EvolutionTracker) EndSession() {
	et.mu.Lock()
	defer et.mu.Unlock()

	sessionDuration := time.Since(et.sessionStart)
	
	// Update model stats with session data
	if et.currentModel != "" {
		key := fmt.Sprintf("%s/%s", et.currentProvider, et.currentModel)
		if stats, ok := et.state.ModelPerformance[key]; ok {
			stats.LastUsed = time.Now()
			// Could add session duration tracking here
			_ = sessionDuration
		}
	}

	et.persist()
}

// RecordAPICall records a successful API call
func (et *EvolutionTracker) RecordAPICall(model, provider string, tokens int, latency time.Duration) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.sessionAPICalls++
	et.state.TotalAPICalls++
	et.pendingUpdates = true

	// Update model stats
	key := fmt.Sprintf("%s/%s", provider, model)
	stats, ok := et.state.ModelPerformance[key]
	if !ok {
		stats = &ModelStats{
			Model:         model,
			Provider:      provider,
			ErrorPatterns: make(map[string]int),
		}
		et.state.ModelPerformance[key] = stats
	}

	stats.TotalCalls++
	stats.SuccessCalls++
	stats.TotalTokens += tokens
	stats.LastUsed = time.Now()

	// Update rolling average latency
	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = float64(latency.Milliseconds())
	} else {
		stats.AvgLatencyMs = (stats.AvgLatencyMs*float64(stats.TotalCalls-1) + float64(latency.Milliseconds())) / float64(stats.TotalCalls)
	}
}

// RecordError records an error occurrence
func (et *EvolutionTracker) RecordError(errorReason ErrorReason, errorMsg string, retryCount int, recovered bool, recoveryTime time.Duration) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.pendingUpdates = true

	// Update error patterns
	pattern := et.extractPattern(errorMsg)
	if ep, ok := et.state.ErrorPatterns[pattern]; ok {
		ep.Count++
		ep.LastSeen = time.Now()
		if recovered {
			ep.RecoveryRate = (ep.RecoveryRate*float64(ep.Count-1) + 1.0) / float64(ep.Count)
		} else {
			ep.RecoveryRate = (ep.RecoveryRate * float64(ep.Count-1)) / float64(ep.Count)
		}
	} else {
		et.state.ErrorPatterns[pattern] = &ErrorPattern{
			Pattern:      pattern,
			ErrorReason:  errorReason,
			Count:        1,
			FirstSeen:    time.Now(),
			LastSeen:     time.Now(),
			RecoveryRate: 0,
		}
		if recovered {
			et.state.ErrorPatterns[pattern].RecoveryRate = 1.0
		}
	}

	// Update recovery strategy stats
	if rs, ok := et.state.RecoveryStrategies[errorReason]; ok {
		rs.Attempts++
		if recovered {
			rs.Successes++
		} else {
			rs.Failures++
		}
		rs.AvgRetries = (rs.AvgRetries*float64(rs.Attempts-1) + float64(retryCount)) / float64(rs.Attempts)
		rs.AvgLatency = (rs.AvgLatency*float64(rs.Attempts-1) + float64(recoveryTime.Milliseconds())) / float64(rs.Attempts)
	} else {
		rs := &RecoveryStats{
			Attempts:   1,
			AvgRetries: float64(retryCount),
			AvgLatency: float64(recoveryTime.Milliseconds()),
		}
		if recovered {
			rs.Successes = 1
		} else {
			rs.Failures = 1
		}
		et.state.RecoveryStrategies[errorReason] = rs
	}

	// Update model error patterns
	if et.currentModel != "" {
		key := fmt.Sprintf("%s/%s", et.currentProvider, et.currentModel)
		if stats, ok := et.state.ModelPerformance[key]; ok {
			stats.FailedCalls++
			stats.ErrorPatterns[string(errorReason)]++
		}
	}
}

// RecordToolCall records a tool execution
func (et *EvolutionTracker) RecordToolCall(toolName string, success bool, executionTime time.Duration) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.pendingUpdates = true

	stats, ok := et.state.ToolEfficiency[toolName]
	if !ok {
		stats = &ToolStats{ToolName: toolName}
		et.state.ToolEfficiency[toolName] = stats
	}

	stats.TotalCalls++
	if success {
		stats.SuccessCalls++
	} else {
		stats.FailedCalls++
	}

	// Update rolling average execution time
	if stats.AvgExecutionTime == 0 {
		stats.AvgExecutionTime = float64(executionTime.Milliseconds())
	} else {
		stats.AvgExecutionTime = (stats.AvgExecutionTime*float64(stats.TotalCalls-1) + float64(executionTime.Milliseconds())) / float64(stats.TotalCalls)
	}
}

// GetRecoveryRecommendation gets a recommendation for handling an error type
func (et *EvolutionTracker) GetRecoveryRecommendation(errorReason ErrorReason) *RecoveryRecommendation {
	et.mu.RLock()
	defer et.mu.RUnlock()

	rec := &RecoveryRecommendation{
		ErrorReason: errorReason,
	}

	// Check recovery strategy effectiveness
	if rs, ok := et.state.RecoveryStrategies[errorReason]; ok && rs.Attempts > 0 {
		successRate := float64(rs.Successes) / float64(rs.Attempts)
		rec.EstimatedSuccessRate = successRate
		rec.RecommendedRetries = int(rs.AvgRetries + 0.5)
		rec.EstimatedRecoveryTime = time.Duration(rs.AvgLatency) * time.Millisecond

		if successRate > 0.7 {
			rec.Confidence = "high"
		} else if successRate > 0.4 {
			rec.Confidence = "medium"
		} else {
			rec.Confidence = "low"
		}
	} else {
		// No data, use defaults
		rec.EstimatedSuccessRate = 0.5
		rec.RecommendedRetries = 3
		rec.EstimatedRecoveryTime = 5 * time.Second
		rec.Confidence = "unknown"
	}

	return rec
}

// GetBestModelForTask suggests the best model based on historical performance
func (et *EvolutionTracker) GetBestModelForTask(taskType string) *ModelRecommendation {
	et.mu.RLock()
	defer et.mu.RUnlock()

	// Simple heuristic: highest success rate, then lowest latency
	var bestModel *ModelStats
	bestScore := -1.0

	for _, stats := range et.state.ModelPerformance {
		if stats.TotalCalls < 5 {
			continue // Not enough data
		}

		successRate := float64(stats.SuccessCalls) / float64(stats.TotalCalls)
		// Score: success rate weighted by log of total calls (prefer proven models)
		score := successRate * (1 + float64(stats.TotalCalls)/100.0)

		if score > bestScore {
			bestScore = score
			bestModel = stats
		}
	}

	if bestModel == nil {
		return nil
	}

	return &ModelRecommendation{
		Model:       bestModel.Model,
		Provider:    bestModel.Provider,
		SuccessRate: float64(bestModel.SuccessCalls) / float64(bestModel.TotalCalls),
		AvgLatency:  time.Duration(bestModel.AvgLatencyMs) * time.Millisecond,
		TotalCalls:  bestModel.TotalCalls,
	}
}

// LearnParameter learns and stores a parameter value
func (et *EvolutionTracker) LearnParameter(key string, value interface{}) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.state.LearnedParameters[key] = value
	et.pendingUpdates = true
}

// GetLearnedParameter retrieves a learned parameter
func (et *EvolutionTracker) GetLearnedParameter(key string) (interface{}, bool) {
	et.mu.RLock()
	defer et.mu.RUnlock()

	val, ok := et.state.LearnedParameters[key]
	return val, ok
}

// GetStats returns evolution statistics
func (et *EvolutionTracker) GetStats() map[string]interface{} {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return map[string]interface{}{
		"session_count":       et.state.SessionCount,
		"total_api_calls":     et.state.TotalAPICalls,
		"error_patterns":      len(et.state.ErrorPatterns),
		"recovery_strategies": len(et.state.RecoveryStrategies),
		"models_tracked":      len(et.state.ModelPerformance),
		"tools_tracked":       len(et.state.ToolEfficiency),
		"learned_parameters":  len(et.state.LearnedParameters),
		"current_session": map[string]interface{}{
			"duration_sec": time.Since(et.sessionStart).Seconds(),
			"api_calls":    et.sessionAPICalls,
		},
	}
}

// GetState returns a copy of the current evolution state
func (et *EvolutionTracker) GetState() *EvolutionState {
	et.mu.RLock()
	defer et.mu.RUnlock()

	// Deep copy
	data, _ := json.Marshal(et.state)
	var copy EvolutionState
	json.Unmarshal(data, &copy)
	return &copy
}

// persist saves the evolution state to disk
func (et *EvolutionTracker) persist() error {
	if !et.pendingUpdates {
		return nil
	}

	et.state.LastUpdated = time.Now()

	// Ensure directory exists
	evolutionDir := filepath.Join(et.workspace, ".evolution")
	if err := os.MkdirAll(evolutionDir, 0755); err != nil {
		return fmt.Errorf("failed to create evolution directory: %w", err)
	}

	// Save state
	statePath := filepath.Join(evolutionDir, "state.json")
	data, err := json.MarshalIndent(et.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal evolution state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write evolution state: %w", err)
	}

	et.pendingUpdates = false
	return nil
}

// load loads the evolution state from disk
func (et *EvolutionTracker) load() error {
	statePath := filepath.Join(et.workspace, ".evolution", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved state, use defaults
		}
		return fmt.Errorf("failed to read evolution state: %w", err)
	}

	if err := json.Unmarshal(data, &et.state); err != nil {
		return fmt.Errorf("failed to parse evolution state: %w", err)
	}

	return nil
}

// extractPattern extracts a normalized error pattern from an error message
func (et *EvolutionTracker) extractPattern(errorMsg string) string {
	// Normalize: lowercase, trim, extract key terms
	pattern := errorMsg
	
	// Remove variable parts (numbers, IDs, timestamps)
	// This is a simplified version - could be more sophisticated
	if len(pattern) > 100 {
		pattern = pattern[:100]
	}
	
	// Extract the core error type
	for _, reason := range []ErrorReason{
		ErrorReasonAuth,
		ErrorReasonBilling,
		ErrorReasonRateLimit,
		ErrorReasonContextOverflow,
		ErrorReasonTimeout,
		ErrorReasonModelNotFound,
	} {
		if containsIgnoreCase(pattern, string(reason)) {
			return string(reason)
		}
	}
	
	return "unknown"
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		findSubstr(s, substr)))
}

func findSubstr(s, substr string) bool {
	lowerS := ""
	lowerSubstr := ""
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			lowerS += string(s[i] + 32)
		} else {
			lowerS += string(s[i])
		}
	}
	for i := 0; i < len(substr); i++ {
		if substr[i] >= 'A' && substr[i] <= 'Z' {
			lowerSubstr += string(substr[i] + 32)
		} else {
			lowerSubstr += string(substr[i])
		}
	}
	for i := 0; i <= len(lowerS)-len(lowerSubstr); i++ {
		if lowerS[i:i+len(lowerSubstr)] == lowerSubstr {
			return true
		}
	}
	return false
}

// RecoveryRecommendation provides recovery recommendations
type RecoveryRecommendation struct {
	ErrorReason           ErrorReason
	EstimatedSuccessRate  float64
	RecommendedRetries    int
	EstimatedRecoveryTime time.Duration
	Confidence            string // "high", "medium", "low", "unknown"
}

// ModelRecommendation suggests a model for a task
type ModelRecommendation struct {
	Model       string
	Provider    string
	SuccessRate float64
	AvgLatency  time.Duration
	TotalCalls  int
}

// AdaptiveThreshold provides adaptive threshold values based on learning
type AdaptiveThreshold struct {
	Name          string  `json:"name"`
	CurrentValue  float64 `json:"current_value"`
	DefaultValue  float64 `json:"default_value"`
	MinValue      float64 `json:"min_value"`
	MaxValue      float64 `json:"max_value"`
	LearningRate  float64 `json:"learning_rate"`
	LastAdjusted  time.Time `json:"last_adjusted"`
}

// AdaptiveThresholdManager manages adaptive thresholds
type AdaptiveThresholdManager struct {
	mu         sync.RWMutex
	thresholds map[string]*AdaptiveThreshold
}

// NewAdaptiveThresholdManager creates a new adaptive threshold manager
func NewAdaptiveThresholdManager() *AdaptiveThresholdManager {
	return &AdaptiveThresholdManager{
		thresholds: make(map[string]*AdaptiveThreshold),
	}
}

// RegisterThreshold registers a new adaptive threshold
func (atm *AdaptiveThresholdManager) RegisterThreshold(name string, defaultValue, minValue, maxValue, learningRate float64) {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	atm.thresholds[name] = &AdaptiveThreshold{
		Name:         name,
		CurrentValue: defaultValue,
		DefaultValue: defaultValue,
		MinValue:     minValue,
		MaxValue:     maxValue,
		LearningRate: learningRate,
		LastAdjusted: time.Now(),
	}
}

// GetValue gets the current value of a threshold
func (atm *AdaptiveThresholdManager) GetValue(name string) (float64, bool) {
	atm.mu.RLock()
	defer atm.mu.RUnlock()

	if t, ok := atm.thresholds[name]; ok {
		return t.CurrentValue, true
	}
	return 0, false
}

// UpdateValue updates a threshold based on feedback (positive = increase, negative = decrease)
func (atm *AdaptiveThresholdManager) UpdateValue(name string, feedback float64) bool {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	t, ok := atm.thresholds[name]
	if !ok {
		return false
	}

	// Apply learning rate
	adjustment := feedback * t.LearningRate
	newValue := t.CurrentValue + adjustment

	// Clamp to bounds
	if newValue < t.MinValue {
		newValue = t.MinValue
	}
	if newValue > t.MaxValue {
		newValue = t.MaxValue
	}

	t.CurrentValue = newValue
	t.LastAdjusted = time.Now()
	return true
}
