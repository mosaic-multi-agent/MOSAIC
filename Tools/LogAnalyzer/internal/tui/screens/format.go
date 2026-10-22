package screens

import (
	"fmt"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// AbsentMarker is rendered for a token category with no data. It must never be
// "0" and must never be confused with a zero figure.
const AbsentMarker = "—"

// UnpricedMarker is rendered for a money cell whose model has no pricing entry.
// It must never be "$0.00".
const UnpricedMarker = "unpriced"

// ProvisionalLabel marks a run with no run_end/session_end so its cost is not
// read as final.
const ProvisionalLabel = "in progress"

// UnattributableLabel titles the separate unknown-run section.
const UnattributableLabel = "Unattributable"

// AssumedTierLabel marks cache-write tokens priced at the 5m rate because their
// request reported no tier split. It labels an estimate, never reported data.
const AssumedTierLabel = "assumed 5m tier"

// Labels for figures whose data is incomplete or unnamed in the per-run views.
const (
	OpenLabel         = "open"           // invocation with no end event
	NoStatusLabel     = "no status"      // ended without status_code
	MissingUsageLabel = "no token usage" // invocation without any token count
	UnknownTypeLabel  = "(unknown type)" // AgentType == ""
	UnknownModelLabel = "(unknown model)"
	UnnamedToolLabel  = "(unnamed tool)" // Tool == ""
	UnpairedLabel     = "unpaired"       // tool call without an unambiguous start/end pair
)

// FormatDuration renders a duration, or AbsentMarker when ok is false.
func FormatDuration(d time.Duration, ok bool) string {
	if !ok {
		return AbsentMarker
	}
	return d.Round(time.Millisecond).String()
}

// FormatAssumedTier renders a labelled count of cache-write tokens priced at an
// assumed tier: "" when tokens == 0, otherwise a string containing
// AssumedTierLabel and the grouped count.
func FormatAssumedTier(tokens int64) string {
	if tokens == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %s cache-write tokens", AssumedTierLabel, formatGrouped(tokens))
}

// FormatRatio renders a ratio as a percentage with one decimal ("66.7%"), or
// AbsentMarker when the ratio is undefined.
func FormatRatio(r domain.Ratio) string {
	if r.Denominator <= 0 {
		return AbsentMarker
	}
	return fmt.Sprintf("%.1f%%", float64(r.Numerator)/float64(r.Denominator)*100)
}

// AssumedTierNotice renders a labelled count of cache-write tokens priced at an
// assumed tier.
func AssumedTierNotice(tokens int64) string {
	return fmt.Sprintf("Cache writes priced at %s: %s tokens (no tier split reported)", AssumedTierLabel, formatGrouped(tokens))
}

// FormatTokens renders a TokenCount: AbsentMarker when absent, the grouped
// number otherwise (including "0" for a present zero).
func FormatTokens(t domain.TokenCount) string {
	v, ok := t.Value()
	if !ok {
		return AbsentMarker
	}
	return formatGrouped(v)
}

// FormatMoney renders a MoneyValue: AbsentMarker for MoneyNoData,
// UnpricedMarker for MoneyUnpriced, and a currency-formatted amount for
// MoneyKnown (including "$0.00").
func FormatMoney(v domain.MoneyValue) string {
	switch v.State {
	case domain.MoneyNoData:
		return AbsentMarker
	case domain.MoneyUnpriced:
		return UnpricedMarker
	case domain.MoneyKnown:
		return "$" + v.Amount.StringFixed(2)
	default:
		return AbsentMarker
	}
}

// FormatTotal renders a CategoryMoney total, appending a partial-total marker
// when Complete is false.
func FormatTotal(c domain.CategoryMoney) string {
	s := FormatMoney(c.Total)
	if !c.Complete {
		s += " (partial)"
	}
	return s
}

// FormatTotalsLine renders a compact single-line summary of a Totals value
// showing token categories and money side by side. Category names come from
// domain.TokenCategory.Label().
func FormatTotalsLine(t domain.Totals) string {
	parts := make([]string, 0, len(domain.BillableCategories())+1)
	for _, cat := range domain.BillableCategories() {
		count, _ := t.Tokens.Get(cat)
		parts = append(parts, fmt.Sprintf("%s: %s", cat.Label(), FormatTokens(count)))
	}
	parts = append(parts, FormatTotal(t.Money))
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "  "
		}
		result += p
	}
	return result
}

// formatGrouped formats an int64 with comma grouping for readability.
func formatGrouped(n int64) string {
	if n < 0 {
		return "-" + formatGrouped(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return formatGrouped(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}
