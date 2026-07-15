package metrics

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/loop-eng/loopctl/internal/parser"
)

type SpinResult struct {
	IsSpinning bool
	Reasons    []string
	Heuristic  string
}

type SpinDetector struct {
	cfg SpinConfig

	recentTools  []toolFingerprint
	toolHead     int
	toolCount    int

	recentErrors []string
	errorHead    int
	errorCount   int

	lastFileEdit time.Time
	hasActivity  bool

	costWindow []timedCost
}

type SpinConfig struct {
	RepeatedCalls      int
	ErrorEcho          int
	StallMinutes       int
	CostVelocityPerMin float64
	WindowSize         int
}

type toolFingerprint struct {
	hash      string
	timestamp time.Time
}

type timedCost struct {
	timestamp time.Time
	cost      float64
}

func DefaultSpinConfig() SpinConfig {
	return SpinConfig{
		RepeatedCalls:      3,
		ErrorEcho:          3,
		StallMinutes:       10,
		CostVelocityPerMin: 2.0,
		WindowSize:         50,
	}
}

func NewSpinDetector(cfg SpinConfig) *SpinDetector {
	return &SpinDetector{
		cfg:          cfg,
		recentTools:  make([]toolFingerprint, cfg.WindowSize),
		recentErrors: make([]string, cfg.WindowSize),
	}
}

func (sd *SpinDetector) Check(event *parser.ParsedEvent, sessionCost float64) SpinResult {
	var result SpinResult

	switch event.ContentType {
	case parser.ContentToolUse:
		sd.recordTool(event)
		if r := sd.checkRepeatedTools(); r != "" {
			result.IsSpinning = true
			result.Reasons = append(result.Reasons, r)
			if result.Heuristic == "" {
				result.Heuristic = "repeated_tool_calls"
			}
		}

		if parser.IsFileModifyingTool(event.ToolName) {
			sd.lastFileEdit = event.Timestamp
		}

	case parser.ContentToolResult:
		if event.IsError {
			sd.recordError(event.ErrorMsg)
			if r := sd.checkErrorEcho(); r != "" {
				result.IsSpinning = true
				result.Reasons = append(result.Reasons, r)
				if result.Heuristic == "" {
					result.Heuristic = "error_echo"
				}
			}
		}
	}

	if event.Tokens.Total() > 0 {
		sd.hasActivity = true
	}

	if r := sd.checkStall(event.Timestamp); r != "" {
		result.Reasons = append(result.Reasons, r)
	}

	if sessionCost > 0 {
		sd.recordCost(event.Timestamp, sessionCost)
		if r := sd.checkCostVelocity(); r != "" {
			result.IsSpinning = true
			result.Reasons = append(result.Reasons, r)
			if result.Heuristic == "" {
				result.Heuristic = "cost_velocity"
			}
		}
	}

	return result
}

func (sd *SpinDetector) recordTool(event *parser.ParsedEvent) {
	hash := fingerprint(event.ToolName, event.ToolInput)
	sd.recentTools[sd.toolHead] = toolFingerprint{hash: hash, timestamp: event.Timestamp}
	sd.toolHead = (sd.toolHead + 1) % sd.cfg.WindowSize
	if sd.toolCount < sd.cfg.WindowSize {
		sd.toolCount++
	}
}

func (sd *SpinDetector) checkRepeatedTools() string {
	if sd.toolCount < sd.cfg.RepeatedCalls {
		return ""
	}

	counts := make(map[string]int)
	for i := 0; i < sd.toolCount; i++ {
		h := sd.recentTools[i].hash
		if h != "" {
			counts[h]++
		}
	}

	for _, count := range counts {
		if count >= sd.cfg.RepeatedCalls {
			return fmt.Sprintf("same tool call repeated %d times (threshold: %d)", count, sd.cfg.RepeatedCalls)
		}
	}
	return ""
}

func (sd *SpinDetector) recordError(errMsg string) {
	normalized := normalizeError(errMsg)
	sd.recentErrors[sd.errorHead] = normalized
	sd.errorHead = (sd.errorHead + 1) % sd.cfg.WindowSize
	if sd.errorCount < sd.cfg.WindowSize {
		sd.errorCount++
	}
}

func (sd *SpinDetector) checkErrorEcho() string {
	if sd.errorCount < sd.cfg.ErrorEcho {
		return ""
	}

	counts := make(map[string]int)
	for i := 0; i < sd.errorCount; i++ {
		e := sd.recentErrors[i]
		if e != "" {
			counts[e]++
		}
	}

	for _, count := range counts {
		if count >= sd.cfg.ErrorEcho {
			return fmt.Sprintf("same error repeated %d times (threshold: %d)", count, sd.cfg.ErrorEcho)
		}
	}
	return ""
}

func (sd *SpinDetector) checkStall(now time.Time) string {
	if !sd.hasActivity || sd.lastFileEdit.IsZero() {
		return ""
	}

	stallDuration := time.Duration(sd.cfg.StallMinutes) * time.Minute
	if now.Sub(sd.lastFileEdit) > stallDuration {
		return fmt.Sprintf("no file modifications for %d minutes despite ongoing activity", sd.cfg.StallMinutes)
	}
	return ""
}

func (sd *SpinDetector) recordCost(ts time.Time, cost float64) {
	sd.costWindow = append(sd.costWindow, timedCost{timestamp: ts, cost: cost})

	cutoff := ts.Add(-5 * time.Minute)
	trimIdx := 0
	for trimIdx < len(sd.costWindow) && sd.costWindow[trimIdx].timestamp.Before(cutoff) {
		trimIdx++
	}
	if trimIdx > 0 {
		remaining := make([]timedCost, len(sd.costWindow)-trimIdx)
		copy(remaining, sd.costWindow[trimIdx:])
		sd.costWindow = remaining
	}
}

func (sd *SpinDetector) checkCostVelocity() string {
	if len(sd.costWindow) < 2 {
		return ""
	}

	first := sd.costWindow[0]
	last := sd.costWindow[len(sd.costWindow)-1]

	elapsed := last.timestamp.Sub(first.timestamp).Minutes()
	if elapsed < 1.0 {
		return ""
	}

	costDelta := last.cost - first.cost
	velocity := costDelta / elapsed

	if velocity > sd.cfg.CostVelocityPerMin {
		return fmt.Sprintf("cost velocity $%.2f/min exceeds threshold $%.2f/min", velocity, sd.cfg.CostVelocityPerMin)
	}
	return ""
}

func fingerprint(toolName, toolInput string) string {
	h := sha256.Sum256([]byte(toolName + "|" + toolInput))
	return fmt.Sprintf("%x", h[:8])
}

func normalizeError(msg string) string {
	s := strings.TrimSpace(msg)
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.ToLower(s)
}
