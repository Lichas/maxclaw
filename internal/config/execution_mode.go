package config

import "strings"

const (
	ExecutionModeSafe = "safe"
	ExecutionModeAsk  = "ask"
	ExecutionModeAuto = "auto"
)

// NormalizeExecutionMode normalizes user config to supported execution modes.
// Any unknown value falls back to ask mode.
func NormalizeExecutionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ExecutionModeSafe:
		return ExecutionModeSafe
	case ExecutionModeAuto:
		return ExecutionModeAuto
	default:
		return ExecutionModeAsk
	}
}
