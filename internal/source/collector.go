package source

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/config"
	"github.com/loop-eng/loopctl/internal/metrics"
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

	mu     sync.Mutex
	alerts []model.Alert

	cancel context.CancelFunc
}

func NewCollector(logger *slog.Logger, cfg *config.Config) *Collector {
	discoverers := []Discoverer{
		NewClaudeDiscoverer(logger),
	}
	if cfg.Sources.Codex != "disabled" {
		discoverers = append(discoverers, NewCodexDiscoverer(logger))
	}

	return &Collector{
		logger:      logger,
		registry:    NewRegistry(),
		discoverers: discoverers,
		store:       metrics.NewSessionStore(logger),
		cfg:         cfg,
		tailers:     make(map[string]*Tailer),
		parsers:     make(map[string]parser.Parser),
	}
}

func (c *Collector) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)

	c.runDiscovery()
	c.processAllTails()

	go c.loop(ctx)
}

func (c *Collector) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Collector) loop(ctx context.Context) {
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
	for _, d := range c.discoverers {
		sessions := d.Discover(24 * time.Hour)
		for _, s := range sessions {
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
			continue
		}

		for _, line := range lines {
			events, err := p.Parse(line)
			if err != nil {
				continue
			}
			for _, ev := range events {
				c.store.ProcessEvent(sessionID, ev)
			}
		}
	}

	c.buildAlerts()
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

		views[i] = model.SessionView{
			SessionID:       s.SessionID,
			Agent:           s.Agent,
			ProjectDir:      s.ProjectDir,
			ProjectName:     filepath.Base(s.ProjectDir),
			Model:           s.Model,
			Active:          s.Active,
			PID:             s.PID,
			Duration:        duration,
			TotalCost:       s.TotalCost,
			BurnRate:        s.BurnRate,
			ToolCallCount:   s.ToolCallCount,
			LastToolName:    s.LastToolName,
			IterationCount:  s.IterationCount,
			ErrorCount:      s.ErrorCount,
			ContextFillPct:  s.ContextFillPct,
			CompactionCount: s.CompactionCount,
			CacheHitRate:    s.CacheHitRate,
			TokenEfficiency: s.TokenEfficiency,
			IsSpinning:      s.Spin.IsSpinning,
			SpinReasons:     s.Spin.Reasons,
			TotalInput:      s.TotalInput,
			TotalOutput:     s.TotalOutput,
			TotalCacheRead:  s.TotalCacheRead,
			TotalCacheWrite: s.TotalCacheWrite,
			FilesChanged:    len(s.FilesChanged),
		}
	}

	c.mu.Lock()
	alerts := make([]model.Alert, len(c.alerts))
	copy(alerts, c.alerts)
	c.mu.Unlock()

	return model.DataMsg{
		Sessions:   views,
		DailyTotal: c.store.DailyTotal(),
		Alerts:     alerts,
	}
}
