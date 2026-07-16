package source

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/config"
	"github.com/loop-eng/loopctl/internal/metrics"
	"github.com/loop-eng/loopctl/internal/parser"
)

func TestLiveClaudeDiscovery(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	claudeDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Skip("no claude projects dir")
	}

	d := NewClaudeDiscoverer(slog.Default())
	sessions := d.Discover(720 * time.Hour)

	if len(sessions) == 0 {
		t.Skip("no sessions found")
	}

	t.Logf("Discovered %d sessions", len(sessions))

	for _, s := range sessions {
		if s.ID == "" {
			t.Error("session has empty ID")
		}
		if s.Path == "" {
			t.Error("session has empty Path")
		}
		if s.Agent != "claude" {
			t.Errorf("session %s has wrong agent: %s", s.ID, s.Agent)
		}
		if _, err := os.Stat(s.Path); err != nil {
			t.Errorf("session %s path does not exist: %s", s.ID, s.Path)
		}

		t.Logf("  %s: project=%s active=%v pid=%d", s.ID[:8], filepath.Base(s.ProjectDir), s.Active, s.PID)
	}
}

func TestLiveParsePipeline(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	claudeDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Skip("no claude projects dir")
	}

	d := NewClaudeDiscoverer(slog.Default())
	sessions := d.Discover(720 * time.Hour)
	if len(sessions) == 0 {
		t.Skip("no sessions found")
	}

	store := metrics.NewSessionStore(slog.Default())
	p := parser.NewClaudeParser()

	for _, s := range sessions {
		store.InitSession(s.ID, s.Agent, s.ProjectDir, s.PID, s.Active, s.StartedAt)

		tailer := NewTailer(s.Path)
		lines, err := tailer.ReadNewLines()
		if err != nil {
			t.Logf("  %s: tailer error (ok): %v", s.ID[:8], err)
			continue
		}

		parsed := 0
		errors := 0
		for _, line := range lines {
			events, err := p.Parse(line)
			if err != nil {
				errors++
				continue
			}
			for _, ev := range events {
				store.ProcessEvent(s.ID, ev)
				parsed++
			}
		}
		t.Logf("  %s: %d lines, %d events, %d parse errors", s.ID[:8], len(lines), parsed, errors)
	}

	snap := store.Snapshot()
	t.Logf("\n=== Pipeline Results ===")
	t.Logf("Sessions: %d", len(snap))

	totalCost := 0.0
	for _, s := range snap {
		totalCost += s.TotalCost
		t.Logf("  %-8s | model=%-20s | cost=$%.4f | tools=%d | errors=%d | ctx=%.0f%% | spin=%v",
			s.SessionID[:8], s.Model, s.TotalCost, s.ToolCallCount, s.ErrorCount, s.ContextFillPct, s.Spin.IsSpinning)

		if s.TotalCost < 0 {
			t.Errorf("session %s has negative cost: $%.4f", s.SessionID[:8], s.TotalCost)
		}
		if s.ContextFillPct < 0 || s.ContextFillPct > 100 {
			t.Errorf("session %s has invalid context fill: %.1f%%", s.SessionID[:8], s.ContextFillPct)
		}
		if s.CacheHitRate < 0 || s.CacheHitRate > 100 {
			t.Errorf("session %s has invalid cache hit rate: %.1f%%", s.SessionID[:8], s.CacheHitRate)
		}
		if s.TokenEfficiency < 0 || s.TokenEfficiency > 100 {
			t.Errorf("session %s has invalid token efficiency: %.1f%%", s.SessionID[:8], s.TokenEfficiency)
		}
	}
	t.Logf("Total cost across all sessions: $%.4f", totalCost)
	t.Logf("Daily total: $%.4f", store.DailyTotal())
}

func TestLiveCollectorSnapshot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	claudeDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Skip("no claude projects dir")
	}

	cfg := config.Default()
	collector := NewCollector(slog.Default(), cfg)

	collector.runDiscovery()
	collector.processAllTails()

	snap := collector.Snapshot()

	if len(snap.Sessions) == 0 {
		t.Skip("no sessions in collector snapshot")
	}

	t.Logf("Collector snapshot: %d sessions, daily=$%.4f, alerts=%d",
		len(snap.Sessions), snap.DailyTotal, len(snap.Alerts))

	for _, s := range snap.Sessions {
		if s.SessionID == "" {
			t.Error("empty session ID in snapshot")
		}
		if s.ProjectName == "" && s.ProjectDir == "" {
			t.Errorf("session %s has no project info", s.SessionID[:8])
		}

		status := "Done"
		if s.Active {
			status = "Active"
		}
		if s.IsSpinning {
			status = "SPIN"
		}

		t.Logf("  [%s] %-12s | %-20s | $%.4f | %d tools | ctx=%.0f%%",
			status, s.ProjectName, s.Model, s.TotalCost, s.ToolCallCount, s.ContextFillPct)
	}

	for _, a := range snap.Alerts {
		t.Logf("  ALERT [%s]: %s", a.Severity, a.Message)
	}

	for _, s := range snap.Sessions {
		if s.TotalCost < 0 {
			t.Errorf("negative cost: %s = $%.4f", s.SessionID[:8], s.TotalCost)
		}
		if s.ContextFillPct < 0 || s.ContextFillPct > 100 {
			t.Errorf("invalid context%%: %s = %.1f%%", s.SessionID[:8], s.ContextFillPct)
		}
		if s.CacheHitRate < 0 || s.CacheHitRate > 100 {
			t.Errorf("invalid cache hit: %s = %.1f%%", s.SessionID[:8], s.CacheHitRate)
		}
		if strings.Contains(s.ProjectDir, "-") && !strings.Contains(s.ProjectDir, "/") {
			t.Errorf("project dir looks encoded, not decoded: %s", s.ProjectDir)
		}
	}
}
