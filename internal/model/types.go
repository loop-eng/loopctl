package model

import (
	"fmt"
	"time"
)

type SessionView struct {
	SessionID       string
	Agent           string
	ProjectDir      string
	ProjectName     string
	Model           string
	Active          bool
	PID             int
	Duration        time.Duration
	TotalCost       float64
	BurnRate        float64
	ToolCallCount   int
	LastToolName    string
	IterationCount  int
	ErrorCount      int
	ContextFillPct  float64
	CompactionCount int
	CacheHitRate    float64
	TokenEfficiency float64
	IsSpinning      bool
	SpinReasons     []string
	TotalInput      int
	TotalOutput     int
	TotalCacheRead  int
	TotalCacheWrite int
	FilesChanged    int
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
