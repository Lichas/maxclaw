package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/session"
)

// Checkpoint represents a snapshot of the agent state at a point in time
type Checkpoint struct {
	ID             string                 `json:"id"`
	Timestamp      time.Time              `json:"timestamp"`
	SessionKey     string                 `json:"session_key"`
	Messages       []session.Message      `json:"messages"`
	SystemPrompt   string                 `json:"system_prompt,omitempty"`
	IterationCount int                    `json:"iteration_count"`
	APITokenUsage  map[string]int         `json:"api_token_usage,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Compressed     bool                   `json:"compressed,omitempty"`
}

// CheckpointManager manages filesystem checkpoints for session state
type CheckpointManager struct {
	mu sync.RWMutex

	Enabled      bool
	MaxSnapshots int
	Workspace    string

	currentTurn     int
	snapshotsThisTurn int
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(enabled bool, maxSnapshots int, workspace string) *CheckpointManager {
	if maxSnapshots <= 0 {
		maxSnapshots = 50
	}
	return &CheckpointManager{
		Enabled:       enabled,
		MaxSnapshots:  maxSnapshots,
		Workspace:     workspace,
		currentTurn:   0,
		snapshotsThisTurn: 0,
	}
}

// NewTurn marks the beginning of a new turn, resetting per-turn dedup
func (cm *CheckpointManager) NewTurn() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.currentTurn++
	cm.snapshotsThisTurn = 0
}

// Save creates a new checkpoint
func (cm *CheckpointManager) Save(sessionKey string, messages []session.Message, systemPrompt string, iterationCount int, metadata map[string]interface{}) (*Checkpoint, error) {
	if !cm.Enabled {
		return nil, nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Per-turn dedup - only one snapshot per turn
	if cm.snapshotsThisTurn > 0 {
		return nil, nil
	}
	cm.snapshotsThisTurn++

	checkpoint := &Checkpoint{
		ID:             generateCheckpointID(),
		Timestamp:      time.Now(),
		SessionKey:     sessionKey,
		Messages:       make([]session.Message, len(messages)),
		SystemPrompt:   systemPrompt,
		IterationCount: iterationCount,
		Metadata:       metadata,
	}

	// Deep copy messages
	copy(checkpoint.Messages, messages)

	// Ensure checkpoint directory exists
	checkpointDir := cm.checkpointDir(sessionKey)
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	// Save checkpoint
	checkpointPath := filepath.Join(checkpointDir, fmt.Sprintf("%s.json", checkpoint.ID))
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write checkpoint: %w", err)
	}

	// Clean up old checkpoints
	if err := cm.cleanupOldCheckpoints(sessionKey); err != nil {
		// Non-fatal: log but don't fail
		fmt.Printf("[CheckpointManager] Warning: failed to cleanup old checkpoints: %v\n", err)
	}

	return checkpoint, nil
}

// Load loads a checkpoint by ID
func (cm *CheckpointManager) Load(sessionKey, checkpointID string) (*Checkpoint, error) {
	if !cm.Enabled {
		return nil, fmt.Errorf("checkpoint manager is disabled")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	checkpointPath := filepath.Join(cm.checkpointDir(sessionKey), fmt.Sprintf("%s.json", checkpointID))
	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
		}
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// LoadLatest loads the most recent checkpoint for a session
func (cm *CheckpointManager) LoadLatest(sessionKey string) (*Checkpoint, error) {
	if !cm.Enabled {
		return nil, fmt.Errorf("checkpoint manager is disabled")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	checkpointDir := cm.checkpointDir(sessionKey)
	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no checkpoints found for session: %s", sessionKey)
		}
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no checkpoints found for session: %s", sessionKey)
	}

	// Sort by modification time (newest first)
	sort.Slice(entries, func(i, j int) bool {
		infoI, _ := entries[i].Info()
		infoJ, _ := entries[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	latestPath := filepath.Join(checkpointDir, entries[0].Name())
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read latest checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// List returns all checkpoint IDs for a session
func (cm *CheckpointManager) List(sessionKey string) ([]string, error) {
	if !cm.Enabled {
		return nil, fmt.Errorf("checkpoint manager is disabled")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	checkpointDir := cm.checkpointDir(sessionKey)
	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// Delete removes a specific checkpoint
func (cm *CheckpointManager) Delete(sessionKey, checkpointID string) error {
	if !cm.Enabled {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	checkpointPath := filepath.Join(cm.checkpointDir(sessionKey), fmt.Sprintf("%s.json", checkpointID))
	if err := os.Remove(checkpointPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	return nil
}

// DeleteAll removes all checkpoints for a session
func (cm *CheckpointManager) DeleteAll(sessionKey string) error {
	if !cm.Enabled {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	checkpointDir := cm.checkpointDir(sessionKey)
	if err := os.RemoveAll(checkpointDir); err != nil {
		return fmt.Errorf("failed to delete checkpoint directory: %w", err)
	}

	return nil
}

// cleanupOldCheckpoints removes old checkpoints to stay within MaxSnapshots
func (cm *CheckpointManager) cleanupOldCheckpoints(sessionKey string) error {
	checkpointDir := cm.checkpointDir(sessionKey)
	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		return err
	}

	if len(entries) <= cm.MaxSnapshots {
		return nil
	}

	// Sort by modification time (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		infoI, _ := entries[i].Info()
		infoJ, _ := entries[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// Delete oldest entries
	toDelete := len(entries) - cm.MaxSnapshots
	for i := 0; i < toDelete; i++ {
		path := filepath.Join(checkpointDir, entries[i].Name())
		if err := os.Remove(path); err != nil {
			// Log but continue
			fmt.Printf("[CheckpointManager] Warning: failed to delete old checkpoint %s: %v\n", entries[i].Name(), err)
		}
	}

	return nil
}

// checkpointDir returns the directory path for a session's checkpoints
func (cm *CheckpointManager) checkpointDir(sessionKey string) string {
	safeKey := sanitizeSessionKey(sessionKey)
	return filepath.Join(cm.Workspace, ".checkpoints", safeKey)
}

// GetStats returns checkpoint statistics for a session
func (cm *CheckpointManager) GetStats(sessionKey string) (map[string]interface{}, error) {
	if !cm.Enabled {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	ids, err := cm.List(sessionKey)
	if err != nil {
		return nil, err
	}

	var totalSize int64
	var oldest, newest time.Time
	checkpointDir := cm.checkpointDir(sessionKey)

	for _, id := range ids {
		path := filepath.Join(checkpointDir, fmt.Sprintf("%s.json", id))
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		totalSize += info.Size()
		modTime := info.ModTime()
		if oldest.IsZero() || modTime.Before(oldest) {
			oldest = modTime
		}
		if newest.IsZero() || modTime.After(newest) {
			newest = modTime
		}
	}

	return map[string]interface{}{
		"enabled":       true,
		"count":         len(ids),
		"max_snapshots": cm.MaxSnapshots,
		"total_size":    totalSize,
		"oldest":        oldest,
		"newest":        newest,
	}, nil
}

// GenerateCheckpointID generates a unique checkpoint ID
func generateCheckpointID() string {
	return fmt.Sprintf("chk_%d_%s", time.Now().Unix(), generateShortID(6))
}

func generateShortID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// SessionPersistence handles session-level persistence
type SessionPersistence struct {
	workspace string
	mu        sync.RWMutex
}

// NewSessionPersistence creates a new session persistence manager
func NewSessionPersistence(workspace string) *SessionPersistence {
	return &SessionPersistence{
		workspace: workspace,
	}
}

// PersistSession saves the complete session state
func (sp *SessionPersistence) PersistSession(sess *session.Session, conversationHistory []session.Message) error {
	if sess == nil {
		return fmt.Errorf("session is nil")
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Ensure sessions directory exists
	sessionsDir := filepath.Join(sp.workspace, ".sessions", sanitizeSessionKey(sess.Key))
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Save session metadata
	sessionPath := filepath.Join(sessionsDir, "session.json")
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}

	// Save conversation history if provided
	if len(conversationHistory) > 0 {
		historyPath := filepath.Join(sessionsDir, "history.json")
		data, err = json.MarshalIndent(conversationHistory, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal history: %w", err)
		}
		if err := os.WriteFile(historyPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write history: %w", err)
		}
	}

	return nil
}

// LoadSession loads a persisted session
func (sp *SessionPersistence) LoadSession(sessionKey string) (*session.Session, []session.Message, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	sessionsDir := filepath.Join(sp.workspace, ".sessions", sanitizeSessionKey(sessionKey))

	// Load session
	sessionPath := filepath.Join(sessionsDir, "session.json")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("session not found: %s", sessionKey)
		}
		return nil, nil, fmt.Errorf("failed to read session: %w", err)
	}

	var sess session.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, nil, fmt.Errorf("failed to parse session: %w", err)
	}

	// Load history
	historyPath := filepath.Join(sessionsDir, "history.json")
	data, err = os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &sess, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to read history: %w", err)
	}

	var history []session.Message
	if err := json.Unmarshal(data, &history); err != nil {
		return &sess, nil, fmt.Errorf("failed to parse history: %w", err)
	}

	return &sess, history, nil
}

// ToolResultStorage handles persistence of tool execution results
type ToolResultStorage struct {
	workspace string
	mu        sync.RWMutex
}

// NewToolResultStorage creates a new tool result storage
func NewToolResultStorage(workspace string) *ToolResultStorage {
	return &ToolResultStorage{
		workspace: workspace,
	}
}

// Store saves a tool execution result
func (trs *ToolResultStorage) Store(sessionKey, toolName, toolCallID string, result interface{}) error {
	trs.mu.Lock()
	defer trs.mu.Unlock()

	// Ensure directory exists
	resultsDir := filepath.Join(trs.workspace, ".tool_results", sanitizeSessionKey(sessionKey))
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	// Save result
	resultPath := filepath.Join(resultsDir, fmt.Sprintf("%s_%s.json", toolName, toolCallID))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(resultPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

// Load retrieves a tool execution result
func (trs *ToolResultStorage) Load(sessionKey, toolName, toolCallID string, result interface{}) error {
	trs.mu.RLock()
	defer trs.mu.RUnlock()

	resultPath := filepath.Join(trs.workspace, ".tool_results", sanitizeSessionKey(sessionKey), fmt.Sprintf("%s_%s.json", toolName, toolCallID))
	data, err := os.ReadFile(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("result not found")
		}
		return fmt.Errorf("failed to read result: %w", err)
	}

	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	return nil
}

// Cleanup removes all tool results for a session
func (trs *ToolResultStorage) Cleanup(sessionKey string) error {
	trs.mu.Lock()
	defer trs.mu.Unlock()

	resultsDir := filepath.Join(trs.workspace, ".tool_results", sanitizeSessionKey(sessionKey))
	if err := os.RemoveAll(resultsDir); err != nil {
		return fmt.Errorf("failed to cleanup results: %w", err)
	}

	return nil
}
