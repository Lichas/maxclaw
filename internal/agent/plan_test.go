package agent

import (
	"testing"
	"time"
)

func TestPlan_BasicOperations(t *testing.T) {
	plan := &Plan{
		ID:        "plan_test",
		Goal:      "test goal",
		Status:    PlanStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Steps:     []*Step{},
	}

	// Add steps
	plan.AddStep("step 1")
	plan.AddStep("step 2")

	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(plan.Steps))
	}

	// Start first step
	plan.Steps[0].Status = StepStatusRunning
	now := time.Now()
	plan.Steps[0].StartedAt = &now

	// Complete first step
	plan.CompleteCurrentStep("result 1")

	if plan.Steps[0].Status != StepStatusCompleted {
		t.Errorf("expected step 1 completed, got %s", plan.Steps[0].Status)
	}

	if plan.CurrentStepIndex != 1 {
		t.Errorf("expected current step index 1, got %d", plan.CurrentStepIndex)
	}

	if plan.Steps[1].Status != StepStatusRunning {
		t.Errorf("expected step 2 running, got %s", plan.Steps[1].Status)
	}
}

func TestPlanManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	pm := NewPlanManager(tmpDir)
	sessionKey := "test:session"

	// Create and save plan
	plan := CreatePlan("test goal")
	plan.AddStep("step 1")
	plan.Steps[0].Status = StepStatusRunning
	now := time.Now()
	plan.Steps[0].StartedAt = &now

	if err := pm.Save(sessionKey, plan); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Load plan
	loaded, err := pm.Load(sessionKey)
	if err != nil {
		t.Fatalf("failed to load plan: %v", err)
	}

	if loaded == nil {
		t.Fatal("expected plan to be loaded")
	}

	if loaded.Goal != "test goal" {
		t.Errorf("expected goal 'test goal', got %s", loaded.Goal)
	}

	if len(loaded.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(loaded.Steps))
	}
}

func TestPlanManager_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	pm := NewPlanManager(tmpDir)
	sessionKey := "test:session"

	if pm.Exists(sessionKey) {
		t.Error("expected plan to not exist")
	}

	plan := CreatePlan("test goal")
	pm.Save(sessionKey, plan)

	if !pm.Exists(sessionKey) {
		t.Error("expected plan to exist")
	}
}

func TestStepDetector_DetectCompletion(t *testing.T) {
	sd := NewStepDetector()

	tests := []struct {
		name            string
		output          string
		iterationInStep int
		wantComplete    bool
	}{
		{
			name:            "transition word detected",
			output:          "现在让我下载文件",
			iterationInStep: 3,
			wantComplete:    true,
		},
		{
			name:            "transition word but too early",
			output:          "现在开始下载",
			iterationInStep: 1,
			wantComplete:    false,
		},
		{
			name:            "timeout fallback",
			output:          "some random output",
			iterationInStep: 10,
			wantComplete:    true,
		},
		{
			name:            "no completion",
			output:          "still working on it",
			iterationInStep: 3,
			wantComplete:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sd.DetectCompletion(tt.output, tt.iterationInStep)
			if got != tt.wantComplete {
				t.Errorf("DetectCompletion() = %v, want %v", got, tt.wantComplete)
			}
		})
	}
}

func TestExtractStepDeclarations(t *testing.T) {
	content := `
I'll help you with this task.
[Step] Download the PDF files
[Step] Extract financial data
[Step] Create charts
Let's start.
`

	steps := ExtractStepDeclarations(content)
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}

	expected := []string{"Download the PDF files", "Extract financial data", "Create charts"}
	for i, exp := range expected {
		if steps[i] != exp {
			t.Errorf("step %d: expected %q, got %q", i, exp, steps[i])
		}
	}
}

func TestIsContinueIntent(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"继续", true},
		{"continue", true},
		{"请继续执行", true},
		{"resume", true},
		{"好的", false},
		{"hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			got := IsContinueIntent(tt.content)
			if got != tt.want {
				t.Errorf("IsContinueIntent(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
