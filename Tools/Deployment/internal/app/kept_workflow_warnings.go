package app

// kept_workflow_warnings.go records the warning for workflows that the update flow keeps as
// deployed because the catalog stack does not supply them.

import "mosaic-deploy/internal/domain"

// keptWorkflowWarningText is the warning for one kept workflow id.
func keptWorkflowWarningText(id string) string {
	return id + " is not in the catalog stack; kept as deployed, not updated or validated."
}

// recordKeptWorkflowWarnings adds one gap per kept workflow id. The ids are already
// deduplicated across files, so a workflow held by both orchestrator-role files warns once.
func (s *service) recordKeptWorkflowWarnings(ids []string) {
	for _, id := range ids {
		s.deps.Todo.AddGap(domain.Gap{
			Kind:    domain.GapWorkflowKept,
			Subject: id,
			Detail:  keptWorkflowWarningText(id),
		})
	}
}
