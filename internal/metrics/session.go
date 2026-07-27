package metrics

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

const maxErrorMessages = 50

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
	ErrorMessages  []string // capped ring buffer, most-recent-last, max maxErrorMessages entries
	IterationCount int
	ErrorCount     int

	ContextFillPct  float64
	CompactionCount int
	CacheHitRate    float64
	TokenEfficiency float64

	Spin SpinResult

	// LTF enrichment (see SessionStore.ApplyLTFEvent)
	LTFAvailable         bool
	LTFIterationCount    int
	TerminationReason    string
	VerificationPassRate float64
}

type CostEntry struct {
	SessionID  string
	ProjectDir string
	Cost       float64
}

// SessionStore is safe for concurrent use. Lock ordering: callers that also
// hold Collector.mu (internal/source) must acquire it BEFORE SessionStore.mu
// — never the reverse — to avoid a future deadlock. See GitHub issue #2.
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
		SessionID:    id,
		Agent:        agent,
		ProjectDir:   projectDir,
		PID:          pid,
		Active:       active,
		StartedAt:    startedAt,
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
		model := s.Model
		if model == "" {
			model = "claude-sonnet-4-5"
		}
		cost := ss.costCalc.Calculate(event.Tokens, model)
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
			if len(s.FilesChanged) < 10000 {
				s.FilesChanged[f] = true
			}
		}
	}

	if event.IsError {
		s.ErrorCount++
		if event.ErrorMsg != "" {
			s.ErrorMessages = append(s.ErrorMessages, event.ErrorMsg)
			if len(s.ErrorMessages) > maxErrorMessages {
				s.ErrorMessages = s.ErrorMessages[len(s.ErrorMessages)-maxErrorMessages:]
			}
		}
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

// ApplyLTFEvent merges supplemental LTF trace data into an already-tracked
// session. It is a no-op if sessionID is not currently known — e.g. the
// trace outlives the session, or belongs to a concurrent/older session in
// the same project directory that isn't registered (yet or anymore).
//
// ApplyLTFEvent never modifies TotalCost, TotalInput, TotalOutput,
// TotalCacheRead, TotalCacheWrite, or FilesChanged — those remain sourced
// exclusively from the native JSONL parser via ProcessEvent.
func (ss *SessionStore) ApplyLTFEvent(sessionID string, ev *parser.LTFEvent) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	s, ok := ss.sessions[sessionID]
	if !ok {
		return
	}

	s.LTFAvailable = true

	if ev.IsSummary {
		s.LTFIterationCount = ev.TotalIterations
		if ev.TerminationReason != "" {
			s.TerminationReason = ev.TerminationReason
		}
		s.VerificationPassRate = ev.VerificationPassRate
		return
	}

	if ev.Iteration > s.LTFIterationCount {
		s.LTFIterationCount = ev.Iteration
	}
	if ev.Phase == "terminate" && ev.TerminationReason != "" {
		s.TerminationReason = ev.TerminationReason
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
		cp.ErrorMessages = append([]string(nil), s.ErrorMessages...)
		cp.Spin.Reasons = append([]string(nil), s.Spin.Reasons...)
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

// dailyCutoff returns local midnight for "today" — used consistently by
// DailyTotal and DailyTotalFromSnapshot so both agree on the day boundary.
func dailyCutoff() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func (ss *SessionStore) DailyTotal() float64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	today := dailyCutoff()
	var total float64
	for _, s := range ss.sessions {
		if s.StartedAt.After(today) {
			total += s.TotalCost
		}
	}
	return total
}

// DailyTotalFromSnapshot computes the same figure as DailyTotal but from an
// already-captured snapshot slice, so the result reflects the exact same
// point-in-time session state as the snapshot's per-session costs instead of
// a second, independently-locked read taken a moment later. See GitHub
// issue #1 (Collector.Snapshot previously combined Sessions and DailyTotal
// from two separate SessionStore locks).
func DailyTotalFromSnapshot(snap []SessionMetrics) float64 {
	today := dailyCutoff()
	var total float64
	for _, s := range snap {
		if s.StartedAt.After(today) {
			total += s.TotalCost
		}
	}
	return total
}

func (ss *SessionStore) TopSessions(n int) []CostEntry {
	if n <= 0 {
		return nil
	}
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
