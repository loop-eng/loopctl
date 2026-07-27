package source

import (
	"context"
	"log/slog"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/loop-eng/loopctl/internal/config"
	"github.com/loop-eng/loopctl/internal/metrics"
	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/parser"
)

type Collector struct {
	logger      *slog.Logger
	registry    *Registry
	discoverers []Discoverer
	store       *metrics.SessionStore
	cfg         *config.Config
	tailers     map[string]*Tailer
	parsers     map[string]parser.Parser

	ltfEnricher *LTFEnricher
	ltfTailers  map[string]*Tailer // keyed by ProjectDir, not session ID

	// mu guards alerts and alertedTermination. Lock ordering: if a caller
	// ever needs both mu and the SessionStore's internal lock, mu must be
	// acquired first (as buildAlerts already does) — never the reverse.
	// See GitHub issue #2.
	mu                 sync.Mutex
	alerts             []model.Alert
	alertedTermination map[string]bool

	cancelMu sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewCollector(logger *slog.Logger, cfg *config.Config) *Collector {
	discoverers := []Discoverer{
		NewClaudeDiscoverer(logger),
	}
	if cfg.Sources.Codex != "disabled" {
		discoverers = append(discoverers, NewCodexDiscoverer(logger))
	}

	return &Collector{
		logger:             logger,
		registry:           NewRegistry(),
		discoverers:        discoverers,
		store:              metrics.NewSessionStore(logger),
		cfg:                cfg,
		tailers:            make(map[string]*Tailer),
		parsers:            make(map[string]parser.Parser),
		ltfEnricher:        NewLTFEnricher(logger),
		ltfTailers:         make(map[string]*Tailer),
		alertedTermination: make(map[string]bool),
	}
}

// Start begins background discovery/tailing. It is safe to call Start again
// after Close returns — Close blocks until the previous loop goroutine has
// fully exited before this method returns, so a second Start's synchronous
// runDiscovery/processAllTails calls never race with it. See GitHub issue
// A6 in the Phase 2 adversarial testing plan.
func (c *Collector) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancelMu.Lock()
	c.cancel = cancel
	c.cancelMu.Unlock()

	c.runDiscovery()
	c.processAllTails()

	c.wg.Add(1)
	go c.loop(ctx)
}

// Close signals the background loop to stop and blocks until it has
// actually exited, so tailers/parsers are no longer being mutated by the
// time Close returns (safe to call Start again afterward, or to tear down
// the process cleanly).
func (c *Collector) Close() {
	c.cancelMu.Lock()
	cancel := c.cancel
	c.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
}

func (c *Collector) loop(ctx context.Context) {
	defer c.wg.Done()
	discoveryTicker := time.NewTicker(30 * time.Second)
	tailTicker := time.NewTicker(2 * time.Second)
	defer discoveryTicker.Stop()
	defer tailTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-discoveryTicker.C:
			c.runDiscovery()
		case <-tailTicker.C:
			c.processAllTails()
		}
	}
}

func (c *Collector) runDiscovery() {
	discovered := make(map[string]bool)
	discoveredProjects := make(map[string]bool)
	for _, d := range c.discoverers {
		sessions := d.Discover(24 * time.Hour)
		for _, s := range sessions {
			discovered[s.ID] = true
			c.registry.Add(s)
			c.store.InitSession(s.ID, s.Agent, s.ProjectDir, s.PID, s.Active, s.StartedAt)

			if _, exists := c.tailers[s.ID]; !exists {
				t := NewTailer(s.Path)
				c.tailers[s.ID] = t
			}
			if _, exists := c.parsers[s.ID]; !exists {
				switch s.Agent {
				case "claude":
					c.parsers[s.ID] = parser.NewClaudeParser()
				case "codex":
					c.parsers[s.ID] = parser.NewCodexParser()
				}
			}

			if s.Agent == "claude" && s.ProjectDir != "" {
				discoveredProjects[s.ProjectDir] = true
				if _, exists := c.ltfTailers[s.ProjectDir]; !exists && c.ltfEnricher.Available(s.ProjectDir) {
					c.ltfTailers[s.ProjectDir] = NewTailer(c.ltfEnricher.TracePath(s.ProjectDir))
				}
			}
		}
	}

	for id := range c.tailers {
		if !discovered[id] {
			delete(c.tailers, id)
			delete(c.parsers, id)
		}
	}
	for dir := range c.ltfTailers {
		if !discoveredProjects[dir] {
			delete(c.ltfTailers, dir)
		}
	}
}

func (c *Collector) processAllTails() {
	for sessionID, tailer := range c.tailers {
		p, ok := c.parsers[sessionID]
		if !ok {
			continue
		}

		lines, err := tailer.ReadNewLines()
		if err != nil {
			c.logger.Debug("tail error", "session", sessionID, "error", err)
		}
		if len(lines) == 0 {
			continue
		}

		for _, line := range lines {
			events, err := p.Parse(line)
			if err != nil {
				c.logger.Debug("parse error", "session", sessionID, "error", err)
				continue
			}
			for _, ev := range events {
				c.store.ProcessEvent(sessionID, ev)
			}
		}
	}

	c.processLTFTails()
	c.buildAlerts()
}

func (c *Collector) processLTFTails() {
	for projectDir, tailer := range c.ltfTailers {
		lines, err := tailer.ReadNewLines()
		if err != nil {
			c.logger.Debug("ltf tail error", "project", projectDir, "error", err)
		}
		for _, line := range lines {
			ev, err := parser.ParseLTFLine(line)
			if err != nil {
				c.logger.Debug("ltf parse error", "project", projectDir, "error", err)
				continue
			}
			if ev.SessionID == "" {
				continue
			}
			c.store.ApplyLTFEvent(ev.SessionID, ev)
		}
	}
}

func (c *Collector) buildAlerts() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.alerts = nil

	snap := c.store.Snapshot()
	for _, s := range snap {
		if s.Spin.IsSpinning {
			for _, reason := range s.Spin.Reasons {
				c.alerts = append(c.alerts, model.Alert{
					SessionID: s.SessionID,
					Severity:  "critical",
					Message:   filepath.Base(s.ProjectDir) + ": " + reason,
					Timestamp: time.Now(),
				})
			}
		} else if s.Spin.HasWarnings {
			for _, reason := range s.Spin.Reasons {
				c.alerts = append(c.alerts, model.Alert{
					SessionID: s.SessionID,
					Severity:  "warning",
					Message:   filepath.Base(s.ProjectDir) + ": " + reason,
					Timestamp: time.Now(),
				})
			}
		}

		if c.cfg.Budget.PerSessionUSD > 0 {
			pct := s.TotalCost / c.cfg.Budget.PerSessionUSD * 100
			if pct >= float64(c.cfg.Budget.WarnAtPercent) && pct < 100 {
				c.alerts = append(c.alerts, model.Alert{
					SessionID: s.SessionID,
					Severity:  "warning",
					Message:   filepath.Base(s.ProjectDir) + ": budget " + percentStr(pct),
					Timestamp: time.Now(),
				})
			}
			if pct >= 100 {
				c.alerts = append(c.alerts, model.Alert{
					SessionID: s.SessionID,
					Severity:  "critical",
					Message:   filepath.Base(s.ProjectDir) + ": budget exceeded",
					Timestamp: time.Now(),
				})
			}
		}

		if !s.Active && s.TerminationReason != "" && !c.alertedTermination[s.SessionID] {
			switch s.TerminationReason {
			case "spin_detected", "stall_detected", "error", "budget_exhausted":
				c.alerts = append(c.alerts, model.Alert{
					SessionID: s.SessionID,
					Severity:  "warning",
					Message:   filepath.Base(s.ProjectDir) + ": loop ended (" + s.TerminationReason + ")",
					Timestamp: time.Now(),
				})
			}
			c.alertedTermination[s.SessionID] = true
		}
	}
}

func percentStr(pct float64) string {
	if pct > 999 {
		pct = 999
	}
	return model.FormatPercent(pct)
}

func (c *Collector) Snapshot() model.DataMsg {
	snap := c.store.Snapshot()

	views := make([]model.SessionView, len(snap))
	for i, s := range snap {
		duration := time.Duration(0)
		if !s.StartedAt.IsZero() {
			if s.Active {
				duration = time.Since(s.StartedAt)
			} else if !s.LastActivity.IsZero() {
				duration = s.LastActivity.Sub(s.StartedAt)
			}
		}

		sortedFiles := make([]string, 0, len(s.FilesChanged))
		for f := range s.FilesChanged {
			sortedFiles = append(sortedFiles, f)
		}
		sort.Strings(sortedFiles)

		views[i] = model.SessionView{
			SessionID:            s.SessionID,
			Agent:                s.Agent,
			ProjectDir:           s.ProjectDir,
			ProjectName:          filepath.Base(s.ProjectDir),
			Model:                s.Model,
			Active:               s.Active,
			PID:                  s.PID,
			StartedAt:            s.StartedAt,
			LastActivity:         s.LastActivity,
			Duration:             duration,
			TotalCost:            s.TotalCost,
			BurnRate:             s.BurnRate,
			ToolCallCount:        s.ToolCallCount,
			LastToolName:         s.LastToolName,
			IterationCount:       s.IterationCount,
			ErrorCount:           s.ErrorCount,
			ContextFillPct:       s.ContextFillPct,
			CompactionCount:      s.CompactionCount,
			CacheHitRate:         s.CacheHitRate,
			TokenEfficiency:      s.TokenEfficiency,
			IsSpinning:           s.Spin.IsSpinning,
			HasWarnings:          s.Spin.HasWarnings,
			SpinReasons:          s.Spin.Reasons,
			TotalInput:           s.TotalInput,
			TotalOutput:          s.TotalOutput,
			TotalCacheRead:       s.TotalCacheRead,
			TotalCacheWrite:      s.TotalCacheWrite,
			FilesChanged:         len(s.FilesChanged),
			FilesChangedList:     sortedFiles,
			ErrorMessages:        append([]string(nil), s.ErrorMessages...),
			LTFAvailable:         s.LTFAvailable,
			LTFIterationCount:    s.LTFIterationCount,
			TerminationReason:    s.TerminationReason,
			VerificationPassRate: s.VerificationPassRate,
		}
	}

	c.mu.Lock()
	alerts := make([]model.Alert, len(c.alerts))
	copy(alerts, c.alerts)
	c.mu.Unlock()

	return model.DataMsg{
		Sessions:   views,
		DailyTotal: metrics.DailyTotalFromSnapshot(snap),
		Alerts:     alerts,
	}
}
