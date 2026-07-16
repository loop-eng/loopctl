package metrics

import (
	"strings"

	"github.com/loop-eng/loopctl/internal/parser"
)

type ContextTracker struct {
	maxContextTokens  int
	totalInput        int
	totalOutput       int
	totalCacheRead    int
	totalCacheWrite   int
	compactionCount   int
	prevContextLoad   int
	lastModel         string
	turnCount         int
}

func NewContextTracker() *ContextTracker {
	return &ContextTracker{
		maxContextTokens: 200_000,
	}
}

func (ct *ContextTracker) Record(tokens parser.TokenUsage) {
	if tokens.Total() == 0 {
		return
	}

	ct.totalInput += tokens.InputTokens
	ct.totalOutput += tokens.OutputTokens
	ct.totalCacheRead += tokens.CacheReadTokens
	ct.totalCacheWrite += tokens.CacheWriteTokens

	contextLoad := tokens.InputTokens + tokens.CacheReadTokens + tokens.CacheWriteTokens
	if contextLoad > 0 {
		if ct.prevContextLoad > 0 && contextLoad < ct.prevContextLoad/2 {
			ct.compactionCount++
		}
		ct.prevContextLoad = contextLoad
	}

	ct.turnCount++
}

func (ct *ContextTracker) FillPercent() float64 {
	if ct.maxContextTokens == 0 || ct.prevContextLoad == 0 {
		return 0
	}
	pct := float64(ct.prevContextLoad) / float64(ct.maxContextTokens) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (ct *ContextTracker) CacheHitRate() float64 {
	if ct.totalInput == 0 {
		return 0
	}
	rate := float64(ct.totalCacheRead) / float64(ct.totalInput) * 100
	if rate > 100 {
		rate = 100
	}
	return rate
}

func (ct *ContextTracker) TokenEfficiency() float64 {
	total := ct.totalInput + ct.totalOutput
	if total == 0 {
		return 0
	}
	return float64(ct.totalOutput) / float64(total) * 100
}

func (ct *ContextTracker) CompactionCount() int {
	return ct.compactionCount
}

func (ct *ContextTracker) SetMaxContext(modelName string) {
	if modelName == ct.lastModel {
		return
	}
	oldMax := ct.maxContextTokens
	ct.lastModel = modelName

	switch {
	case strings.HasPrefix(modelName, "claude"):
		ct.maxContextTokens = 200_000
	case strings.HasPrefix(modelName, "gpt"):
		ct.maxContextTokens = 128_000
	case strings.HasPrefix(modelName, "gemini"):
		ct.maxContextTokens = 1_000_000
	default:
		ct.maxContextTokens = 200_000
	}

	// Reset compaction baseline on model switch to avoid false positives
	if ct.maxContextTokens != oldMax {
		ct.prevContextLoad = 0
	}
}
