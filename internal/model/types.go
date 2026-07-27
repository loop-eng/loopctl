package model

import (
	"fmt"
	"time"
)

type SessionView struct {
	SessionID        string
	Agent            string
	ProjectDir       string
	ProjectName      string
	Model            string
	Active           bool
	PID              int
	StartedAt        time.Time
	LastActivity     time.Time
	Duration         time.Duration
	TotalCost        float64
	BurnRate         float64
	ToolCallCount    int
	LastToolName     string
	IterationCount   int
	ErrorCount       int
	ContextFillPct   float64
	CompactionCount  int
	CacheHitRate     float64
	TokenEfficiency  float64
	IsSpinning       bool
	HasWarnings      bool
	SpinReasons      []string
	TotalInput       int
	TotalOutput      int
	TotalCacheRead   int
	TotalCacheWrite  int
	FilesChanged     int
	FilesChangedList []string
	ErrorMessages    []string

	// LTF enrichment (only populated when a .loop/trace.ltf.jsonl adapter
	// trace is available for the session's project; see internal/source/ltf.go)
	LTFAvailable         bool
	LTFIterationCount    int
	TerminationReason    string
	VerificationPassRate float64
}

type Alert struct {
	SessionID string
	Severity  string
	Message   string
	Timestamp time.Time
}

type DataMsg struct {
	Sessions   []SessionView
	DailyTotal float64
	Alerts     []Alert
}

type TickMsg time.Time

type ExportDoneMsg struct {
	Path string
	Err  error
}

func FormatPercent(pct float64) string {
	return fmt.Sprintf("%.0f%%", pct)
}
