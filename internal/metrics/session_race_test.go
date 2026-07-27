package metrics

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

// TestSessionStoreConcurrentStress hammers ProcessEvent/Snapshot/DailyTotal/
// TopSessions from many goroutines simultaneously, mirroring the real
// production shape: Collector's tail-processing goroutine writes while the
// TUI's render loop reads, both against a shared, small set of session IDs
// so map contention and per-session mutation are both exercised. Run with
// -race — this is what verifies GitHub issues #1 and #3 aren't live bugs
// under load, not just "safe by inspection."
func TestSessionStoreConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	ss := NewSessionStore(slog.Default())

	const numSessions = 20
	ids := make([]string, numSessions)
	for i := 0; i < numSessions; i++ {
		id := fmt.Sprintf("session-%d", i)
		ids[i] = id
		ss.InitSession(id, "claude", "/tmp/"+id, 100+i, true, time.Now())
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Writers: many goroutines calling ProcessEvent against shared keys.
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-done:
					return
				default:
				}
				id := ids[(worker+i)%numSessions]
				ev := &parser.ParsedEvent{
					SessionID:   id,
					ContentType: parser.ContentToolUse,
					ToolName:    fmt.Sprintf("Tool%d", i%5),
					ToolInput:   fmt.Sprintf(`{"n":%d}`, i),
					Model:       "claude-opus-4-6",
					Timestamp:   time.Now(),
					Tokens: parser.TokenUsage{
						InputTokens:  1000 + i%50,
						OutputTokens: 500 + i%20,
					},
					FilesChanged: []string{fmt.Sprintf("/tmp/f%d.go", i%10)},
				}
				if i%13 == 0 {
					ev.IsError = true
					ev.ErrorMsg = "boom"
				}
				ss.ProcessEvent(id, ev)
				i++
			}
		}(w)
	}

	// Readers: Snapshot/DailyTotal/TopSessions in a tight loop, asserting
	// internal self-consistency (never a full proof of atomicity, but it
	// catches torn-read symptoms cheaply).
	for r := 0; r < 6; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				snap := ss.Snapshot()
				if len(snap) != numSessions {
					t.Errorf("Snapshot returned %d sessions, want %d", len(snap), numSessions)
				}
				for _, s := range snap {
					if s.TotalCost < 0 || math.IsNaN(s.TotalCost) || math.IsInf(s.TotalCost, 0) {
						t.Errorf("session %s has invalid TotalCost %v", s.SessionID, s.TotalCost)
					}
				}
				_ = ss.DailyTotal()
				top := ss.TopSessions(5)
				if len(top) > 5 {
					t.Errorf("TopSessions(5) returned %d entries, want <= 5", len(top))
				}
			}
		}()
	}

	// Regression guard for GitHub issue #3: mutating a previously-returned
	// Snapshot's Spin.Reasons slice must never affect a later Snapshot.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			snap := ss.Snapshot()
			for i := range snap {
				if len(snap[i].Spin.Reasons) > 0 {
					snap[i].Spin.Reasons[0] = "MUTATED"
				}
				if len(snap[i].ErrorMessages) > 0 {
					snap[i].ErrorMessages[0] = "MUTATED"
				}
			}
		}
	}()

	time.Sleep(2 * time.Second)
	close(done)

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(10 * time.Second):
		t.Fatal("stress test did not complete — possible deadlock")
	}

	final := ss.Snapshot()
	if len(final) != numSessions {
		t.Fatalf("final snapshot has %d sessions, want %d", len(final), numSessions)
	}
	for _, s := range final {
		for _, r := range s.Spin.Reasons {
			if r == "MUTATED" {
				t.Fatal("a mutated Spin.Reasons slice leaked into a later Snapshot — issue #3 defensive copy is broken")
			}
		}
		for _, e := range s.ErrorMessages {
			if e == "MUTATED" {
				t.Fatal("a mutated ErrorMessages slice leaked into a later Snapshot")
			}
		}
	}
}

// TestDailyTotalFromSnapshotMatchesConcurrentReads proves DailyTotalFromSnapshot
// (the issue #1 fix) computed from an already-taken Snapshot stays internally
// consistent even while writers are concurrently mutating the store: the sum
// of each session's TotalCost within one snapshot must equal
// DailyTotalFromSnapshot's result for that same snapshot, with no additional
// locking required.
func TestDailyTotalFromSnapshotMatchesConcurrentReads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	ss := NewSessionStore(slog.Default())
	const numSessions = 10
	now := time.Now()
	for i := 0; i < numSessions; i++ {
		id := fmt.Sprintf("s-%d", i)
		ss.InitSession(id, "claude", "/tmp/"+id, 0, true, now)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-done:
					return
				default:
				}
				id := fmt.Sprintf("s-%d", (worker+i)%numSessions)
				ss.ProcessEvent(id, &parser.ParsedEvent{
					SessionID:   id,
					ContentType: parser.ContentText,
					Model:       "claude-opus-4-6",
					Timestamp:   time.Now(),
					Tokens:      parser.TokenUsage{InputTokens: 100, OutputTokens: 50},
				})
				i++
			}
		}(w)
	}

	for r := 0; r < 200; r++ {
		snap := ss.Snapshot()
		var wantTotal float64
		for _, s := range snap {
			wantTotal += s.TotalCost
		}
		got := DailyTotalFromSnapshot(snap)
		if got != wantTotal {
			t.Fatalf("DailyTotalFromSnapshot = %v, want %v (sum of same snapshot's TotalCost)", got, wantTotal)
		}
	}

	close(done)
	wg.Wait()
}
