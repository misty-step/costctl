package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListAgents(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()

	// Create agent directories with sessions
	agents := []string{"amos", "urza", "pepper"}
	for _, agent := range agents {
		sessionsDir := filepath.Join(tempDir, agent, "sessions")
		if err := os.MkdirAll(sessionsDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-agent directory (no sessions subdirectory)
	os.MkdirAll(filepath.Join(tempDir, "not-an-agent"), 0755)

	p := New(tempDir)
	result, err := p.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 agents, got %d: %v", len(result), result)
	}
}

func TestParseSessionKey(t *testing.T) {
	tests := []struct {
		sessionID    string
		wantType     SessionType
		wantCronID   string
		wantCronName string
		wantSubID    string
	}{
		{
			sessionID: "agent:urza:cron:daily-kickoff-abc123:run:xyz789",
			wantType:  SessionTypeCron,
			wantCronID:   "daily-kickoff-abc123",
			wantCronName: "daily-kickoff",
		},
		{
			sessionID: "agent:amos:subagent:task-123",
			wantType:  SessionTypeSubagent,
			wantSubID: "task-123",
		},
		{
			sessionID: "agent:main",
			wantType:  SessionTypeInteractive,
		},
		{
			sessionID: "simple-session-id",
			wantType:  SessionTypeInteractive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.sessionID, func(t *testing.T) {
			s := Session{}
			s.parseSessionKey(tt.sessionID)

			if s.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", s.Type, tt.wantType)
			}
			if s.CronID != tt.wantCronID {
				t.Errorf("CronID = %v, want %v", s.CronID, tt.wantCronID)
			}
			if s.CronName != tt.wantCronName {
				t.Errorf("CronName = %v, want %v", s.CronName, tt.wantCronName)
			}
			if s.SubagentID != tt.wantSubID {
				t.Errorf("SubagentID = %v, want %v", s.SubagentID, tt.wantSubID)
			}
		})
	}
}

func TestParseSessionFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test session file
	sessionContent := `{"type":"session","version":3,"id":"test-session","timestamp":"2026-02-10T16:53:15.416Z"}
{"type":"message","id":"msg1","timestamp":"2026-02-10T16:53:15.420Z","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}],"usage":{"input":100,"output":50,"totalTokens":150,"cost":{"input":0.0005,"output":0.00075,"total":0.00125}},"model":"moonshotai/kimi-k2.5"}}
{"type":"message","id":"msg2","timestamp":"2026-02-10T16:54:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":"World"}],"usage":{"input":200,"output":100,"totalTokens":300,"cost":{"input":0.001,"output":0.0015,"total":0.0025}},"model":"moonshotai/kimi-k2.5"}}`

	sessionFile := filepath.Join(tempDir, "test-session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent), 0644); err != nil {
		t.Fatal(err)
	}

	p := New(tempDir)
	session, err := p.parseSessionFile("urza", "test-session", sessionFile)
	if err != nil {
		t.Fatalf("parseSessionFile failed: %v", err)
	}

	if len(session.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(session.Messages))
	}

	// Check usage aggregation
	expectedCost := 0.00375 // 0.00125 + 0.0025
	if session.Usage.CostTotal != expectedCost {
		t.Errorf("expected total cost %.5f, got %.5f", expectedCost, session.Usage.CostTotal)
	}

	expectedTokens := 450 // 150 + 300
	if session.Usage.Total != expectedTokens {
		t.Errorf("expected total tokens %d, got %d", expectedTokens, session.Usage.Total)
	}
}

func TestDeriveCronName(t *testing.T) {
	tests := []struct {
		cronID   string
		expected string
	}{
		{"daily-kickoff-abc123", "daily-kickoff"},
		{"code-reviewer-xyz789", "code-reviewer"},
		{"simple", "simple"},
		{"backup-123", "backup-123"}, // too short to be a hash
		{"health-check-a1b2c3d4", "health-check"},
	}

	for _, tt := range tests {
		t.Run(tt.cronID, func(t *testing.T) {
			result := deriveCronName(tt.cronID)
			if result != tt.expected {
				t.Errorf("deriveCronName(%q) = %q, want %q", tt.cronID, result, tt.expected)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0.00125, "$0.0013"},
		{0.12345, "$0.12"}, // rounds to 2 decimals
		{1.5, "$1.50"},
		{10.999, "$11.00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCost(tt.cost)
			if result != tt.expected {
				t.Errorf("FormatCost(%f) = %s, want %s", tt.cost, result, tt.expected)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		tokens   int
		expected string
	}{
		{500, "500"},
		{1500, "1.5k"},
		{50000, "50.0k"},
		{1500000, "1.50M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatTokens(tt.tokens)
			if result != tt.expected {
				t.Errorf("FormatTokens(%d) = %s, want %s", tt.tokens, result, tt.expected)
			}
		})
	}
}

func TestSessionKey(t *testing.T) {
	tests := []struct {
		session  Session
		expected string
	}{
		{
			session:  Session{Agent: "urza", Type: SessionTypeCron, CronID: "daily-kickoff", ID: "run123"},
			expected: "agent:urza:cron:daily-kickoff:run:run123",
		},
		{
			session:  Session{Agent: "amos", Type: SessionTypeSubagent, ID: "task456"},
			expected: "agent:amos:subagent:task456",
		},
		{
			session:  Session{Agent: "main", Type: SessionTypeInteractive},
			expected: "agent:main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.session.Key()
			if result != tt.expected {
				t.Errorf("Key() = %s, want %s", result, tt.expected)
			}
		})
	}
}
