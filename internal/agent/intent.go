package agent

import (
	"strings"
)

// UserIntent 用户意图类型
type UserIntent string

const (
	IntentContinue   UserIntent = "continue"   // 继续当前话题
	IntentCorrection UserIntent = "correction" // 纠正/否定
	IntentAppend     UserIntent = "append"     // 补充信息
	IntentNewTopic   UserIntent = "new_topic"  // 新话题
	IntentStop       UserIntent = "stop"       // 明确要求停止
)

// IntentResult 意图分析结果
type IntentResult struct {
	Intent      UserIntent
	Confidence  float64
	IsInterrupt bool
	Explanation string
}

// IntentAnalyzer 意图分析器
type IntentAnalyzer struct {
	// 打断关键词（否定、纠正）
	correctionPatterns []string
	// 补充关键词
	appendPatterns []string
	// 停止关键词
	stopPatterns []string
}

// NewIntentAnalyzer 创建分析器
func NewIntentAnalyzer() *IntentAnalyzer {
	return &IntentAnalyzer{
		correctionPatterns: []string{
			"不对", "错了", "不是这样", "改一下", "换成", "修改为",
			"no", "wrong", "incorrect", "change to", "use",
			"不是", "要的是", "应该是", "更正", "纠正",
			"等一下", "stop", "停止", "别", "不要",
		},
		appendPatterns: []string{
			"对了", "还有", "另外", "补充", "记得", "别忘了",
			"also", "plus", "add", "remember", "by the way",
			"顺便", "以及", "并且", "而且",
		},
		stopPatterns: []string{
			"停止生成", "不要生成了", "stop", "够了", "可以了",
			"cancel", "abort", "停",
		},
	}
}

// Analyze 分析用户输入意图
func (a *IntentAnalyzer) Analyze(input string, currentContext string) IntentResult {
	inputLower := strings.ToLower(strings.TrimSpace(input))

	// 1. 检查停止意图（最高优先级）
	for _, pattern := range a.stopPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return IntentResult{
				Intent:      IntentStop,
				Confidence:  0.95,
				IsInterrupt: true,
				Explanation: "检测到明确的停止请求",
			}
		}
	}

	// 2. 检查纠正意图
	for _, pattern := range a.correctionPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return IntentResult{
				Intent:      IntentCorrection,
				Confidence:  0.85,
				IsInterrupt: true,
				Explanation: "检测到纠正/否定意图",
			}
		}
	}

	// 3. 检查补充意图
	for _, pattern := range a.appendPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return IntentResult{
				Intent:      IntentAppend,
				Confidence:  0.80,
				IsInterrupt: false,
				Explanation: "检测到补充信息意图",
			}
		}
	}

	// 4. 长度启发式：短消息更可能是打断
	if len(input) < 20 {
		return IntentResult{
			Intent:      IntentCorrection,
			Confidence:  0.60,
			IsInterrupt: true,
			Explanation: "短消息倾向于打断",
		}
	}

	// 默认继续
	return IntentResult{
		Intent:      IntentContinue,
		Confidence:  0.70,
		IsInterrupt: false,
		Explanation: "未检测到特殊意图，继续当前话题",
	}
}
