package parser

import (
	"testing"
)

func TestCodexParserToolCallStarted(t *testing.T) {
	line := []byte(`{"type":"tool_call_started","id":"c1","session_id":"s1","timestamp":"2026-07-15T10:00:00Z","data":{"name":"shell","input":"{\"command\":\"ls\"}"}}`)

	p := NewCodexParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ContentType != ContentToolUse {
		t.Errorf("expected ContentToolUse, got %d", events[0].ContentType)
	}
	if events[0].ToolName != "shell" {
		t.Errorf("tool name = %q, want shell", events[0].ToolName)
	}
}

func TestCodexParserToolCallEnded(t *testing.T) {
	line := []byte(`{"type":"tool_call_ended","id":"c1","session_id":"s1","timestamp":"2026-07-15T10:00:00Z","data":{"output":"file not found","is_error":true}}`)

	p := NewCodexParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].IsError {
		t.Error("expected IsError=true")
	}
	if events[0].ErrorMsg != "file not found" {
		t.Errorf("error msg = %q", events[0].ErrorMsg)
	}
}

func TestCodexParserInference(t *testing.T) {
	line := []byte(`{"type":"inference_completed","id":"i1","session_id":"s1","timestamp":"2026-07-15T10:00:00Z","data":{"model":"o4-mini","input_tokens":500,"output_tokens":200,"reasoning_output_tokens":50}}`)

	p := NewCodexParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Model != "o4-mini" {
		t.Errorf("model = %q, want o4-mini", ev.Model)
	}
	if ev.Tokens.InputTokens != 500 {
		t.Errorf("input = %d, want 500", ev.Tokens.InputTokens)
	}
	if ev.Tokens.OutputTokens != 250 {
		t.Errorf("output = %d, want 250 (200+50)", ev.Tokens.OutputTokens)
	}
}

func TestCodexParserDedup(t *testing.T) {
	line := []byte(`{"type":"inference_completed","id":"i1","session_id":"s1","timestamp":"2026-07-15T10:00:00Z","data":{"model":"o4-mini","input_tokens":500,"output_tokens":200}}`)

	p := NewCodexParser()
	events1, _ := p.Parse(line)
	if len(events1) != 1 {
		t.Fatal("first parse should return event")
	}

	events2, _ := p.Parse(line)
	if events2 != nil {
		t.Fatal("second parse of same id should return nil")
	}
}

func TestCodexParserUnknownType(t *testing.T) {
	line := []byte(`{"type":"unknown_event","id":"x1","timestamp":"2026-07-15T10:00:00Z","data":{}}`)

	p := NewCodexParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Fatal("expected nil for unknown type")
	}
}
