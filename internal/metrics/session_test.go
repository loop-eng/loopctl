package metrics

import (
	"log/slog"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

func TestSessionStoreInitAndProcess(t *testing.T) {
	ss := NewSessionStore(slog.Default())

	ss.InitSession("s1", "claude", "/tmp/project", 123, true, time.Now().Add(-5*time.Minute))

	ev := &parser.ParsedEvent{
		SessionID:   "s1",
		ContentType: parser.ContentToolUse,
		ToolName:    "Edit",
		ToolInput:   `{"file_path":"/tmp/foo.go"}`,
		Model:       "claude-opus-4-6",
		Timestamp:   time.Now(),
		Tokens: parser.TokenUsage{
			InputTokens:  10000,
			OutputTokens: 5000,
		},
		FilesChanged: []string{"/tmp/foo.go"},
	}

	ss.ProcessEvent("s1", ev)

	snap := ss.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap))
	}

	s := snap[0]
	if s.Model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", s.Model)
	}
	if s.TotalCost == 0 {
		t.Error("expected non-zero cost")
	}
	if s.ToolCallCount != 1 {
		t.Errorf("tool call count = %d, want 1", s.ToolCallCount)
	}
	if s.TotalInput != 10000 {
		t.Errorf("total input = %d, want 10000", s.TotalInput)
	}
	if _, ok := s.FilesChanged["/tmp/foo.go"]; !ok {
		t.Error("expected /tmp/foo.go in files changed")
	}
}

func TestSessionStoreSnapshotCopy(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 0, false, time.Now())

	snap1 := ss.Snapshot()
	snap1[0].TotalCost = 999.99

	snap2 := ss.Snapshot()
	if snap2[0].TotalCost == 999.99 {
		t.Fatal("snapshot should be a copy, not a reference")
	}
}

func TestSessionStoreDailyTotal(t *testing.T) {
	ss := NewSessionStore(slog.Default())

	ss.InitSession("s1", "claude", "/tmp", 0, false, time.Now())
	ev := &parser.ParsedEvent{
		SessionID:   "s1",
		ContentType: parser.ContentText,
		Model:       "claude-opus-4-6",
		Timestamp:   time.Now(),
		Tokens:      parser.TokenUsage{InputTokens: 100000, OutputTokens: 50000},
	}
	ss.ProcessEvent("s1", ev)

	total := ss.DailyTotal()
	if total == 0 {
		t.Error("expected non-zero daily total")
	}
}

func TestSessionStoreTopSessions(t *testing.T) {
	ss := NewSessionStore(slog.Default())

	for i, cost := range []int{10000, 50000, 30000} {
		id := string(rune('a' + i))
		ss.InitSession(id, "claude", "/tmp/"+id, 0, false, time.Now())
		ev := &parser.ParsedEvent{
			SessionID:   id,
			ContentType: parser.ContentText,
			Model:       "claude-opus-4-6",
			Timestamp:   time.Now(),
			Tokens:      parser.TokenUsage{InputTokens: cost},
		}
		ss.ProcessEvent(id, ev)
	}

	top := ss.TopSessions(2)
	if len(top) != 2 {
		t.Fatalf("expected 2 top sessions, got %d", len(top))
	}
	if top[0].Cost < top[1].Cost {
		t.Error("top sessions should be sorted by cost descending")
	}
}

func TestSessionStoreProcessEventMissingSession(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ev := &parser.ParsedEvent{SessionID: "nonexistent"}
	ss.ProcessEvent("nonexistent", ev)
}

func TestSessionStoreInitSessionIdempotent(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 100, true, time.Now())
	ss.InitSession("s1", "claude", "/tmp", 200, true, time.Now())

	snap := ss.Snapshot()
	if len(snap) != 1 {
		t.Fatal("expected 1 session after double init")
	}
	if snap[0].PID != 200 {
		t.Errorf("PID should be updated to 200, got %d", snap[0].PID)
	}
}

func TestProcessEvent_CapsErrorMessages(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 0, true, time.Now())

	for i := 0; i < maxErrorMessages+10; i++ {
		ev := &parser.ParsedEvent{
			SessionID:   "s1",
			ContentType: parser.ContentToolResult,
			IsError:     true,
			ErrorMsg:    string(rune('a' + i%26)),
			Timestamp:   time.Now(),
		}
		ss.ProcessEvent("s1", ev)
	}

	snap := ss.Snapshot()
	if len(snap[0].ErrorMessages) != maxErrorMessages {
		t.Errorf("expected %d capped error messages, got %d", maxErrorMessages, len(snap[0].ErrorMessages))
	}
	if snap[0].ErrorCount != maxErrorMessages+10 {
		t.Errorf("ErrorCount should track all errors uncapped, got %d", snap[0].ErrorCount)
	}
}

func TestSnapshot_ErrorMessagesDefensiveCopy(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 0, true, time.Now())
	ev := &parser.ParsedEvent{
		SessionID:   "s1",
		ContentType: parser.ContentToolResult,
		IsError:     true,
		ErrorMsg:    "boom",
		Timestamp:   time.Now(),
	}
	ss.ProcessEvent("s1", ev)

	snap := ss.Snapshot()
	snap[0].ErrorMessages[0] = "mutated"

	snap2 := ss.Snapshot()
	if snap2[0].ErrorMessages[0] == "mutated" {
		t.Fatal("ErrorMessages should be defensively copied in Snapshot")
	}
}

func TestApplyLTFEvent_PhaseEventUpdatesIteration(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 0, true, time.Now())

	ss.ApplyLTFEvent("s1", &parser.LTFEvent{SessionID: "s1", Phase: parser.LTFPhaseAct, Iteration: 3})

	snap := ss.Snapshot()
	if !snap[0].LTFAvailable {
		t.Error("expected LTFAvailable=true after ApplyLTFEvent")
	}
	if snap[0].LTFIterationCount != 3 {
		t.Errorf("LTFIterationCount = %d, want 3", snap[0].LTFIterationCount)
	}
}

func TestApplyLTFEvent_SummaryOverridesIteration(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 0, true, time.Now())

	ss.ApplyLTFEvent("s1", &parser.LTFEvent{SessionID: "s1", Phase: parser.LTFPhaseAct, Iteration: 3})
	ss.ApplyLTFEvent("s1", &parser.LTFEvent{
		SessionID:            "s1",
		IsSummary:            true,
		TotalIterations:      7,
		TerminationReason:    "goal_met",
		VerificationPassRate: 0.9,
	})

	snap := ss.Snapshot()
	if snap[0].LTFIterationCount != 7 {
		t.Errorf("LTFIterationCount = %d, want 7 (summary should override)", snap[0].LTFIterationCount)
	}
	if snap[0].TerminationReason != "goal_met" {
		t.Errorf("TerminationReason = %q, want goal_met", snap[0].TerminationReason)
	}
	if snap[0].VerificationPassRate != 0.9 {
		t.Errorf("VerificationPassRate = %f, want 0.9", snap[0].VerificationPassRate)
	}
}

func TestApplyLTFEvent_UnknownSessionIsNoOp(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	// Should not panic and should not create a session.
	ss.ApplyLTFEvent("nonexistent", &parser.LTFEvent{SessionID: "nonexistent", Iteration: 1})

	if len(ss.Snapshot()) != 0 {
		t.Error("ApplyLTFEvent should not create a session for an unknown ID")
	}
}

func TestApplyLTFEvent_NeverAffectsCostOrTokens(t *testing.T) {
	ss := NewSessionStore(slog.Default())
	ss.InitSession("s1", "claude", "/tmp", 0, true, time.Now())
	ss.ProcessEvent("s1", &parser.ParsedEvent{
		SessionID:   "s1",
		Model:       "claude-opus-4-6",
		ContentType: parser.ContentText,
		Timestamp:   time.Now(),
		Tokens:      parser.TokenUsage{InputTokens: 1000, OutputTokens: 500},
	})

	before := ss.Snapshot()[0]

	ss.ApplyLTFEvent("s1", &parser.LTFEvent{SessionID: "s1", Phase: parser.LTFPhaseAct, Iteration: 1})

	after := ss.Snapshot()[0]
	if before.TotalCost != after.TotalCost || before.TotalInput != after.TotalInput || before.TotalOutput != after.TotalOutput {
		t.Error("ApplyLTFEvent must never modify cost or token fields")
	}
}
