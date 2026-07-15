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

func TestContextTrackerCompaction(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 180_000, OutputTokens: 5000})
	ct.Record(parser.TokenUsage{InputTokens: 50_000, OutputTokens: 5000})

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

	ct.Record(parser.TokenUsage{InputTokens: 100_000, CacheReadTokens: 80_000, OutputTokens: 5000})

	rate := ct.CacheHitRate()
	if math.Abs(rate-80.0) > 0.5 {
		t.Errorf("cache hit rate = %.1f, want ~80.0", rate)
	}
}

func TestContextTrackerTokenEfficiency(t *testing.T) {
	ct := NewContextTracker()

	ct.Record(parser.TokenUsage{InputTokens: 100_000, OutputTokens: 25_000})

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
