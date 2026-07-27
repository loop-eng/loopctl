package source

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/loop-eng/loopctl/internal/config"
	"github.com/loop-eng/loopctl/internal/metrics"
)

// This file drives the full Discoverer -> Tailer -> Parser -> SessionStore
// -> Collector.Snapshot pipeline against fully synthetic, deterministic
// fixtures built in a t.TempDir(), so results are identical on every
// machine and in CI, unlike live_test.go (which is a valuable supplementary
// sanity check against real local session data, but skips cleanly when
// none exists).

// writeClaudeSession creates <projectsDir>/<projectDirName>/<sessionID>.jsonl,
// mirroring the real on-disk layout so ClaudeDiscoverer can find it.
func writeClaudeSession(t *testing.T, projectsDir, projectDirName, sessionID string, lines []string) string {
	t.Helper()
	dir := filepath.Join(projectsDir, projectDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

var assistantRequestSeq int

// assistantToolUse builds a valid Claude JSONL assistant line with a
// tool_use content block. Each call gets a fresh requestId — ClaudeParser
// only counts a usage block's tokens the first time it sees a given
// requestId (its two-generation dedup guard), so every fixture line here
// needs a distinct one to actually contribute to TotalCost/TotalInput/etc.
func assistantToolUse(sessionID, model, tool, filePath string, inTok, outTok int) string {
	assistantRequestSeq++
	return fmt.Sprintf(
		`{"type":"assistant","sessionId":%q,"requestId":"req-%d","message":{"model":%q,"content":[{"type":"tool_use","name":%q,"id":"t1","input":{"file_path":%q}}],"usage":{"input_tokens":%d,"output_tokens":%d}}}`,
		sessionID, assistantRequestSeq, model, tool, filePath, inTok, outTok)
}

func userToolResult(sessionID string, isError bool) string {
	return fmt.Sprintf(`{"type":"user","sessionId":%q,"message":{"content":[{"type":"tool_result","is_error":%t,"content":"ok"}]}}`, sessionID, isError)
}

// pipelineOnce runs one full discovery + tail + parse + store + alert
// cycle against a Collector rooted at the given temp claudeDir.
func pipelineOnce(c *Collector) {
	c.runDiscovery()
	c.processAllTails()
}

func TestIntegrationNormalSession(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-normal"

	lines := []string{
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/a.go", 1000, 200),
		userToolResult(sessionID, false),
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/b.go", 1200, 300),
		userToolResult(sessionID, false),
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/c.go", 900, 150),
		userToolResult(sessionID, false),
	}
	writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, lines)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}

	pipelineOnce(c)

	snap := c.Snapshot()
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap.Sessions))
	}
	s := snap.Sessions[0]

	if s.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want claude-opus-4-6", s.Model)
	}
	if s.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want 3", s.ToolCallCount)
	}
	if s.IterationCount != 3 {
		t.Errorf("IterationCount = %d, want 3", s.IterationCount)
	}
	if s.FilesChanged != 3 {
		t.Errorf("FilesChanged = %d, want 3", s.FilesChanged)
	}
	if s.IsSpinning {
		t.Error("expected IsSpinning=false for 3 distinct tool calls on 3 distinct files")
	}
	if s.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", s.ErrorCount)
	}

	wantCost := (float64(1000+1200+900)*5.00 + float64(200+300+150)*25.00) / 1_000_000
	if diff := s.TotalCost - wantCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("TotalCost = %v, want %v", s.TotalCost, wantCost)
	}
}

func TestIntegrationSpinSession(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-spin"

	// 4 identical Bash tool calls within the same synthetic timestamp
	// window trip the repeated-tool-call heuristic (default threshold: 3).
	var lines []string
	for i := 0; i < 4; i++ {
		lines = append(lines, fmt.Sprintf(
			`{"type":"assistant","sessionId":%q,"message":{"model":"claude-opus-4-6","content":[{"type":"tool_use","name":"Bash","id":"t%d","input":{"command":"go test ./..."}}],"usage":{"input_tokens":10,"output_tokens":5}}}`,
			sessionID, i))
	}
	writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, lines)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}

	pipelineOnce(c)

	snap := c.Snapshot()
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap.Sessions))
	}
	s := snap.Sessions[0]
	if !s.IsSpinning {
		t.Error("expected IsSpinning=true after 4 identical repeated tool calls")
	}
	found := false
	for _, r := range s.SpinReasons {
		if len(r) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected non-empty SpinReasons")
	}

	var gotCritical bool
	for _, a := range snap.Alerts {
		if a.SessionID == s.SessionID && a.Severity == "critical" {
			gotCritical = true
		}
	}
	if !gotCritical {
		t.Error("expected a critical alert for the spinning session")
	}
}

func TestIntegrationBudgetExceeded(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-budget"

	lines := []string{
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/a.go", 100000, 50000),
	}
	writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, lines)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}
	c.cfg = config.Default()
	c.cfg.Budget.PerSessionUSD = 0.01

	pipelineOnce(c)

	snap := c.Snapshot()
	var gotExceeded bool
	for _, a := range snap.Alerts {
		if a.SessionID == sessionID && a.Severity == "critical" {
			gotExceeded = true
			if want := "budget exceeded"; len(a.Message) < len(want) || a.Message[len(a.Message)-len(want):] != want {
				t.Errorf("alert message = %q, want suffix %q", a.Message, want)
			}
		}
	}
	if !gotExceeded {
		t.Error("expected a critical budget-exceeded alert")
	}
}

func TestIntegrationBudgetWarningBoundary(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-budget-warn"

	// Engineer cost to land at exactly 90% of a $1 budget: 0.9 = tokens *
	// price / 1e6. Use opus input pricing ($5/MTok): 180000 tokens * $5/1e6
	// = $0.90.
	lines := []string{
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/a.go", 180000, 0),
	}
	writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, lines)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}
	c.cfg = config.Default()
	c.cfg.Budget.PerSessionUSD = 1.0
	c.cfg.Budget.WarnAtPercent = 80

	pipelineOnce(c)

	snap := c.Snapshot()
	var gotWarning, gotCritical bool
	for _, a := range snap.Alerts {
		if a.SessionID != sessionID {
			continue
		}
		switch a.Severity {
		case "warning":
			gotWarning = true
		case "critical":
			gotCritical = true
		}
	}
	if !gotWarning {
		t.Error("expected a warning alert at 90% of budget (>= 80% threshold, < 100%)")
	}
	if gotCritical {
		t.Error("did not expect a critical (budget exceeded) alert at 90% of budget")
	}
}

func TestIntegrationMalformedJSONLMixedWithValid(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-malformed"

	valid1 := assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/a.go", 100, 50)
	valid2 := assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/b.go", 200, 60)
	lines := []string{valid1, "not json at all", valid2}
	writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, lines)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}

	pipelineOnce(c)

	snap := c.Snapshot()
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected the session to still be tracked, got %d sessions", len(snap.Sessions))
	}
	s := snap.Sessions[0]
	if s.ToolCallCount != 2 {
		t.Errorf("ToolCallCount = %d, want 2 (malformed line must not abort the batch)", s.ToolCallCount)
	}
}

func TestIntegrationModelSwitchMidStream(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-model-switch"

	lines := []string{
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/a.go", 1000, 100),
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/b.go", 1000, 100),
		assistantToolUse(sessionID, "gpt-4.1", "Edit", "/tmp/c.go", 1000, 100),
		assistantToolUse(sessionID, "gpt-4.1", "Edit", "/tmp/d.go", 1000, 100),
	}
	writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, lines)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}

	pipelineOnce(c)

	snap := c.Snapshot()
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap.Sessions))
	}
	s := snap.Sessions[0]
	if s.Model != "gpt-4.1" {
		t.Errorf("Model = %q, want gpt-4.1 (last-write-wins)", s.Model)
	}
	if s.CompactionCount != 0 {
		t.Errorf("CompactionCount = %d, want 0 — a model switch alone must not register as a compaction", s.CompactionCount)
	}

	opusPricing := metrics.DefaultPricing()["claude-opus-4-6"]
	gptPricing := metrics.DefaultPricing()["gpt-4.1"]
	wantCost := 2*(1000*opusPricing.InputPerMTok+100*opusPricing.OutputPerMTok)/1_000_000 +
		2*(1000*gptPricing.InputPerMTok+100*gptPricing.OutputPerMTok)/1_000_000
	if diff := s.TotalCost - wantCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("TotalCost = %v, want %v (mixed opus+gpt pricing)", s.TotalCost, wantCost)
	}
}

func TestIntegrationTailerTruncationRotation(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude", "projects")
	sessionID := "sess-rotate"

	var initial []string
	for i := 0; i < 5; i++ {
		initial = append(initial, assistantToolUse(sessionID, "claude-opus-4-6", "Edit", fmt.Sprintf("/tmp/f%d.go", i), 10, 5))
	}
	path := writeClaudeSession(t, claudeDir, "-tmp-proj", sessionID, initial)

	c := newTestCollector(t)
	c.discoverers = []Discoverer{NewClaudeDiscovererAt(c.logger, claudeDir)}

	pipelineOnce(c)
	snap := c.Snapshot()
	if len(snap.Sessions) != 1 || snap.Sessions[0].ToolCallCount != 5 {
		t.Fatalf("expected 5 tool calls before rotation, got %+v", snap.Sessions)
	}

	// Simulate log rotation: truncate to 0, then write 2 new lines.
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	rotated := []string{
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/new1.go", 10, 5),
		assistantToolUse(sessionID, "claude-opus-4-6", "Edit", "/tmp/new2.go", 10, 5),
	}
	content := ""
	for _, l := range rotated {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pipelineOnce(c)
	snap = c.Snapshot()
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected 1 session after rotation, got %d", len(snap.Sessions))
	}
	if got := snap.Sessions[0].ToolCallCount; got != 7 {
		t.Errorf("ToolCallCount after rotation = %d, want 7 (5 pre-rotation + 2 post-rotation, cumulative in SessionStore)", got)
	}
}

func TestTailerSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.jsonl")
	if err := os.WriteFile(real, []byte(`{"type":"assistant"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.jsonl")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	tailer := NewTailer(link)
	lines, err := tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("expected no error tailing a symlink, got %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil lines when tailing a symlink, got %d lines", len(lines))
	}
}

// Concurrency stress coverage for SessionStore/Collector lives in
// session_race_test.go (TestSessionStoreConcurrentStress, in
// internal/metrics) and collector_race_test.go
// (TestCollectorSnapshotAndBuildAlertsConcurrent, TestCollectorRestartCycleRace)
// rather than here, so `go test -race` is what exercises them.
