package main

import (
	"fmt"
	"os"

	"github.com/misty-step/costctl/formats"
	"github.com/misty-step/costctl/parser"
	"github.com/misty-step/costctl/reporter"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "costctl",
	Short: "OpenClaw cost observability CLI",
	Long: `costctl parses OpenClaw session transcripts and produces granular cost reports.

Data sources:
  - Session transcripts: ~/.openclaw/agents/{agent}/sessions/*.jsonl
  - Session index: ~/.openclaw/agents/{agent}/sessions/sessions.json

Each message contains: usage.cost.total (dollars), model, usage.input/output (tokens)`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func init() {
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("costctl version %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

// report command flags
var (
	reportPeriod    string
	reportAgent     string
	reportCrons     bool
	reportModels    bool
	reportFull      bool
	reportFormat    string
	reportThreshold float64
	agentsDir       string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate cost reports from session transcripts",
	Long: `Generate cost reports from OpenClaw session transcripts.

Examples:
  costctl report --period today
  costctl report --period week --agent urza
  costctl report --crons
  costctl report --models --format json
  costctl report --full --format text`,
	RunE: runReport,
}

func init() {
	reportCmd.Flags().StringVar(&reportPeriod, "period", "", "Time period: today|yesterday|week|month|all")
	reportCmd.Flags().StringVar(&reportAgent, "agent", "", "Filter by agent: amos|kaylee|pepper|urza|...")
	reportCmd.Flags().BoolVar(&reportCrons, "crons", false, "Show cron cost ranking")
	reportCmd.Flags().BoolVar(&reportModels, "models", false, "Show model cost comparison")
	reportCmd.Flags().BoolVar(&reportFull, "full", false, "Show all dimensions")
	reportCmd.Flags().StringVar(&reportFormat, "format", "text", "Output format: json|text")
	reportCmd.Flags().Float64Var(&reportThreshold, "threshold", 0.50, "Anomaly threshold for expensive crons ($)")
	reportCmd.Flags().StringVar(&agentsDir, "agents-dir", "", "Path to agents directory (default: ~/.openclaw/agents)")
}

func runReport(cmd *cobra.Command, args []string) error {
	// Resolve agents directory
	dir := agentsDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = home + "/.openclaw/agents"
	}

	// Validate period if specified
	if reportPeriod != "" {
		validPeriods := map[string]bool{"today": true, "yesterday": true, "week": true, "month": true, "all": true}
		if !validPeriods[reportPeriod] {
			return fmt.Errorf("invalid period: %s (valid: today, yesterday, week, month, all)", reportPeriod)
		}
	}

	// Validate format
	if reportFormat != "json" && reportFormat != "text" {
		return fmt.Errorf("invalid format: %s (valid: json, text)", reportFormat)
	}

	// Parse all sessions
	p := parser.New(dir)
	sessions, err := p.ParseAll(reportAgent)
	if err != nil {
		return fmt.Errorf("failed to parse sessions: %w", err)
	}

	// Build report configuration
	cfg := reporter.Config{
		Period:    reportPeriod,
		Agent:     reportAgent,
		Crons:     reportCrons,
		Models:    reportModels,
		Full:      reportFull,
		Threshold: reportThreshold,
	}

	// Generate report
	r := reporter.New(sessions, cfg)
	report := r.Generate()

	// Output report
	var formatter formats.Formatter
	if reportFormat == "json" {
		formatter = formats.NewJSONFormatter()
	} else {
		formatter = formats.NewTextFormatter()
	}

	output, err := formatter.Format(report)
	if err != nil {
		return fmt.Errorf("failed to format report: %w", err)
	}

	fmt.Print(output)
	return nil
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List available agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := agentsDir
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			dir = home + "/.openclaw/agents"
		}

		p := parser.New(dir)
		agents, err := p.ListAgents()
		if err != nil {
			return fmt.Errorf("failed to list agents: %w", err)
		}

		if len(agents) == 0 {
			fmt.Println("No agents found")
			return nil
		}

		fmt.Println("Available agents:")
		for _, agent := range agents {
			fmt.Printf("  - %s\n", agent)
		}
		return nil
	},
}
