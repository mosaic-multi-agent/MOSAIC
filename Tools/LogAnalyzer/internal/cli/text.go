package cli

import (
	"fmt"
	"io"
	"strconv"

	"mosaic-log-analyzer/internal/domain"
)

// EncodeText writes a human-readable rendering of the report to w.
//
// The output includes:
//   - All-runs totals (named runs only; unattributable is separate)
//   - Per-run rows with per-actor breakdown
//   - Provisional label for in-progress runs (no run_end/session_end observed)
//   - Unpriced marker for money cells whose model has no pricing entry
//   - The unattributable section (if present), separate from named-run totals
//   - Data-quality summary (findings count and individual findings)
func EncodeText(w io.Writer, r domain.Report) error {
	// All-runs total (named runs only)
	fmt.Fprintf(w, "=== All Runs Total ===\n")
	writeTextTotals(w, r.Currency, r.AllRuns, "  ")

	// Named runs
	if len(r.Runs) > 0 {
		fmt.Fprintf(w, "\n=== Runs ===\n")
		for _, run := range r.Runs {
			writeTextRun(w, r.Currency, run)
		}
	}

	// Unattributable bucket (distinct section, not folded into named-run totals)
	if r.Unattributable != nil {
		fmt.Fprintf(w, "\n=== Unattributable ===\n")
		writeTextTotals(w, r.Currency, r.Unattributable.Totals, "  ")
		for _, agent := range r.Unattributable.Agents {
			fmt.Fprintf(w, "  Agent: %s\n", agent.Actor.Label())
			writeTextTotals(w, r.Currency, agent.Totals, "    ")
		}
	}

	// Data-quality summary
	if !r.Quality.IsClean() {
		fmt.Fprintf(w, "\n=== Data Quality Findings ===\n")
		for kind, count := range r.Quality.Counts {
			fmt.Fprintf(w, "  %-30s  %d\n", string(kind), count)
		}
		if len(r.Quality.Findings) > 0 {
			fmt.Fprintf(w, "\nIndividual findings:\n")
			for _, f := range r.Quality.Findings {
				severity := "info"
				if f.Severity == domain.SeverityWarning {
					severity = "warning"
				}
				if f.Path != "" && f.Line > 0 {
					fmt.Fprintf(w, "  [%s] %s at %s:%d  %s\n", severity, string(f.Kind), f.Path, f.Line, f.Detail)
				} else if f.Path != "" {
					fmt.Fprintf(w, "  [%s] %s at %s  %s\n", severity, string(f.Kind), f.Path, f.Detail)
				} else {
					fmt.Fprintf(w, "  [%s] %s  %s\n", severity, string(f.Kind), f.Detail)
				}
			}
		}
	} else if r.Quality.Incomplete() {
		fmt.Fprintf(w, "\nData quality: incomplete (some data could not be read)\n")
	}

	// Unpriced model list
	if len(r.UnpricedModels) > 0 {
		fmt.Fprintf(w, "\nUnpriced models (no pricing entry found):\n")
		reasons := make(map[domain.PricingKey]string, len(r.UnpricedReasons))
		for _, ur := range r.UnpricedReasons {
			reasons[ur.Key] = ur.Reason
		}
		for _, m := range r.UnpricedModels {
			if reason := reasons[m]; reason != "" {
				fmt.Fprintf(w, "  %s: %s\n", m.String(), reason)
				continue
			}
			fmt.Fprintf(w, "  %s\n", m.String())
		}
	}

	if assumed := reportAssumedTierTokens(r); assumed > 0 {
		fmt.Fprintf(w, "\nNote: these costs are estimates; cache writes priced at the %s (no tier split reported): %s tokens\n", domain.AssumedTierLabel(reportAssumedTiers(r)), formatGrouped(assumed))
	}

	return nil
}

// writeTextRun writes one run's section to w.
func writeTextRun(w io.Writer, cur domain.CurrencyCode, run domain.RunReport) {
	label := run.Run.Label()
	if run.Provisional {
		fmt.Fprintf(w, "\nRun: %s  [in progress]\n", label)
	} else {
		fmt.Fprintf(w, "\nRun: %s\n", label)
	}
	writeTextTotals(w, cur, run.Totals, "  ")
	writeTextRewrites(w, cur, run.Cache.Rewrites, "  ")

	// Orchestrator actor
	fmt.Fprintf(w, "  Orchestrator:\n")
	writeTextEffort(w, run.Orchestrator.Effort, "    ")
	writeTextTotals(w, cur, run.Orchestrator.Totals, "    ")

	// Agent-instance actors
	for _, agent := range run.Agents {
		fmt.Fprintf(w, "  Agent: %s\n", agent.Actor.Label())
		writeTextEffort(w, agent.Effort, "    ")
		writeTextTotals(w, cur, agent.Totals, "    ")
	}

	writeTextUnattributed(w, cur, run)
	writeTextUsageIngest(w, run)
}

// writeTextTotals writes a compact token + money summary at the given indent level.
func writeTextTotals(w io.Writer, cur domain.CurrencyCode, t domain.Totals, indent string) {
	// Token figures
	writeTextTokens(w, t.Tokens, indent)
	// Money summary (total)
	fmt.Fprintf(w, "%sMoney: %s\n", indent, formatTextMoney(cur, t.Money.Total))
	if t.AssumedTierCacheWrite > 0 {
		fmt.Fprintf(w, "%sCache writes priced at %s: %s tokens\n", indent, domain.AssumedTierLabel(t.AssumedTiers), formatGrouped(t.AssumedTierCacheWrite))
	}
}

// writeTextTokens writes per-category token counts with thousand-separator grouping.
// Labels come from domain.TokenCategory.Label() so the CLI and TUI vocabularies
// cannot drift apart.
func writeTextTokens(w io.Writer, u domain.TokenUsage, indent string) {
	for _, cat := range domain.BillableCategories() {
		count, _ := u.Get(cat)
		v, ok := count.Value()
		if ok {
			fmt.Fprintf(w, "%s%s: %s tokens\n", indent, cat.Label(), formatGrouped(v))
		} else {
			fmt.Fprintf(w, "%s%s: -\n", indent, cat.Label())
		}
	}
	// Reasoning is part of output, shown beneath it; it is never a cost line.
	if v, ok := u.Reasoning.Value(); ok {
		fmt.Fprintf(w, "%s  %s: %s tokens\n", indent, domain.CategoryReasoning.Label(), formatGrouped(v))
	} else {
		fmt.Fprintf(w, "%s  %s: -\n", indent, domain.CategoryReasoning.Label())
	}
}

// formatGrouped formats n with comma thousand-separators (e.g. 1234567 → "1,234,567").
func formatGrouped(n int64) string {
	if n < 0 {
		return "-" + formatGrouped(-n)
	}
	s := strconv.FormatInt(n, 10)
	// Insert commas every three digits from the right.
	out := make([]byte, 0, len(s)+(len(s)-1)/3)
	offset := len(s) % 3
	if offset == 0 {
		offset = 3
	}
	out = append(out, s[:offset]...)
	for i := offset; i < len(s); i += 3 {
		out = append(out, ',')
		out = append(out, s[i:i+3]...)
	}
	return string(out)
}

// formatTextMoney returns a human-readable money string.
// Unpriced is rendered as "unpriced" (not a zero amount) to distinguish it from zero.
// No-data is rendered as "-".
func formatTextMoney(cur domain.CurrencyCode, mv domain.MoneyValue) string {
	switch mv.State {
	case domain.MoneyKnown:
		return domain.FormatAmount(mv.Amount, cur, false)
	case domain.MoneyUnpriced:
		return "unpriced"
	default: // MoneyNoData
		return "-"
	}
}

// reportAssumedTierTokens sums the assumed-tier cache-write tokens of the named
// runs and the unattributable bucket.
func reportAssumedTierTokens(r domain.Report) int64 {
	total := r.AllRuns.AssumedTierCacheWrite
	if r.Unattributable != nil {
		total += r.Unattributable.Totals.AssumedTierCacheWrite
	}
	return total
}

// writeTextEffort writes the effort line; "-" marks that no level was reported.
func writeTextEffort(w io.Writer, l domain.EffortLevels, indent string) {
	label := l.Label()
	if l.IsEmpty() {
		label = "-"
	}
	fmt.Fprintf(w, "%sEffort: %s\n", indent, label)
}

// reportAssumedTiers merges the assumed tiers of the named runs and the
// unattributable bucket.
func reportAssumedTiers(r domain.Report) []domain.CacheWriteTier {
	if r.Unattributable == nil {
		return r.AllRuns.AssumedTiers
	}
	return domain.MergeCacheWriteTiers(r.AllRuns.AssumedTiers, r.Unattributable.Totals.AssumedTiers)
}
