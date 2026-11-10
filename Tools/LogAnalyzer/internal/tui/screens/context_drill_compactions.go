package screens

import (
	"fmt"

	"mosaic-log-analyzer/internal/domain"
)

// The compaction lines of the Session screen: the count of each kind, the
// compaction calls each harness compaction took, and the unapplied attempts.

// compactionLines lists the plain-text lines that describe an actor's
// compactions and compaction calls.
func compactionLines(cur domain.CurrencyCode, a domain.ActorContext) []string {
	lines := []string{fmt.Sprintf("Compactions: %d logged, %d harness (OTel), %d detected only",
		a.LoggedCount(), a.HarnessCount(), a.DetectedOnlyCount())}
	for _, m := range a.Compactions {
		if m.Kind != domain.CompactionDetected || !m.Harness || m.MatchedLogged {
			continue
		}
		lines = append(lines, fmt.Sprintf("Harness compaction (OTel) %s to %s: %s, %s",
			FormatTokens(domain.Tokens(m.BeforeTokens)), FormatTokens(domain.Tokens(m.AfterTokens)),
			plural(m.CallCount, "compaction call"), FormatMoney(cur, m.CallCost)))
	}
	if a.CompactionCalls.UnappliedCount > 0 {
		lines = append(lines, fmt.Sprintf("Unapplied compaction calls: %d, %s",
			a.CompactionCalls.UnappliedCount, FormatMoney(cur, a.CompactionCalls.UnappliedCost)))
	}
	if a.CompactionCalls.Count > 0 {
		lines = append(lines, fmt.Sprintf("Compaction calls in total: %d, %s (part of the actor's spend)",
			a.CompactionCalls.Count, FormatMoney(cur, a.CompactionCalls.Cost)))
	}
	return lines
}

// plural renders a count with its noun, adding an s unless the count is one.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
