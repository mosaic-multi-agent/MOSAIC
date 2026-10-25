package cost

import "mosaic-log-analyzer/internal/domain"

// Tier names which rate set applies to a request.
type Tier uint8

const (
	TierStandard    Tier = iota // all categories priced from the standard rate set
	TierLongContext             // all categories priced from the long-context rate set
)

// SelectTier chooses the rate-set tier for ONE request.
//
// Binding rules:
//   - Total context = input + cache read + cache creation (absent excluded).
//   - Threshold comparison is >= : exactly at the threshold selects TierLongContext.
//   - A model without a long_context block (HasLongContext == false) is always
//     TierStandard.
//   - When total context is absent entirely, TierStandard is selected and ok is
//     false so the caller can raise a finding.
//   - Selection is per request, NEVER per run. Summing a run's tokens and then
//     tiering is incorrect by contract. The chosen tier applies to every token
//     category of the request.
func SelectTier(usage domain.TokenUsage, entry domain.ModelPricing) (t Tier, ok bool) {
	totalContext := usage.TotalContext()
	ctxValue, ctxPresent := totalContext.Value()

	// A model with no long-context tier is always TierStandard.
	// ok reflects whether context data was present, regardless of threshold config.
	if !entry.HasLongContext {
		return TierStandard, ctxPresent
	}

	// Total context absent: cannot determine tier; default to standard.
	if !ctxPresent {
		return TierStandard, false
	}

	// The comparison is >=: exactly at the threshold triggers the long-context tier.
	if ctxValue >= entry.LongContext.Threshold {
		return TierLongContext, true
	}
	return TierStandard, true
}
