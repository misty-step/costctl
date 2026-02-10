# costctl

OpenClaw cost observability CLI tool. Parses session transcripts and produces granular cost reports.

## Overview

`costctl` analyzes OpenClaw session JSONL files to provide insights into LLM usage costs across agents, session types, cron jobs, and models.

## Installation

```bash
cd ~/clawd/costctl
go build -o costctl
```

Or install directly:

```bash
go install github.com/misty-step/costctl@latest
```

## Usage

### List available agents

```bash
costctl agents
```

### Generate reports

```bash
# Today's costs across all agents
costctl report --period today

# Yesterday's costs
costctl report --period yesterday

# Last 7 days
costctl report --period week

# Last 30 days
costctl report --period month

# All time
costctl report --period all

# Filter by specific agent
costctl report --period today --agent urza

# Show cron cost ranking
costctl report --crons

# Show model cost comparison
costctl report --models

# Full report with all dimensions
costctl report --full

# JSON output for Cortex dashboard
costctl report --full --format json

# Custom anomaly threshold (default $0.50)
costctl report --crons --threshold 1.00

# Custom agents directory
costctl report --agents-dir /custom/path/to/agents
```

## Report Dimensions

1. **By Agent** - amos, kaylee, pepper, abra, urza, pluto, mishra, cato, venser
2. **By Session Type** - interactive, cron, subagent
3. **By Cron Job** - daily-kickoff, code-reviewer, etc.
4. **By Model** - claude-opus-4-6, moonshotai/kimi-k2.5, etc.
5. **By Time Period** - hourly, daily, weekly buckets
6. **Trending** - cost per day, anomaly detection

## Anomaly Detection

`costctl` automatically detects:

- **Expensive Crons** - Cron jobs exceeding the configured threshold (default $0.50)
- **High Token Counts** - Sessions with unusually high token counts (>100k)
- **Opus Overkill** - Opus model usage where cheaper models would suffice (<5k tokens)

## Output Formats

### Text (default)
Human-readable tables optimized for Discord/terminal display.

### JSON
Structured output for Cortex dashboard integration.

## Data Sources

- **Session transcripts**: `~/.openclaw/agents/{agent}/sessions/*.jsonl`
- **Session index**: `~/.openclaw/agents/{agent}/sessions/sessions.json`

Each message contains:
- `usage.cost.total` - Total cost in dollars
- `model` - Model identifier
- `usage.input/output` - Token counts

## Session Key Formats

- `agent:{name}:cron:{id}:run:{sid}` → cron job
- `agent:{name}:subagent:{sid}` → sub-agent  
- `agent:{name}` → interactive

## Development

### Build

```bash
go build -o costctl
```

### Test

```bash
go test ./...
```

### Run with verbose output

```bash
go run . report --period today --full
```

## Project Structure

```
costctl/
├── main.go              # CLI entry point
├── go.mod               # Go module
├── parser/              # Session file parsing
│   ├── parser.go
│   └── parser_test.go
├── reporter/            # Report generation
│   ├── reporter.go
│   └── reporter_test.go
├── formats/             # Output formatting
│   └── formats.go
└── README.md
```

## License

MIT - Misty Step LLC
