package agent

import (
	"fmt"
	"testing"
)

func TestBuildSystemPromptWithEmptyPlan(t *testing.T) {
	cb := NewContextBuilderWithConfig("/tmp/workspace", false)
	
	// Test 1: Empty plan
	plan := CreatePlan("下载分析腾讯财报")
	
	fmt.Println("=== System Prompt with Empty Plan ===")
	prompt := cb.BuildSystemPromptWithPlan(plan)
	// Find the planning section
	if containsSubstring(prompt, "请先规划任务步骤") {
		fmt.Println("✓ Contains '请先规划任务步骤'")
	} else if containsSubstring(prompt, "使用 [Step] 描述") {
		fmt.Println("✓ Contains '[Step]' instruction")
	}
	fmt.Println()
	
	// Test 2: Plan with steps
	plan.AddStep("搜索PDF链接")
	plan.Steps[0].Status = StepStatusRunning
	
	fmt.Println("=== System Prompt with Steps ===")
	prompt = cb.BuildSystemPromptWithPlan(plan)
	if containsSubstring(prompt, "请先规划任务步骤") {
		fmt.Println("✗ Should NOT contain '请先规划任务步骤' when steps exist")
	} else {
		fmt.Println("✓ Does not contain planning request (steps already defined)")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
