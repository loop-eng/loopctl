package parser

import (
	"encoding/json"
	"fmt"
)

type CodexParser struct {
	seenRequests map[string]bool
	seenCount    int
}

func NewCodexParser() *CodexParser {
	return &CodexParser{
		seenRequests: make(map[string]bool, 256),
	}
}

type codexEntry struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type codexToolCall struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

type codexInference struct {
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	ReasoningOut int    `json:"reasoning_output_tokens"`
	TokenCount   int    `json:"token_count"`
}

type codexToolResult struct {
	Output  string `json:"output"`
	IsError bool   `json:"is_error"`
}

func (p *CodexParser) Parse(line []byte) ([]*ParsedEvent, error) {
	var entry codexEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil, err
	}

	ts := parseTimestamp(entry.Timestamp)

	switch entry.Type {
	case "tool_call_started":
		var tc codexToolCall
		if err := json.Unmarshal(entry.Data, &tc); err != nil {
			return nil, fmt.Errorf("codex tool_call_started data: %w", err)
		}
		return []*ParsedEvent{{
			SessionID:    entry.SessionID,
			UUID:         entry.ID,
			Timestamp:    ts,
			EntryType:    "assistant",
			ContentType:  ContentToolUse,
			ToolName:     tc.Name,
			ToolInput:    tc.Input,
			FilesChanged: extractFilesChanged(tc.Name, parseInputMap(tc.Input)),
		}}, nil

	case "tool_call_ended":
		var tr codexToolResult
		if err := json.Unmarshal(entry.Data, &tr); err != nil {
			return nil, fmt.Errorf("codex tool_call_ended data: %w", err)
		}
		ev := &ParsedEvent{
			SessionID:   entry.SessionID,
			UUID:        entry.ID,
			Timestamp:   ts,
			EntryType:   "user",
			ContentType: ContentToolResult,
			ToolResult:  tr.Output,
			IsError:     tr.IsError,
		}
		if tr.IsError {
			ev.ErrorMsg = tr.Output
		}
		return []*ParsedEvent{ev}, nil

	case "inference_completed":
		var inf codexInference
		if err := json.Unmarshal(entry.Data, &inf); err != nil {
			return nil, fmt.Errorf("codex inference_completed data: %w", err)
		}

		rid := entry.ID
		if rid != "" && p.seenRequests[rid] {
			return nil, nil
		}
		if rid != "" {
			p.seenRequests[rid] = true
			p.seenCount++
			if p.seenCount > maxSeenRequests {
				p.seenRequests = make(map[string]bool, 256)
				p.seenCount = 0
			}
		}

		return []*ParsedEvent{{
			SessionID:   entry.SessionID,
			UUID:        entry.ID,
			Timestamp:   ts,
			EntryType:   "assistant",
			Model:       inf.Model,
			ContentType: ContentText,
			Tokens: TokenUsage{
				InputTokens:  inf.InputTokens,
				OutputTokens: inf.OutputTokens + inf.ReasoningOut,
			},
		}}, nil

	default:
		return nil, nil
	}
}

func parseInputMap(input string) any {
	var m map[string]any
	json.Unmarshal([]byte(input), &m)
	return m
}
