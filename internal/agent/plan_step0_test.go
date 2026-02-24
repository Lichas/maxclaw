package agent

import (
	"fmt"
	"testing"
	"time"
)

func TestPlan_FirstStepRunningOnCreation(t *testing.T) {
	// Simulate step 0 scenario:
	// 1. Create plan
	// 2. Add steps from LLM output
	// 3. First step should be running
	
	plan := CreatePlan("下载分析腾讯财报")
	
	// Simulate extracting steps from LLM output on iteration 0
	plan.AddStep("搜索PDF链接")
	plan.AddStep("下载PDF文件")
	plan.AddStep("提取数据")
	
	// Simulate what loop.go does: mark first step as running
	if len(plan.Steps) > 0 {
		plan.Steps[0].Status = StepStatusRunning
		now := time.Now()
		plan.Steps[0].StartedAt = &now
	}
	
	// Verify
	if plan.Steps[0].Status != StepStatusRunning {
		t.Errorf("expected first step to be running, got %s", plan.Steps[0].Status)
	}
	
	if plan.Steps[1].Status != StepStatusPending {
		t.Errorf("expected second step to be pending, got %s", plan.Steps[1].Status)
	}
	
	// Verify CurrentStep returns the first step
	current := plan.CurrentStep()
	if current == nil {
		t.Fatal("expected CurrentStep to return first step")
	}
	if current.Description != "搜索PDF链接" {
		t.Errorf("expected current step to be '搜索PDF链接', got %s", current.Description)
	}
	
	fmt.Println("=== Plan after Step 0 setup ===")
	fmt.Println(plan.GenerateProgressSummary())
}
