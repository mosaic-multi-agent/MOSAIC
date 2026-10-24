package app

import (
	"context"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// loadedSource is a resolved source read and aggregated, before pricing.
type loadedSource struct {
	noData    bool             // usable source with nothing in it
	aggregate domain.Aggregate // logs merged with digest runs
	// findings holds every finding from enumerating, reading, aggregating and
	// loading digests, for the log report's quality summary.
	findings []domain.Finding
	// exportFindings holds the findings a digest export carries.
	exportFindings []domain.Finding
}

// readSource enumerates, reads and aggregates a resolved source and merges the
// digest files found with it. Pricing is not involved.
func (s *Service) readSource(ctx context.Context, src domain.Source, digestFiles []string) (loadedSource, error) {
	inv, enumFindings := s.deps.Source.Enumerate(src)
	if inv.IsEmpty() && len(digestFiles) == 0 {
		return loadedSource{noData: true}, nil
	}

	// Read every discovered event file exactly once.
	streams, readFindings, err := s.readAllStreams(ctx, inv)
	if err != nil {
		return loadedSource{}, err
	}

	// Aggregate (pure attribution).
	agg, aggFindings := analysis.Aggregate(analysis.Input{Streams: streams})

	logFindings := make([]domain.Finding, 0, len(enumFindings)+len(readFindings)+len(aggFindings))
	logFindings = append(logFindings, enumFindings...)
	logFindings = append(logFindings, readFindings...)
	logFindings = append(logFindings, aggFindings...)

	// Merge digest runs into the aggregate. Load-time findings (skipped or
	// duplicate digests) join the report's findings but are never exported.
	agg, digestFindings, digestCarried, err := s.mergeDigestSources(ctx, agg, digestFiles)
	if err != nil {
		return loadedSource{}, err
	}

	return loadedSource{
		aggregate:      agg,
		findings:       append(logFindings[:len(logFindings):len(logFindings)], digestFindings...),
		exportFindings: exportableFindings(inv, logFindings, digestCarried),
	}, nil
}
