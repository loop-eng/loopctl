package parser

import (
	"testing"
)

func TestClaudeParserAssistant(t *testing.T) {
	line := []byte(`{"type":"assistant","uuid":"u1","requestId":"r1","sessionId":"s1","timestamp":"2026-07-15T10:00:00Z","message":{"role":"assistant","model":"claude-opus-4-6","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/tmp/foo.go"}}],"usage":{"input_tokens":1000,"output_tokens":500,"cache_read_input_tokens":200,"cache_creation_input_tokens":100}}}`)

	p := NewClaudeParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", ev.Model)
	}
	if ev.ContentType != ContentToolUse {
		t.Errorf("content type = %d, want ContentToolUse", ev.ContentType)
	}
	if ev.ToolName != "Edit" {
		t.Errorf("tool name = %q, want Edit", ev.ToolName)
	}
	if ev.Tokens.InputTokens != 1000 {
		t.Errorf("input tokens = %d, want 1000", ev.Tokens.InputTokens)
	}
	if ev.Tokens.OutputTokens != 500 {
		t.Errorf("output tokens = %d, want 500", ev.Tokens.OutputTokens)
	}
	if len(ev.FilesChanged) != 1 || ev.FilesChanged[0] != "/tmp/foo.go" {
		t.Errorf("files changed = %v, want [/tmp/foo.go]", ev.FilesChanged)
	}
}

func TestClaudeParserDedup(t *testing.T) {
	line := []byte(`{"type":"assistant","requestId":"r1","sessionId":"s1","timestamp":"2026-07-15T10:00:00Z","message":{"role":"assistant","model":"claude-opus-4-6","content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	p := NewClaudeParser()

	events1, _ := p.Parse(line)
	if len(events1) != 1 || events1[0].Tokens.InputTokens != 100 {
		t.Fatal("first parse should include tokens")
	}

	events2, _ := p.Parse(line)
	if len(events2) != 1 || events2[0].Tokens.InputTokens != 0 {
		t.Fatal("second parse of same requestId should have zero tokens")
	}
}

func TestClaudeParserUser(t *testing.T) {
	line := []byte(`{"type":"user","uuid":"u2","sessionId":"s1","timestamp":"2026-07-15T10:01:00Z","message":{"role":"user","content":[{"type":"tool_result","content":"build failed: exit code 1","is_error":true}]}}`)

	p := NewClaudeParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.ContentType != ContentToolResult {
		t.Errorf("content type = %d, want ContentToolResult", ev.ContentType)
	}
	if !ev.IsError {
		t.Error("expected IsError=true")
	}
	if ev.ErrorMsg != "build failed: exit code 1" {
		t.Errorf("error msg = %q", ev.ErrorMsg)
	}
}

func TestClaudeParserUnknownType(t *testing.T) {
	line := []byte(`{"type":"system","timestamp":"2026-07-15T10:00:00Z"}`)

	p := NewClaudeParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for unknown type, got %d", len(events))
	}
}

func TestClaudeParserInvalidJSON(t *testing.T) {
	p := NewClaudeParser()
	_, err := p.Parse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClaudeParserMultipleContentBlocks(t *testing.T) {
	line := []byte(`{"type":"assistant","requestId":"r1","sessionId":"s1","timestamp":"2026-07-15T10:00:00Z","message":{"role":"assistant","model":"claude-opus-4-6","content":[{"type":"thinking"},{"type":"tool_use","name":"Bash","input":{"command":"ls"}}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	p := NewClaudeParser()
	events, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Tokens.InputTokens != 100 {
		t.Error("first block should have tokens")
	}
	if events[1].Tokens.InputTokens != 0 {
		t.Error("second block should have zero tokens")
	}
}

func TestIsFileModifyingTool(t *testing.T) {
	if !IsFileModifyingTool("Edit") {
		t.Error("Edit should be file-modifying")
	}
	if !IsFileModifyingTool("Write") {
		t.Error("Write should be file-modifying")
	}
	if IsFileModifyingTool("Read") {
		t.Error("Read should not be file-modifying")
	}
}

func TestIsTestCommand(t *testing.T) {
	if !IsTestCommand(`{"command":"npm test"}`) {
		t.Error("npm test should match")
	}
	if !IsTestCommand(`{"command":"go test ./..."}`) {
		t.Error("go test should match")
	}
	if IsTestCommand(`{"command":"echo hello"}`) {
		t.Error("echo hello should not match")
	}
}
