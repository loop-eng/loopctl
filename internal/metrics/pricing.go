package metrics

type ModelPricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheReadPerMTok  float64
	CacheWritePerMTok float64
}

func DefaultPricing() map[string]ModelPricing {
	return map[string]ModelPricing{
		"claude-opus-4-8":   {5.00, 25.00, 0.50, 6.25},
		"claude-opus-4-7":   {5.00, 25.00, 0.50, 6.25},
		"claude-opus-4-6":   {5.00, 25.00, 0.50, 6.25},
		"claude-sonnet-4-6": {3.00, 15.00, 0.30, 3.75},
		"claude-sonnet-4-5": {3.00, 15.00, 0.30, 3.75},
		"claude-haiku-4-5":  {1.00, 5.00, 0.10, 1.25},

		"gpt-5.5":      {5.00, 30.00, 0, 0},
		"gpt-4.1":      {2.00, 8.00, 0, 0},
		"gpt-4.1-mini": {0.40, 1.60, 0, 0},
		"o4-mini":      {1.10, 4.40, 0, 0},
		"o3":           {2.00, 8.00, 0, 0},

		"gemini-2.5-pro":   {1.25, 10.00, 0, 0},
		"gemini-2.5-flash": {0.15, 0.60, 0, 0},
	}
}

var FallbackPricing = ModelPricing{3.00, 15.00, 0.30, 3.75}
