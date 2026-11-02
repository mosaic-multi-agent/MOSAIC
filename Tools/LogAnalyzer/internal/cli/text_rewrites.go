package cli

import (
	"fmt"
	"io"

	"mosaic-log-analyzer/internal/domain"
)

// writeTextRewrites writes the run's context rewrite line; a run without
// rewrites gets none.
func writeTextRewrites(w io.Writer, cur domain.CurrencyCode, f domain.RewriteFigures, indent string) {
	if f.Count() == 0 {
		return
	}
	tokens, _ := f.Tokens.Value()
	fmt.Fprintf(w, "%sContext rewrites: %d (model changed %d, cold after idle %d, partial %d), %s tokens, cost %s, extra %s\n",
		indent, f.Count(), f.ModelChanged, f.ColdIdle, f.Partial, formatGrouped(tokens),
		formatTextMoney(cur, f.Cost), formatTextMoney(cur, f.ExtraCost))
}
