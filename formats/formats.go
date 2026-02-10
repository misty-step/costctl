// Package formats handles output formatting for reports.
package formats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/misty-step/costctl/parser"
	"github.com/misty-step/costctl/reporter"
)

// Formatter defines the interface for report formatters.
type Formatter interface {
	Format(report reporter.Report) (string, error)
}

// JSONFormatter outputs reports in JSON format.
type JSONFormatter struct {
	Pretty bool
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{Pretty: true}
}

// Format formats the report as JSON.
func (f *JSONFormatter) Format(report reporter.Report) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TextFormatter outputs reports in human-readable text format.
type TextFormatter struct{}

// NewTextFormatter creates a new text formatter.
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{}
}

// Format formats the report as human-readable text.
func (f *TextFormatter) Format(r reporter.Report) (string, error) {
	var b strings.Builder

	// Header
	b.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
	b.WriteString("║              OpenClaw Cost Report                              ║\n")
	b.WriteString("╚════════════════════════════════════════════════════════════════╝\n\n")

	b.WriteString(fmt.Sprintf("Generated: %s\n", r.GeneratedAt.Format(time.RFC3339)))
	if r.Period != "" {
		b.WriteString(fmt.Sprintf("Period:    %s\n", r.Period))
	}
	b.WriteString("\n")

	// Summary
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(" SUMMARY\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(fmt.Sprintf("  Total Sessions: %d\n", r.TotalSessions))
	b.WriteString(fmt.Sprintf("  Total Cost:     %s\n", parser.FormatCost(r.TotalCost)))
	b.WriteString(fmt.Sprintf("  Total Tokens:   %s\n", parser.FormatTokens(r.TotalTokens)))
	b.WriteString("\n")

	// By Agent
	if len(r.ByAgent) > 0 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" BY AGENT\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("  %-12s %8s %12s %12s\n", "AGENT", "SESSIONS", "COST", "TOKENS"))
		for _, a := range r.ByAgent {
			b.WriteString(fmt.Sprintf("  %-12s %8d %12s %12s\n",
				a.Agent,
				a.Sessions,
				parser.FormatCost(a.TotalCost),
				parser.FormatTokens(a.TotalTokens)))
		}
		b.WriteString("\n")
	}

	// By Session Type
	if len(r.BySessionType) > 0 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" BY SESSION TYPE\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("  %-15s %8s %12s %12s\n", "TYPE", "SESSIONS", "COST", "TOKENS"))
		for _, t := range r.BySessionType {
			b.WriteString(fmt.Sprintf("  %-15s %8d %12s %12s\n",
				t.Type,
				t.Sessions,
				parser.FormatCost(t.TotalCost),
				parser.FormatTokens(t.TotalTokens)))
		}
		b.WriteString("\n")
	}

	// By Cron
	if len(r.ByCron) > 0 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" BY CRON JOB\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("  %-25s %6s %10s %10s %10s\n", "CRON NAME", "RUNS", "TOTAL", "AVG", "MAX"))
		for _, c := range r.ByCron {
			name := c.CronName
			if len(name) > 25 {
				name = name[:22] + "..."
			}
			b.WriteString(fmt.Sprintf("  %-25s %6d %10s %10s %10s\n",
				name,
				c.Runs,
				parser.FormatCost(c.TotalCost),
				parser.FormatCost(c.AvgCost),
				parser.FormatCost(c.MaxCost)))
		}
		b.WriteString("\n")
	}

	// By Model
	if len(r.ByModel) > 0 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" BY MODEL\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("  %-35s %8s %10s %10s\n", "MODEL", "SESSIONS", "COST", "TOKENS"))
		for _, m := range r.ByModel {
			model := m.Model
			if len(model) > 35 {
				model = model[:32] + "..."
			}
			b.WriteString(fmt.Sprintf("  %-35s %8d %10s %10s\n",
				model,
				m.Sessions,
				parser.FormatCost(m.TotalCost),
				parser.FormatTokens(m.TotalTokens)))
		}
		b.WriteString("\n")
	}

	// By Day (if showing trends)
	if len(r.ByDay) > 1 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" DAILY TREND\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("  %-12s %8s %12s %12s\n", "DATE", "SESSIONS", "COST", "TOKENS"))
		for _, d := range r.ByDay {
			b.WriteString(fmt.Sprintf("  %-12s %8d %12s %12s\n",
				d.Date,
				d.Sessions,
				parser.FormatCost(d.TotalCost),
				parser.FormatTokens(d.TotalTokens)))
		}
		b.WriteString("\n")
	}

	// Anomalies
	if len(r.Anomalies) > 0 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" ANOMALIES\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		for _, a := range r.Anomalies {
			severity := "⚠️ "
			if a.Severity == "error" {
				severity = "❌"
			} else if a.Severity == "info" {
				severity = "ℹ️ "
			}
			b.WriteString(fmt.Sprintf("  %s [%s] %s\n", severity, a.Type, a.Description))
			if a.Cost > 0 {
				b.WriteString(fmt.Sprintf("     Cost: %s", parser.FormatCost(a.Cost)))
				if a.Agent != "" {
					b.WriteString(fmt.Sprintf(" | Agent: %s", a.Agent))
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	// Top Sessions (if full report)
	if len(r.Sessions) > 0 && len(r.Sessions) <= 20 {
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(" TOP EXPENSIVE SESSIONS\n")
		b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("  %-12s %-15s %10s %10s %s\n", "AGENT", "TYPE", "COST", "TOKENS", "MODEL"))
		for i, s := range r.Sessions {
			if i >= 10 {
				break
			}
			model := s.Model
			if len(model) > 20 {
				model = model[:17] + "..."
			}
			b.WriteString(fmt.Sprintf("  %-12s %-15s %10s %10s %s\n",
				s.Agent,
				s.Type,
				parser.FormatCost(s.Cost),
				parser.FormatTokens(s.Tokens),
				model))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

// Helper to format session type for display
func formatSessionType(t parser.SessionType) string {
	switch t {
	case parser.SessionTypeInteractive:
		return "interactive"
	case parser.SessionTypeCron:
		return "cron"
	case parser.SessionTypeSubagent:
		return "subagent"
	default:
		return string(t)
	}
}
