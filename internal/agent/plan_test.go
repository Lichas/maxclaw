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
