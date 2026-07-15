package metrics

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/loop-eng/loopctl/internal/parser"
)

type modelEntry struct {
	name    string
	pricing ModelPricing
}

type CostCalculator struct {
	pricing map[string]ModelPricing
	sorted  []modelEntry
	logger  *slog.Logger
}

func NewCostCalculator(logger *slog.Logger) *CostCalculator {
	pricing := DefaultPricing()
	sorted := make([]modelEntry, 0, len(pricing))
	for name, p := range pricing {
		sorted = append(sorted, modelEntry{name, p})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].name) > len(sorted[j].name)
	})
	return &CostCalculator{
		pricing: pricing,
		sorted:  sorted,
		logger:  logger,
	}
}

func (cc *CostCalculator) Calculate(usage parser.TokenUsage, model string) float64 {
	p := cc.resolve(model)

	cost := float64(usage.InputTokens) * p.InputPerMTok / 1_000_000
	cost += float64(usage.OutputTokens) * p.OutputPerMTok / 1_000_000
	cost += float64(usage.CacheReadTokens) * p.CacheReadPerMTok / 1_000_000
	cost += float64(usage.CacheWriteTokens) * p.CacheWritePerMTok / 1_000_000

	return cost
}

func (cc *CostCalculator) resolve(model string) ModelPricing {
	if p, ok := cc.pricing[model]; ok {
		return p
	}

	for _, entry := range cc.sorted {
		if strings.HasPrefix(model, entry.name) {
			return entry.pricing
		}
	}

	cc.logger.Warn("unknown model, using fallback pricing", "model", model)
	return FallbackPricing
}
