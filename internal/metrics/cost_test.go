package metrics

import (
	"log/slog"
	"math"
	"testing"

	"github.com/loop-eng/loopctl/internal/parser"
)

func TestCostCalculatorKnownModel(t *testing.T) {
	cc := NewCostCalculator(slog.Default())

	usage := parser.TokenUsage{
		InputTokens:      1_000_000,
		OutputTokens:     1_000_000,
		CacheReadTokens:  1_000_000,
		CacheWriteTokens: 1_000_000,
	}

	cost := cc.Calculate(usage, "claude-opus-4-6")

	expected := 5.00 + 25.00 + 0.50 + 6.25
	if math.Abs(cost-expected) > 0.01 {
		t.Errorf("cost = %.2f, want %.2f", cost, expected)
	}
}

func TestCostCalculatorPrefixMatch(t *testing.T) {
	cc := NewCostCalculator(slog.Default())

	usage := parser.TokenUsage{InputTokens: 1_000_000}
	cost := cc.Calculate(usage, "claude-opus-4-6[1m]")

	if math.Abs(cost-5.00) > 0.01 {
		t.Errorf("prefix match cost = %.2f, want 5.00", cost)
	}
}

func TestCostCalculatorFallback(t *testing.T) {
	cc := NewCostCalculator(slog.Default())

	usage := parser.TokenUsage{InputTokens: 1_000_000}
	cost := cc.Calculate(usage, "unknown-model")

	if math.Abs(cost-3.00) > 0.01 {
		t.Errorf("fallback cost = %.2f, want 3.00", cost)
	}
}

func TestCostCalculatorZeroTokens(t *testing.T) {
	cc := NewCostCalculator(slog.Default())
	cost := cc.Calculate(parser.TokenUsage{}, "claude-opus-4-6")
	if cost != 0 {
		t.Errorf("zero tokens should have zero cost, got %.6f", cost)
	}
}

func TestCostCalculatorLongestPrefixWins(t *testing.T) {
	cc := NewCostCalculator(slog.Default())

	usage := parser.TokenUsage{InputTokens: 1_000_000}

	costMini := cc.Calculate(usage, "gpt-4.1-mini")
	costFull := cc.Calculate(usage, "gpt-4.1")

	if math.Abs(costMini-0.40) > 0.01 {
		t.Errorf("gpt-4.1-mini cost = %.2f, want 0.40", costMini)
	}
	if math.Abs(costFull-2.00) > 0.01 {
		t.Errorf("gpt-4.1 cost = %.2f, want 2.00", costFull)
	}
}
