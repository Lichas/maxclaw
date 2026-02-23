package agent

import (
	"testing"
)

func TestIntentAnalyzer(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name        string
		input       string
		expected    UserIntent
		isInterrupt bool
	}{
		{"纠正", "不对，应该用 Go", IntentCorrection, true},
		{"补充", "对了，记得加上错误处理", IntentAppend, false},
		{"停止", "停止生成", IntentStop, true},
		{"继续", "详细的解释一下", IntentContinue, false},
		{"短消息打断", "用 Python", IntentCorrection, true},
		{"英文纠正", "no, use Python", IntentCorrection, true},
		{"英文补充", "also add error handling", IntentAppend, false},
		{"英文停止", "stop", IntentStop, true},
		{"改为", "改成用 Go 实现", IntentCorrection, true},
		{"等一下", "等一下，用中文", IntentCorrection, true},
		{"还有", "还有，加上日志", IntentAppend, false},
		{"顺便", "顺便优化一下", IntentAppend, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Analyze(tt.input, "")
			if result.Intent != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result.Intent)
			}
			if result.IsInterrupt != tt.isInterrupt {
				t.Errorf("isInterrupt expected %v, got %v", tt.isInterrupt, result.IsInterrupt)
			}
		})
	}
}

func TestIntentAnalyzer_Confidence(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	// 停止应该有最高置信度
	stopResult := analyzer.Analyze("停止生成", "")
	if stopResult.Confidence != 0.95 {
		t.Errorf("stop confidence should be 0.95, got %f", stopResult.Confidence)
	}

	// 纠正应该有高置信度
	correctResult := analyzer.Analyze("不对", "")
	if correctResult.Confidence != 0.85 {
		t.Errorf("correction confidence should be 0.85, got %f", correctResult.Confidence)
	}

	// 补充应该有中高置信度
	appendResult := analyzer.Analyze("对了", "")
	if appendResult.Confidence != 0.80 {
		t.Errorf("append confidence should be 0.80, got %f", appendResult.Confidence)
	}

	// 默认应该有中等置信度
	continueResult := analyzer.Analyze("这是一个很长的消息，应该被当作继续处理", "")
	if continueResult.Confidence != 0.70 {
		t.Errorf("continue confidence should be 0.70, got %f", continueResult.Confidence)
	}

	// 短消息应该有较低置信度
	shortResult := analyzer.Analyze("短消息", "")
	if shortResult.Confidence != 0.60 {
		t.Errorf("short message confidence should be 0.60, got %f", shortResult.Confidence)
	}
}

func TestIntentAnalyzer_Explanation(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	result := analyzer.Analyze("不对，错了", "")
	if result.Explanation == "" {
		t.Error("explanation should not be empty")
	}
}
