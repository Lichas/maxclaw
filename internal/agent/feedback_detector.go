package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/providers"
)

// FeedbackType represents the type of user feedback
type FeedbackType int

const (
	FeedbackUnknown FeedbackType = iota
	FeedbackPositive      // User is satisfied: "good", "thanks", "perfect"
	FeedbackNegative      // User is dissatisfied: "wrong", "bad", "no"
	FeedbackCorrection    // User provides specific correction: "should use X instead of Y"
	FeedbackClarification // User clarifies intent: "I mean...", "What I want is..."
	FeedbackNeutral       // No clear sentiment: questions, etc.
	FeedbackQuestion      // User asks question (could be confusion/skepticism)
)

func (f FeedbackType) String() string {
	switch f {
	case FeedbackPositive:
		return "positive"
	case FeedbackNegative:
		return "negative"
	case FeedbackCorrection:
		return "correction"
	case FeedbackClarification:
		return "clarification"
	case FeedbackQuestion:
		return "question"
	case FeedbackNeutral:
		return "neutral"
	default:
		return "unknown"
	}
}

// FeedbackResult contains the detection result
type FeedbackResult struct {
	Type       FeedbackType
	Confidence float64
	Reason     string
	IssueType  string // For negative feedback: understanding/implementation/style/omission
	Action     string // Suggested action
}

// FeedbackRuleEngine uses regex patterns for fast feedback detection
type FeedbackRuleEngine struct {
	// Explicit negative patterns (English + Chinese)
	negativePatterns []*regexp.Regexp

	// Explicit positive patterns (English + Chinese)
	positivePatterns []*regexp.Regexp

	// Correction patterns (English + Chinese)
	correctionPatterns []*regexp.Regexp

	// Clarification patterns (English + Chinese)
	clarificationPatterns []*regexp.Regexp

	// Question patterns (to distinguish from dissatisfaction)
	questionPatterns []*regexp.Regexp

	// Negation words that flip sentiment when combined with positive words
	negationWords []string
}

// NewFeedbackRuleEngine creates a new rule engine with multilingual support
func NewFeedbackRuleEngine() *FeedbackRuleEngine {
	return &FeedbackRuleEngine{
		negativePatterns: []*regexp.Regexp{
			// Chinese direct negation
			regexp.MustCompile(`^(不对|错了|有问题|不行|不好|重做|回滚|撤销|删了|删掉|改回|回退)`),
			regexp.MustCompile(`(?i)(还是?不对|依然?有问题|没解决|更糟|变坏了)`),
			regexp.MustCompile(`(?i)(这样.{0,3}对吗\?|确定.{0,3}吗\?|真的吗\?)`),
			regexp.MustCompile(`(?i)(不是我要的|不符合|不满足|缺|漏了|忘了)`),

			// English direct negation
			regexp.MustCompile(`(?i)^(wrong|incorrect|bad|terrible|awful|no good|not good|doesn't work|not working)`),
			regexp.MustCompile(`(?i)^(undo|revert|rollback|start over|do it again|redo)`),
			regexp.MustCompile(`(?i)(still wrong|still broken|still not|still has issue|still doesn't work)`),
			regexp.MustCompile(`(?i)(is this correct\?|are you sure\?|is that right\?)`),
			regexp.MustCompile(`(?i)(not what I want|not what I asked|missing|forgot to)`),
		},

		positivePatterns: []*regexp.Regexp{
			// Chinese positive
			regexp.MustCompile(`^(好的?$|完美|谢谢|可以|不错|没问题|👍|👌|OK|ok$)`),
			regexp.MustCompile(`(?i)(就按这个|保持这样|可以了|行了|搞定了|完美)`),
			regexp.MustCompile(`(?i)(很好|非常棒|满意|感谢|多谢|谢谢)`),

			// English positive
			regexp.MustCompile(`(?i)^(good|great|perfect|awesome|excellent|thanks|thank you|nice|cool|ok$|okay$|👍|👌)`),
			regexp.MustCompile(`(?i)(looks? good|sounds? good|works? (fine|well|perfectly)|that's it|exactly|precisely)`),
			regexp.MustCompile(`(?i)(just what I wanted|perfect for me|satisfied|appreciate it)`),
		},

		correctionPatterns: []*regexp.Regexp{
			// Chinese correction: "应该用 X 而不是 Y", "改成 X", "不是 X 是 Y"
			regexp.MustCompile(`(?i)(应该|建议|最好|需要).{0,10}(用|使用|采用|选).{0,15}(而不是|不是|而非)`),
			regexp.MustCompile(`(?i)(改成|改为|换成|调整为).{0,15}`),
			regexp.MustCompile(`(?i)(不是.+应该是|不要.+要|别用.+用)`),
			regexp.MustCompile(`(?i)(漏了|忘了|少了|缺|还需要).{0,10}`),
			regexp.MustCompile(`(?i)(这样改|修改一下|调整下|优化下)`),

			// English correction
			regexp.MustCompile(`(?i)(should|need to|ought to|supposed to|better to).{0,20}(instead of|rather than|not).{0,20}`),
			regexp.MustCompile(`(?i)(change|switch|replace|make it).{0,15}(to|with|use)`),
			regexp.MustCompile(`(?i)(not .+ but .+|don't .+ do .+|missing|forgot|also need)`),
			regexp.MustCompile(`(?i)(you should|try|consider).{0,10}(using|adding|removing)`),
		},

		clarificationPatterns: []*regexp.Regexp{
			// Chinese clarification
			regexp.MustCompile(`(?i)(我的意思是|我想说的是|其实我是想|我其实是想|更准确地说)`),
			regexp.MustCompile(`(?i)(不是这个意思|你理解错了|你没明白|我的需求是)`),

			// English clarification
			regexp.MustCompile(`(?i)(what I mean is|I meant|actually|to be clear|let me clarify)`),
			regexp.MustCompile(`(?i)(you misunderstand|you got it wrong|that's not what I|I want)`),
		},

		questionPatterns: []*regexp.Regexp{
			// Questions that indicate confusion/skepticism
			regexp.MustCompile(`(?i)(why|how come|what if|but what about).{0,30}\?`),
			regexp.MustCompile(`(?i)(will this|can it|is it possible).{0,30}\?`),
		},

		negationWords: []string{
			"不是", "不对", "不行", "不好", "不要", "别",
			"not", "no", "don't", "doesn't", "isn't", "aren't", "wasn't", "weren't",
			"didn't", "haven't", "hasn't", "hadn't", "won't", "wouldn't", "can't", "cannot",
		},
	}
}

// Detect analyzes user message using rules
func (re *FeedbackRuleEngine) Detect(msg string) (*FeedbackResult, bool) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil, false
	}

	// Check for negation + positive combination (e.g., "not good")
	if re.hasNegation(msg) {
		for _, pattern := range re.positivePatterns {
			if pattern.MatchString(msg) {
				return &FeedbackResult{
					Type:       FeedbackNegative,
					Confidence: 0.85,
					Reason:     "Negation of positive statement",
				}, true
			}
		}
	}

	// Short message = high confidence direct match
	if len(msg) < 30 {
		// Check correction first (strong signal)
		for _, pattern := range re.correctionPatterns {
			if pattern.MatchString(msg) {
				return &FeedbackResult{
					Type:       FeedbackCorrection,
					Confidence: 0.95,
					Reason:     "Direct correction pattern in short message",
				}, true
			}
		}

		// Check negative
		for _, pattern := range re.negativePatterns {
			if pattern.MatchString(msg) {
				return &FeedbackResult{
					Type:       FeedbackNegative,
					Confidence: 0.95,
					Reason:     "Direct negative pattern in short message",
				}, true
			}
		}

		// Check positive
		for _, pattern := range re.positivePatterns {
			if pattern.MatchString(msg) {
				return &FeedbackResult{
					Type:       FeedbackPositive,
					Confidence: 0.95,
					Reason:     "Direct positive pattern in short message",
				}, true
			}
		}
	}

	// Longer message: check all patterns
	for _, pattern := range re.correctionPatterns {
		if pattern.MatchString(msg) {
			return &FeedbackResult{
				Type:       FeedbackCorrection,
				Confidence: 0.90,
				Reason:     "Correction pattern detected",
			}, true
		}
	}

	for _, pattern := range re.negativePatterns {
		if pattern.MatchString(msg) {
			return &FeedbackResult{
				Type:       FeedbackNegative,
				Confidence: 0.90,
				Reason:     "Negative pattern detected",
			}, true
		}
	}

	for _, pattern := range re.positivePatterns {
		if pattern.MatchString(msg) {
			return &FeedbackResult{
				Type:       FeedbackPositive,
				Confidence: 0.90,
				Reason:     "Positive pattern detected",
			}, true
		}
	}

	for _, pattern := range re.clarificationPatterns {
		if pattern.MatchString(msg) {
			return &FeedbackResult{
				Type:       FeedbackClarification,
				Confidence: 0.85,
				Reason:     "Clarification pattern detected",
			}, true
		}
	}

	return nil, false
}

func (re *FeedbackRuleEngine) hasNegation(msg string) bool {
	lower := strings.ToLower(msg)
	for _, word := range re.negationWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// ContextualPatternDetector detects implicit dissatisfaction from context
type ContextualPatternDetector struct {
	recentMessages []MessageWithRole
	maxHistory     int
}

// MessageWithRole represents a message with its sender role
type MessageWithRole struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// NewContextualPatternDetector creates a new context detector
func NewContextualPatternDetector() *ContextualPatternDetector {
	return &ContextualPatternDetector{
		recentMessages: make([]MessageWithRole, 0),
		maxHistory:     10,
	}
}

// AddMessage adds a message to history
func (cpd *ContextualPatternDetector) AddMessage(role, content string) {
	cpd.recentMessages = append(cpd.recentMessages, MessageWithRole{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})

	// Trim history
	if len(cpd.recentMessages) > cpd.maxHistory {
		cpd.recentMessages = cpd.recentMessages[len(cpd.recentMessages)-cpd.maxHistory:]
	}
}

// Detect analyzes context for implicit patterns
func (cpd *ContextualPatternDetector) Detect(userMsg, agentLastOutput string) *FeedbackResult {
	// Pattern 1: Repeated similar complaints
	if cpd.isRepeatedComplaint(userMsg) {
		return &FeedbackResult{
			Type:       FeedbackNegative,
			Confidence: 0.85,
			Reason:     "Repeated similar complaints detected",
			IssueType:  "persistence",
		}
	}

	// Pattern 2: Agent asked for confirmation, user avoids direct answer
	if cpd.isAvoidingConfirmation(userMsg, agentLastOutput) {
		return &FeedbackResult{
			Type:       FeedbackNegative,
			Confidence: 0.75,
			Reason:     "User avoiding confirmation - likely dissatisfied",
			IssueType:  "uncertainty",
		}
	}

	// Pattern 3: User starts giving step-by-step instructions (teaching mode)
	if cpd.isTeachingMode(userMsg) {
		return &FeedbackResult{
			Type:       FeedbackCorrection,
			Confidence: 0.80,
			Reason:     "User providing detailed instructions - likely correcting approach",
			IssueType:  "approach",
		}
	}

	// Pattern 4: User asks "why" questions after implementation
	if cpd.isQuestioningApproach(userMsg, agentLastOutput) {
		return &FeedbackResult{
			Type:       FeedbackQuestion,
			Confidence: 0.70,
			Reason:     "Questioning approach - potential skepticism",
			IssueType:  "understanding",
		}
	}

	return nil
}

func (cpd *ContextualPatternDetector) isRepeatedComplaint(msg string) bool {
	// Count recent user messages with negative sentiment keywords
	complaintKeywords := []string{"不对", "错了", "有问题", "不行", "wrong", "issue", "problem", "not working"}
	complaintCount := 0

	for _, m := range cpd.recentMessages {
		if m.Role == "user" {
			lower := strings.ToLower(m.Content)
			for _, kw := range complaintKeywords {
				if strings.Contains(lower, strings.ToLower(kw)) {
					complaintCount++
					break
				}
			}
		}
	}

	return complaintCount >= 2
}

func (cpd *ContextualPatternDetector) isAvoidingConfirmation(userMsg, agentOutput string) bool {
	// Agent asked for confirmation
	confirmationAsk := regexp.MustCompile(`(?i)(可以吗|可以吗\?|可以吗？|ok\?|okay\?|is this ok|does this work)`)
	if !confirmationAsk.MatchString(agentOutput) {
		return false
	}

	// User didn't give direct yes/no, but raised concern or alternative
	avoidancePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(但是|不过|然而|but|however|though|although).{0,30}`),
		regexp.MustCompile(`(?i)(如果|要是|万一|if|what if).{0,30}`),
		regexp.MustCompile(`(?i)(感觉|好像|似乎|seems|feels|looks like).{0,30}`),
	}

	for _, pattern := range avoidancePatterns {
		if pattern.MatchString(userMsg) {
			return true
		}
	}

	return false
}

func (cpd *ContextualPatternDetector) isTeachingMode(msg string) bool {
	// User provides step-by-step instructions
	stepPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(你应该|你要先|第一步|先.*然后.*最后)`),
		regexp.MustCompile(`(?i)(you should|you need to|first.*then.*finally)`),
		regexp.MustCompile(`(?i)(step 1|step one|1\.|2\.|3\.)`),
	}

	for _, pattern := range stepPatterns {
		if pattern.MatchString(msg) {
			return true
		}
	}

	return false
}

func (cpd *ContextualPatternDetector) isQuestioningApproach(userMsg, agentOutput string) bool {
	// Agent provided implementation, user asks "why not X"
	whyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(为什么不|为啥不|怎么不|why not|how about)`),
		regexp.MustCompile(`(?i)(能不能|可以吗|可以吗|can we|could we)`),
	}

	// Check if agent output was code/implementation
	isImplementation := strings.Contains(agentOutput, "```") ||
		strings.Contains(agentOutput, "func ") ||
		strings.Contains(agentOutput, "function ")

	if !isImplementation {
		return false
	}

	for _, pattern := range whyPatterns {
		if pattern.MatchString(userMsg) {
			return true
		}
	}

	return false
}

// LLMFeedbackAnalyzer uses LLM for semantic understanding
type LLMFeedbackAnalyzer struct {
	provider     providers.LLMProvider
	model        string
	rateLimiter  *RateLimiter
	sampleRate   float64
}

// RateLimiter controls LLM call frequency
type RateLimiter struct {
	mu          sync.Mutex
	calls       int
	lastReset   time.Time
	maxPerHour  int
}

// NewRateLimiter creates a rate limiter
func NewRateLimiter(maxPerHour int) *RateLimiter {
	return &RateLimiter{
		lastReset:  time.Now(),
		maxPerHour: maxPerHour,
	}
}

// Allow checks if a call is allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Reset counter every hour
	if time.Since(rl.lastReset) > time.Hour {
		rl.calls = 0
		rl.lastReset = time.Now()
	}

	if rl.calls >= rl.maxPerHour {
		return false
	}

	rl.calls++
	return true
}

// NewLLMFeedbackAnalyzer creates an LLM analyzer
func NewLLMFeedbackAnalyzer(provider providers.LLMProvider, model string, maxCallsPerHour int) *LLMFeedbackAnalyzer {
	return &LLMFeedbackAnalyzer{
		provider:    provider,
		model:       model,
		rateLimiter: NewRateLimiter(maxCallsPerHour),
		sampleRate:  0.3, // 30% of ambiguous cases
	}
}

// ShouldAnalyze checks if this message warrants LLM analysis
func (la *LLMFeedbackAnalyzer) ShouldAnalyze(msg string) bool {
	// Skip if rate limited
	if !la.rateLimiter.Allow() {
		return false
	}

	// Skip very short or very long messages
	if len(msg) < 15 || len(msg) > 800 {
		return false
	}

	// Check for fuzzy words that indicate ambiguity
	fuzzyWords := []string{
		"感觉", "好像", "似乎", "可能", "也许", "大概",
		"feel", "seems", "maybe", "perhaps", "probably", "might",
	}

	lower := strings.ToLower(msg)
	for _, word := range fuzzyWords {
		if strings.Contains(lower, word) {
			return true
		}
	}

	// Check for contrast words (potential hidden dissatisfaction)
	contrastWords := []string{
		"但是", "不过", "然而", "只是",
		"but", "however", "though", "although", "yet",
	}

	for _, word := range contrastWords {
		if strings.Contains(lower, word) {
			return true
		}
	}

	// Random sampling for neutral messages (to improve rule engine)
	return la.sample()
}

func (la *LLMFeedbackAnalyzer) sample() bool {
	return time.Now().UnixNano()%100 < int64(la.sampleRate*100)
}

// Analyze uses LLM to understand feedback semantics
func (la *LLMFeedbackAnalyzer) Analyze(ctx context.Context, userMsg, agentOutput string, context []string) (*FeedbackResult, error) {
	prompt := la.buildPrompt(userMsg, agentOutput, context)

	resp, err := la.callLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return la.parseResponse(resp)
}

func (la *LLMFeedbackAnalyzer) buildPrompt(userMsg, agentOutput string, context []string) string {
	contextStr := ""
	if len(context) > 0 {
		contextStr = "Recent conversation:\n" + strings.Join(context, "\n") + "\n\n"
	}

	return fmt.Sprintf(`You are analyzing user feedback to an AI assistant. Determine the user's sentiment and intent.

%sAI Assistant's last output:
"""
%s
"""

User's feedback:
"""
%s
"""

Analyze:
1. Is the user satisfied, dissatisfied, or providing correction?
2. If dissatisfied, what is the issue? (understanding/implementation/style/omission/other)
3. What should the AI do next? (accept/correct/clarify/apologize)
4. Confidence 0-1

Output JSON only:
{
  "feedback_type": "satisfied|dissatisfied|correction|clarification|neutral",
  "issue_type": "understanding|implementation|style|omission|none",
  "confidence": 0.85,
  "action": "accept|correct|clarify|apologize|continue",
  "reason": "brief explanation"
}`, contextStr, agentOutput, userMsg)
}

func (la *LLMFeedbackAnalyzer) callLLM(ctx context.Context, prompt string) (string, error) {
	if la.provider == nil {
		return "", fmt.Errorf("LLM provider not configured")
	}

	messages := []providers.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := la.provider.Chat(ctx, messages, nil, la.model)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (la *LLMFeedbackAnalyzer) parseResponse(content string) (*FeedbackResult, error) {
	// Extract JSON from response
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := content[start : end+1]

	var result struct {
		FeedbackType string  `json:"feedback_type"`
		IssueType    string  `json:"issue_type"`
		Confidence   float64 `json:"confidence"`
		Action       string  `json:"action"`
		Reason       string  `json:"reason"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Map to internal types
	feedbackType := FeedbackNeutral
	switch result.FeedbackType {
	case "satisfied":
		feedbackType = FeedbackPositive
	case "dissatisfied":
		feedbackType = FeedbackNegative
	case "correction":
		feedbackType = FeedbackCorrection
	case "clarification":
		feedbackType = FeedbackClarification
	}

	return &FeedbackResult{
		Type:       feedbackType,
		Confidence: result.Confidence,
		IssueType:  result.IssueType,
		Action:     result.Action,
		Reason:     result.Reason,
	}, nil
}

// FeedbackDetector is the main detector combining all layers
type FeedbackDetector struct {
	ruleEngine    *FeedbackRuleEngine
	contextDetector *ContextualPatternDetector
	llmAnalyzer   *LLMFeedbackAnalyzer

	// Statistics
	stats struct {
		TotalChecks    int
		RuleHits       int
		ContextHits    int
		LLMCalls       int
		LLMHits        int
	}
}

// NewFeedbackDetector creates the complete detector
func NewFeedbackDetector(llmProvider providers.LLMProvider, llmModel string) *FeedbackDetector {
	fd := &FeedbackDetector{
		ruleEngine:      NewFeedbackRuleEngine(),
		contextDetector: NewContextualPatternDetector(),
	}

	// Only enable LLM if provider available
	if llmProvider != nil {
		fd.llmAnalyzer = NewLLMFeedbackAnalyzer(llmProvider, llmModel, 50) // Max 50 calls/hour
	}

	return fd
}

// Detect analyzes user feedback through all layers
func (fd *FeedbackDetector) Detect(ctx context.Context, userMsg, agentOutput string) *FeedbackResult {
	fd.stats.TotalChecks++

	// Layer 1: Rule Engine (fast, zero cost)
	if result, ok := fd.ruleEngine.Detect(userMsg); ok {
		fd.stats.RuleHits++
		fd.contextDetector.AddMessage("user", userMsg)
		return result
	}

	// Layer 2: Contextual Patterns
	fd.contextDetector.AddMessage("agent", agentOutput)
	if result := fd.contextDetector.Detect(userMsg, agentOutput); result != nil {
		fd.stats.ContextHits++
		fd.contextDetector.AddMessage("user", userMsg)
		return result
	}

	// Layer 3: LLM Analysis (expensive, conditional)
	if fd.llmAnalyzer != nil && fd.llmAnalyzer.ShouldAnalyze(userMsg) {
		fd.stats.LLMCalls++
		if result, err := fd.llmAnalyzer.Analyze(ctx, userMsg, agentOutput, fd.getRecentContext()); err == nil && result.Confidence > 0.7 {
			fd.stats.LLMHits++
			fd.contextDetector.AddMessage("user", userMsg)
			return result
		}
	}

	// Default: Neutral
	fd.contextDetector.AddMessage("user", userMsg)
	return &FeedbackResult{
		Type:       FeedbackNeutral,
		Confidence: 0.5,
		Reason:     "No clear sentiment detected",
	}
}

// GetStats returns detection statistics
func (fd *FeedbackDetector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_checks": fd.stats.TotalChecks,
		"rule_hits":    fd.stats.RuleHits,
		"context_hits": fd.stats.ContextHits,
		"llm_calls":    fd.stats.LLMCalls,
		"llm_hits":     fd.stats.LLMHits,
		"rule_rate":    float64(fd.stats.RuleHits) / float64(fd.stats.TotalChecks),
	}
}

func (fd *FeedbackDetector) getRecentContext() []string {
	var context []string
	for _, m := range fd.contextDetector.recentMessages {
		context = append(context, fmt.Sprintf("%s: %s", m.Role, m.Content))
	}
	return context
}
