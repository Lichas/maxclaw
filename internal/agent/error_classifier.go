package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ErrorReason represents the type of error encountered
type ErrorReason string

const (
	// Authentication / authorization
	ErrorReasonAuth         ErrorReason = "auth"          // Transient auth (401/403) — refresh/rotate
	ErrorReasonAuthPermanent ErrorReason = "auth_permanent" // Auth failed after refresh — abort

	// Billing / quota
	ErrorReasonBilling   ErrorReason = "billing"    // 402 or confirmed credit exhaustion
	ErrorReasonRateLimit ErrorReason = "rate_limit" // 429 or quota-based throttling

	// Server-side
	ErrorReasonOverloaded  ErrorReason = "overloaded"   // 503/529 — provider overloaded
	ErrorReasonServerError ErrorReason = "server_error" // 500/502 — internal server error

	// Transport
	ErrorReasonTimeout ErrorReason = "timeout" // Connection/read timeout

	// Context / payload
	ErrorReasonContextOverflow ErrorReason = "context_overflow"  // Context too large
	ErrorReasonPayloadTooLarge ErrorReason = "payload_too_large" // 413

	// Model
	ErrorReasonModelNotFound ErrorReason = "model_not_found" // 404 or invalid model

	// Request format
	ErrorReasonFormatError ErrorReason = "format_error" // 400 bad request

	// Provider-specific
	ErrorReasonThinkingSignature ErrorReason = "thinking_signature" // Anthropic thinking block sig invalid
	ErrorReasonLongContextTier   ErrorReason = "long_context_tier"  // Anthropic "extra usage" tier gate

	// Catch-all
	ErrorReasonUnknown ErrorReason = "unknown" // Unclassifiable
)

// ClassifiedError represents a structured error classification with recovery hints
type ClassifiedError struct {
	Reason                 ErrorReason
	StatusCode             int
	Provider               string
	Model                  string
	Message                string
	ErrorContext           map[string]interface{}
	Retryable              bool
	ShouldCompress         bool
	ShouldRotateCredential bool
	ShouldFallback         bool
}

// IsAuth returns true if the error is authentication-related
func (e *ClassifiedError) IsAuth() bool {
	return e.Reason == ErrorReasonAuth || e.Reason == ErrorReasonAuthPermanent
}

// IsTransient returns true if the error is expected to resolve on retry
func (e *ClassifiedError) IsTransient() bool {
	switch e.Reason {
	case ErrorReasonRateLimit, ErrorReasonOverloaded, ErrorReasonServerError,
		ErrorReasonTimeout, ErrorReasonUnknown:
		return true
	default:
		return false
	}
}

// ErrorClassifier provides structured error classification for API failures
type ErrorClassifier struct {
	// Pattern caches for performance
	billingPatterns      []string
	rateLimitPatterns    []string
	contextPatterns      []string
	modelNotFoundPatterns []string
	authPatterns         []string
}

// NewErrorClassifier creates a new error classifier with default patterns
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{
		billingPatterns: []string{
			"insufficient credits",
			"insufficient_quota",
			"credit balance",
			"credits have been exhausted",
			"top up your credits",
			"payment required",
			"billing hard limit",
			"exceeded your current quota",
			"account is deactivated",
			"plan does not include",
		},
		rateLimitPatterns: []string{
			"rate limit",
			"rate_limit",
			"too many requests",
			"throttled",
			"requests per minute",
			"tokens per minute",
			"requests per day",
			"try again in",
			"please retry after",
			"resource_exhausted",
		},
		contextPatterns: []string{
			"context length",
			"context size",
			"maximum context",
			"token limit",
			"too many tokens",
			"reduce the length",
			"exceeds the limit",
			"context window",
			"prompt is too long",
			"prompt exceeds max length",
			"max_tokens",
			"maximum number of tokens",
			"超过最大长度",
			"上下文长度",
		},
		modelNotFoundPatterns: []string{
			"is not a valid model",
			"invalid model",
			"model not found",
			"model_not_found",
			"does not exist",
			"no such model",
			"unknown model",
			"unsupported model",
		},
		authPatterns: []string{
			"invalid api key",
			"invalid_api_key",
			"authentication",
			"unauthorized",
			"forbidden",
			"invalid token",
			"token expired",
			"token revoked",
			"access denied",
		},
	}
}

// ClassifyError classifies an error into a structured recovery recommendation
// Priority-ordered pipeline:
// 1. Special-case provider-specific patterns
// 2. HTTP status code + message-aware refinement
// 3. Message pattern matching
// 4. Transport error heuristics
// 5. Fallback: unknown
func (ec *ErrorClassifier) ClassifyError(
	err error,
	provider string,
	model string,
	approxTokens int,
	contextLength int,
	numMessages int,
) *ClassifiedError {
	if err == nil {
		return nil
	}

	statusCode := extractStatusCode(err)
	errorType := fmt.Sprintf("%T", err)
	errorMsg := strings.ToLower(err.Error())
	body := extractErrorBody(err)

	// Build result helper
	result := func(reason ErrorReason, overrides ...func(*ClassifiedError)) *ClassifiedError {
		ce := &ClassifiedError{
			Reason:     reason,
			StatusCode: statusCode,
			Provider:   provider,
			Model:      model,
			Message:    extractMessage(err, body),
			ErrorContext: map[string]interface{}{
				"error_type": errorType,
				"body":       body,
			},
		}
		// Apply defaults based on reason
		ec.applyDefaults(ce)
		// Apply overrides
		for _, fn := range overrides {
			fn(ce)
		}
		return ce
	}

	// 1. Provider-specific patterns (highest priority)
	if statusCode == 400 && strings.Contains(errorMsg, "signature") && strings.Contains(errorMsg, "thinking") {
		return result(ErrorReasonThinkingSignature, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldCompress = false
		})
	}

	if statusCode == 429 && strings.Contains(errorMsg, "extra usage") && strings.Contains(errorMsg, "long context") {
		return result(ErrorReasonLongContextTier, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldCompress = true
		})
	}

	// 2. HTTP status code classification
	if statusCode > 0 {
		classified := ec.classifyByStatus(statusCode, errorMsg, body, provider, model, approxTokens, contextLength, numMessages, result)
		if classified != nil {
			return classified
		}
	}

	// 3. Message pattern matching
	classified := ec.classifyByMessage(errorMsg, errorType, approxTokens, contextLength, result)
	if classified != nil {
		return classified
	}

	// 4. Transport error heuristics
	if isTransportError(errorType, err) {
		return result(ErrorReasonTimeout, func(ce *ClassifiedError) {
			ce.Retryable = true
		})
	}

	// 5. Server disconnect + large session → context overflow
	if isServerDisconnect(errorMsg) && statusCode == 0 {
		isLarge := approxTokens > int(float64(contextLength)*0.6) || approxTokens > 120000 || numMessages > 200
		if isLarge {
			return result(ErrorReasonContextOverflow, func(ce *ClassifiedError) {
				ce.Retryable = true
				ce.ShouldCompress = true
			})
		}
		return result(ErrorReasonTimeout, func(ce *ClassifiedError) {
			ce.Retryable = true
		})
	}

	// 6. Fallback: unknown
	return result(ErrorReasonUnknown, func(ce *ClassifiedError) {
		ce.Retryable = true
	})
}

// applyDefaults sets default recovery flags based on error reason
func (ec *ErrorClassifier) applyDefaults(ce *ClassifiedError) {
	switch ce.Reason {
	case ErrorReasonAuth:
		ce.Retryable = false
		ce.ShouldRotateCredential = true
		ce.ShouldFallback = true
	case ErrorReasonAuthPermanent:
		ce.Retryable = false
	case ErrorReasonBilling:
		ce.Retryable = false
		ce.ShouldRotateCredential = true
		ce.ShouldFallback = true
	case ErrorReasonRateLimit:
		ce.Retryable = true
		ce.ShouldRotateCredential = true
		ce.ShouldFallback = true
	case ErrorReasonOverloaded, ErrorReasonServerError:
		ce.Retryable = true
	case ErrorReasonTimeout:
		ce.Retryable = true
	case ErrorReasonContextOverflow:
		ce.Retryable = true
		ce.ShouldCompress = true
	case ErrorReasonPayloadTooLarge:
		ce.Retryable = true
		ce.ShouldCompress = true
	case ErrorReasonModelNotFound:
		ce.Retryable = false
		ce.ShouldFallback = true
	case ErrorReasonFormatError:
		ce.Retryable = false
	default:
		ce.Retryable = true
	}
}

// classifyByStatus classifies based on HTTP status code
func (ec *ErrorClassifier) classifyByStatus(
	statusCode int,
	errorMsg string,
	body map[string]interface{},
	provider, model string,
	approxTokens, contextLength, numMessages int,
	result func(ErrorReason, ...func(*ClassifiedError)) *ClassifiedError,
) *ClassifiedError {
	switch statusCode {
	case 401:
		return result(ErrorReasonAuth, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldRotateCredential = true
			ce.ShouldFallback = true
		})

	case 403:
		if strings.Contains(errorMsg, "key limit exceeded") || strings.Contains(errorMsg, "spending limit") {
			return result(ErrorReasonBilling, func(ce *ClassifiedError) {
				ce.Retryable = false
				ce.ShouldRotateCredential = true
				ce.ShouldFallback = true
			})
		}
		return result(ErrorReasonAuth, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldFallback = true
		})

	case 402:
		return result(ErrorReasonBilling, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldRotateCredential = true
			ce.ShouldFallback = true
		})

	case 404:
		if ec.matchesAnyPattern(errorMsg, ec.modelNotFoundPatterns) {
			return result(ErrorReasonModelNotFound, func(ce *ClassifiedError) {
				ce.Retryable = false
				ce.ShouldFallback = true
			})
		}
		return result(ErrorReasonModelNotFound, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldFallback = true
		})

	case 413:
		return result(ErrorReasonPayloadTooLarge, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldCompress = true
		})

	case 429:
		return result(ErrorReasonRateLimit, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldRotateCredential = true
			ce.ShouldFallback = true
		})

	case 400:
		return ec.classify400(errorMsg, body, approxTokens, contextLength, numMessages, result)

	case 500, 502:
		return result(ErrorReasonServerError, func(ce *ClassifiedError) {
			ce.Retryable = true
		})

	case 503, 529:
		return result(ErrorReasonOverloaded, func(ce *ClassifiedError) {
			ce.Retryable = true
		})
	}

	return nil
}

// classify400 handles the complex 400 error classification
func (ec *ErrorClassifier) classify400(
	errorMsg string,
	body map[string]interface{},
	approxTokens, contextLength, numMessages int,
	result func(ErrorReason, ...func(*ClassifiedError)) *ClassifiedError,
) *ClassifiedError {
	// Check for context overflow patterns first
	if ec.matchesAnyPattern(errorMsg, ec.contextPatterns) {
		return result(ErrorReasonContextOverflow, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldCompress = true
		})
	}

	// Check for model not found
	if ec.matchesAnyPattern(errorMsg, ec.modelNotFoundPatterns) {
		return result(ErrorReasonModelNotFound, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldFallback = true
		})
	}

	// Large session + 400 → likely context overflow
	isLarge := approxTokens > int(float64(contextLength)*0.7) || approxTokens > 100000 || numMessages > 150
	if isLarge {
		return result(ErrorReasonContextOverflow, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldCompress = true
		})
	}

	return result(ErrorReasonFormatError, func(ce *ClassifiedError) {
		ce.Retryable = false
	})
}

// classifyByMessage classifies based on error message patterns
func (ec *ErrorClassifier) classifyByMessage(
	errorMsg, errorType string,
	approxTokens, contextLength int,
	result func(ErrorReason, ...func(*ClassifiedError)) *ClassifiedError,
) *ClassifiedError {
	// Check billing patterns
	if ec.matchesAnyPattern(errorMsg, ec.billingPatterns) {
		return result(ErrorReasonBilling, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldRotateCredential = true
			ce.ShouldFallback = true
		})
	}

	// Check rate limit patterns
	if ec.matchesAnyPattern(errorMsg, ec.rateLimitPatterns) {
		return result(ErrorReasonRateLimit, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldRotateCredential = true
			ce.ShouldFallback = true
		})
	}

	// Check context overflow patterns
	if ec.matchesAnyPattern(errorMsg, ec.contextPatterns) {
		return result(ErrorReasonContextOverflow, func(ce *ClassifiedError) {
			ce.Retryable = true
			ce.ShouldCompress = true
		})
	}

	// Check model not found patterns
	if ec.matchesAnyPattern(errorMsg, ec.modelNotFoundPatterns) {
		return result(ErrorReasonModelNotFound, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldFallback = true
		})
	}

	// Check auth patterns
	if ec.matchesAnyPattern(errorMsg, ec.authPatterns) {
		return result(ErrorReasonAuth, func(ce *ClassifiedError) {
			ce.Retryable = false
			ce.ShouldRotateCredential = true
			ce.ShouldFallback = true
		})
	}

	return nil
}

// matchesAnyPattern checks if error message matches any pattern
func (ec *ErrorClassifier) matchesAnyPattern(errorMsg string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}
	return false
}

// Helper functions

var statusCodeRegex = regexp.MustCompile(`status code[:\s]*(\d+)`)

func extractStatusCode(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	matches := statusCodeRegex.FindStringSubmatch(msg)
	if len(matches) > 1 {
		var code int
		fmt.Sscanf(matches[1], "%d", &code)
		return code
	}
	return 0
}

func extractErrorBody(err error) map[string]interface{} {
	if err == nil {
		return nil
	}
	// Try to extract JSON from error message
	msg := err.Error()
	// Look for JSON-like structures
	start := strings.Index(msg, "{")
	if start == -1 {
		return nil
	}
	// Try to find matching closing brace
	depth := 0
	end := start
	for i := start; i < len(msg); i++ {
		switch msg[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if depth == 0 {
			break
		}
	}
	if end <= start {
		return nil
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(msg[start:end]), &body); err != nil {
		return nil
	}
	return body
}

func extractMessage(err error, body map[string]interface{}) string {
	if err == nil {
		return ""
	}
	// Try to extract from body first
	if body != nil {
		if errObj, ok := body["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok && msg != "" {
				return msg
			}
		}
		if msg, ok := body["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return err.Error()
}

func isTransportError(errorType string, err error) bool {
	transportErrors := []string{
		"ReadTimeout", "ConnectTimeout", "PoolTimeout",
		"ConnectError", "RemoteProtocolError",
		"ConnectionError", "ConnectionResetError",
		"ConnectionAbortedError", "BrokenPipeError",
		"TimeoutError", "ReadError",
		"ServerDisconnectedError",
		"APIConnectionError", "APITimeoutError",
	}
	for _, te := range transportErrors {
		if strings.Contains(errorType, te) {
			return true
		}
	}
	return false
}

func isServerDisconnect(errorMsg string) bool {
	patterns := []string{
		"server disconnected",
		"peer closed connection",
		"connection reset by peer",
		"connection was closed",
		"network connection lost",
		"unexpected eof",
		"incomplete chunked read",
	}
	for _, pattern := range patterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}
	return false
}

// ParseContextLimitFromError attempts to extract context limit from error message
func ParseContextLimitFromError(errorMsg string) (int, bool) {
	// Pattern: "context length of X tokens exceeds maximum of Y"
	// Pattern: "maximum context length is X tokens"
	// Pattern: "reduce the length to X tokens"
	
	patterns := []string{
		`maximum of (\d+)`,
		`maximum.*?(\d+) tokens`,
		`limit is (\d+)`,
		`exceeds.*?limit of (\d+)`,
		`reduce the length to (\d+)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(errorMsg)
		if len(matches) > 1 {
			var limit int
			if _, err := fmt.Sscanf(matches[1], "%d", &limit); err == nil && limit > 0 {
				return limit, true
			}
		}
	}
	return 0, false
}

// ParseAvailableOutputTokensFromError extracts available output tokens from error
func ParseAvailableOutputTokensFromError(errorMsg string) (int, bool) {
	// Pattern: "you requested X tokens but only Y are available"
	// Pattern: "available tokens: Y"
	patterns := []string{
		`only (\d+) are available`,
		`available tokens[:\s]*(\d+)`,
		`(\d+) tokens available`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(errorMsg)
		if len(matches) > 1 {
			var available int
			if _, err := fmt.Sscanf(matches[1], "%d", &available); err == nil && available > 0 {
				return available, true
			}
		}
	}
	return 0, false
}
