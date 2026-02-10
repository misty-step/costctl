package reporter

import (
	"testing"
	"time"

	"github.com/misty-step/costctl/parser"
)

func TestAggregateByAgent(t *testing.T) {
	sessions := []parser.Session{
		{Agent: "urza", Usage: parser.Usage{CostTotal: 1.5, Total: 1000}},
		{Agent: "urza", Usage: parser.Usage{CostTotal: 0.5, Total: 500}},
		{Agent: "amos", Usage: parser.Usage{CostTotal: 3.0, Total: 2000}}, // Higher cost
	}

	r := New(sessions, Config{})
	result := r.aggregateByAgent(sessions)

	if len(result) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(result))
	}

	// Should be sorted by cost descending
	if result[0].Agent != "amos" {
		t.Errorf("expected first agent to be amos, got %s", result[0].Agent)
	}
	if result[0].TotalCost != 3.0 {
		t.Errorf("expected amos cost 3.0, got %f", result[0].TotalCost)
	}
	if result[0].Sessions != 1 {
		t.Errorf("expected amos sessions 1, got %d", result[0].Sessions)
	}
}

func TestAggregateBySessionType(t *testing.T) {
	sessions := []parser.Session{
		{Type: parser.SessionTypeInteractive, Usage: parser.Usage{CostTotal: 1.0}},
		{Type: parser.SessionTypeCron, Usage: parser.Usage{CostTotal: 2.0}},
		{Type: parser.SessionTypeInteractive, Usage: parser.Usage{CostTotal: 0.5}},
		{Type: parser.SessionTypeSubagent, Usage: parser.Usage{CostTotal: 0.25}},
	}

	r := New(sessions, Config{})
	result := r.aggregateBySessionType(sessions)

	if len(result) != 3 {
		t.Fatalf("expected 3 types, got %d", len(result))
	}

	// Should be sorted: interactive, cron, subagent
	expected := []parser.SessionType{
		parser.SessionTypeInteractive,
		parser.SessionTypeCron,
		parser.SessionTypeSubagent,
	}
	for i, exp := range expected {
		if result[i].Type != exp {
			t.Errorf("position %d: expected %v, got %v", i, exp, result[i].Type)
		}
	}
}

func TestAggregateByCron(t *testing.T) {
	sessions := []parser.Session{
		{Type: parser.SessionTypeCron, CronName: "daily-kickoff", CronID: "cron1", Usage: parser.Usage{CostTotal: 1.0}},
		{Type: parser.SessionTypeCron, CronName: "daily-kickoff", CronID: "cron1", Usage: parser.Usage{CostTotal: 1.5}},
		{Type: parser.SessionTypeCron, CronName: "code-reviewer", CronID: "cron2", Usage: parser.Usage{CostTotal: 0.5}},
		{Type: parser.SessionTypeInteractive, Usage: parser.Usage{CostTotal: 5.0}}, // Should be excluded
	}

	r := New(sessions, Config{})
	result := r.aggregateByCron(sessions)

	if len(result) != 2 {
		t.Fatalf("expected 2 crons, got %d", len(result))
	}

	// Should be sorted by total cost
	if result[0].CronName != "daily-kickoff" {
		t.Errorf("expected first cron to be daily-kickoff, got %s", result[0].CronName)
	}
	if result[0].Runs != 2 {
		t.Errorf("expected 2 runs, got %d", result[0].Runs)
	}
	if result[0].TotalCost != 2.5 {
		t.Errorf("expected total cost 2.5, got %f", result[0].TotalCost)
	}
	if result[0].AvgCost != 1.25 {
		t.Errorf("expected avg cost 1.25, got %f", result[0].AvgCost)
	}
	if result[0].MaxCost != 1.5 {
		t.Errorf("expected max cost 1.5, got %f", result[0].MaxCost)
	}
}

func TestAggregateByModel(t *testing.T) {
	sessions := []parser.Session{
		{Usage: parser.Usage{CostTotal: 1.0, Model: "moonshotai/kimi-k2.5"}},
		{Usage: parser.Usage{CostTotal: 2.0, Model: "claude-opus-4-6"}},
		{Usage: parser.Usage{CostTotal: 0.5, Model: "moonshotai/kimi-k2.5"}},
	}

	r := New(sessions, Config{})
	result := r.aggregateByModel(sessions)

	if len(result) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result))
	}

	// Should be sorted by cost
	if result[0].Model != "claude-opus-4-6" {
		t.Errorf("expected first model to be claude-opus-4-6, got %s", result[0].Model)
	}
	if result[0].TotalCost != 2.0 {
		t.Errorf("expected claude-opus-4-6 cost 2.0, got %f", result[0].TotalCost)
	}
	if result[1].Sessions != 2 {
		t.Errorf("expected kimi-k2.5 sessions 2, got %d", result[1].Sessions)
	}
}

func TestAggregateByDay(t *testing.T) {
	sessions := []parser.Session{
		{StartedAt: time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC), Usage: parser.Usage{CostTotal: 1.0}},
		{StartedAt: time.Date(2026, 2, 10, 15, 0, 0, 0, time.UTC), Usage: parser.Usage{CostTotal: 0.5}},
		{StartedAt: time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC), Usage: parser.Usage{CostTotal: 2.0}},
	}

	r := New(sessions, Config{})
	result := r.aggregateByDay(sessions)

	if len(result) != 2 {
		t.Fatalf("expected 2 days, got %d", len(result))
	}

	if result[0].Date != "2026-02-10" {
		t.Errorf("expected first date 2026-02-10, got %s", result[0].Date)
	}
	if result[0].TotalCost != 1.5 {
		t.Errorf("expected 2026-02-10 cost 1.5, got %f", result[0].TotalCost)
	}
	if result[1].Date != "2026-02-11" {
		t.Errorf("expected second date 2026-02-11, got %s", result[1].Date)
	}
}

func TestFilterByPeriod(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	lastWeek := today.AddDate(0, 0, -7)

	sessions := []parser.Session{
		{StartedAt: today, Usage: parser.Usage{CostTotal: 1.0}},
		{StartedAt: yesterday, Usage: parser.Usage{CostTotal: 2.0}},
		{StartedAt: lastWeek, Usage: parser.Usage{CostTotal: 3.0}},
	}

	tests := []struct {
		period   string
		expected int
	}{
		{"today", 1},
		{"yesterday", 1},
		{"week", 3}, // lastWeek is exactly 7 days ago, so included
		{"all", 3},
		{"", 3},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			r := New(sessions, Config{Period: tt.period})
			result := r.filterByPeriod(sessions)
			if len(result) != tt.expected {
				t.Errorf("period %q: expected %d sessions, got %d", tt.period, tt.expected, len(result))
			}
		})
	}
}

func TestDetectAnomalies(t *testing.T) {
	sessions := []parser.Session{
		{
			Type:     parser.SessionTypeCron,
			CronName: "expensive-cron",
			Agent:    "urza",
			ID:       "session1",
			Usage:    parser.Usage{CostTotal: 1.0, Total: 50000},
		},
		{
			Type:  parser.SessionTypeInteractive,
			Agent: "amos",
			ID:    "session2",
			Usage: parser.Usage{CostTotal: 0.1, Total: 150000}, // High token count
		},
		{
			Type:  parser.SessionTypeInteractive,
			Agent: "pepper",
			ID:    "session3",
			Usage: parser.Usage{CostTotal: 0.5, Total: 1000, Model: "claude-opus-4"}, // Opus overkill
		},
	}

	r := New(sessions, Config{Threshold: 0.50})
	anomalies := r.detectAnomalies(sessions)

	// Should detect: expensive cron, high token count, opus overkill
	if len(anomalies) != 3 {
		t.Errorf("expected 3 anomalies, got %d", len(anomalies))
	}

	// Check types
	types := make(map[string]bool)
	for _, a := range anomalies {
		types[a.Type] = true
	}

	if !types["expensive_cron"] {
		t.Error("expected expensive_cron anomaly")
	}
	if !types["high_token_count"] {
		t.Error("expected high_token_count anomaly")
	}
	if !types["opus_overkill"] {
		t.Error("expected opus_overkill anomaly")
	}
}

func TestContainsOpus(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"claude-opus-4-6", true},
		{"claude-opus", true},
		{"claude-3-opus-20240229", true},
		{"opus-model", true},
		{"moonshotai/kimi-k2.5", false},
		{"gpt-4", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := containsOpus(tt.model)
			if result != tt.expected {
				t.Errorf("containsOpus(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	sessions := []parser.Session{
		{
			Agent:     "urza",
			Type:      parser.SessionTypeInteractive,
			ID:        "session1",
			StartedAt: time.Now(),
			Usage:     parser.Usage{CostTotal: 1.5, Total: 1000, Input: 500, Output: 500, Model: "kimi"},
		},
	}

	r := New(sessions, Config{Period: "today", Full: true})
	report := r.Generate()

	if report.TotalCost != 1.5 {
		t.Errorf("expected total cost 1.5, got %f", report.TotalCost)
	}
	if report.TotalTokens != 1000 {
		t.Errorf("expected total tokens 1000, got %d", report.TotalTokens)
	}
	if report.TotalSessions != 1 {
		t.Errorf("expected total sessions 1, got %d", report.TotalSessions)
	}
	if report.Period != "today" {
		t.Errorf("expected period 'today', got %s", report.Period)
	}
	if len(report.ByAgent) != 1 {
		t.Errorf("expected 1 agent summary, got %d", len(report.ByAgent))
	}
	if len(report.Sessions) != 1 {
		t.Errorf("expected 1 session detail, got %d", len(report.Sessions))
	}
}
