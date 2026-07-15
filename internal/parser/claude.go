package parser

import (
	"encoding/json"
	"strings"
	"time"
)

type ClaudeParser struct {
	currentGen   map[string]bool
	previousGen  map[string]bool
	currentCount int
}

const maxSeenRequests = 10000

func NewClaudeParser() *ClaudeParser {
	return &ClaudeParser{
		currentGen:  make(map[string]bool, 256),
		previousGen: make(map[string]bool),
	}
}

type claudeEntry struct {
	Type      string    `json:"type"`
	UUID      string    `json:"uuid"`
	RequestID string    `json:"requestId"`
	SessionID string    `json:"sessionId"`
	Timestamp string    `json:"timestamp"`
	Message   claudeMsg `json:"message"`
}

type claudeMsg struct {
	Role    string            `json:"role"`
	Model   string            `json:"model"`
	Content []json.RawMessage `json:"content"`
	Usage   *claudeUsage      `json:"usage"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type claudeContent struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	ID        string `json:"id"`
	ToolUseID string `json:"tool_use_id"`
	Text      string `json:"text"`
	Content   any    `json:"content"`
	Input     any    `json:"input"`
	IsError   bool   `json:"is_error"`
}

func (p *ClaudeParser) Parse(line []byte) ([]*ParsedEvent, error) {
	var entry claudeEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil, err
	}

	switch entry.Type {
	case "assistant":
		return p.parseAssistant(&entry)
	case "user":
		return p.parseUser(&entry)
	default:
		return nil, nil
	}
}

func (p *ClaudeParser) parseAssistant(entry *claudeEntry) ([]*ParsedEvent, error) {
	ts := parseTimestamp(entry.Timestamp)

	var tokens TokenUsage
	countTokens := false
	if entry.RequestID != "" && !p.currentGen[entry.RequestID] && !p.previousGen[entry.RequestID] {
		p.currentGen[entry.RequestID] = true
		p.currentCount++
		countTokens = true

		if p.currentCount > maxSeenRequests {
			p.previousGen = p.currentGen
			p.currentGen = make(map[string]bool, 256)
			p.currentCount = 0
		}
	}
	if countTokens && entry.Message.Usage != nil {
		tokens = TokenUsage{
			InputTokens:      entry.Message.Usage.InputTokens,
			OutputTokens:     entry.Message.Usage.OutputTokens,
			CacheReadTokens:  entry.Message.Usage.CacheReadInputTokens,
			CacheWriteTokens: entry.Message.Usage.CacheCreationInputTokens,
		}
	}

	var events []*ParsedEvent
	tokensAssigned := false
	for _, raw := range entry.Message.Content {
		var c claudeContent
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}

		ev := &ParsedEvent{
			SessionID: entry.SessionID,
			RequestID: entry.RequestID,
			UUID:      entry.UUID,
			Timestamp: ts,
			EntryType: "assistant",
			Model:     entry.Message.Model,
		}
		if !tokensAssigned {
			ev.Tokens = tokens
			tokensAssigned = true
		}

		switch c.Type {
		case "tool_use":
			ev.ContentType = ContentToolUse
			ev.ToolName = c.Name
			if inputBytes, err := json.Marshal(c.Input); err == nil {
				ev.ToolInput = string(inputBytes)
			}
			ev.FilesChanged = extractFilesChanged(c.Name, c.Input)
		case "text":
			ev.ContentType = ContentText
		case "thinking":
			ev.ContentType = ContentThinking
		default:
			ev.ContentType = ContentUnknown
		}

		events = append(events, ev)
	}

	return events, nil
}

func (p *ClaudeParser) parseUser(entry *claudeEntry) ([]*ParsedEvent, error) {
	ts := parseTimestamp(entry.Timestamp)
	var events []*ParsedEvent

	for _, raw := range entry.Message.Content {
		var c claudeContent
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}

		if c.Type != "tool_result" {
			continue
		}

		ev := &ParsedEvent{
			SessionID:   entry.SessionID,
			UUID:        entry.UUID,
			Timestamp:   ts,
			EntryType:   "user",
			ContentType: ContentToolResult,
			ToolResult:  contentToString(c.Content),
			IsError:     c.IsError,
		}

		if c.IsError {
			ev.ErrorMsg = contentToString(c.Content)
		}

		events = append(events, ev)
	}

	return events, nil
}

func extractFilesChanged(toolName string, input any) []string {
	inputMap, ok := input.(map[string]any)
	if !ok {
		return nil
	}
	switch toolName {
	case "Edit", "Write":
		if fp, ok := inputMap["file_path"].(string); ok {
			return []string{fp}
		}
	case "NotebookEdit":
		if fp, ok := inputMap["notebook_path"].(string); ok {
			return []string{fp}
		}
	}
	return nil
}

func IsFileModifyingTool(name string) bool {
	switch name {
	case "Edit", "Write", "NotebookEdit":
		return true
	}
	return false
}

func IsTestCommand(toolInput string) bool {
	lower := strings.ToLower(toolInput)
	testPatterns := []string{
		"npm test", "npx jest", "npx vitest", "yarn test",
		"pytest", "python -m pytest", "python -m unittest",
		"go test", "cargo test", "make test",
		"rspec", "bundle exec rspec",
	}
	for _, p := range testPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Now()
	}
	return t
}

func contentToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
