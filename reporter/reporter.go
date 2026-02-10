// Package reporter handles generation of cost reports.
package reporter

import (
	"fmt"
	"sort"
	"time"

	"github.com/misty-step/costctl/parser"
)

// Config configures report generation.
type Config struct {
	Period    string  // today, yesterday, week, month, all
	Agent     string  // filter by agent
	Crons     bool    // show cron ranking
	Models    bool    // show model comparison
	Full      bool    // show all dimensions
	Threshold float64 // anomaly threshold for expensive crons
}

// Report contains all report data.
type Report struct {
	GeneratedAt   time.Time            `json:"generated_at"`
	Period        string               `json:"period"`
	TotalCost     float64              `json:"total_cost"`
	TotalTokens   int                  `json:"total_tokens"`
	TotalSessions int                  `json:"total_sessions"`
	ByAgent       []AgentSummary       `json:"by_agent"`
	BySessionType []SessionTypeSummary `json:"by_session_type"`
	ByCron        []CronSummary        `json:"by_cron,omitempty"`
	ByModel       []ModelSummary       `json:"by_model"`
	ByDay         []DaySummary         `json:"by_day,omitempty"`
	Anomalies     []Anomaly            `json:"anomalies,omitempty"`
	Sessions      []SessionDetail      `json:"sessions,omitempty"`
}

// AgentSummary aggregates costs by agent.
type AgentSummary struct {
	Agent        string  `json:"agent"`
	Sessions     int     `json:"sessions"`
	TotalCost    float64 `json:"total_cost"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
}

// SessionTypeSummary aggregates costs by session type.
type SessionTypeSummary struct {
	Type        parser.SessionType `json:"type"`
	Sessions    int                `json:"sessions"`
	TotalCost   float64            `json:"total_cost"`
	TotalTokens int                `json:"total_tokens"`
}

// CronSummary aggregates costs by cron job.
type CronSummary struct {
	CronName    string  `json:"cron_name"`
	CronID      string  `json:"cron_id,omitempty"`
	Runs        int     `json:"runs"`
	TotalCost   float64 `json:"total_cost"`
	AvgCost     float64 `json:"avg_cost"`
	MaxCost     float64 `json:"max_cost"`
	TotalTokens int     `json:"total_tokens"`
}

// ModelSummary aggregates costs by model.
type ModelSummary struct {
	Model        string  `json:"model"`
	Sessions     int     `json:"sessions"`
	TotalCost    float64 `json:"total_cost"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
}

// DaySummary aggregates costs by day.
type DaySummary struct {
	Date        string  `json:"date"`
	Sessions    int     `json:"sessions"`
	TotalCost   float64 `json:"total_cost"`
	TotalTokens int     `json:"total_tokens"`
}

// Anomaly represents an anomalous session or pattern.
type Anomaly struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"` // warning, error
	Cost        float64 `json:"cost,omitempty"`
	SessionID   string  `json:"session_id,omitempty"`
	Agent       string  `json:"agent,omitempty"`
}

// SessionDetail contains detailed session information.
type SessionDetail struct {
	ID        string             `json:"id"`
	Agent     string             `json:"agent"`
	Type      parser.SessionType `json:"type"`
	CronName  string             `json:"cron_name,omitempty"`
	Model     string             `json:"model"`
	Cost      float64            `json:"cost"`
	Tokens    int                `json:"tokens"`
	StartedAt time.Time          `json:"started_at"`
	Duration  time.Duration      `json:"duration"`
}

// Reporter generates reports from parsed sessions.
type Reporter struct {
	sessions []parser.Session
	config   Config
}

// New creates a new Reporter.
func New(sessions []parser.Session, config Config) *Reporter {
	return &Reporter{
		sessions: sessions,
		config:   config,
	}
}

// Generate produces a complete report.
func (r *Reporter) Generate() Report {
	// Filter sessions by period
	filtered := r.filterByPeriod(r.sessions)

	report := Report{
		GeneratedAt: time.Now().UTC(),
		Period:      r.config.Period,
	}

	// Calculate totals
	for _, s := range filtered {
		report.TotalCost += s.Usage.CostTotal
		report.TotalTokens += s.Usage.Total
		report.TotalSessions++
	}

	// Generate dimensions
	report.ByAgent = r.aggregateByAgent(filtered)
	report.BySessionType = r.aggregateBySessionType(filtered)
	report.ByModel = r.aggregateByModel(filtered)
	report.ByDay = r.aggregateByDay(filtered)

	if r.config.Crons || r.config.Full {
		report.ByCron = r.aggregateByCron(filtered)
	}

	if r.config.Full {
		report.Sessions = r.getSessionDetails(filtered)
	}

	// Detect anomalies
	report.Anomalies = r.detectAnomalies(filtered)

	return report
}

// filterByPeriod filters sessions based on the configured period.
func (r *Reporter) filterByPeriod(sessions []parser.Session) []parser.Session {
	if r.config.Period == "" || r.config.Period == "all" {
		return sessions
	}

	now := time.Now()
	var cutoff time.Time

	switch r.config.Period {
	case "today":
		cutoff = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		nextDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cutoff = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		// Filter to yesterday only
		var result []parser.Session
		for _, s := range sessions {
			if !s.StartedAt.IsZero() && s.StartedAt.After(cutoff) && s.StartedAt.Before(nextDay) {
				result = append(result, s)
			}
		}
		return result
	case "week":
		cutoff = now.AddDate(0, 0, -7)
	case "month":
		cutoff = now.AddDate(0, -1, 0)
	}

	var result []parser.Session
	for _, s := range sessions {
		if !s.StartedAt.IsZero() && s.StartedAt.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}

func (r *Reporter) aggregateByAgent(sessions []parser.Session) []AgentSummary {
	agg := make(map[string]*AgentSummary)

	for _, s := range sessions {
		if _, ok := agg[s.Agent]; !ok {
			agg[s.Agent] = &AgentSummary{Agent: s.Agent}
		}
		a := agg[s.Agent]
		a.Sessions++
		a.TotalCost += s.Usage.CostTotal
		a.InputTokens += s.Usage.Input
		a.OutputTokens += s.Usage.Output
		a.TotalTokens += s.Usage.Total
	}

	result := make([]AgentSummary, 0, len(agg))
	for _, a := range agg {
		result = append(result, *a)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})

	return result
}

func (r *Reporter) aggregateBySessionType(sessions []parser.Session) []SessionTypeSummary {
	agg := make(map[parser.SessionType]*SessionTypeSummary)

	for _, s := range sessions {
		if _, ok := agg[s.Type]; !ok {
			agg[s.Type] = &SessionTypeSummary{Type: s.Type}
		}
		t := agg[s.Type]
		t.Sessions++
		t.TotalCost += s.Usage.CostTotal
		t.TotalTokens += s.Usage.Total
	}

	result := make([]SessionTypeSummary, 0, len(agg))
	for _, t := range agg {
		result = append(result, *t)
	}

	// Order: interactive, cron, subagent
	order := map[parser.SessionType]int{
		parser.SessionTypeInteractive: 0,
		parser.SessionTypeCron:        1,
		parser.SessionTypeSubagent:    2,
	}
	sort.Slice(result, func(i, j int) bool {
		return order[result[i].Type] < order[result[j].Type]
	})

	return result
}

func (r *Reporter) aggregateByCron(sessions []parser.Session) []CronSummary {
	// Only include cron sessions
	type cronKey struct {
		name string
		id   string
	}
	agg := make(map[cronKey]*CronSummary)

	for _, s := range sessions {
		if s.Type != parser.SessionTypeCron {
			continue
		}
		key := cronKey{name: s.CronName, id: s.CronID}
		if _, ok := agg[key]; !ok {
			agg[key] = &CronSummary{CronName: s.CronName, CronID: s.CronID}
		}
		c := agg[key]
		c.Runs++
		c.TotalCost += s.Usage.CostTotal
		c.TotalTokens += s.Usage.Total
		if s.Usage.CostTotal > c.MaxCost {
			c.MaxCost = s.Usage.CostTotal
		}
	}

	result := make([]CronSummary, 0, len(agg))
	for _, c := range agg {
		if c.Runs > 0 {
			c.AvgCost = c.TotalCost / float64(c.Runs)
		}
		result = append(result, *c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})

	return result
}

func (r *Reporter) aggregateByModel(sessions []parser.Session) []ModelSummary {
	agg := make(map[string]*ModelSummary)

	for _, s := range sessions {
		model := s.Usage.Model
		if model == "" {
			model = "unknown"
		}
		if _, ok := agg[model]; !ok {
			agg[model] = &ModelSummary{Model: model}
		}
		m := agg[model]
		m.Sessions++
		m.TotalCost += s.Usage.CostTotal
		m.InputTokens += s.Usage.Input
		m.OutputTokens += s.Usage.Output
		m.TotalTokens += s.Usage.Total
	}

	result := make([]ModelSummary, 0, len(agg))
	for _, m := range agg {
		result = append(result, *m)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})

	return result
}

func (r *Reporter) aggregateByDay(sessions []parser.Session) []DaySummary {
	agg := make(map[string]*DaySummary)

	for _, s := range sessions {
		if s.StartedAt.IsZero() {
			continue
		}
		date := s.StartedAt.Format("2006-01-02")
		if _, ok := agg[date]; !ok {
			agg[date] = &DaySummary{Date: date}
		}
		d := agg[date]
		d.Sessions++
		d.TotalCost += s.Usage.CostTotal
		d.TotalTokens += s.Usage.Total
	}

	result := make([]DaySummary, 0, len(agg))
	for _, d := range agg {
		result = append(result, *d)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result
}

func (r *Reporter) detectAnomalies(sessions []parser.Session) []Anomaly {
	var anomalies []Anomaly

	// Expensive crons
	for _, s := range sessions {
		if s.Type == parser.SessionTypeCron && s.Usage.CostTotal > r.config.Threshold {
			anomalies = append(anomalies, Anomaly{
				Type:        "expensive_cron",
				Description: fmt.Sprintf("Cron %s exceeded $%.2f threshold", s.CronName, r.config.Threshold),
				Severity:    "warning",
				Cost:        s.Usage.CostTotal,
				SessionID:   s.ID,
				Agent:       s.Agent,
			})
		}
	}

	// High token counts (sessions with >100k tokens)
	for _, s := range sessions {
		if s.Usage.Total > 100000 {
			anomalies = append(anomalies, Anomaly{
				Type:        "high_token_count",
				Description: fmt.Sprintf("Session has unusually high token count (%d)", s.Usage.Total),
				Severity:    "warning",
				Cost:        s.Usage.CostTotal,
				SessionID:   s.ID,
				Agent:       s.Agent,
			})
		}
	}

	// Opus usage where cheaper model might suffice
	for _, s := range sessions {
		if containsOpus(s.Usage.Model) && s.Usage.Total < 5000 {
			anomalies = append(anomalies, Anomaly{
				Type:        "opus_overkill",
				Description: fmt.Sprintf("Opus model used for small request (%d tokens), consider cheaper model", s.Usage.Total),
				Severity:    "info",
				Cost:        s.Usage.CostTotal,
				SessionID:   s.ID,
				Agent:       s.Agent,
			})
		}
	}

	return anomalies
}

func (r *Reporter) getSessionDetails(sessions []parser.Session) []SessionDetail {
	result := make([]SessionDetail, 0, len(sessions))

	for _, s := range sessions {
		result = append(result, SessionDetail{
			ID:        s.ID,
			Agent:     s.Agent,
			Type:      s.Type,
			CronName:  s.CronName,
			Model:     s.Usage.Model,
			Cost:      s.Usage.CostTotal,
			Tokens:    s.Usage.Total,
			StartedAt: s.StartedAt,
			Duration:  s.Duration,
		})
	}

	// Sort by cost descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Cost > result[j].Cost
	})

	return result
}

func containsOpus(model string) bool {
	opusModels := []string{"opus", "claude-opus", "claude-3-opus"}
	lower := fmt.Sprintf("%s", model)
	for _, o := range opusModels {
		if contains(lower, o) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(s[:len(substr)] == substr) ||
		(s[len(s)-len(substr):] == substr) ||
		findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
