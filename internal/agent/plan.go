package agent

import (
    "fmt"
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
