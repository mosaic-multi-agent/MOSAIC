package cli

import (
	"fmt"
	"io"

	"mosaic-log-analyzer/internal/domain"
)

// writeTextUsageIngest says that a run's usage is not fully ingested and what
// to do about it. A run with nothing pending writes nothing.
func writeTextUsageIngest(w io.Writer, run domain.RunReport) {
	st := run.UsageIngest
	switch {
	case st.StateUnreadable:
		fmt.Fprintf(w, "  usage not fully ingested: ingest state unreadable by this version\n")
	case st.PendingRecords > 0:
		noun := "records"
		if st.PendingRecords == 1 {
			noun = "record"
		}
		fmt.Fprintf(w, "  usage not fully ingested: %d %s pending; run: mosaic-log-analyzer ingest-otel --run %s\n",
			st.PendingRecords, noun, run.Run.Label())
	}
}
