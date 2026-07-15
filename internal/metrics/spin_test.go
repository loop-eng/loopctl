package metrics

import (
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

func TestSpinDetectorRepeatedTools(t *testing.T) {
	sd := NewSpinDetector(DefaultSpinConfig())

	ev := &parser.ParsedEvent{
		ContentType: parser.ContentToolUse,
		ToolName:    "Edit",
		ToolInput:   `{"file_path":"/tmp/foo.go","old_string":"a","new_string":"b"}`,
		Timestamp:   time.Now(),
	}

	for i := 0; i < 3; i++ {
		result := sd.Check(ev, 0)
		if i < 2 && result.IsSpinning {
			t.Fatalf("should not spin at %d repetitions", i+1)
		}
		if i == 2 && !result.IsSpinning {
			t.Fatal("should spin at 3 repetitions")
		}
	}
}

func TestSpinDetectorErrorEcho(t *testing.T) {
	sd := NewSpinDetector(DefaultSpinConfig())

	ev := &parser.ParsedEvent{
		ContentType: parser.ContentToolResult,
		IsError:     true,
		ErrorMsg:    "command not found: xyz",
		Timestamp:   time.Now(),
	}

	for i := 0; i < 3; i++ {
		result := sd.Check(ev, 0)
		if i < 2 && result.IsSpinning {
			t.Fatalf("should not spin at %d errors", i+1)
		}
		if i == 2 && !result.IsSpinning {
			t.Fatal("should spin at 3 repeated errors")
		}
	}
}

func TestSpinDetectorCostVelocity(t *testing.T) {
	sd := NewSpinDetector(DefaultSpinConfig())
	base := time.Now()

	for i := 0; i < 5; i++ {
		ev := &parser.ParsedEvent{
			ContentType: parser.ContentText,
			Timestamp:   base.Add(time.Duration(i) * time.Minute),
			Tokens:      parser.TokenUsage{InputTokens: 1000},
		}
		cost := float64(i) * 3.0
		result := sd.Check(ev, cost)

		if i >= 2 && result.IsSpinning && result.Heuristic == "cost_velocity" {
			return
		}
	}
	t.Fatal("cost velocity should have triggered")
}

func TestSpinDetectorNoFalsePositive(t *testing.T) {
	sd := NewSpinDetector(DefaultSpinConfig())

	tools := []string{"Edit", "Bash", "Read", "Write", "Grep"}
	for i, name := range tools {
		ev := &parser.ParsedEvent{
			ContentType: parser.ContentToolUse,
			ToolName:    name,
			ToolInput:   `{"unique":"` + name + `"}`,
			Timestamp:   time.Now().Add(time.Duration(i) * time.Second),
		}
		result := sd.Check(ev, float64(i)*0.1)
		if result.IsSpinning {
			t.Fatalf("false positive for diverse tool calls at tool %s", name)
		}
	}
}

func TestSpinDetectorStall(t *testing.T) {
	cfg := DefaultSpinConfig()
	cfg.StallMinutes = 1
	sd := NewSpinDetector(cfg)

	base := time.Now()

	editEv := &parser.ParsedEvent{
		ContentType: parser.ContentToolUse,
		ToolName:    "Edit",
		ToolInput:   `{"file_path":"/a"}`,
		Timestamp:   base,
		Tokens:      parser.TokenUsage{InputTokens: 100},
	}
	sd.Check(editEv, 0)

	readEv := &parser.ParsedEvent{
		ContentType: parser.ContentToolUse,
		ToolName:    "Read",
		ToolInput:   `{"file_path":"/b"}`,
		Timestamp:   base.Add(2 * time.Minute),
		Tokens:      parser.TokenUsage{InputTokens: 100},
	}
	result := sd.Check(readEv, 0)

	hasStallWarning := false
	for _, r := range result.Reasons {
		if len(r) > 0 {
			hasStallWarning = true
		}
	}
	if !hasStallWarning {
		t.Fatal("expected stall warning after 2 minutes with no file edits")
	}
}
