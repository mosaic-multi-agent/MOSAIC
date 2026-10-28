package app

import (
	"context"

	"mosaic-log-analyzer/internal/domain"
)

// IngestRequest parameterises an ingest-otel run.
type IngestRequest struct {
	ExplicitPath string // logs root or single run folder
	RunID        string
	ExportPath   string // the usage export file
}

// IngestOutcome is the result of an ingest-otel run.
type IngestOutcome struct {
	Outcome  Outcome
	RunFound bool
	Result   domain.UsageIngestResult
}

// IngestOTel runs a final recovery ingest into one run's folder. It resolves
// the source from the log structure and never reads event files.
func (s *Service) IngestOTel(ctx context.Context, req IngestRequest) (IngestOutcome, error) {
	src, outcome, err := s.resolveSource(ctx, req.asRequest())
	if err != nil {
		return IngestOutcome{}, err
	}
	if outcome != outcomeContinue {
		return IngestOutcome{Outcome: outcome}, nil
	}
	// A digest holds no run folder to ingest into.
	if src.Kind != domain.SourceLogsRoot && src.Kind != domain.SourceSingleRun {
		return IngestOutcome{Outcome: OutcomeSourceUnusable}, nil
	}

	inv, _ := s.deps.Source.Enumerate(src)
	dir, found := runDirOf(inv, req.RunID)
	if !found {
		return IngestOutcome{Outcome: OutcomeReport}, nil
	}

	result, err := s.deps.Ingester.IngestRun(ctx, domain.UsageIngestRequest{RunDir: dir, ExportPath: req.ExportPath})
	return IngestOutcome{Outcome: OutcomeReport, RunFound: true, Result: result}, err
}

func (r IngestRequest) asRequest() Request {
	return Request{ExplicitPath: r.ExplicitPath}
}

// runDirOf returns the folder of the named run in the inventory.
func runDirOf(inv domain.Inventory, runID string) (string, bool) {
	for _, run := range inv.Runs {
		if run.Run.ID == runID {
			return run.Dir, true
		}
	}
	return "", false
}

// probeUsageIngest asks the probe about each named run's folder and returns
// the answers by run id. Without a probe it returns nil.
func (s *Service) probeUsageIngest(inv domain.Inventory) map[string]domain.UsageIngestState {
	if s.deps.IngestProbe == nil {
		return nil
	}
	states := make(map[string]domain.UsageIngestState, len(inv.Runs))
	for _, run := range inv.Runs {
		states[run.Run.ID] = s.deps.IngestProbe.PendingUsage(run.Dir)
	}
	return states
}

// applyUsageIngest records each run's ingest state on the matching run report.
func applyUsageIngest(report *domain.Report, states map[string]domain.UsageIngestState) {
	for i := range report.Runs {
		report.Runs[i].UsageIngest = states[report.Runs[i].Run.ID]
	}
}
