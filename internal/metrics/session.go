package metrics

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

type SessionMetrics struct {
	SessionID    string
	Agent        string
	ProjectDir   string
	Model        string
	Active       bool
	PID          int
	StartedAt    time.Time
	LastActivity time.Time

	TotalCost float64
	BurnRate  float64

	TotalInput      int
	TotalOutput     int
	TotalCacheRead  int
	TotalCacheWrite int

	ToolCallCount  int
	LastToolName   string
	FilesChanged   map[string]bool
	IterationCount int
	ErrorCount     int

	ContextFillPct  float64
	CompactionCount int
	CacheHitRate    float64
	TokenEfficiency float64

	Spin SpinResult
}

type CostEntry struct {
	SessionID  string
	ProjectDir string
	Cost       float64
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionMetrics
	costCalc *CostCalculator
	spins    map[string]*SpinDetector
	contexts map[string]*ContextTracker
	spinCfg  SpinConfig
	logger   *slog.Logger
}

func NewSessionStore(logger *slog.Logger) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*SessionMetrics),
		costCalc: NewCostCalculator(logger),
		spins:    make(map[string]*SpinDetector),
		contexts: make(map[string]*ContextTracker),
		spinCfg:  DefaultSpinConfig(),
		logger:   logger,
	}
}

func (ss *SessionStore) InitSession(id, agent, projectDir string, pid int, active bool, startedAt time.Time) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, exists := ss.sessions[id]; exists {
		s := ss.sessions[id]
		s.PID = pid
		s.Active = active
		return
	}

	ss.sessions[id] = &SessionMetrics{
		SessionID:  id,
		Agent:      agent,
		ProjectDir: projectDir,
		PID:        pid,
		Active:     active,
		StartedAt:  startedAt,
		FilesChanged: make(map[string]bool),
	}
	ss.spins[id] = NewSpinDetector(ss.spinCfg)
	ss.contexts[id] = NewContextTracker()
}

func (ss *SessionStore) ProcessEvent(sessionID string, event *parser.ParsedEvent) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	s, ok := ss.sessions[sessionID]
	if !ok {
		return
	}

	s.LastActivity = event.Timestamp

	if event.Model != "" {
		s.Model = event.Model
		ss.contexts[sessionID].SetMaxContext(event.Model)
	}

	if event.Tokens.Total() > 0 {
		cost := ss.costCalc.Calculate(event.Tokens, s.Model)
		s.TotalCost += cost
		s.TotalInput += event.Tokens.InputTokens
		s.TotalOutput += event.Tokens.OutputTokens
		s.TotalCacheRead += event.Tokens.CacheReadTokens
		s.TotalCacheWrite += event.Tokens.CacheWriteTokens

		ss.contexts[sessionID].Record(event.Tokens)
	}

	if event.ContentType == parser.ContentToolUse {
		s.ToolCallCount++
		s.LastToolName = event.ToolName
		s.IterationCount++
		for _, f := range event.FilesChanged {
			s.FilesChanged[f] = true
		}
	}

	if event.IsError {
		s.ErrorCount++
	}

	spinResult := ss.spins[sessionID].Check(event, s.TotalCost)
	s.Spin = spinResult

	ct := ss.contexts[sessionID]
	s.ContextFillPct = ct.FillPercent()
	s.CompactionCount = ct.CompactionCount()
	s.CacheHitRate = ct.CacheHitRate()
	s.TokenEfficiency = ct.TokenEfficiency()

	elapsed := s.LastActivity.Sub(s.StartedAt).Minutes()
	if elapsed > 0 {
		s.BurnRate = s.TotalCost / elapsed
	}
}

func (ss *SessionStore) Snapshot() []SessionMetrics {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	result := make([]SessionMetrics, 0, len(ss.sessions))
	for _, s := range ss.sessions {
		cp := *s
		cp.FilesChanged = make(map[string]bool, len(s.FilesChanged))
		for k, v := range s.FilesChanged {
			cp.FilesChanged[k] = v
		}
		result = append(result, cp)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Active != result[j].Active {
			return result[i].Active
		}
		return result[i].LastActivity.After(result[j].LastActivity)
	})

	return result
}

func (ss *SessionStore) DailyTotal() float64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	today := time.Now().Truncate(24 * time.Hour)
	var total float64
	for _, s := range ss.sessions {
		if s.StartedAt.After(today) {
			total += s.TotalCost
		}
	}
	return total
}

func (ss *SessionStore) TopSessions(n int) []CostEntry {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	entries := make([]CostEntry, 0, len(ss.sessions))
	for _, s := range ss.sessions {
		entries = append(entries, CostEntry{
			SessionID:  s.SessionID,
			ProjectDir: s.ProjectDir,
			Cost:       s.TotalCost,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Cost > entries[j].Cost
	})

	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}
