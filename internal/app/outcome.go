package app

import "github.com/loop-eng/loopctl/internal/model"

type outcome string

const (
	outcomeDone   outcome = "Done"
	outcomeFailed outcome = "Failed"
	outcomeKilled outcome = "Killed"
	outcomeWarned outcome = "Warned"
)

// classifyOutcome derives a retrospective outcome label for a completed
// session. killed is best-effort: it records that loopctl sent SIGTERM,
// not that the process actually died as a result — the two are
// indistinguishable without polling exit codes, which loopctl doesn't do.
func classifyOutcome(s model.SessionView, killed map[string]bool) outcome {
	if killed[s.SessionID] {
		return outcomeKilled
	}
	if s.ErrorCount > 0 {
		return outcomeFailed
	}
	if s.HasWarnings || s.IsSpinning {
		return outcomeWarned
	}
	return outcomeDone
}
