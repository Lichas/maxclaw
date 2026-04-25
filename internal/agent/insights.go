package agent

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Lichas/maxclaw/internal/session"
)

// ModelUsage tracks usage for a specific model
type ModelUsage struct {
	Model            string  `json:"model"`
	Sessions         int     `json:"sessions"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	ToolCalls        int     `json:"tool_calls"`
	CostUSD          float64 `json:"cost_usd"`
	HasPricing       bool    `json:"has_pricing"`
}

// ToolUsage tracks usage for a specific tool
type ToolUsage struct {
	Tool       string  `json:"tool"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ActivityStats tracks activity patterns
type ActivityStats struct {
	ByDay       []DayStat  `json:"by_day"`
	ByHour      []HourStat `json:"by_hour"`
	BusiestDay  *DayStat   `json:"busiest_day,omitempty"`
	BusiestHour *HourStat  `json:"busiest_hour,omitempty"`
	ActiveDays  int        `json:"active_days"`
	MaxStreak   int        `json:"max_streak"`
}

// DayStat tracks sessions per day of week
type DayStat struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// HourStat tracks sessions per hour
type HourStat struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

// TopSession represents a notable session
type TopSession struct {
	Label     string `json:"label"`
	SessionID string `json:"session_id"`
	Value     string `json:"value"`
	Date      string `json:"date"`
}

// InsightsReport contains the complete insights analysis
type InsightsReport struct {
	Days         int           `json:"days"`
	SourceFilter string        `json:"source_filter,omitempty"`
	Empty        bool          `json:"empty"`
	GeneratedAt  time.Time     `json:"generated_at"`
	Overview     OverviewStats `json:"overview"`
	Models       []ModelUsage  `json:"models"`
	Tools        []ToolUsage   `json:"tools"`
	Activity     ActivityStats `json:"activity"`
	TopSessions  []TopSession  `json:"top_sessions"`
}

// OverviewStats contains high-level statistics
type OverviewStats struct {
	TotalSessions         int        `json:"total_sessions"`
	TotalMessages         int        `json:"total_messages"`
	TotalToolCalls        int        `json:"total_tool_calls"`
	TotalInputTokens      int        `json:"total_input_tokens"`
	TotalOutputTokens     int        `json:"total_output_tokens"`
	TotalCacheReadTokens  int        `json:"total_cache_read_tokens"`
	TotalCacheWriteTokens int        `json:"total_cache_write_tokens"`
	TotalTokens           int        `json:"total_tokens"`
	EstimatedCostUSD      float64    `json:"estimated_cost_usd"`
	ActualCostUSD         float64    `json:"actual_cost_usd,omitempty"`
	TotalHours            float64    `json:"total_hours"`
	AvgSessionDuration    float64    `json:"avg_session_duration"`
	AvgMessagesPerSession float64    `json:"avg_messages_per_session"`
	AvgTokensPerSession   float64    `json:"avg_tokens_per_session"`
	UserMessages          int        `json:"user_messages"`
	AssistantMessages     int        `json:"assistant_messages"`
	ToolMessages          int        `json:"tool_messages"`
	DateRangeStart        *time.Time `json:"date_range_start,omitempty"`
	DateRangeEnd          *time.Time `json:"date_range_end,omitempty"`
}

// SessionData represents a session for insights analysis
type SessionData struct {
	ID               string
	Source           string
	Model            string
	StartedAt        time.Time
	EndedAt          *time.Time
	MessageCount     int
	ToolCallCount    int
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	EstimatedCostUSD float64
	Messages         []session.Message
}

// InsightsEngine analyzes session history and produces usage insights
type InsightsEngine struct {
	sessions []SessionData
}

// NewInsightsEngine creates a new insights engine
func NewInsightsEngine() *InsightsEngine {
	return &InsightsEngine{
		sessions: make([]SessionData, 0),
	}
}

// AddSession adds a session to the engine
func (ie *InsightsEngine) AddSession(data SessionData) {
	ie.sessions = append(ie.sessions, data)
}

// Generate generates a complete insights report
func (ie *InsightsEngine) Generate(days int, source string) *InsightsReport {
	cutoff := time.Now().AddDate(0, 0, -days)

	// Filter sessions
	var filtered []SessionData
	for _, s := range ie.sessions {
		if s.StartedAt.After(cutoff) {
			if source == "" || s.Source == source {
				filtered = append(filtered, s)
			}
		}
	}

	if len(filtered) == 0 {
		return &InsightsReport{
			Days:         days,
			SourceFilter: source,
			Empty:        true,
		}
	}

	report := &InsightsReport{
		Days:         days,
		SourceFilter: source,
		Empty:        false,
		GeneratedAt:  time.Now(),
	}

	// Compute overview
	report.Overview = ie.computeOverview(filtered)

	// Compute model breakdown
	report.Models = ie.computeModelBreakdown(filtered)

	// Compute tool usage
	report.Tools = ie.computeToolBreakdown(filtered)

	// Compute activity patterns
	report.Activity = ie.computeActivityPatterns(filtered)

	// Compute top sessions
	report.TopSessions = ie.computeTopSessions(filtered)

	return report
}

// computeOverview calculates high-level statistics
func (ie *InsightsEngine) computeOverview(sessions []SessionData) OverviewStats {
	var o OverviewStats

	for _, s := range sessions {
		o.TotalSessions++
		o.TotalMessages += s.MessageCount
		o.TotalToolCalls += s.ToolCallCount
		o.TotalInputTokens += s.InputTokens
		o.TotalOutputTokens += s.OutputTokens
		o.TotalCacheReadTokens += s.CacheReadTokens
		o.TotalCacheWriteTokens += s.CacheWriteTokens
		o.EstimatedCostUSD += s.EstimatedCostUSD

		// Count message types
		for _, m := range s.Messages {
			switch m.Role {
			case "user":
				o.UserMessages++
			case "assistant":
				o.AssistantMessages++
			case "tool":
				o.ToolMessages++
			}
		}

		// Duration
		if s.EndedAt != nil {
			duration := s.EndedAt.Sub(s.StartedAt).Seconds()
			o.TotalHours += duration / 3600
		}

		// Date range
		if o.DateRangeStart == nil || s.StartedAt.Before(*o.DateRangeStart) {
			o.DateRangeStart = &s.StartedAt
		}
		if o.DateRangeEnd == nil || s.StartedAt.After(*o.DateRangeEnd) {
			o.DateRangeEnd = &s.StartedAt
		}
	}

	o.TotalTokens = o.TotalInputTokens + o.TotalOutputTokens +
		o.TotalCacheReadTokens + o.TotalCacheWriteTokens

	if o.TotalSessions > 0 {
		o.AvgMessagesPerSession = float64(o.TotalMessages) / float64(o.TotalSessions)
		o.AvgTokensPerSession = float64(o.TotalTokens) / float64(o.TotalSessions)
	}

	if o.TotalHours > 0 {
		o.AvgSessionDuration = o.TotalHours * 3600 / float64(o.TotalSessions)
	}

	return o
}

// computeModelBreakdown calculates usage by model
func (ie *InsightsEngine) computeModelBreakdown(sessions []SessionData) []ModelUsage {
	modelData := make(map[string]*ModelUsage)

	for _, s := range sessions {
		model := s.Model
		if model == "" {
			model = "unknown"
		}
		// Normalize: strip provider prefix for display
		displayModel := model
		if idx := strings.LastIndex(model, "/"); idx != -1 {
			displayModel = model[idx+1:]
		}

		if _, ok := modelData[displayModel]; !ok {
			modelData[displayModel] = &ModelUsage{Model: displayModel}
		}

		m := modelData[displayModel]
		m.Sessions++
		m.InputTokens += s.InputTokens
		m.OutputTokens += s.OutputTokens
		m.CacheReadTokens += s.CacheReadTokens
		m.CacheWriteTokens += s.CacheWriteTokens
		m.TotalTokens += s.InputTokens + s.OutputTokens + s.CacheReadTokens + s.CacheWriteTokens
		m.ToolCalls += s.ToolCallCount
		m.CostUSD += s.EstimatedCostUSD
	}

	// Convert to slice and sort
	result := make([]ModelUsage, 0, len(modelData))
	for _, m := range modelData {
		result = append(result, *m)
	}

	// Sort by total tokens descending
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalTokens != result[j].TotalTokens {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].Sessions > result[j].Sessions
	})

	return result
}

// computeToolBreakdown calculates tool usage statistics
func (ie *InsightsEngine) computeToolBreakdown(sessions []SessionData) []ToolUsage {
	toolCounts := make(map[string]int)
	totalCalls := 0

	for _, s := range sessions {
		// Note: Tool calls are tracked via session timeline, not directly in messages
		// This is a simplified implementation
		totalCalls += s.ToolCallCount
	}

	// Convert to slice
	result := make([]ToolUsage, 0, len(toolCounts))
	for tool, count := range toolCounts {
		percentage := 0.0
		if totalCalls > 0 {
			percentage = float64(count) / float64(totalCalls) * 100
		}
		result = append(result, ToolUsage{
			Tool:       tool,
			Count:      count,
			Percentage: percentage,
		})
	}

	// Sort by count descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}

// computeActivityPatterns analyzes activity by day and hour
func (ie *InsightsEngine) computeActivityPatterns(sessions []SessionData) ActivityStats {
	dayCounts := make(map[int]int)  // 0=Monday ... 6=Sunday
	hourCounts := make(map[int]int) // 0-23
	dailyCounts := make(map[string]int)

	for _, s := range sessions {
		day := int(s.StartedAt.Weekday())
		// Adjust: Go's Sunday=0, we want Monday=0
		day = (day + 6) % 7
		dayCounts[day]++

		hour := s.StartedAt.Hour()
		hourCounts[hour]++

		dateStr := s.StartedAt.Format("2006-01-02")
		dailyCounts[dateStr]++
	}

	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	byDay := make([]DayStat, 7)
	for i := 0; i < 7; i++ {
		byDay[i] = DayStat{Day: dayNames[i], Count: dayCounts[i]}
	}

	byHour := make([]HourStat, 24)
	for i := 0; i < 24; i++ {
		byHour[i] = HourStat{Hour: i, Count: hourCounts[i]}
	}

	// Find busiest day and hour
	var busiestDay *DayStat
	var busiestHour *HourStat

	for i := range byDay {
		if busiestDay == nil || byDay[i].Count > busiestDay.Count {
			dayCopy := byDay[i]
			busiestDay = &dayCopy
		}
	}

	for i := range byHour {
		if busiestHour == nil || byHour[i].Count > busiestHour.Count {
			hourCopy := byHour[i]
			busiestHour = &hourCopy
		}
	}

	// Calculate streak
	maxStreak := 0
	if len(dailyCounts) > 0 {
		// Sort dates
		dates := make([]string, 0, len(dailyCounts))
		for d := range dailyCounts {
			dates = append(dates, d)
		}
		sort.Strings(dates)

		currentStreak := 1
		maxStreak = 1
		for i := 1; i < len(dates); i++ {
			d1, _ := time.Parse("2006-01-02", dates[i-1])
			d2, _ := time.Parse("2006-01-02", dates[i])
			if d2.Sub(d1).Hours() == 24 {
				currentStreak++
				if currentStreak > maxStreak {
					maxStreak = currentStreak
				}
			} else {
				currentStreak = 1
			}
		}
	}

	return ActivityStats{
		ByDay:       byDay,
		ByHour:      byHour,
		BusiestDay:  busiestDay,
		BusiestHour: busiestHour,
		ActiveDays:  len(dailyCounts),
		MaxStreak:   maxStreak,
	}
}

// computeTopSessions finds notable sessions
func (ie *InsightsEngine) computeTopSessions(sessions []SessionData) []TopSession {
	var top []TopSession

	// Longest by duration
	var longest *SessionData
	for i := range sessions {
		if sessions[i].EndedAt != nil {
			if longest == nil ||
				sessions[i].EndedAt.Sub(sessions[i].StartedAt) > longest.EndedAt.Sub(longest.StartedAt) {
				longest = &sessions[i]
			}
		}
	}
	if longest != nil {
		duration := longest.EndedAt.Sub(longest.StartedAt)
		top = append(top, TopSession{
			Label:     "Longest session",
			SessionID: longest.ID[:min(16, len(longest.ID))],
			Value:     formatDuration(duration),
			Date:      longest.StartedAt.Format("Jan 02"),
		})
	}

	// Most messages
	var mostMsgs *SessionData
	for i := range sessions {
		if mostMsgs == nil || sessions[i].MessageCount > mostMsgs.MessageCount {
			mostMsgs = &sessions[i]
		}
	}
	if mostMsgs != nil && mostMsgs.MessageCount > 0 {
		top = append(top, TopSession{
			Label:     "Most messages",
			SessionID: mostMsgs.ID[:min(16, len(mostMsgs.ID))],
			Value:     fmt.Sprintf("%d msgs", mostMsgs.MessageCount),
			Date:      mostMsgs.StartedAt.Format("Jan 02"),
		})
	}

	// Most tokens
	var mostTokens *SessionData
	for i := range sessions {
		tokens := sessions[i].InputTokens + sessions[i].OutputTokens
		if mostTokens == nil || tokens > (mostTokens.InputTokens+mostTokens.OutputTokens) {
			mostTokens = &sessions[i]
		}
	}
	if mostTokens != nil && (mostTokens.InputTokens+mostTokens.OutputTokens) > 0 {
		top = append(top, TopSession{
			Label:     "Most tokens",
			SessionID: mostTokens.ID[:min(16, len(mostTokens.ID))],
			Value:     fmt.Sprintf("%d tokens", mostTokens.InputTokens+mostTokens.OutputTokens),
			Date:      mostTokens.StartedAt.Format("Jan 02"),
		})
	}

	// Most tool calls
	var mostTools *SessionData
	for i := range sessions {
		if mostTools == nil || sessions[i].ToolCallCount > mostTools.ToolCallCount {
			mostTools = &sessions[i]
		}
	}
	if mostTools != nil && mostTools.ToolCallCount > 0 {
		top = append(top, TopSession{
			Label:     "Most tool calls",
			SessionID: mostTools.ID[:min(16, len(mostTools.ID))],
			Value:     fmt.Sprintf("%d calls", mostTools.ToolCallCount),
			Date:      mostTools.StartedAt.Format("Jan 02"),
		})
	}

	return top
}

// FormatTerminal formats the insights report for terminal display
func (r *InsightsReport) FormatTerminal() string {
	if r.Empty {
		src := ""
		if r.SourceFilter != "" {
			src = fmt.Sprintf(" (source: %s)", r.SourceFilter)
		}
		return fmt.Sprintf("  No sessions found in the last %d days%s.", r.Days, src)
	}

	var lines []string
	o := r.Overview

	// Header
	lines = append(lines, "")
	lines = append(lines, "  ╔══════════════════════════════════════════════════════════╗")
	lines = append(lines, "  ║                    📊 MaxClaw Insights                   ║")
	periodLabel := fmt.Sprintf("Last %d days", r.Days)
	if r.SourceFilter != "" {
		periodLabel += fmt.Sprintf(" (%s)", r.SourceFilter)
	}
	padding := 58 - len(periodLabel) - 2
	leftPad := padding / 2
	rightPad := padding - leftPad
	lines = append(lines, fmt.Sprintf("  ║%s %s %s║", strings.Repeat(" ", leftPad), periodLabel, strings.Repeat(" ", rightPad)))
	lines = append(lines, "  ╚══════════════════════════════════════════════════════════╝")
	lines = append(lines, "")

	// Date range
	if o.DateRangeStart != nil && o.DateRangeEnd != nil {
		startStr := o.DateRangeStart.Format("Jan 02, 2006")
		endStr := o.DateRangeEnd.Format("Jan 02, 2006")
		lines = append(lines, fmt.Sprintf("  Period: %s — %s", startStr, endStr))
		lines = append(lines, "")
	}

	// Overview
	lines = append(lines, "  📋 Overview")
	lines = append(lines, "  "+strings.Repeat("─", 56))
	lines = append(lines, fmt.Sprintf("  Sessions:          %-12d  Messages:        %d", o.TotalSessions, o.TotalMessages))
	lines = append(lines, fmt.Sprintf("  Tool calls:        %-12d  User messages:   %d", o.TotalToolCalls, o.UserMessages))
	lines = append(lines, fmt.Sprintf("  Input tokens:      %-12d  Output tokens:   %d", o.TotalInputTokens, o.TotalOutputTokens))
	cacheTotal := o.TotalCacheReadTokens + o.TotalCacheWriteTokens
	if cacheTotal > 0 {
		lines = append(lines, fmt.Sprintf("  Cache read:        %-12d  Cache write:     %d", o.TotalCacheReadTokens, o.TotalCacheWriteTokens))
	}
	lines = append(lines, fmt.Sprintf("  Total tokens:      %-12d  Est. cost:       $%.2f", o.TotalTokens, o.EstimatedCostUSD))
	if o.TotalHours > 0 {
		lines = append(lines, fmt.Sprintf("  Active time:       ~%-11s  Avg session:     ~%s",
			formatDuration(time.Duration(o.TotalHours*3600)*time.Second),
			formatDuration(time.Duration(o.AvgSessionDuration)*time.Second)))
	}
	lines = append(lines, fmt.Sprintf("  Avg msgs/session:  %.1f", o.AvgMessagesPerSession))
	lines = append(lines, "")

	// Model breakdown
	if len(r.Models) > 0 {
		lines = append(lines, "  🤖 Models Used")
		lines = append(lines, "  "+strings.Repeat("─", 56))
		lines = append(lines, fmt.Sprintf("  %-30s %8s %12s %8s", "Model", "Sessions", "Tokens", "Cost"))
		for _, m := range r.Models {
			modelName := m.Model
			if len(modelName) > 28 {
				modelName = modelName[:28]
			}
			costCell := fmt.Sprintf("$%6.2f", m.CostUSD)
			if !m.HasPricing {
				costCell = "     N/A"
			}
			lines = append(lines, fmt.Sprintf("  %-30s %8d %12d %8s", modelName, m.Sessions, m.TotalTokens, costCell))
		}
		lines = append(lines, "")
	}

	// Tool usage
	if len(r.Tools) > 0 {
		lines = append(lines, "  🔧 Top Tools")
		lines = append(lines, "  "+strings.Repeat("─", 56))
		lines = append(lines, fmt.Sprintf("  %-28s %8s %8s", "Tool", "Calls", "%"))
		for i, t := range r.Tools {
			if i >= 15 { // Top 15
				lines = append(lines, fmt.Sprintf("  ... and %d more tools", len(r.Tools)-15))
				break
			}
			lines = append(lines, fmt.Sprintf("  %-28s %8d %7.1f%%", t.Tool, t.Count, t.Percentage))
		}
		lines = append(lines, "")
	}

	// Activity patterns
	act := r.Activity
	if len(act.ByDay) > 0 {
		lines = append(lines, "  📅 Activity Patterns")
		lines = append(lines, "  "+strings.Repeat("─", 56))

		// Day of week chart
		dayValues := make([]int, 7)
		for i, d := range act.ByDay {
			dayValues[i] = d.Count
		}
		bars := barChart(dayValues, 15)
		for i, d := range act.ByDay {
			lines = append(lines, fmt.Sprintf("  %s  %-15s %d", d.Day, bars[i], d.Count))
		}

		lines = append(lines, "")

		// Peak hours
		busyHours := make([]HourStat, len(act.ByHour))
		copy(busyHours, act.ByHour)
		sort.Slice(busyHours, func(i, j int) bool {
			return busyHours[i].Count > busyHours[j].Count
		})
		var hourStrs []string
		for i, h := range busyHours {
			if i >= 5 || h.Count == 0 {
				break
			}
			hourStrs = append(hourStrs, fmt.Sprintf("%s (%d)", formatHour(h.Hour), h.Count))
		}
		if len(hourStrs) > 0 {
			lines = append(lines, fmt.Sprintf("  Peak hours: %s", strings.Join(hourStrs, ", ")))
		}

		if act.ActiveDays > 0 {
			lines = append(lines, fmt.Sprintf("  Active days: %d", act.ActiveDays))
		}
		if act.MaxStreak > 1 {
			lines = append(lines, fmt.Sprintf("  Best streak: %d consecutive days", act.MaxStreak))
		}
		lines = append(lines, "")
	}

	// Notable sessions
	if len(r.TopSessions) > 0 {
		lines = append(lines, "  🏆 Notable Sessions")
		lines = append(lines, "  "+strings.Repeat("─", 56))
		for _, ts := range r.TopSessions {
			lines = append(lines, fmt.Sprintf("  %-20s %-18s (%s, %s)", ts.Label, ts.Value, ts.Date, ts.SessionID))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// Helper functions

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

func formatHour(hour int) string {
	ampm := "AM"
	if hour >= 12 {
		ampm = "PM"
	}
	displayHour := hour % 12
	if displayHour == 0 {
		displayHour = 12
	}
	return fmt.Sprintf("%d%s", displayHour, ampm)
}

func barChart(values []int, maxWidth int) []string {
	if len(values) == 0 {
		return []string{}
	}
	peak := values[0]
	for _, v := range values {
		if v > peak {
			peak = v
		}
	}
	if peak == 0 {
		return make([]string, len(values))
	}

	result := make([]string, len(values))
	for i, v := range values {
		if v > 0 {
			width := max(1, int(float64(v)/float64(peak)*float64(maxWidth)))
			result[i] = strings.Repeat("█", width)
		}
	}
	return result
}

// EstimateCost estimates the USD cost for a model/token tuple
// This is a simplified implementation - would need actual pricing data
func EstimateCost(model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) float64 {
	// Default pricing per 1K tokens (simplified)
	var inputRate, outputRate float64

	modelLower := strings.ToLower(model)
	switch {
	case strings.Contains(modelLower, "claude-3-opus"):
		inputRate = 0.015
		outputRate = 0.075
	case strings.Contains(modelLower, "claude-3-sonnet"):
		inputRate = 0.003
		outputRate = 0.015
	case strings.Contains(modelLower, "claude-3-haiku"):
		inputRate = 0.00025
		outputRate = 0.00125
	case strings.Contains(modelLower, "gpt-4-turbo"):
		inputRate = 0.01
		outputRate = 0.03
	case strings.Contains(modelLower, "gpt-4"):
		inputRate = 0.03
		outputRate = 0.06
	case strings.Contains(modelLower, "gpt-3.5-turbo"):
		inputRate = 0.0005
		outputRate = 0.0015
	default:
		// Default rates
		inputRate = 0.01
		outputRate = 0.03
	}

	// Apply cache discounts (typical: cache read is 10% of input cost, cache write is 25% extra)
	cost := float64(inputTokens) / 1000 * inputRate
	cost += float64(outputTokens) / 1000 * outputRate
	cost += float64(cacheReadTokens) / 1000 * inputRate * 0.1
	cost += float64(cacheWriteTokens) / 1000 * inputRate * 0.25

	return cost
}
