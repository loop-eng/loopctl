package parser

import (
	"encoding/json"
	"time"
)

// LTFPhase mirrors the SPEC v1.0 phase enum, but is not restricted to it —
// per the tolerant-reader rule (SPEC §6.5/§7), an unrecognized phase string
// is preserved as-is rather than rejected, so a future taxonomy addition
// doesn't break older loopctl binaries.
type LTFPhase string

const (
	LTFPhasePlan      LTFPhase = "plan"
	LTFPhaseAct       LTFPhase = "act"
	LTFPhaseVerify    LTFPhase = "verify"
	LTFPhaseDecide    LTFPhase = "decide"
	LTFPhaseError     LTFPhase = "error"
	LTFPhaseTerminate LTFPhase = "terminate"
)

// LTFEvent is the normalized result of parsing one line of a
// .loop/trace.ltf.jsonl file — either a phase event or a loop summary,
// distinguished by IsSummary. It deliberately does not implement the
// Parser interface: LTF trace files are shared by every session that has
// ever run in a project directory, not owned by a single session the way
// native Claude/Codex JSONL files are.
type LTFEvent struct {
	IsSummary bool

	LoopID    string
	SessionID string
	Timestamp time.Time

	// Phase-event fields (zero value when IsSummary)
	Iteration int
	Phase     LTFPhase

	// Populated on terminate events and on loop_summary records
	TerminationReason string

	// loop_summary-only fields
	TotalIterations      int
	VerificationPassRate float64
	TestsPassed          int
	TestsFailed          int
}

type ltfRecord struct {
	LTFVersion string `json:"ltf_version"`
	Type       string `json:"type"`
	LoopID     string `json:"loop_id"`
	SessionID  string `json:"session_id"`
	Timestamp  string `json:"timestamp"`

	// Phase event fields
	Phase     string `json:"phase"`
	Iteration int    `json:"iteration"`

	// Loop summary fields
	StartedAt         string          `json:"started_at"`
	EndedAt           string          `json:"ended_at"`
	TotalIterations   int             `json:"total_iterations"`
	TerminationReason string          `json:"termination_reason"`
	Convergence       *ltfConvergence `json:"convergence"`
	Tests             *ltfTests       `json:"tests"`
}

type ltfConvergence struct {
	IterationsToFirstSuccess int     `json:"iterations_to_first_success"`
	VerificationPassRate     float64 `json:"verification_pass_rate"`
	DriftScore               float64 `json:"drift_score"`
}

type ltfTests struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Added  int `json:"added"`
}

// ParseLTFLine parses one line of a .loop/trace.ltf.jsonl file.
// Malformed JSON returns an error; the caller should skip the line and log
// at Debug, matching the existing convention in Collector.processAllTails.
func ParseLTFLine(line []byte) (*LTFEvent, error) {
	var rec ltfRecord
	if err := json.Unmarshal(line, &rec); err != nil {
		return nil, err
	}

	ev := &LTFEvent{
		LoopID:    rec.LoopID,
		SessionID: rec.SessionID,
	}

	if rec.Type == "loop_summary" {
		ev.IsSummary = true
		ev.Timestamp = parseTimestamp(rec.EndedAt)
		ev.TotalIterations = rec.TotalIterations
		ev.TerminationReason = rec.TerminationReason
		if rec.Convergence != nil {
			ev.VerificationPassRate = rec.Convergence.VerificationPassRate
		}
		if rec.Tests != nil {
			ev.TestsPassed = rec.Tests.Passed
			ev.TestsFailed = rec.Tests.Failed
		}
		return ev, nil
	}

	ev.Timestamp = parseTimestamp(rec.Timestamp)
	ev.Iteration = rec.Iteration
	ev.Phase = LTFPhase(rec.Phase)
	if ev.Phase == LTFPhaseTerminate {
		ev.TerminationReason = rec.TerminationReason
	}
	return ev, nil
}
