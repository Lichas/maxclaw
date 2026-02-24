package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PlanStatus represents the status of a plan
type PlanStatus string

const (
	PlanStatusPending   PlanStatus = "pending"
	PlanStatusRunning   PlanStatus = "running"
	PlanStatusPaused    PlanStatus = "paused"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
)

// StepStatus represents the status of a step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
)

// Progress tracks sub-progress within a step
type Progress struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// Step represents a single step in a plan
type Step struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Status      StepStatus `json:"status"`
	Result      string     `json:"result"`
	Progress    *Progress  `json:"progress,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Plan represents a task plan for the agent
type Plan struct {
	ID               string     `json:"id"`
	Goal             string     `json:"goal"`
	Status           PlanStatus `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Steps            []*Step    `json:"steps"`
	CurrentStepIndex int        `json:"current_step_index"`
	IterationCount   int        `json:"iteration_count"`
}

// IsTerminal returns true if the plan is in a terminal state
func (p *Plan) IsTerminal() bool {
	return p.Status == PlanStatusCompleted || p.Status == PlanStatusFailed
}

// CurrentStep returns the current step or nil if all steps completed
func (p *Plan) CurrentStep() *Step {
	if p.CurrentStepIndex >= 0 && p.CurrentStepIndex < len(p.Steps) {
		return p.Steps[p.CurrentStepIndex]
	}
	return nil
}

// AddStep adds a new step to the plan
func (p *Plan) AddStep(description string) *Step {
	step := &Step{
		ID:          fmt.Sprintf("step_%d", len(p.Steps)+1),
		Description: description,
		Status:      StepStatusPending,
	}
	p.Steps = append(p.Steps, step)
	p.UpdatedAt = time.Now()
	return step
}

// CompleteCurrentStep marks the current step as completed and advances
func (p *Plan) CompleteCurrentStep(result string) {
	if step := p.CurrentStep(); step != nil {
		step.Status = StepStatusCompleted
		step.Result = result
		now := time.Now()
		step.CompletedAt = &now
	}
	p.CurrentStepIndex++
	if p.CurrentStepIndex < len(p.Steps) {
		p.Steps[p.CurrentStepIndex].Status = StepStatusRunning
		now := time.Now()
		p.Steps[p.CurrentStepIndex].StartedAt = &now
	} else {
		p.Status = PlanStatusCompleted
	}
	p.UpdatedAt = time.Now()
}

// PlanManager manages plan storage and lifecycle
type PlanManager struct {
	workspace string
	mu        sync.RWMutex
	plans     map[string]*Plan // sessionKey -> Plan cache
}

// NewPlanManager creates a new PlanManager
func NewPlanManager(workspace string) *PlanManager {
	return &PlanManager{
		workspace: workspace,
		plans:     make(map[string]*Plan),
	}
}

// planPath returns the path to the plan file for a session
func (pm *PlanManager) planPath(sessionKey string) string {
	safeKey := sanitizeSessionKey(sessionKey)
	return filepath.Join(pm.workspace, ".sessions", safeKey, "plan.json")
}

// sanitizeSessionKey sanitizes a session key for use in file paths
func sanitizeSessionKey(key string) string {
	var b strings.Builder
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	result := b.String()
	if result == "" {
		return "default"
	}
	return result
}

// Load loads a plan for the given session key
func (pm *PlanManager) Load(sessionKey string) (*Plan, error) {
	pm.mu.RLock()
	if plan, ok := pm.plans[sessionKey]; ok {
		pm.mu.RUnlock()
		return plan, nil
	}
	pm.mu.RUnlock()

	path := pm.planPath(sessionKey)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No plan exists
		}
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan file: %w", err)
	}

	pm.mu.Lock()
	pm.plans[sessionKey] = &plan
	pm.mu.Unlock()

	return &plan, nil
}

// Save saves a plan for the given session key
func (pm *PlanManager) Save(sessionKey string, plan *Plan) error {
	if plan == nil {
		return nil
	}

	plan.UpdatedAt = time.Now()

	path := pm.planPath(sessionKey)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create plan directory: %w", err)
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}

	pm.mu.Lock()
	pm.plans[sessionKey] = plan
	pm.mu.Unlock()

	return nil
}

// Exists checks if a plan exists for the given session key
func (pm *PlanManager) Exists(sessionKey string) bool {
	pm.mu.RLock()
	_, cached := pm.plans[sessionKey]
	pm.mu.RUnlock()
	if cached {
		return true
	}

	path := pm.planPath(sessionKey)
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes a plan for the given session key
func (pm *PlanManager) Delete(sessionKey string) error {
	pm.mu.Lock()
	delete(pm.plans, sessionKey)
	pm.mu.Unlock()

	path := pm.planPath(sessionKey)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CreatePlan creates a new plan for a goal
func CreatePlan(goal string) *Plan {
	now := time.Now()
	return &Plan{
		ID:               generatePlanID(),
		Goal:             goal,
		Status:           PlanStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps:            []*Step{},
		CurrentStepIndex: 0,
		IterationCount:   0,
	}
}

func generatePlanID() string {
	return fmt.Sprintf("plan_%d", time.Now().UnixMilli())
}
