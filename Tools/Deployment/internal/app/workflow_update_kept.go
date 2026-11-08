package app

// workflow_update_kept.go holds the custom-workflow handling of the Update workflows flow: the
// checklist options for deployed workflows the catalog does not supply, the split of the
// selection into catalog-supplied and kept ids, and the removal notices.

import (
	"mosaic-deploy/internal/domain"
)

const (
	customWorkflowHint  = "not in catalog; deployed only"
	customWorkflowGroup = "Not in catalog"
)

// customChecklistOptions returns one checklist option for each deployed custom workflow id,
// in first-seen order. None is selected by default.
func customChecklistOptions(kept keptWorkflows) []domain.Option {
	var opts []domain.Option
	for _, id := range kept.ids {
		opts = append(opts, domain.Option{ID: id, Label: id, Hint: customWorkflowHint, Group: customWorkflowGroup})
	}
	return opts
}

// selectedKeptWorkflows narrows kept to the custom ids in keptIDs: only those blocks are kept
// in each file. Kept blocks are placed by the order of the selection, which sets the workflow order of the run.
func selectedKeptWorkflows(kept keptWorkflows, selection, keptIDs []string) keptWorkflows {
	chosen := make(map[string]bool, len(keptIDs))
	for _, id := range keptIDs {
		chosen[id] = true
	}
	out := keptWorkflows{byPath: map[string]DeployedPathWorkflows{}}
	for _, id := range kept.ids {
		if chosen[id] {
			out.ids = append(out.ids, id)
		}
	}
	for path, layout := range kept.byPath {
		narrowed := DeployedPathWorkflows{Order: selection}
		for _, b := range layout.Kept {
			if chosen[b.ID] {
				narrowed.Kept = append(narrowed.Kept, b)
			}
		}
		if len(narrowed.Kept) > 0 {
			out.byPath[path] = narrowed
		}
	}
	return out
}

// removedCustomWorkflowIDs returns the deployed custom ids that the selection leaves out.
func removedCustomWorkflowIDs(kept keptWorkflows, keptIDs []string) []string {
	chosen := make(map[string]bool, len(keptIDs))
	for _, id := range keptIDs {
		chosen[id] = true
	}
	var removed []string
	for _, id := range kept.ids {
		if !chosen[id] {
			removed = append(removed, id)
		}
	}
	return removed
}

// recordRemovedWorkflows adds one gap per removed custom workflow id.
func (s *service) recordRemovedWorkflows(ids []string) {
	for _, id := range ids {
		s.deps.Todo.AddGap(domain.Gap{
			Kind:    domain.GapWorkflowRemoved,
			Subject: id,
			Detail:  id + " is not in the catalog stack and was not selected; removed from the orchestrator files that held it.",
		})
	}
}
