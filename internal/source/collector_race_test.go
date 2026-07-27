package source

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

// TestCollectorRestartCycleRace repeatedly starts and closes a Collector
// back-to-back, targeting c.tailers/c.parsers — plain maps mutated only
// from within loop()/Start()/runDiscovery(), never individually
// mutex-protected. Before Close() waited for the loop goroutine to fully
// exit, a fast restart could race the previous cycle's background
// goroutine against the next cycle's synchronous runDiscovery/
// processAllTails calls (GitHub adversarial-test finding A6). Run with
// -race: this must be clean now that Close() blocks on the loop's
// WaitGroup.
func TestCollectorRestartCycleRace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	c := newTestCollector(t)
	c.discoverers = []Discoverer{}

	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		c.Start(ctx)
		time.Sleep(2 * time.Millisecond)
		c.Close()
		cancel()
	}
}

func sessionID(i int) string {
	return fmt.Sprintf("session-%d", i)
}

// TestCollectorSnapshotAndBuildAlertsConcurrent stresses Collector.Snapshot
// (which reads SessionStore + Collector.alerts) running concurrently with
// buildAlerts (which locks Collector.mu then reads SessionStore) and with
// ProcessEvent writers, to verify GitHub issues #1 (temporal consistency
// across three separately-locked reads) and #2 (c.mu -> ss.mu lock
// ordering) never manifest as a race or a deadlock under load.
func TestCollectorSnapshotAndBuildAlertsConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	c := newTestCollector(t)
	const numSessions = 10
	for i := 0; i < numSessions; i++ {
		id := sessionID(i)
		c.store.InitSession(id, "claude", "/tmp/"+id, 0, true, time.Now())
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			c.buildAlerts()
		}
	}()

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
				id := sessionID((worker + i) % numSessions)
				c.store.ProcessEvent(id, &parser.ParsedEvent{
					SessionID:   id,
					ContentType: parser.ContentToolUse,
					ToolName:    "Bash",
					Timestamp:   time.Now(),
					Tokens:      parser.TokenUsage{InputTokens: 100, OutputTokens: 50},
				})
				i++
			}
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			snap := c.Snapshot()
			if len(snap.Sessions) != numSessions {
				t.Errorf("Snapshot returned %d sessions, want %d", len(snap.Sessions), numSessions)
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
		t.Fatal("stress test did not complete — possible deadlock between Collector.mu and SessionStore.mu")
	}
}
