# Planning System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 maxclaw Agent 实现基于文件的规划系统，支持任务步骤跟踪、中断恢复和"继续"指令。

**Architecture:** 在 `internal/agent` 新增 Plan 模块，与 AgentLoop 集成。Plan 存储在每个 session 目录的 `plan.json` 文件中，Agent 自主管理生命周期（创建、步骤推进、暂停/恢复）。

**Tech Stack:** Go 1.21, 现有 session 和 agent loop 架构

---

### Task 1: Create Plan Data Structures

**Files:**
- Create: `internal/agent/plan.go`
- Test: `internal/agent/plan_test.go`

**Step 1: Write the Plan struct definitions**

```go
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
```

**Step 2: Write PlanManager struct**

```go
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
```

**Step 3: Write tests for basic operations**

```go
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
```

**Step 4: Run tests**

```bash
cd /Users/lua/git/nanobot-go
go test ./internal/agent -run TestPlan_BasicOperations -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/plan.go internal/agent/plan_test.go
git commit -m "feat(planning): add Plan data structures and basic operations"
```

---

### Task 2: Implement Plan Persistence

**Files:**
- Modify: `internal/agent/plan.go`
- Test: `internal/agent/plan_test.go`

**Step 1: Add Load and Save methods**

```go
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
```

**Step 2: Add CreatePlan helper function**

```go
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
```

**Step 3: Write persistence tests**

```go
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
```

**Step 4: Run tests**

```bash
go test ./internal/agent -run TestPlanManager -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/plan.go internal/agent/plan_test.go
git commit -m "feat(planning): add plan persistence with Load/Save/Exists/Delete"
```

---

### Task 3: Implement Step Completion Detection

**Files:**
- Modify: `internal/agent/plan.go`
- Test: `internal/agent/plan_test.go`

**Step 1: Add step detection logic**

```go
// StepDetector handles detecting when a step is completed
type StepDetector struct {
    transitionWords      []string
    maxIterationsPerStep int
}

// NewStepDetector creates a new StepDetector with default settings
func NewStepDetector() *StepDetector {
    return &StepDetector{
        transitionWords: []string{
            "现在", "接下来", "然后", "继续", "开始", "现在让我",
            "next", "now", "then", "continue", "let me", "proceed",
        },
        maxIterationsPerStep: 10,
    }
}

// DetectCompletion analyzes LLM output to determine if current step is complete
func (sd *StepDetector) DetectCompletion(llmOutput string, iterationInStep int) bool {
    // Strategy 1: Transition word detection (requires at least 2 iterations)
    if iterationInStep >= 2 {
        lowerOutput := strings.ToLower(llmOutput)
        for _, word := range sd.transitionWords {
            if strings.Contains(lowerOutput, strings.ToLower(word)) {
                return true
            }
        }
    }

    // Strategy 2: Timeout fallback
    if iterationInStep >= sd.maxIterationsPerStep {
        return true
    }

    return false
}

// ExtractStepDeclarations extracts new step declarations from LLM output
func ExtractStepDeclarations(content string) []string {
    var steps []string
    lines := strings.Split(content, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        // Match [Step] description or similar patterns
        if strings.HasPrefix(line, "[Step]") {
            desc := strings.TrimSpace(strings.TrimPrefix(line, "[Step]"))
            if desc != "" {
                steps = append(steps, desc)
            }
        }
    }
    return steps
}
```

**Step 2: Add Continue Intent detection**

```go
// IsContinueIntent detects if the user wants to continue a paused task
func IsContinueIntent(content string) bool {
    content = strings.ToLower(strings.TrimSpace(content))

    continuePatterns := []string{
        "继续", "continue", "go on", "proceed",
        "继续执行", "resume", "继续任务",
    }

    for _, pattern := range continuePatterns {
        if content == pattern || strings.Contains(content, pattern) {
            return true
        }
    }

    return false
}
```

**Step 3: Write detection tests**

```go
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
```

**Step 4: Run tests**

```bash
go test ./internal/agent -run "TestStepDetector|TestExtractStepDeclarations|TestIsContinueIntent" -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/plan.go internal/agent/plan_test.go
git commit -m "feat(planning): add step completion detection and continue intent recognition"
```

---

### Task 4: Add Progress Summary Generation

**Files:**
- Modify: `internal/agent/plan.go`

**Step 1: Add summary generation method**

```go
// GenerateProgressSummary creates a human-readable progress summary
func (p *Plan) GenerateProgressSummary() string {
    var b strings.Builder

    b.WriteString(fmt.Sprintf("当前任务: %s\n", p.Goal))

    completedSteps := 0
    for _, step := range p.Steps {
        if step.Status == StepStatusCompleted {
            completedSteps++
        }
    }

    totalSteps := len(p.Steps)
    if totalSteps > 0 {
        b.WriteString(fmt.Sprintf("进度: %d/%d 步已完成\n", completedSteps, totalSteps))
    } else {
        b.WriteString("进度: 计划中...\n")
    }

    if currentStep := p.CurrentStep(); currentStep != nil {
        progress := ""
        if currentStep.Progress != nil && currentStep.Progress.Total > 0 {
            progress = fmt.Sprintf(" (%d/%d)", currentStep.Progress.Current, currentStep.Progress.Total)
        }
        b.WriteString(fmt.Sprintf("当前步骤: %s%s (进行中)\n", currentStep.Description, progress))
    }

    b.WriteString(fmt.Sprintf("已用迭代: %d\n", p.IterationCount))

    if len(p.Steps) > 0 {
        b.WriteString("\n历史步骤:\n")
        for i, step := range p.Steps {
            status := "⏸"
            if step.Status == StepStatusCompleted {
                status = "✓"
            } else if step.Status == StepStatusRunning {
                status = "⏳"
            }
            b.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, status, step.Description))
        }
    }

    return b.String()
}

// ToContextString returns a concise version for LLM context
func (p *Plan) ToContextString() string {
    var b strings.Builder
    b.WriteString(fmt.Sprintf("当前任务: %s\n", p.Goal))

    if currentStep := p.CurrentStep(); currentStep != nil {
        progress := ""
        if currentStep.Progress != nil && currentStep.Progress.Total > 0 {
            progress = fmt.Sprintf(" (%d/%d)", currentStep.Progress.Current, currentStep.Progress.Total)
        }
        b.WriteString(fmt.Sprintf("当前步骤 (%d/%d): %s%s\n",
            p.CurrentStepIndex+1, len(p.Steps), currentStep.Description, progress))
    } else if len(p.Steps) > 0 {
        b.WriteString("所有步骤已完成\n")
    } else {
        b.WriteString("步骤规划中...\n")
    }

    return b.String()
}
```

**Step 2: Write tests**

```go
func TestPlan_GenerateProgressSummary(t *testing.T) {
    plan := &Plan{
        ID:               "plan_test",
        Goal:             "下载分析腾讯财报",
        Status:           PlanStatusRunning,
        CurrentStepIndex: 1,
        IterationCount:   15,
        Steps: []*Step{
            {
                ID:          "step_1",
                Description: "搜索PDF链接",
                Status:      StepStatusCompleted,
                Result:      "找到5个链接",
            },
            {
                ID:          "step_2",
                Description: "下载PDF文件",
                Status:      StepStatusRunning,
                Progress:    &Progress{Current: 2, Total: 5},
            },
            {
                ID:          "step_3",
                Description: "提取数据",
                Status:      StepStatusPending,
            },
        },
    }

    summary := plan.GenerateProgressSummary()

    if !strings.Contains(summary, "下载分析腾讯财报") {
        t.Error("summary should contain goal")
    }
    if !strings.Contains(summary, "1/3 步已完成") {
        t.Error("summary should show correct progress")
    }
    if !strings.Contains(summary, "下载PDF文件") {
        t.Error("summary should show current step")
    }
    if !strings.Contains(summary, "(2/5)") {
        t.Error("summary should show step progress")
    }
}
```

**Step 3: Run tests**

```bash
go test ./internal/agent -run TestPlan_GenerateProgressSummary -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/plan.go internal/agent/plan_test.go
git commit -m "feat(planning): add progress summary generation"
```

---

### Task 5: Integrate PlanManager into AgentLoop

**Files:**
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/context.go`

**Step 1: Add PlanManager to AgentLoop**

```go
// In internal/agent/loop.go, add to AgentLoop struct:
type AgentLoop struct {
    // ... existing fields ...
    PlanManager *PlanManager  // Add this field
}

// In NewAgentLoop function, add:
func NewAgentLoop(...) *AgentLoop {
    // ... existing initialization ...
    loop := &AgentLoop{
        // ... existing fields ...
        PlanManager: NewPlanManager(workspace),  // Add this
    }
    // ... rest of initialization ...
}
```

**Step 2: Add plan-aware system prompt**

```go
// In internal/agent/context.go, add method:
func (cb *ContextBuilder) BuildSystemPromptWithPlan(plan *Plan) string {
    basePrompt := cb.BuildSystemPrompt() // existing method

    if plan == nil {
        return basePrompt
    }

    planContext := "\n\n## 当前任务规划\n\n"
    planContext += plan.ToContextString()
    planContext += "\n你可以使用 [Step] 描述 格式声明新步骤。\n"
    planContext += "完成当前步骤后，系统会自动推进到下一步。\n"

    return basePrompt + planContext
}
```

**Step 3: Modify BuildMessages to include plan context**

```go
// In internal/agent/context.go, modify or add:
func (cb *ContextBuilder) BuildMessagesWithPlanAndSkillRefs(
    history []providers.Message,
    userContent string,
    skillRefs []string,
    media []bus.MediaAttachment,
    channel, chatID string,
    plan *Plan,
) []providers.Message {
    // Get base system prompt with plan
    systemPrompt := cb.BuildSystemPromptWithPlan(plan)

    // ... rest similar to existing BuildMessagesWithSkillRefs ...
}
```

**Step 4: Commit**

```bash
git add internal/agent/loop.go internal/agent/context.go
git commit -m "feat(planning): integrate PlanManager into AgentLoop"
```

---

### Task 6: Implement Plan Lifecycle in ProcessMessage

**Files:**
- Modify: `internal/agent/loop.go`

**Step 1: Add plan creation on first tool call**

```go
func (a *AgentLoop) processMessageWithIC(...) (*bus.OutboundMessage, error) {
    // ... existing setup ...

    sess := a.sessions.GetOrCreate(msg.SessionKey)

    // Check for "continue" intent and handle paused plan
    plan, _ := a.PlanManager.Load(msg.SessionKey)
    if plan != nil && plan.Status == PlanStatusPaused && IsContinueIntent(msg.Content) {
        plan.Status = PlanStatusRunning
        a.PlanManager.Save(msg.SessionKey, plan)

        // Inject plan summary into user message
        summary := plan.GenerateProgressSummary()
        msg.Content = fmt.Sprintf("[恢复任务]\n%s\n\n用户指令: %s", summary, msg.Content)
    }

    // ... existing slash command handling ...

    // Persist user input
    sess.AddMessage("user", msg.Content)
    a.sessions.Save(sess)

    // Get history
    history := a.convertSessionMessages(sess.GetHistory(sessionContextWindow))

    // Build messages with plan context if exists
    selectedSkillRefs := normalizeSkillRefs(msg.SelectedSkills)
    var messages []providers.Message
    if plan != nil && plan.Status == PlanStatusRunning {
        messages = a.context.BuildMessagesWithPlanAndSkillRefs(history, msg.Content, selectedSkillRefs, msg.Media, msg.Channel, msg.ChatID, plan)
    } else {
        messages = a.context.BuildMessagesWithSkillRefs(history, msg.Content, selectedSkillRefs, msg.Media, msg.Channel, msg.ChatID)
    }

    // ... rest of the loop ...
}
```

**Step 2: Add plan management inside the iteration loop**

```go
stepDetector := NewStepDetector()
iterationsInCurrentStep := 0

for i := 0; i < a.MaxIterations; i++ {
    iteration := i + 1

    // Check for interruption
    select {
    case <-ic.Done():
        return nil, ctx.Err()
    default:
    }

    emitEvent(StreamEvent{
        Type:      "status",
        Iteration: iteration,
        Message:   fmt.Sprintf("Iteration %d", iteration),
    })

    // Stream LLM call
    // ... existing streaming code ...

    content := handler.GetContent()
    toolCalls := handler.GetToolCalls()

    // Create plan on first tool call if not exists
    if plan == nil && len(toolCalls) > 0 && i == 0 {
        plan = CreatePlan(msg.Content)
        a.PlanManager.Save(msg.SessionKey, plan)

        // Rebuild messages with plan context
        messages = a.context.BuildMessagesWithPlanAndSkillRefs(history, msg.Content, selectedSkillRefs, msg.Media, msg.Channel, msg.ChatID, plan)
    }

    // Process tool calls
    if len(toolCalls) > 0 {
        // ... existing tool execution ...

        // After tool execution, update plan
        if plan != nil && plan.Status == PlanStatusRunning {
            plan.IterationCount++

            // Check for step declarations in LLM output
            newSteps := ExtractStepDeclarations(content)
            for _, desc := range newSteps {
                plan.AddStep(desc)
            }

            // Check if current step is complete
            if stepDetector.DetectCompletion(content, iterationsInCurrentStep) {
                result := summarizeTimeline(timeline)
                plan.CompleteCurrentStep(result)
                iterationsInCurrentStep = 0
            } else {
                iterationsInCurrentStep++
            }

            a.PlanManager.Save(msg.SessionKey, plan)
        }
    } else {
        // No tool calls, task might be complete
        finalContent = content
        if plan != nil && plan.Status == PlanStatusRunning {
            plan.Status = PlanStatusCompleted
            a.PlanManager.Save(msg.SessionKey, plan)
        }
        break
    }
}
```

**Step 3: Add iteration limit handling**

```go
if finalContent == "" {
    if maxIterationReached {
        finalContent = fmt.Sprintf("Reached %d iterations without completion.", a.MaxIterations)

        // Pause plan if exists
        if plan != nil && plan.Status == PlanStatusRunning {
            plan.Status = PlanStatusPaused
            a.PlanManager.Save(msg.SessionKey, plan)

            summary := plan.GenerateProgressSummary()
            finalContent += fmt.Sprintf("\n\n%s\n\n输入'继续'以恢复执行。", summary)
        }
    } else {
        finalContent = "I've completed processing but have no response to give."
    }
}
```

**Step 4: Add helper function for timeline summarization**

```go
func summarizeTimeline(timeline []session.TimelineEntry) string {
    var summaries []string
    for _, entry := range timeline {
        if entry.Kind == "activity" && entry.Activity != nil {
            if entry.Activity.Type == "tool_result" {
                summaries = append(summaries, entry.Activity.Summary)
            }
        }
    }
    // Return last 3 tool results as summary
    if len(summaries) > 3 {
        summaries = summaries[len(summaries)-3:]
    }
    return strings.Join(summaries, "; ")
}
```

**Step 5: Run build to check for errors**

```bash
cd /Users/lua/git/nanobot-go
go build ./...
```

Expected: SUCCESS

**Step 6: Commit**

```bash
git add internal/agent/loop.go
git commit -m "feat(planning): implement plan lifecycle in ProcessMessage"
```

---

### Task 7: Add System Prompt Integration

**Files:**
- Modify: `internal/agent/context.go`

**Step 1: Ensure BuildSystemPromptWithPlan is properly integrated**

The method was added in Task 5, now ensure it's complete:

```go
// BuildSystemPromptWithPlan creates system prompt with plan context
func (cb *ContextBuilder) BuildSystemPromptWithPlan(plan *Plan) string {
    basePrompt := cb.buildBaseSystemPrompt()

    if plan == nil {
        return basePrompt
    }

    var b strings.Builder
    b.WriteString(basePrompt)
    b.WriteString("\n\n## 当前任务规划\n\n")
    b.WriteString(plan.ToContextString())
    b.WriteString("\n")
    b.WriteString("你可以使用 [Step] 描述 格式声明新步骤。\n")
    b.WriteString("当当前步骤目标达成时，系统会自动推进到下一步。\n")

    return b.String()
}

// Rename existing BuildSystemPrompt to buildBaseSystemPrompt
// and create wrapper that doesn't include plan
func (cb *ContextBuilder) buildBaseSystemPrompt() string {
    // Move existing BuildSystemPrompt logic here
    // ...
}
```

**Step 2: Run tests**

```bash
go test ./internal/agent -v
```

Expected: PASS (or at least no new failures)

**Step 3: Commit**

```bash
git add internal/agent/context.go
git commit -m "feat(planning): integrate plan context into system prompt"
```

---

### Task 8: Integration Testing

**Files:**
- Test: `internal/agent/loop_test.go` (existing)

**Step 1: Add plan integration test**

```go
func TestAgentLoop_PlanLifecycle(t *testing.T) {
    // This is a basic smoke test - full integration test would require
    // mocking the LLM provider

    tmpDir := t.TempDir()

    // Create test setup
    messageBus := bus.NewMessageBus()
    provider := &mockProvider{} // You'd need to implement this

    loop := NewAgentLoop(
        messageBus,
        provider,
        tmpDir,
        "test-model",
        20,
        "",
        tools.WebFetchOptions{},
        config.ExecToolConfig{Timeout: 60},
        false,
        nil,
        nil,
        false,
    )

    // Test plan creation
    sessionKey := "test:123"
    plan := CreatePlan("test goal")
    plan.AddStep("step 1")
    plan.Steps[0].Status = StepStatusRunning
    now := time.Now()
    plan.Steps[0].StartedAt = &now

    err := loop.PlanManager.Save(sessionKey, plan)
    if err != nil {
        t.Fatalf("failed to save plan: %v", err)
    }

    // Verify plan can be loaded
    loaded, err := loop.PlanManager.Load(sessionKey)
    if err != nil {
        t.Fatalf("failed to load plan: %v", err)
    }

    if loaded.Goal != "test goal" {
        t.Errorf("expected goal 'test goal', got %s", loaded.Goal)
    }
}
```

**Step 2: Run the test**

```bash
go test ./internal/agent -run TestAgentLoop_PlanLifecycle -v
```

Expected: PASS (may need to adjust based on actual test setup)

**Step 3: Run all tests**

```bash
go test ./... -short
```

Expected: All tests pass (or at least no new failures from planning changes)

**Step 4: Commit**

```bash
git add internal/agent/loop_test.go
git commit -m "test(planning): add plan lifecycle integration test"
```

---

### Task 9: Manual Testing Script

**Files:**
- Create: `e2e_test/planning_test.sh`

**Step 1: Create manual test script**

```bash
#!/bin/bash
# e2e_test/planning_test.sh - Manual testing for planning system

set -e

echo "=== Planning System Manual Test ==="
echo ""
echo "Prerequisites:"
echo "1. Build maxclaw: make build"
echo "2. Start gateway: maxclaw gateway"
echo "3. Open web UI or use CLI"
echo ""

echo "Test 1: Basic Planning"
echo "----------------------"
echo "Input: '下载分析腾讯最近5年财报'"
echo "Expected:"
echo "- Plan created in ~/.maxclaw/workspace/.sessions/{session}/plan.json"
echo "- Progress shown in responses"
echo "- Steps auto-advance"
echo ""

echo "Test 2: Pause and Resume"
echo "-------------------------"
echo "Input: (a task that exceeds 20 iterations)"
echo "Expected:"
echo "- Plan status becomes 'paused'"
echo "- Response shows progress summary"
echo "- Input '继续' resumes from where left off"
echo ""

echo "Test 3: Step Declaration"
echo "-------------------------"
echo "Input: '帮我完成一个复杂任务'"
echo "LLM Output containing: '[Step] 第一步描述'"
echo "Expected:"
echo "- New step added to plan.json"
echo "- Step appears in progress summary"
echo ""

echo "Verification commands:"
echo "cat ~/.maxclaw/workspace/.sessions/desktop_*/plan.json"
echo ""
```

**Step 2: Make executable and commit**

```bash
chmod +x e2e_test/planning_test.sh
git add e2e_test/planning_test.sh
git commit -m "test(planning): add manual testing script"
```

---

### Task 10: Documentation Update

**Files:**
- Create: `docs/planning.md`

**Step 1: Write user-facing documentation**

```markdown
# Task Planning System

maxclaw now supports automatic task planning for complex multi-step tasks.

## How It Works

When you give maxclaw a complex task that requires multiple steps (like downloading files, analyzing data, and creating charts), it will:

1. **Automatically create a plan** - The task is broken into manageable steps
2. **Track progress** - Each step's status is tracked and saved
3. **Handle interruptions** - If the task exceeds iteration limits, it pauses gracefully
4. **Resume on "continue"** - Type "继续" or "continue" to resume from where it left off

## Usage

### Normal Flow

Simply ask maxclaw to perform a complex task:

```
下载分析腾讯最近5年年度财报PDF，提取关键财务指标，制作成图表
```

maxclaw will:
- Create a plan with steps like "搜索财报链接", "下载PDF文件", "提取数据", "制作图表"
- Execute each step automatically
- Show progress as it works

### Pause and Resume

If a task is very complex and exceeds the iteration limit:

```
任务执行中（2/5 步已完成）。输入'继续'以恢复执行。
```

Simply type:

```
继续
```

maxclaw will resume from the exact step where it left off.

### Declaring Steps

The agent can declare new steps during execution using the `[Step]` syntax:

```
[Step] 下载2024年财报
[Step] 提取收入数据
```

These steps will be tracked in the plan.

## Plan File Location

Plans are stored per-session at:

```
~/.maxclaw/workspace/.sessions/{session_key}/plan.json
```

You can inspect this file to see the current plan state.

## Plan Status

- `running` - Task is actively executing
- `paused` - Task exceeded iteration limit, waiting for "continue"
- `completed` - All steps finished successfully
- `failed` - Task encountered an error
```

**Step 2: Commit**

```bash
git add docs/planning.md
git commit -m "docs(planning): add user-facing documentation"
```

---

### Task 11: Final Build and Verification

**Step 1: Run full build**

```bash
make build
```

Expected: Build successful

**Step 2: Run all tests**

```bash
make test
```

Expected: All tests pass

**Step 3: Run linting**

```bash
make lint
```

Expected: No errors

**Step 4: Update CHANGELOG**

```bash
# Append to CHANGELOG.md under ## [Unreleased]
cat >> CHANGELOG.md << 'EOF'

### Added
- File-based planning system for complex multi-step tasks
  - Auto-create plan on first tool call
  - Step tracking with progress indicators
  - Pause/resume on iteration limit with "继续" command
  - Plan persisted per-session in ~/.maxclaw/workspace/.sessions/{session}/plan.json
EOF
```

**Step 5: Final commit**

```bash
git add CHANGELOG.md
git commit -m "chore: update changelog for planning system"
```

---

## Summary

This implementation adds a complete file-based planning system to maxclaw:

1. **Core structures** (`plan.go`): Plan, Step, PlanManager with CRUD operations
2. **Detection logic**: Step completion detection via transition words + timeout
3. **Continue intent**: Recognizes "继续"/"continue" to resume paused plans
4. **Integration**: Seamlessly integrated into AgentLoop's ProcessMessage
5. **Persistence**: Plans saved per-session, survive restarts
6. **User experience**: Progress shown in responses, graceful pause/resume

The system is automatic - no user configuration needed. Complex tasks get planning automatically, simple tasks work as before.
