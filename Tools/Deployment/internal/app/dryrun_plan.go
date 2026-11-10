package app

import "mosaic-deploy/internal/domain"

// withDryRunPlan attaches the reviewed plan to the summary of a dry run so the frontends can
// print it; a real run's summary is returned unchanged.
func withDryRunPlan(summary domain.RunSummary, dryRun bool, p domain.Plan) domain.RunSummary {
	if dryRun {
		reviewed := p
		summary.DryRunPlan = &reviewed
	}
	return summary
}
