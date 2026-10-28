package cli

// Stable exit codes for mosaic-log-analyzer.
// Values 0, 1, 3 match mosaic-deploy and mosaic-run for consistency.
const (
	// ExitSuccess indicates a report or total was produced successfully.
	ExitSuccess = 0

	// ExitFailure indicates an unexpected infrastructure error (e.g. malformed
	// pricing document, context cancellation).
	ExitFailure = 1

	// ExitNoData indicates the source resolved but contained no runs.
	// Output is still valid JSON; the field set is empty but well-formed.
	ExitNoData = 2

	// ExitUsage indicates an argument or flag error, or a required source could
	// not be resolved (no --path flag and no default location found).
	ExitUsage = 3

	// ExitUnusable indicates the supplied --path exists but is not a usable
	// logs tree (SourceUnusable).
	ExitUnusable = 4
)

// ExitIngestSkipped indicates ingest-otel did nothing because the run's ingest
// is locked by another process or its state is of an unknown version.
const ExitIngestSkipped = 5
