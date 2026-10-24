package runner

import (
	"context"
	"fmt"

	"mosaic-agent-test/internal/domain"
)

// queryObservedModels asks the observed-model port which model(s) the
// subject session of the run used. A missing port or a port error is never a
// model: both yield evidence of nothing, so an unknown model cannot read as a
// valid run.
func queryObservedModels(ctx context.Context, d Deps, snap Snapshot, runID string) domain.ObservedModelEvidence {
	if d.ObservedModels == nil {
		return domain.ObservedModelEvidence{
			NoEvidence: domain.NoEvidencePortUnavailable,
			Detail:     "no observed-model provider is configured",
		}
	}
	var form domain.ObservedModelForm
	if r, ok := d.Adapter.(domain.ObservedModelFormReporter); ok {
		form = r.ObservedModelForm()
	}
	evidence, err := d.ObservedModels.ObservedSubjectModels(ctx, domain.ObservedModelQuery{
		LogRoot:  snap.LogRoot,
		LogsRoot: snap.LogsRoot,
		RunID:    runID,
		Form:     form,
	})
	if err != nil {
		return domain.ObservedModelEvidence{
			NoEvidence: domain.NoEvidencePortError,
			Detail:     fmt.Sprintf("observed-model provider failed: %v", err),
		}
	}
	return evidence
}
