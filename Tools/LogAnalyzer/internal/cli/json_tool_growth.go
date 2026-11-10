package cli

import "mosaic-log-analyzer/internal/domain"

// jsonToolGrowth is the context growth of the singular calls that requested
// only one tool. The figures are null when no call was measured.
type jsonToolGrowth struct {
	Samples       int    `json:"samples"`
	AverageTokens *int64 `json:"average_tokens"`
	MinTokens     *int64 `json:"min_tokens"`
	MaxTokens     *int64 `json:"max_tokens"`
	Negative      int    `json:"negative"`
	NotMeasurable int    `json:"not_measurable"`
}

func encodeToolGrowth(g domain.ToolGrowth) jsonToolGrowth {
	out := jsonToolGrowth{Samples: g.Samples, Negative: g.Negative, NotMeasurable: g.NotMeasurable}
	if g.Samples > 0 {
		out.AverageTokens = tokenPtr(g.Average)
		out.MinTokens = tokenPtr(g.Min)
		out.MaxTokens = tokenPtr(g.Max)
	}
	return out
}

func tokenPtr(t domain.TokenCount) *int64 {
	v, ok := t.Value()
	if !ok {
		return nil
	}
	return &v
}
