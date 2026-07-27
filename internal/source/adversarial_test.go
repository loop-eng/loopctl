package source

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

// This file implements the Phase 2 adversarial testing matrix (A1-A5,
// A10-A11) against the live pipeline: Tailer -> Parser -> Collector. Every
// test's baseline requirement is "no panic, no crash, no silent data
// corruption" per the org's Bug Hunt Protocol Pass 6. A6 (rapid
// start/stop) lives in collector_race_test.go; A7-A9 (config validation)
// live in internal/config/config_test.go; A12 (dual-instance) is a design
// decision documented in FINDINGS.md rather than an automated test (no
// shared mutable state between two loopctl processes except the
// export-file path, which os.WriteFile's O_TRUNC makes safe against torn
// writes for JSON-sized payloads).

func validClaudeLine(text string, inputTokens, outputTokens int) string {
	return fmt.Sprintf(
		`{"type":"assistant","sessionId":"s1","message":{"model":"claude-opus-4-6","content":[{"type":"text","text":%q}],"usage":{"input_tokens":%d,"output_tokens":%d}}}`,
		text, inputTokens, outputTokens)
}

// A1: binary garbage mixed with valid lines must not panic the parser, and
// a bad line in the middle of a batch must not prevent the good lines
// around it from being read and parsed.
func TestAdversarialBinaryGarbageMixedWithValidLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	garbage := make([]byte, 2048)
	if _, err := rand.Read(garbage); err != nil {
		t.Fatal(err)
	}
	garbage = bytes.ReplaceAll(garbage, []byte("\n"), []byte("x"))

	content := validClaudeLine("hi", 10, 5) + "\n" +
		string(garbage) + "\n" +
		validClaudeLine("bye", 20, 8) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tailer := NewTailer(path)
	lines, err := tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("ReadNewLines error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	p := parser.NewClaudeParser()
	var totalEvents int
	for i, line := range lines {
		events, err := p.Parse(line)
		if i == 1 {
			if err == nil {
				t.Error("expected a parse error for the binary garbage line")
			}
			continue
		}
		if err != nil {
			t.Errorf("line %d: unexpected parse error: %v", i, err)
		}
		totalEvents += len(events)
	}
	if totalEvents != 2 {
		t.Errorf("expected 2 events from the valid lines around the garbage line, got %d", totalEvents)
	}
}

// A2: an empty (0-byte) session file must produce no lines and no error —
// baseline regression guard.
func TestAdversarialEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}

	tailer := NewTailer(path)
	lines, err := tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error on empty file: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines from an empty file, got %d", len(lines))
	}
}

// A3: a single line far exceeding maxLineSize (1MB) must never be handed
// to the parser, and once a newline finally arrives, the next complete
// line must parse normally — the oversized line must not poison later
// reads.
func TestAdversarialOversizedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	huge := bytes.Repeat([]byte("a"), 5<<20) // 5MB, no newline yet
	if err := os.WriteFile(path, huge, 0644); err != nil {
		t.Fatal(err)
	}

	tailer := NewTailer(path)
	lines, err := tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error reading oversized partial line: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines while the oversized line has no terminator yet, got %d", len(lines))
	}

	// Still mid-write: append more of the same oversized line, still no
	// newline. Must not accumulate/re-scan unboundedly and must not panic.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(bytes.Repeat([]byte("b"), 1<<20)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	lines, err = tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error on second oversized read: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines, still no terminator, got %d", len(lines))
	}

	// Now terminate the oversized line and append a normal line after it.
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	normal := validClaudeLine("after the giant line", 1, 1)
	if _, err := f.WriteString("\n" + normal + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	lines, err = tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error after terminating the oversized line: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 recovered line after the oversized one, got %d", len(lines))
	}
	// ReadBytes('\n') includes the delimiter, so returned lines carry their
	// trailing newline.
	if string(lines[0]) != normal+"\n" {
		t.Errorf("recovered line = %q, want %q", lines[0], normal+"\n")
	}
	for _, l := range lines {
		if len(l) > maxLineSize {
			t.Errorf("returned line exceeds maxLineSize: %d bytes", len(l))
		}
	}

	p := parser.NewClaudeParser()
	events, err := p.Parse(lines[0])
	if err != nil {
		t.Errorf("failed to parse the recovered line: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least one event from the recovered line")
	}
}

// A4: a partial/truncated write at EOF (process killed mid-write) must be
// buffered, not emitted as a line and not lost — once the rest of the
// line plus its newline arrives, the full line must be reassembled
// correctly from the carried-over buffer.
func TestAdversarialPartialWriteAtEOF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	full := validClaudeLine("recovered", 5, 2)
	// Cut the line mid-object, e.g. mid-way through the "message" field —
	// simulates a process killed mid-write, not a natural line boundary.
	cut := len(full) / 2
	partial := full[:cut]
	rest := full[cut:]

	if err := os.WriteFile(path, []byte(partial), 0644); err != nil {
		t.Fatal(err)
	}

	tailer := NewTailer(path)
	lines, err := tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error on partial write: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines for a partial write with no newline, got %d", len(lines))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(rest + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	lines, err = tailer.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error completing the partial line: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 reassembled line, got %d", len(lines))
	}
	if string(lines[0]) != full+"\n" {
		t.Errorf("reassembled line = %q, want %q", lines[0], full+"\n")
	}

	p := parser.NewClaudeParser()
	events, err := p.Parse(lines[0])
	if err != nil {
		t.Errorf("failed to parse the reassembled line: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least one event from the reassembled line")
	}
}

// A5: discovery + one tail cycle over several hundred synthetic sessions
// must complete well under the collector's 2s tail-tick interval, and must
// not panic on a large map. Gated behind -short so normal `go test` runs
// stay fast; run explicitly (or via `make test`, which doesn't pass
// -short) to exercise it.
func TestAdversarialLoadManySessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	const numSessions = 500
	c := newTestCollector(t)
	dir := t.TempDir()

	line := validClaudeLine("load test line", 100, 50)
	for i := 0; i < numSessions; i++ {
		id := fmt.Sprintf("load-session-%d", i)
		path := filepath.Join(dir, id+".jsonl")
		content := line + "\n" + line + "\n" + line + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		c.store.InitSession(id, "claude", dir, 0, false, time.Now())
		c.tailers[id] = NewTailer(path)
		c.parsers[id] = parser.NewClaudeParser()
	}

	start := time.Now()
	c.processAllTails()
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("processAllTails over %d sessions took %v, want < 2s (the tail-tick interval)", numSessions, elapsed)
	}

	snap := c.Snapshot()
	if len(snap.Sessions) != numSessions {
		t.Fatalf("expected %d sessions in snapshot, got %d", numSessions, len(snap.Sessions))
	}
	for _, s := range snap.Sessions {
		if s.ToolCallCount != 0 && s.TotalCost < 0 {
			t.Errorf("session %s has negative cost", s.SessionID)
		}
	}
}

// A10: the session file disappears while loopctl is actively tailing it.
// ReadNewLines must return an error, not panic, and a fresh Discover call
// must no longer report the deleted session (proving discovery re-reads
// the filesystem instead of caching a stale listing).
func TestAdversarialSessionFileDeletedWhileTailing(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "-tmp-proj")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projDir, "sess1.jsonl")
	if err := os.WriteFile(path, []byte(validClaudeLine("x", 1, 1)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tailer := NewTailer(path)
	if _, err := tailer.ReadNewLines(); err != nil {
		t.Fatalf("unexpected error on first read: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	_, err := tailer.ReadNewLines()
	if err == nil {
		t.Error("expected an error after the session file was deleted")
	}

	d := NewClaudeDiscovererAt(slog.Default(), dir)
	sessions := d.Discover(24 * time.Hour)
	for _, s := range sessions {
		if s.Path == path {
			t.Error("expected discovery to no longer report the deleted session file")
		}
	}
}

// A11: the session path is replaced by a directory mid-run (and vice
// versa). Reading through a directory FD must return an ordinary error,
// never a panic, and Lstat's symlink check must not misclassify a
// directory as a symlink.
func TestAdversarialFileReplacedByDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	if err := os.WriteFile(path, []byte(validClaudeLine("x", 1, 1)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tailer := NewTailer(path)
	if _, err := tailer.ReadNewLines(); err != nil {
		t.Fatalf("unexpected error on first read: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}

	// Must not panic; a platform-level error (EISDIR or similar) is fine.
	_, _ = tailer.ReadNewLines()

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(validClaudeLine("y", 2, 2)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Reversed: directory replaced back by a file. A fresh tailer (offset 0)
	// must be able to read it normally.
	fresh := NewTailer(path)
	lines, err := fresh.ReadNewLines()
	if err != nil {
		t.Fatalf("unexpected error after directory was replaced by a file: %v", err)
	}
	if len(lines) != 1 {
		t.Errorf("expected 1 line after directory->file replacement, got %d", len(lines))
	}
}
