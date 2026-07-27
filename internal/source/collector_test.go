package source

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/config"
	"github.com/loop-eng/loopctl/internal/metrics"
	"github.com/loop-eng/loopctl/internal/parser"
)

// newTestCollector builds a Collector without wiring real discoverers, for
// whitebox testing of LTF demuxing logic in isolation.
func newTestCollector(t *testing.T) *Collector {
	t.Helper()
	return &Collector{
		logger:             slog.Default(),
		registry:           NewRegistry(),
		store:              metrics.NewSessionStore(slog.Default()),
		cfg:                config.Default(),
		tailers:            make(map[string]*Tailer),
		parsers:            make(map[string]parser.Parser),
		ltfEnricher:        NewLTFEnricher(slog.Default()),
		ltfTailers:         make(map[string]*Tailer),
		alertedTermination: make(map[string]bool),
	}
}

func writeLTFTrace(t *testing.T, dir string, lines []string) string {
	t.Helper()
	loopDir := filepath.Join(dir, ".loop")
	if err := os.MkdirAll(loopDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(loopDir, "trace.ltf.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLTFDemuxTwoSessionsSameProject(t *testing.T) {
	dir := t.TempDir()
	path := writeLTFTrace(t, dir, []string{
		`{"ltf_version":"1.0","loop_id":"loop-a","session_id":"session-a","timestamp":"2026-07-15T10:00:00Z","phase":"act","iteration":1}`,
		`{"ltf_version":"1.0","loop_id":"loop-a","session_id":"session-a","timestamp":"2026-07-15T10:01:00Z","phase":"act","iteration":2}`,
		`{"ltf_version":"1.0","loop_id":"loop-b","session_id":"session-b","timestamp":"2026-07-15T10:02:00Z","phase":"act","iteration":1}`,
	})

	c := newTestCollector(t)
	c.store.InitSession("session-a", "claude", dir, 0, false, time.Now())
	c.store.InitSession("session-b", "claude", dir, 0, false, time.Now())
	c.ltfTailers[dir] = NewTailer(path)

	c.processLTFTails()

	snap := c.Snapshot()
	byID := map[string]int{}
	for _, s := range snap.Sessions {
		byID[s.SessionID] = s.LTFIterationCount
	}
	if byID["session-a"] != 2 {
		t.Errorf("session-a LTF iteration count = %d, want 2", byID["session-a"])
	}
	if byID["session-b"] != 1 {
		t.Errorf("session-b LTF iteration count = %d, want 1", byID["session-b"])
	}
}

func TestLTFEventForUnknownSessionDropped(t *testing.T) {
	dir := t.TempDir()
	path := writeLTFTrace(t, dir, []string{
		`{"ltf_version":"1.0","loop_id":"loop-a","session_id":"session-unregistered","timestamp":"2026-07-15T10:00:00Z","phase":"act","iteration":1}`,
	})

	c := newTestCollector(t)
	c.ltfTailers[dir] = NewTailer(path)

	// Must not panic, and must not create a phantom session.
	c.processLTFTails()

	snap := c.Snapshot()
	if len(snap.Sessions) != 0 {
		t.Errorf("expected no sessions created from an unregistered LTF event, got %d", len(snap.Sessions))
	}
}

func TestLTFDoesNotAffectCost(t *testing.T) {
	dir := t.TempDir()
	path := writeLTFTrace(t, dir, []string{
		`{"ltf_version":"1.0","type":"loop_summary","loop_id":"loop-a","session_id":"session-a","started_at":"2026-07-15T10:00:00Z","ended_at":"2026-07-15T10:10:00Z","total_iterations":3,"termination_reason":"goal_met"}`,
	})

	c := newTestCollector(t)
	c.store.InitSession("session-a", "claude", dir, 0, false, time.Now())

	ev := &parser.ParsedEvent{
		SessionID:   "session-a",
		Model:       "claude-opus-4-6",
		ContentType: parser.ContentText,
		Timestamp:   time.Now(),
		Tokens:      parser.TokenUsage{InputTokens: 1000, OutputTokens: 500},
	}
	c.store.ProcessEvent("session-a", ev)

	before := c.Snapshot().Sessions[0]

	c.ltfTailers[dir] = NewTailer(path)
	c.processLTFTails()

	after := c.Snapshot().Sessions[0]

	if before.TotalCost != after.TotalCost {
		t.Errorf("LTF processing changed TotalCost: %.4f -> %.4f", before.TotalCost, after.TotalCost)
	}
	if before.TotalInput != after.TotalInput || before.TotalOutput != after.TotalOutput {
		t.Errorf("LTF processing changed token counts: input %d->%d, output %d->%d",
			before.TotalInput, after.TotalInput, before.TotalOutput, after.TotalOutput)
	}
	if !after.LTFAvailable {
		t.Error("expected LTFAvailable=true after applying an LTF event")
	}
}

func TestLTFTerminationReasonSurfaced(t *testing.T) {
	dir := t.TempDir()
	path := writeLTFTrace(t, dir, []string{
		`{"ltf_version":"1.0","type":"loop_summary","loop_id":"loop-a","session_id":"session-a","started_at":"2026-07-15T10:00:00Z","ended_at":"2026-07-15T10:10:00Z","total_iterations":5,"termination_reason":"goal_met"}`,
	})

	c := newTestCollector(t)
	c.store.InitSession("session-a", "claude", dir, 0, false, time.Now())
	c.ltfTailers[dir] = NewTailer(path)

	c.processLTFTails()

	snap := c.Snapshot()
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap.Sessions))
	}
	if snap.Sessions[0].TerminationReason != "goal_met" {
		t.Errorf("TerminationReason = %q, want goal_met", snap.Sessions[0].TerminationReason)
	}
	if snap.Sessions[0].LTFIterationCount != 5 {
		t.Errorf("LTFIterationCount = %d, want 5", snap.Sessions[0].LTFIterationCount)
	}
}

func TestRunDiscoveryPrunesStaleLTFTailers(t *testing.T) {
	c := newTestCollector(t)
	c.ltfTailers["/some/stale/project"] = NewTailer("/some/stale/project/.loop/trace.ltf.jsonl")

	// runDiscovery with the default (real, empty-in-CI) discoverers will not
	// rediscover this synthetic project dir, so it must be pruned.
	c.discoverers = []Discoverer{}
	c.runDiscovery()

	if _, exists := c.ltfTailers["/some/stale/project"]; exists {
		t.Error("expected stale LTF tailer to be pruned when its project is no longer discovered")
	}
}
