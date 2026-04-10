package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/providers"
)

const (
	// Minimum tokens for the summary output
	minSummaryTokens = 2000
	// Proportion of compressed content to allocate for summary
	summaryRatio = 0.20
	// Absolute ceiling for summary tokens
	summaryTokensCeiling = 12000
	// Characters per token rough estimate
	charsPerToken = 4
	// Summary failure cooldown
	summaryFailureCooldownSeconds = 600
	// Placeholder for pruned tool results
	prunedToolPlaceholder = "[Old tool output cleared to save context space]"
	// Summary prefix
	summaryPrefix = "[CONTEXT COMPACTION] Earlier turns in this conversation were compacted to save context space. The summary below describes work that was already completed, and the current session state may still reflect that work (for example, files may already be changed). Use the summary and the current state to continue from where things left off, and avoid repeating work:"
)

// Message represents a conversation message for compression
type CompressorMessage struct {
	Role         string                 `json:"role"`
	Content      string                 `json:"content"`
	ToolCalls    []providers.ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
	Reasoning    string                 `json:"reasoning,omitempty"`
}

// ContextCompressor compresses conversation context when approaching model's context limit
type ContextCompressor struct {
	Model                string
	BaseURL              string
	APIKey               string
	Provider             string
	ThresholdPercent     float64
	ProtectFirstN        int
	ProtectLastN         int
	SummaryTargetRatio   float64
	QuietMode            bool
	SummaryModelOverride string
	ConfigContextLength  int

	ContextLength        int
	ThresholdTokens      int
	CompressionCount     int
	TailTokenBudget      int
	MaxSummaryTokens     int

	lastPromptTokens     int
	lastCompletionTokens int
	lastTotalTokens      int

	contextProbed        bool
	contextProbePersistable bool
	previousSummary      string
	summaryFailureCooldownUntil time.Time

	mu sync.RWMutex
}

// CompressionResult holds the result of a compression operation
type CompressionResult struct {
	Messages        []CompressorMessage
	Summary         string
	PrunedCount     int
	CompressedCount int
}

// NewContextCompressor creates a new context compressor
func NewContextCompressor(
	model string,
	thresholdPercent float64,
	protectFirstN int,
	protectLastN int,
	summaryTargetRatio float64,
	quietMode bool,
	summaryModelOverride string,
	baseURL string,
	apiKey string,
	configContextLength int,
	provider string,
) *ContextCompressor {
	if thresholdPercent <= 0 {
		thresholdPercent = 0.50
	}
	if protectFirstN <= 0 {
		protectFirstN = 3
	}
	if protectLastN <= 0 {
		protectLastN = 20
	}
	summaryTargetRatio = maxFloat64(0.10, minFloat64(summaryTargetRatio, 0.80))

	// Derive context length from model or config
	contextLength := getModelContextLength(model, baseURL, configContextLength)
	thresholdTokens := int(float64(contextLength) * thresholdPercent)

	// Derive token budgets
	targetTokens := int(float64(thresholdTokens) * summaryTargetRatio)
	maxSummaryTokens := min(int(float64(contextLength)*0.05), summaryTokensCeiling)

	return &ContextCompressor{
		Model:                model,
		BaseURL:              baseURL,
		APIKey:               apiKey,
		Provider:             provider,
		ThresholdPercent:     thresholdPercent,
		ProtectFirstN:        protectFirstN,
		ProtectLastN:         protectLastN,
		SummaryTargetRatio:   summaryTargetRatio,
		QuietMode:            quietMode,
		SummaryModelOverride: summaryModelOverride,
		ConfigContextLength:  configContextLength,
		ContextLength:        contextLength,
		ThresholdTokens:      thresholdTokens,
		TailTokenBudget:      targetTokens,
		MaxSummaryTokens:     maxSummaryTokens,
	}
}

// UpdateFromResponse updates tracked token usage from API response
func (cc *ContextCompressor) UpdateFromResponse(usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.lastPromptTokens = usage.PromptTokens
	cc.lastCompletionTokens = usage.CompletionTokens
	cc.lastTotalTokens = usage.TotalTokens
}

// ShouldCompress checks if context exceeds the compression threshold
func (cc *ContextCompressor) ShouldCompress(promptTokens int) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	tokens := promptTokens
	if tokens == 0 {
		tokens = cc.lastPromptTokens
	}
	return tokens >= cc.ThresholdTokens
}

// ShouldCompressPreflight quick pre-flight check using rough estimate
func (cc *ContextCompressor) ShouldCompressPreflight(messages []CompressorMessage) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	roughEstimate := estimateMessagesTokensRough(messages)
	return roughEstimate >= cc.ThresholdTokens
}

// GetStatus returns current compression status for display/logging
func (cc *ContextCompressor) GetStatus() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	usagePercent := 0.0
	if cc.ContextLength > 0 {
		usagePercent = minFloat64(100.0, float64(cc.lastPromptTokens)/float64(cc.ContextLength)*100.0)
	}
	return map[string]interface{}{
		"last_prompt_tokens": cc.lastPromptTokens,
		"threshold_tokens":   cc.ThresholdTokens,
		"context_length":     cc.ContextLength,
		"usage_percent":      usagePercent,
		"compression_count":  cc.CompressionCount,
	}
}

// Compress compresses the message list by summarizing middle turns
func (cc *ContextCompressor) Compress(ctx context.Context, messages []CompressorMessage, systemPrompt string) (*CompressionResult, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(messages) <= cc.ProtectFirstN+cc.ProtectLastN+1 {
		return &CompressionResult{Messages: messages}, nil // Not enough messages to compress
	}

	// Step 1: Prune old tool results (cheap pre-pass)
	prunedMessages, prunedCount := cc.pruneOldToolResults(messages, cc.ProtectLastN, cc.TailTokenBudget)

	// Step 2: Identify segments
	headEnd := cc.ProtectFirstN
	if headEnd > len(prunedMessages) {
		headEnd = len(prunedMessages)
	}
	
	tailStart := len(prunedMessages) - cc.ProtectLastN
	if tailStart < headEnd {
		tailStart = headEnd
	}

	head := make([]CompressorMessage, headEnd)
	copy(head, prunedMessages[:headEnd])
	
	middle := make([]CompressorMessage, tailStart-headEnd)
	copy(middle, prunedMessages[headEnd:tailStart])
	
	tail := make([]CompressorMessage, len(prunedMessages)-tailStart)
	copy(tail, prunedMessages[tailStart:])

	if len(middle) == 0 {
		return &CompressionResult{
			Messages:    prunedMessages,
			PrunedCount: prunedCount,
		}, nil
	}

	// Step 3: Generate summary for middle section
	summary, err := cc.generateSummary(ctx, middle, systemPrompt)
	if err != nil {
		// If summary generation fails, just drop middle messages
		if !cc.QuietMode {
			fmt.Printf("[ContextCompressor] Summary generation failed: %v, dropping middle messages\n", err)
		}
		result := append(head, tail...)
		cc.CompressionCount++
		return &CompressionResult{
			Messages:        result,
			PrunedCount:     prunedCount,
			CompressedCount: len(middle),
		}, nil
	}

	// Step 4: Build result with summary
	summaryMsg := CompressorMessage{
		Role:    "system",
		Content: summary,
	}

	result := append(head, summaryMsg)
	result = append(result, tail...)

	// Step 5: Sanitize tool-call/tool-result pairs
	result = cc.sanitizeToolPairs(result)

	cc.CompressionCount++
	cc.previousSummary = summary

	return &CompressionResult{
		Messages:        result,
		Summary:         summary,
		PrunedCount:     prunedCount,
		CompressedCount: len(middle),
	}, nil
}

// pruneOldToolResults replaces old tool result contents with a short placeholder
func (cc *ContextCompressor) pruneOldToolResults(
	messages []CompressorMessage,
	protectTailCount int,
	protectTailTokens int,
) ([]CompressorMessage, int) {
	if len(messages) == 0 {
		return messages, 0
	}

	result := make([]CompressorMessage, len(messages))
	copy(result, messages)
	pruned := 0

	// Determine prune boundary
	pruneBoundary := len(result) - protectTailCount
	if protectTailTokens > 0 {
		accumulated := 0
		boundary := len(result)
		minProtect := min(protectTailCount, len(result)-1)
		for i := len(result) - 1; i >= 0; i-- {
			msg := result[i]
			contentLen := len(msg.Content)
			msgTokens := contentLen/charsPerToken + 10
			for _, tc := range msg.ToolCalls {
				msgTokens += len(tc.Function.Arguments) / charsPerToken
			}
			if accumulated+msgTokens > protectTailTokens && (len(result)-i) >= minProtect {
				boundary = i
				break
			}
			accumulated += msgTokens
			boundary = i
		}
		pruneBoundary = max(boundary, len(result)-minProtect)
	}

	for i := 0; i < pruneBoundary; i++ {
		if result[i].Role != "tool" {
			continue
		}
		content := result[i].Content
		if content == "" || content == prunedToolPlaceholder {
			continue
		}
		if len(content) > 200 {
			result[i].Content = prunedToolPlaceholder
			pruned++
		}
	}

	return result, pruned
}

// generateSummary generates a structured summary of conversation turns
func (cc *ContextCompressor) generateSummary(ctx context.Context, turns []CompressorMessage, systemPrompt string) (string, error) {
	now := time.Now()
	if now.Before(cc.summaryFailureCooldownUntil) {
		return "", fmt.Errorf("summary generation in cooldown")
	}

	summaryBudget := cc.computeSummaryBudget(turns)
	contentToSummarize := cc.serializeForSummary(turns)

	var prompt string
	if cc.previousSummary != "" {
		// Iterative update
		prompt = fmt.Sprintf(`You are updating a context compaction summary. A previous compaction produced the summary below. New conversation turns have occurred since then and need to be incorporated.

PREVIOUS SUMMARY:
%s

NEW TURNS TO INCORPORATE:
%s

Update the summary using this exact structure. PRESERVE all existing information that is still relevant. ADD new progress. Move items from "In Progress" to "Done" when completed. Remove information only if it is clearly obsolete.

## Goal
[What the user is trying to accomplish — preserve from previous summary, update if goal evolved]

## Constraints & Preferences
[User preferences, coding style, constraints, important decisions — accumulate across compactions]

## Progress
### Done
[Completed work — include specific file paths, commands run, results obtained]
### In Progress
[Work currently underway]
### Blocked
[Any blockers or issues encountered]

## Key Decisions
[Important technical decisions and why they were made]

## Relevant Files
[Files read, modified, or created — with brief note on each. Accumulate across compactions.]

## Next Steps
[What needs to happen next to continue the work]

## Critical Context
[Any specific values, error messages, configuration details, or data that would be lost without explicit preservation]

## Tools & Patterns
[Which tools were used, how they were used effectively, and any tool-specific discoveries. Accumulate across compactions.]

Target ~%d tokens. Be specific — include file paths, command outputs, error messages, and concrete values rather than vague descriptions.

Write only the summary body. Do not include any preamble or prefix.`,
			cc.previousSummary, contentToSummarize, summaryBudget)
	} else {
		// First compaction
		prompt = fmt.Sprintf(`Create a structured handoff summary for a later assistant that will continue this conversation after earlier turns are compacted.

TURNS TO SUMMARIZE:
%s

Use this exact structure:

## Goal
[What the user is trying to accomplish]

## Constraints & Preferences
[User preferences, coding style, constraints, important decisions]

## Progress
### Done
[Completed work — include specific file paths, commands run, results obtained]
### In Progress
[Work currently underway]
### Blocked
[Any blockers or issues encountered]

## Key Decisions
[Important technical decisions and why they were made]

## Relevant Files
[Files read, modified, or created — with brief note on each]

## Next Steps
[What needs to happen next to continue the work]

## Critical Context
[Any specific values, error messages, configuration details, or data that would be lost without explicit preservation]

## Tools & Patterns
[Which tools were used, how they were used effectively, and any tool-specific discoveries (e.g., preferred flags, working invocations, successful command patterns)]

Target ~%d tokens. Be specific — include file paths, command outputs, error messages, and concrete values rather than vague descriptions. The goal is to prevent the next assistant from repeating work or losing important details.

Write only the summary body. Do not include any preamble or prefix.`,
			contentToSummarize, summaryBudget)
	}

	// Call LLM for summary (simplified - in production would use actual provider)
	summary, err := cc.callSummaryLLM(ctx, prompt, summaryBudget)
	if err != nil {
		cc.summaryFailureCooldownUntil = time.Now().Add(summaryFailureCooldownSeconds * time.Second)
		return "", err
	}

	return cc.withSummaryPrefix(summary), nil
}

// computeSummaryBudget scales summary token budget with content being compressed
func (cc *ContextCompressor) computeSummaryBudget(turns []CompressorMessage) int {
	contentTokens := estimateMessagesTokensRough(turns)
	budget := int(float64(contentTokens) * summaryRatio)
	return max(minSummaryTokens, min(budget, cc.MaxSummaryTokens))
}

// serializeForSummary serializes conversation turns into labeled text
func (cc *ContextCompressor) serializeForSummary(turns []CompressorMessage) string {
	const (
		contentMax    = 6000
		contentHead   = 4000
		contentTail   = 1500
		toolArgsMax   = 1500
		toolArgsHead  = 1200
	)

	parts := make([]string, 0, len(turns))
	for _, msg := range turns {
		role := msg.Role
		content := msg.Content

		switch role {
		case "tool":
			toolID := msg.ToolCallID
			if len(content) > contentMax {
				content = content[:contentHead] + "\n...[truncated]...\n" + content[len(content)-contentTail:]
			}
			parts = append(parts, fmt.Sprintf("[TOOL RESULT %s]: %s", toolID, content))

		case "assistant":
			if len(content) > contentMax {
				content = content[:contentHead] + "\n...[truncated]...\n" + content[len(content)-contentTail:]
			}
			if len(msg.ToolCalls) > 0 {
				tcParts := make([]string, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					name := tc.Function.Name
					args := tc.Function.Arguments
					if len(args) > toolArgsMax {
						args = args[:toolArgsHead] + "..."
					}
					tcParts = append(tcParts, fmt.Sprintf("  %s(%s)", name, args))
				}
				content += "\n[Tool calls:\n" + strings.Join(tcParts, "\n") + "\n]"
			}
			parts = append(parts, fmt.Sprintf("[ASSISTANT]: %s", content))

		default:
			if len(content) > contentMax {
				content = content[:contentHead] + "\n...[truncated]...\n" + content[len(content)-contentTail:]
			}
			parts = append(parts, fmt.Sprintf("[%s]: %s", strings.ToUpper(role), content))
		}
	}

	return strings.Join(parts, "\n\n")
}

// sanitizeToolPairs fixes orphaned tool_call/tool_result pairs
func (cc *ContextCompressor) sanitizeToolPairs(messages []CompressorMessage) []CompressorMessage {
	// Collect surviving call IDs from assistant messages
	survivingCallIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					survivingCallIDs[tc.ID] = true
				}
			}
		}
	}

	// Collect result call IDs from tool messages
	resultCallIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			resultCallIDs[msg.ToolCallID] = true
		}
	}

	// Remove orphaned results
	orphanedResults := make(map[string]bool)
	for id := range resultCallIDs {
		if !survivingCallIDs[id] {
			orphanedResults[id] = true
		}
	}

	filtered := make([]CompressorMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "tool" && orphanedResults[msg.ToolCallID] {
			continue
		}
		filtered = append(filtered, msg)
	}

	// Add stub results for orphaned calls
	missingResults := make(map[string]bool)
	for id := range survivingCallIDs {
		if !resultCallIDs[id] {
			missingResults[id] = true
		}
	}

	if len(missingResults) > 0 {
		patched := make([]CompressorMessage, 0, len(filtered)+len(missingResults))
		for _, msg := range filtered {
			patched = append(patched, msg)
			if msg.Role == "assistant" {
				for _, tc := range msg.ToolCalls {
					if missingResults[tc.ID] {
						patched = append(patched, CompressorMessage{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    "[Result not available — context was compacted]",
						})
					}
				}
			}
		}
		filtered = patched
	}

	return filtered
}

// withSummaryPrefix normalizes summary text
func (cc *ContextCompressor) withSummaryPrefix(summary string) string {
	text := strings.TrimSpace(summary)
	// Remove legacy prefixes if present
	legacyPrefix := "[CONTEXT SUMMARY]:"
	if strings.HasPrefix(text, legacyPrefix) {
		text = strings.TrimSpace(text[len(legacyPrefix):])
	}
	if strings.HasPrefix(text, summaryPrefix) {
		text = strings.TrimSpace(text[len(summaryPrefix):])
	}
	return summaryPrefix + "\n" + text
}

// callSummaryLLM calls an auxiliary LLM for summarization
// In production, this would use the auxiliary client with a cheap/fast model
func (cc *ContextCompressor) callSummaryLLM(ctx context.Context, prompt string, maxTokens int) (string, error) {
	// Simplified implementation - would integrate with providers package
	// For now, return a placeholder indicating compression occurred
	return fmt.Sprintf("[Context compressed: %d tokens budget]\n\nPrevious conversation turns summarized. Key context preserved.", maxTokens), nil
}

// Helper functions

func getModelContextLength(model, baseURL string, configContextLength int) int {
	if configContextLength > 0 {
		return configContextLength
	}
	// Default context lengths for known models
	modelLower := strings.ToLower(model)
	switch {
	case strings.Contains(modelLower, "claude-3-opus"):
		return 200000
	case strings.Contains(modelLower, "claude-3-sonnet"):
		return 200000
	case strings.Contains(modelLower, "claude-3-haiku"):
		return 200000
	case strings.Contains(modelLower, "gpt-4-turbo"):
		return 128000
	case strings.Contains(modelLower, "gpt-4"):
		return 8192
	case strings.Contains(modelLower, "gpt-3.5-turbo"):
		return 16385
	default:
		return 128000 // Conservative default
	}
}

func estimateMessagesTokensRough(messages []CompressorMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			totalChars += len(tc.Function.Name)
			totalChars += len(tc.Function.Arguments)
		}
	}
	return totalChars / charsPerToken
}

// GetNextProbeTier returns the next lower context tier for probing
func GetNextProbeTier(current int) int {
	tiers := []int{2000000, 1000000, 500000, 256000, 200000, 128000, 100000, 64000, 32000, 16000, 8000}
	for _, tier := range tiers {
		if tier < current {
			return tier
		}
	}
	return current / 2
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
