package cli

import "mosaic-log-analyzer/internal/domain"

// jsonUnpricedReason is one entry of the report's unpriced_reasons list.
type jsonUnpricedReason struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Reason   string `json:"reason"`
}

// encodeUnpricedReasons maps the report's reasons to their wire shape; nil when
// there are none so the list is omitted.
func encodeUnpricedReasons(reasons []domain.UnpricedReason) []jsonUnpricedReason {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]jsonUnpricedReason, 0, len(reasons))
	for _, r := range reasons {
		out = append(out, jsonUnpricedReason{
			Provider: string(r.Key.Provider),
			Model:    string(r.Key.Model),
			Reason:   r.Reason,
		})
	}
	return out
}

// jsonCurrency reads an unset currency as the default, so a hand-built value
// still encodes a valid code.
func jsonCurrency(c domain.CurrencyCode) domain.CurrencyCode {
	if c == "" {
		return domain.DefaultCurrency
	}
	return c
}
