// Package parser handles parsing of OpenClaw session transcripts.
package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SessionKey formats:
//   - agent:{name}:cron:{id}:run:{sid} → cron job
//   - agent:{name}:subagent:{sid} → sub-agent
//   - agent:{name} → interactive

// Message represents a single event in a session transcript.
type Message struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			Input      int `json:"input"`
			Output     int `json:"output"`
			Total      int `json:"totalTokens"`
			CacheRead  int `json:"cacheRead"`
			CacheWrite int `json:"cacheWrite"`
			Cost       struct {
				Input      float64 `json:"input"`
				Output     float64 `json:"output"`
				CacheRead  float64 `json:"cacheRead"`
				CacheWrite float64 `json:"cacheWrite"`
				Total      float64 `json:"total"`
			} `json:"cost"`
		} `json:"usage"`
		Model string `json:"model"`
	} `json:"message"`
	Model string `json:"model"`
}

// Usage contains token and cost information.
type Usage struct {
	Input       int
	Output      int
	Total       int
	CacheRead   int
	CacheWrite  int
	CostInput   float64
	CostOutput  float64
	CostTotal   float64
	Model       string
}

// SessionType categorizes the session.
type SessionType string

const (
	SessionTypeInteractive SessionType = "interactive"
	SessionTypeCron        SessionType = "cron"
	SessionTypeSubagent    SessionType = "subagent"
)

// Session represents a parsed session with all its messages and metadata.
type Session struct {
	ID         string
	Agent      string
	Type       SessionType
	CronID     string // For cron sessions
	CronName   string // For cron sessions (derived from cron ID)
	SubagentID string // For subagent sessions
	FilePath   string
	Messages   []Message
	Usage      Usage
	StartedAt  time.Time
	Duration   time.Duration
}

// Parser handles parsing of session files.
type Parser struct {
	agentsDir string
}

// New creates a new Parser.
func New(agentsDir string) *Parser {
	return &Parser{agentsDir: agentsDir}
}

// ListAgents returns a list of available agents.
func (p *Parser) ListAgents() ([]string, error) {
	entries, err := os.ReadDir(p.agentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read agents directory: %w", err)
	}

	var agents []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it has a sessions directory
			sessionsDir := filepath.Join(p.agentsDir, entry.Name(), "sessions")
			if _, err := os.Stat(sessionsDir); err == nil {
				agents = append(agents, entry.Name())
			}
		}
	}

	return agents, nil
}

// ParseAll parses all sessions for all agents or a specific agent.
func (p *Parser) ParseAll(agentFilter string) ([]Session, error) {
	var sessions []Session

	agents, err := p.ListAgents()
	if err != nil {
		return nil, err
	}

	for _, agent := range agents {
		if agentFilter != "" && agent != agentFilter {
			continue
		}

		agentSessions, err := p.parseAgentSessions(agent)
		if err != nil {
			// Log error but continue with other agents
			fmt.Fprintf(os.Stderr, "Warning: failed to parse sessions for agent %s: %v\n", agent, err)
			continue
		}

		sessions = append(sessions, agentSessions...)
	}

	return sessions, nil
}

// parseAgentSessions parses all sessions for a specific agent.
func (p *Parser) parseAgentSessions(agent string) ([]Session, error) {
	sessionsDir := filepath.Join(p.agentsDir, agent, "sessions")

	// Read session index if available
	indexPath := filepath.Join(sessionsDir, "sessions.json")
	sessionIndex := make(map[string]SessionIndexEntry)
	if data, err := os.ReadFile(indexPath); err == nil {
		var index map[string]interface{}
		if err := json.Unmarshal(data, &index); err == nil {
			for key, val := range index {
				if entryMap, ok := val.(map[string]interface{}); ok {
					entry := SessionIndexEntry{Key: key}
					if id, ok := entryMap["sessionId"].(string); ok {
						entry.SessionID = id
					}
					if ts, ok := entryMap["updatedAt"].(float64); ok {
						entry.UpdatedAt = int64(ts)
					}
					sessionIndex[key] = entry
				}
			}
		}
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		filePath := filepath.Join(sessionsDir, entry.Name())

		session, err := p.parseSessionFile(agent, sessionID, filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse session %s: %v\n", filePath, err)
			continue
		}

		// Try to get additional metadata from index
		if indexEntry, ok := sessionIndex[session.Key()]; ok {
			session.StartedAt = time.UnixMilli(indexEntry.UpdatedAt)
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// SessionIndexEntry represents an entry in sessions.json.
type SessionIndexEntry struct {
	Key       string
	SessionID string
	UpdatedAt int64
}

// parseSessionFile parses a single session file.
func (p *Parser) parseSessionFile(agent, sessionID, filePath string) (Session, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	session := Session{
		ID:       sessionID,
		Agent:    agent,
		FilePath: filePath,
		Messages: []Message{},
	}

	// Parse session type from session ID format
	session.parseSessionKey(sessionID)

	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle long lines (up to 10MB)
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	var firstTimestamp, lastTimestamp time.Time

	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			// Skip malformed lines
			continue
		}

		// Only process assistant messages with usage
		if msg.Type == "message" && msg.Message.Role == "assistant" {
			session.Messages = append(session.Messages, msg)

			// Track timestamps
			if !msg.Timestamp.IsZero() {
				if firstTimestamp.IsZero() {
					firstTimestamp = msg.Timestamp
				}
				lastTimestamp = msg.Timestamp
			}

			// Aggregate usage
			session.Usage.Input += msg.Message.Usage.Input
			session.Usage.Output += msg.Message.Usage.Output
			session.Usage.Total += msg.Message.Usage.Total
			session.Usage.CacheRead += msg.Message.Usage.CacheRead
			session.Usage.CacheWrite += msg.Message.Usage.CacheWrite
			session.Usage.CostInput += msg.Message.Usage.Cost.Input
			session.Usage.CostOutput += msg.Message.Usage.Cost.Output
			session.Usage.CostTotal += msg.Message.Usage.Cost.Total

			// Track model
			if msg.Message.Model != "" {
				session.Usage.Model = msg.Message.Model
			} else if msg.Model != "" {
				session.Usage.Model = msg.Model
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return session, err
	}

	if !firstTimestamp.IsZero() && !lastTimestamp.IsZero() {
		session.StartedAt = firstTimestamp
		session.Duration = lastTimestamp.Sub(firstTimestamp)
	}

	return session, nil
}

// Key returns the full session key for index lookup.
func (s *Session) Key() string {
	switch s.Type {
	case SessionTypeCron:
		return fmt.Sprintf("agent:%s:cron:%s:run:%s", s.Agent, s.CronID, s.ID)
	case SessionTypeSubagent:
		return fmt.Sprintf("agent:%s:subagent:%s", s.Agent, s.ID)
	default:
		return fmt.Sprintf("agent:%s", s.Agent)
	}
}

// Session key parsing regex
var (
	cronPattern        = regexp.MustCompile(`^agent:([^:]+):cron:([^:]+):run:(.+)$`)
	subagentPattern    = regexp.MustCompile(`^agent:([^:]+):subagent:(.+)$`)
	interactivePattern = regexp.MustCompile(`^agent:([^:]+)$`)
)

// parseSessionKey parses the session key to extract metadata.
func (s *Session) parseSessionKey(sessionID string) {
	// Try cron pattern: agent:{name}:cron:{id}:run:{sid}
	if matches := cronPattern.FindStringSubmatch(sessionID); len(matches) == 4 {
		s.Type = SessionTypeCron
		s.CronID = matches[2]
		s.CronName = deriveCronName(s.CronID)
		return
	}

	// Try subagent pattern: agent:{name}:subagent:{sid}
	if matches := subagentPattern.FindStringSubmatch(sessionID); len(matches) == 3 {
		s.Type = SessionTypeSubagent
		s.SubagentID = matches[2]
		return
	}

	// Try interactive pattern: agent:{name}
	if matches := interactivePattern.FindStringSubmatch(sessionID); len(matches) == 2 {
		s.Type = SessionTypeInteractive
		return
	}

	// Default to interactive if we can't parse
	s.Type = SessionTypeInteractive
}

// deriveCronName attempts to extract a readable name from a cron ID.
func deriveCronName(cronID string) string {
	// Common patterns:
	// - daily-kickoff-abc123 → daily-kickoff
	// - code-reviewer-xyz789 → code-reviewer

	// Try to split on common separators
	parts := strings.Split(cronID, "-")
	if len(parts) > 1 {
		// Check if last part looks like a hash (length > 6 and alphanumeric)
		last := parts[len(parts)-1]
		if len(last) >= 6 && isAlphaNumeric(last) {
			return strings.Join(parts[:len(parts)-1], "-")
		}
	}

	return cronID
}

func isAlphaNumeric(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return len(s) > 0
}

// FormatCost formats a cost value for display.
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// FormatTokens formats a token count for display.
func FormatTokens(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return strconv.Itoa(tokens)
}
