package metrics

import (
	"math"
	"testing"

	"github.com/loop-eng/loopctl/internal/parser"
)

func TestContextTrackerFillPercent(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 100_000, OutputTokens: 5000})

	pct := ct.FillPercent()
	if math.Abs(pct-50.0) > 0.5 {
		t.Errorf("fill percent = %.1f, want ~50.0", pct)
	}
}

func TestContextTrackerFillPercentWithCache(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 3, CacheReadTokens: 80_000, CacheWriteTokens: 20_000})

	pct := ct.FillPercent()
	if math.Abs(pct-50.0) > 1.0 {
		t.Errorf("fill with cache tokens = %.1f, want ~50.0 (100003/200000)", pct)
	}
}

func TestContextTrackerCompaction(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 3, CacheReadTokens: 150_000, CacheWriteTokens: 30_000})
	ct.Record(parser.TokenUsage{InputTokens: 3, CacheReadTokens: 30_000, CacheWriteTokens: 10_000})

	if ct.CompactionCount() != 1 {
		t.Errorf("compaction count = %d, want 1", ct.CompactionCount())
	}
}

func TestContextTrackerNoCompactionOnGradualGrowth(t *testing.T) {
	ct := NewContextTracker()

	for i := 0; i < 10; i++ {
		ct.Record(parser.TokenUsage{InputTokens: 10_000 * (i + 1), OutputTokens: 1000})
	}

	if ct.CompactionCount() != 0 {
		t.Errorf("compaction count = %d, want 0 for gradual growth", ct.CompactionCount())
	}
}

func TestContextTrackerCacheHitRate(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 100_000, CacheReadTokens: 50_000, OutputTokens: 5000})

	rate := ct.CacheHitRate()
	if math.Abs(rate-50.0) > 0.5 {
		t.Errorf("cache hit rate = %.1f, want ~50.0", rate)
	}
}

func TestContextTrackerCacheHitRateCapped(t *testing.T) {
	ct := NewContextTracker()

	for i := 0; i < 10; i++ {
		ct.Record(parser.TokenUsage{InputTokens: 1000, CacheReadTokens: 5000, OutputTokens: 100})
	}

	rate := ct.CacheHitRate()
	if rate > 100 {
		t.Errorf("cache hit rate = %.1f, should not exceed 100", rate)
	}
}

func TestContextTrackerTokenEfficiency(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 80_000, OutputTokens: 20_000})

	eff := ct.TokenEfficiency()
	if math.Abs(eff-20.0) > 0.5 {
		t.Errorf("token efficiency = %.1f, want ~20.0", eff)
	}
}

func TestContextTrackerZeroTokens(t *testing.T) {
	ct := NewContextTracker()
	ct.Record(parser.TokenUsage{})

	if ct.FillPercent() != 0 {
		t.Error("empty record should give 0% fill")
	}
	if ct.CacheHitRate() != 0 {
		t.Error("empty record should give 0% cache hit")
	}
}

func TestContextTrackerSetMaxContext(t *testing.T) {
	ct := NewContextTracker()

	ct.SetMaxContext("gemini-2.5-pro")
	ct.Record(parser.TokenUsage{InputTokens: 100_000})

	pct := ct.FillPercent()
	if math.Abs(pct-10.0) > 0.5 {
		t.Errorf("gemini fill = %.1f, want ~10.0 (100k/1M)", pct)
	}
}

func TestContextTrackerModelSwitchNoFalseCompaction(t *testing.T) {
	ct := NewContextTracker()

	ct.SetMaxContext("claude-opus-4-6")
	ct.Record(parser.TokenUsage{InputTokens: 3, CacheReadTokens: 150_000, CacheWriteTokens: 30_000})

	ct.SetMaxContext("gpt-4.1")
	ct.Record(parser.TokenUsage{InputTokens: 3, CacheReadTokens: 40_000, CacheWriteTokens: 10_000})

	if ct.CompactionCount() != 0 {
		t.Errorf("compaction count = %d, want 0 after model switch", ct.CompactionCount())
	}
}

func TestContextTrackerAccumulatesCorrectly(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200})
	ct.Record(parser.TokenUsage{InputTokens: 2000, OutputTokens: 800, CacheReadTokens: 300})

	eff := ct.TokenEfficiency()
	expectedEff := float64(500+800) / float64(1000+2000+500+800) * 100
	if math.Abs(eff-expectedEff) > 0.5 {
		t.Errorf("token efficiency = %.1f, want ~%.1f", eff, expectedEff)
	}
}
