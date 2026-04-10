package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/memory"
)

// FeedbackLesson represents a learned lesson from user feedback
type FeedbackLesson struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	TaskType      string    `json:"task_type"`
	IssueType     string    `json:"issue_type"` // understanding/implementation/style/omission
	OriginalApproach string `json:"original_approach"`
	Correction    string    `json:"correction"`
	Lesson        string    `json:"lesson"`
	Occurrences   int       `json:"occurrences"`
	LastApplied   time.Time `json:"last_applied,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
}

// FeedbackLearner learns from user feedback and applies lessons to future tasks
type FeedbackLearner struct {
	workspace string
	mu        sync.RWMutex

	lessons      map[string]*FeedbackLesson
	userPatterns map[string]int // Pattern -> occurrence count

	// Auto-save
	autoSave     bool
	lastSave     time.Time
	saveInterval time.Duration
}

// NewFeedbackLearner creates a new feedback learner
func NewFeedbackLearner(workspace string) *FeedbackLearner {
	fl := &FeedbackLearner{
		workspace:    workspace,
		lessons:      make(map[string]*FeedbackLesson),
		userPatterns: make(map[string]int),
		autoSave:     true,
		saveInterval: 5 * time.Minute,
	}

	// Load existing lessons
	fl.load()

	return fl
}

// RecordFeedback records user feedback and extracts lesson
func (fl *FeedbackLearner) RecordFeedback(
	result *FeedbackResult,
	taskContext string,
	agentOutput string,
	userFeedback string,
) *FeedbackLesson {
	if result == nil || result.Type == FeedbackPositive || result.Type == FeedbackNeutral {
		return nil
	}

	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Extract task type from context
	taskType := fl.extractTaskType(taskContext)

	// Generate lesson
	lesson := fl.generateLesson(result, agentOutput, userFeedback)

	// Create lesson key for deduplication
	key := fl.generateLessonKey(taskType, result.IssueType, lesson)

	if existing, ok := fl.lessons[key]; ok {
		// Update existing lesson
		existing.Occurrences++
		existing.LastApplied = time.Now()
		fl.autoSaveIfNeeded()
		return existing
	}

	// Create new lesson
	newLesson := &FeedbackLesson{
		ID:               generateLessonID(),
		Timestamp:        time.Now(),
		TaskType:         taskType,
		IssueType:        result.IssueType,
		OriginalApproach: agentOutput,
		Correction:       userFeedback,
		Lesson:           lesson,
		Occurrences:      1,
		Tags:             fl.extractTags(taskContext, result),
	}

	fl.lessons[key] = newLesson

	// Also update user patterns
	fl.updateUserPatterns(lesson)

	// Persist to memory
	fl.persistToMemory(newLesson)

	fl.autoSaveIfNeeded()

	return newLesson
}

// GetRelevantLessons retrieves lessons relevant to a task
func (fl *FeedbackLearner) GetRelevantLessons(taskType string, maxResults int) []*FeedbackLesson {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	var relevant []*FeedbackLesson

	for _, lesson := range fl.lessons {
		// Match by task type or tags
		if fl.matchesTaskType(lesson.TaskType, taskType) ||
			fl.hasMatchingTags(lesson.Tags, taskType) {
			relevant = append(relevant, lesson)
		}
	}

	// Sort by relevance (occurrences, then recency)
	fl.sortByRelevance(relevant)

	if len(relevant) > maxResults {
		return relevant[:maxResults]
	}
	return relevant
}

// BuildSystemPromptEnhancement generates prompt text from relevant lessons
func (fl *FeedbackLearner) BuildSystemPromptEnhancement(taskType string) string {
	lessons := fl.GetRelevantLessons(taskType, 3)
	if len(lessons) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "\n[Previous Feedback Lessons]")

	for i, lesson := range lessons {
		parts = append(parts, fmt.Sprintf("%d. %s (%d times): %s",
			i+1,
			lesson.IssueType,
			lesson.Occurrences,
			lesson.Lesson,
		))
	}

	return strings.Join(parts, "\n")
}

// GetUserPreference retrieves a learned user preference
func (fl *FeedbackLearner) GetUserPreference(key string) (string, bool) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	if val, ok := fl.userPatterns[key]; ok && val > 1 {
		return "confirmed", true
	}
	return "", false
}

// GetStats returns learning statistics
func (fl *FeedbackLearner) GetStats() map[string]interface{} {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	issueTypeCounts := make(map[string]int)
	for _, lesson := range fl.lessons {
		issueTypeCounts[lesson.IssueType]++
	}

	return map[string]interface{}{
		"total_lessons":     len(fl.lessons),
		"user_patterns":     len(fl.userPatterns),
		"issue_breakdown":   issueTypeCounts,
		"last_save":         fl.lastSave,
	}
}

// Extract lesson methods

func (fl *FeedbackLearner) generateLesson(result *FeedbackResult, agentOutput, userFeedback string) string {
	switch result.IssueType {
	case "implementation":
		return fl.extractImplementationLesson(agentOutput, userFeedback)
	case "style":
		return fl.extractStyleLesson(userFeedback)
	case "understanding":
		return fl.extractUnderstandingLesson(userFeedback)
	case "omission":
		return fl.extractOmissionLesson(userFeedback)
	default:
		return fl.extractGenericLesson(userFeedback)
	}
}

func (fl *FeedbackLearner) extractImplementationLesson(agentOutput, userFeedback string) string {
	// Extract what approach user wants instead
	
	// Pattern: "应该用 X 而不是 Y" / "should use X instead of Y"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(应该|建议|should|use|用).{0,20}(instead of|rather than|而不是|而非).{0,20}`),
		regexp.MustCompile(`(?i)(改成|改为|change|switch).{0,15}(to|成|为)`),
	}

	for _, pattern := range patterns {
		if match := pattern.FindString(userFeedback); match != "" {
			return fmt.Sprintf("Prefer %s", match)
		}
	}

	// Generic extraction
	if strings.Contains(strings.ToLower(userFeedback), "promise") &&
		strings.Contains(strings.ToLower(userFeedback), "async") {
		return "User prefers async/await over callbacks for async operations"
	}

	if strings.Contains(strings.ToLower(userFeedback), "loop") ||
		strings.Contains(userFeedback, "循环") {
		return "User has specific preference for iteration approach"
	}

	return "User prefers different implementation approach"
}

func (fl *FeedbackLearner) extractStyleLesson(userFeedback string) string {
	// Code style preferences
	if strings.Contains(userFeedback, "命名") || strings.Contains(strings.ToLower(userFeedback), "name") {
		return "User cares about naming conventions"
	}

	if strings.Contains(userFeedback, "注释") || strings.Contains(strings.ToLower(userFeedback), "comment") {
		return "User prefers comprehensive comments"
	}

	if strings.Contains(userFeedback, "格式") || strings.Contains(strings.ToLower(userFeedback), "format") {
		return "User has specific formatting preferences"
	}

	return "User has specific code style preference"
}

func (fl *FeedbackLearner) extractUnderstandingLesson(userFeedback string) string {
	if strings.Contains(userFeedback, "意思") || strings.Contains(strings.ToLower(userFeedback), "mean") {
		return "Need to clarify user intent before implementation"
	}

	if strings.Contains(userFeedback, "理解") || strings.Contains(strings.ToLower(userFeedback), "understand") {
		return "Confirm understanding of requirements before proceeding"
	}

	return "User's intent may differ from initial request"
}

func (fl *FeedbackLearner) extractOmissionLesson(userFeedback string) string {
	if strings.Contains(userFeedback, "漏了") || strings.Contains(userFeedback, "少了") ||
		strings.Contains(strings.ToLower(userFeedback), "missing") ||
		strings.Contains(strings.ToLower(userFeedback), "forgot") {
		return "Check for missing requirements or edge cases"
	}

	return "User expects more comprehensive coverage"
}

func (fl *FeedbackLearner) extractGenericLesson(userFeedback string) string {
	// Extract key phrases
	keyPhrases := []string{
		"not correct", "wrong", "不对", "错了",
		"should be", "应该是", "应该是",
		"prefer", "更喜欢", "偏好",
	}

	for _, phrase := range keyPhrases {
		if strings.Contains(strings.ToLower(userFeedback), phrase) {
			return fmt.Sprintf("User indicated: %s", phrase)
		}
	}

	return "User provided feedback on approach"
}

// Helper methods

func (fl *FeedbackLearner) extractTaskType(taskContext string) string {
	// Simple task type extraction
	taskTypes := map[string]string{
		"refactor":   "refactoring",
		"重构":       "refactoring",
		"debug":      "debugging",
		"调试":       "debugging",
		"test":       "testing",
		"测试":       "testing",
		"implement":  "implementation",
		"实现":       "implementation",
		"review":     "code review",
		"optimize":   "optimization",
		"优化":       "optimization",
	}

	lower := strings.ToLower(taskContext)
	for key, taskType := range taskTypes {
		if strings.Contains(lower, key) {
			return taskType
		}
	}

	return "general"
}

func (fl *FeedbackLearner) generateLessonKey(taskType, issueType, lesson string) string {
	// Simplified key for deduplication
	simplified := strings.ToLower(lesson)
	simplified = strings.ReplaceAll(simplified, " ", "_")
	if len(simplified) > 50 {
		simplified = simplified[:50]
	}
	return fmt.Sprintf("%s:%s:%s", taskType, issueType, simplified)
}

func (fl *FeedbackLearner) extractTags(taskContext string, result *FeedbackResult) []string {
	var tags []string

	// Language tags
	languages := []string{"javascript", "typescript", "python", "go", "rust", "java", "cpp"}
	for _, lang := range languages {
		if strings.Contains(strings.ToLower(taskContext), lang) {
			tags = append(tags, lang)
		}
	}

	// Technology tags
	techs := []string{"react", "vue", "angular", "node", "docker", "kubernetes"}
	for _, tech := range techs {
		if strings.Contains(strings.ToLower(taskContext), tech) {
			tags = append(tags, tech)
		}
	}

	return tags
}

func (fl *FeedbackLearner) updateUserPatterns(lesson string) {
	// Extract user preferences from lesson
	patterns := map[string]string{
		"async/await":          "user_prefers_async_await",
		"promise":              "user_prefers_promises",
		"functional":           "user_prefers_functional",
		"oop":                  "user_prefers_oop",
		"typescript":           "user_prefers_typescript",
		"strict":               "user_prefers_strict_typing",
		"parallel":             "user_prefers_parallel_processing",
		"concurrent":           "user_prefers_concurrency",
	}

	lower := strings.ToLower(lesson)
	for keyword, pattern := range patterns {
		if strings.Contains(lower, keyword) {
			fl.userPatterns[pattern]++
		}
	}
}

func (fl *FeedbackLearner) matchesTaskType(lessonTaskType, currentTaskType string) bool {
	if lessonTaskType == currentTaskType {
		return true
	}
	// Fuzzy match
	return strings.Contains(lessonTaskType, currentTaskType) ||
		strings.Contains(currentTaskType, lessonTaskType)
}

func (fl *FeedbackLearner) hasMatchingTags(lessonTags []string, taskContext string) bool {
	for _, tag := range lessonTags {
		if strings.Contains(strings.ToLower(taskContext), tag) {
			return true
		}
	}
	return false
}

func (fl *FeedbackLearner) sortByRelevance(lessons []*FeedbackLesson) {
	// Sort by occurrences (desc), then recency (desc)
	for i := 0; i < len(lessons)-1; i++ {
		for j := i + 1; j < len(lessons); j++ {
			if lessons[i].Occurrences < lessons[j].Occurrences ||
				(lessons[i].Occurrences == lessons[j].Occurrences &&
					lessons[i].Timestamp.Before(lessons[j].Timestamp)) {
				lessons[i], lessons[j] = lessons[j], lessons[i]
			}
		}
	}
}

// Persistence methods

func (fl *FeedbackLearner) persistToMemory(lesson *FeedbackLesson) {
	// Write to MEMORY.md in the workspace
	store := memory.NewStore(fl.workspace)

	entry := fmt.Sprintf("\n## User Feedback Lesson [%s]\n\n"+
		"- **Task Type**: %s\n"+
		"- **Issue Type**: %s\n"+
		"- **Lesson**: %s\n"+
		"- **Occurrences**: %d\n"+
		"- **Learned**: %s\n",
		lesson.ID[:8],
		lesson.TaskType,
		lesson.IssueType,
		lesson.Lesson,
		lesson.Occurrences,
		lesson.Timestamp.Format("2006-01-02"),
	)

	// Append to memory
	content, _ := store.ReadLongTerm()
	content += entry
	store.WriteLongTerm(content)
}

func (fl *FeedbackLearner) autoSaveIfNeeded() {
	if !fl.autoSave || time.Since(fl.lastSave) < fl.saveInterval {
		return
	}

	fl.save()
	fl.lastSave = time.Now()
}

func (fl *FeedbackLearner) save() error {
	data := struct {
		Lessons      map[string]*FeedbackLesson `json:"lessons"`
		UserPatterns map[string]int             `json:"user_patterns"`
		SavedAt      time.Time                  `json:"saved_at"`
	}{
		Lessons:      fl.lessons,
		UserPatterns: fl.userPatterns,
		SavedAt:      time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(fl.workspace, ".feedback", "lessons.json")
	os.MkdirAll(filepath.Dir(path), 0755)

	return os.WriteFile(path, jsonData, 0644)
}

func (fl *FeedbackLearner) load() error {
	path := filepath.Join(fl.workspace, ".feedback", "lessons.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing data
		}
		return err
	}

	var saved struct {
		Lessons      map[string]*FeedbackLesson `json:"lessons"`
		UserPatterns map[string]int             `json:"user_patterns"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	fl.lessons = saved.Lessons
	fl.userPatterns = saved.UserPatterns

	return nil
}

func generateLessonID() string {
	return fmt.Sprintf("lesson_%d_%s", time.Now().Unix(), generateRandomID(4))
}

func generateRandomID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
