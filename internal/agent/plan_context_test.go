package agent

import (
	"fmt"
	"testing"
)

func TestPlanContextWithEmptySteps(t *testing.T) {
	// Create plan with no steps
	plan := CreatePlan("下载分析腾讯财报")
	
	fmt.Println("=== Empty Plan ToContextString ===")
	fmt.Println(plan.ToContextString())
	fmt.Println()
	
	// Add a step
	plan.AddStep("搜索PDF链接")
	plan.Steps[0].Status = StepStatusRunning
	
	fmt.Println("=== Plan with Step ToContextString ===")
	fmt.Println(plan.ToContextString())
}
