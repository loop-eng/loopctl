package parser

import "testing"

func TestParsePhaseEvent(t *testing.T) {
	line := []byte(`{"ltf_version":"1.0","loop_id":"loop-1","session_id":"sess-1","timestamp":"2026-07-15T10:00:00Z","phase":"act","iteration":2}`)
	ev, err := ParseLTFLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.IsSummary {
		t.Error("expected IsSummary=false for a phase event")
	}
	if ev.LoopID != "loop-1" || ev.SessionID != "sess-1" {
		t.Errorf("loop_id/session_id not parsed correctly: %+v", ev)
	}
	if ev.Phase != LTFPhaseAct {
		t.Errorf("phase = %q, want act", ev.Phase)
	}
	if ev.Iteration != 2 {
		t.Errorf("iteration = %d, want 2", ev.Iteration)
	}
}

func TestParseTerminateEvent(t *testing.T) {
	line := []byte(`{"ltf_version":"1.0","loop_id":"loop-1","session_id":"sess-1","timestamp":"2026-07-15T10:05:00Z","phase":"terminate","termination_reason":"goal_met"}`)
	ev, err := ParseLTFLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.TerminationReason != "goal_met" {
		t.Errorf("termination reason = %q, want goal_met", ev.TerminationReason)
	}
}

func TestParseLoopSummary(t *testing.T) {
	line := []byte(`{"ltf_version":"1.0","type":"loop_summary","loop_id":"loop-1","session_id":"sess-1","started_at":"2026-07-15T10:00:00Z","ended_at":"2026-07-15T10:10:00Z","total_iterations":4,"termination_reason":"goal_met","convergence":{"verification_pass_rate":0.75}}`)
	ev, err := ParseLTFLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev.IsSummary {
		t.Error("expected IsSummary=true for a loop_summary record")
	}
	if ev.TotalIterations != 4 {
		t.Errorf("total_iterations = %d, want 4", ev.TotalIterations)
	}
	if ev.TerminationReason != "goal_met" {
		t.Errorf("termination reason = %q, want goal_met", ev.TerminationReason)
	}
	if ev.VerificationPassRate != 0.75 {
		t.Errorf("verification pass rate = %f, want 0.75", ev.VerificationPassRate)
	}
}

func TestParseUnknownPhaseIsTolerant(t *testing.T) {
	line := []byte(`{"ltf_version":"1.0","loop_id":"loop-1","session_id":"sess-1","timestamp":"2026-07-15T10:00:00Z","phase":"some_future_phase"}`)
	ev, err := ParseLTFLine(line)
	if err != nil {
		t.Fatalf("tolerant reader should not error on unknown phase: %v", err)
	}
	if ev.Phase != "some_future_phase" {
		t.Errorf("expected unknown phase preserved as-is, got %q", ev.Phase)
	}
}

func TestParseMissingOptionalFields(t *testing.T) {
	line := []byte(`{"ltf_version":"1.0","loop_id":"loop-1","timestamp":"2026-07-15T10:00:00Z","phase":"plan"}`)
	ev, err := ParseLTFLine(line)
	if err != nil {
		t.Fatalf("unexpected error on minimal required-only event: %v", err)
	}
	if ev.SessionID != "" {
		t.Errorf("expected zero-value SessionID, got %q", ev.SessionID)
	}
	if ev.Iteration != 0 {
		t.Errorf("expected zero-value Iteration, got %d", ev.Iteration)
	}
}

func TestParseMalformedJSON(t *testing.T) {
	_, err := ParseLTFLine([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
