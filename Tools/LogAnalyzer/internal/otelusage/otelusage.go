// Package otelusage implements the usage-ingest ports over mosaic-common/otelingest.
package otelusage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mosaic-common/otelingest"

	"mosaic-log-analyzer/internal/domain"
)

// Adapter implements domain.UsageIngester and domain.IngestStateProbe.
type Adapter struct{}

// New returns an Adapter.
func New() *Adapter { return &Adapter{} }

// IngestRun runs a final, whole-file ingest of the export into the run folder.
// The error is non-nil only for an infrastructure failure.
func (a *Adapter) IngestRun(_ context.Context, req domain.UsageIngestRequest) (domain.UsageIngestResult, error) {
	res := otelingest.Ingest(otelingest.Request{
		RunDir:    req.RunDir,
		OTelPath:  req.ExportPath,
		Final:     true,
		WholeFile: true,
	})
	out := domain.UsageIngestResult{
		Status:         statusOf(res.Status),
		RecordsWritten: res.RecordsWritten,
		Finalized:      res.Finalized,
		Pending:        res.Pending,
	}
	for _, d := range res.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, domain.UsageIngestDiagnostic{
			Kind: string(d.Kind), Message: diagnosticMessage(d.Message, d.Counts, d.IDs),
		})
	}
	return out, res.Err
}

// PendingUsage reads the run's ingest state without modifying it.
func (a *Adapter) PendingUsage(runDir string) domain.UsageIngestState {
	st := otelingest.ReadPendingStatus(runDir)
	return domain.UsageIngestState{
		PendingRecords:  st.Pending,
		StateUnreadable: st.StatePresent && !st.StateKnown,
	}
}

func statusOf(s otelingest.Status) domain.UsageIngestStatus {
	switch s {
	case otelingest.StatusIngested:
		return domain.IngestDone
	case otelingest.StatusLockHeld:
		return domain.IngestLockHeld
	case otelingest.StatusFileMissing:
		return domain.IngestFileMissing
	case otelingest.StatusUnknownState:
		return domain.IngestUnknownState
	}
	return domain.IngestFailed
}

// diagnosticMessage appends the counts and ids a diagnostic carries to its text.
func diagnosticMessage(msg string, counts map[string]int, ids []string) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	if len(ids) > 0 {
		parts = append(parts, "ids="+strings.Join(ids, ","))
	}
	if len(parts) == 0 {
		return msg
	}
	return msg + " (" + strings.Join(parts, ", ") + ")"
}
