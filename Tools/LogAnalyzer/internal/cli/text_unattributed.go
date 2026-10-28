package cli

import (
	"fmt"
	"io"

	"mosaic-log-analyzer/internal/domain"
)

// writeTextUnattributed writes the run-level unattributed usage: its share of
// the run total from the overview, then its own row with figures. A run
// without unattributed usage writes nothing.
func writeTextUnattributed(w io.Writer, run domain.RunReport) {
	if run.Unattributed == nil {
		return
	}
	if share := run.Overview.UnattributedShare; share.Denominator > 0 {
		pct := float64(share.Numerator) * 100 / float64(share.Denominator)
		fmt.Fprintf(w, "  Unattributed share of run total: %.1f%%\n", pct)
	}
	fmt.Fprintf(w, "  Unattributed:\n")
	writeTextEffort(w, run.Unattributed.Effort, "    ")
	writeTextTotals(w, run.Unattributed.Totals, "    ")
}
