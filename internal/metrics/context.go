package metrics

import "github.com/loop-eng/loopctl/internal/parser"

type ContextTracker struct {
	maxContextTokens int
	cumulativeInput  int
	cumulativeOutput int
	cacheReads       int
	cacheWrites      int
	compactionCount  int
	prevInputTotal   int
	turnCount        int
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

	ct.cumulativeOutput += tokens.OutputTokens
	ct.cacheReads += tokens.CacheReadTokens
	ct.cacheWrites += tokens.CacheWriteTokens

	if tokens.InputTokens > 0 {
		if ct.prevInputTotal > 0 && tokens.InputTokens < ct.prevInputTotal/2 {
			ct.compactionCount++
		}
		ct.prevInputTotal = tokens.InputTokens
		ct.cumulativeInput = tokens.InputTokens
	}

	ct.turnCount++
}

func (ct *ContextTracker) FillPercent() float64 {
	if ct.maxContextTokens == 0 || ct.cumulativeInput == 0 {
		return 0
	}
	pct := float64(ct.cumulativeInput) / float64(ct.maxContextTokens) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (ct *ContextTracker) CacheHitRate() float64 {
	totalInput := ct.cumulativeInput
	if totalInput == 0 {
		return 0
	}
	return float64(ct.cacheReads) / float64(totalInput) * 100
}

func (ct *ContextTracker) TokenEfficiency() float64 {
	total := ct.cumulativeInput + ct.cumulativeOutput
	if total == 0 {
		return 0
	}
	return float64(ct.cumulativeOutput) / float64(total) * 100
}

func (ct *ContextTracker) CompactionCount() int {
	return ct.compactionCount
}

func (ct *ContextTracker) SetMaxContext(model string) {
	switch {
	case len(model) >= 6 && model[:6] == "claude":
		ct.maxContextTokens = 200_000
	case len(model) >= 3 && model[:3] == "gpt":
		ct.maxContextTokens = 128_000
	case len(model) >= 6 && model[:6] == "gemini":
		ct.maxContextTokens = 1_000_000
	default:
		ct.maxContextTokens = 200_000
	}
}
