package plan

import "mosaic-deploy/internal/domain"

// keptSet returns the kept workflow ids as a membership set.
func keptSet(kept []string) map[string]bool {
	set := make(map[string]bool, len(kept))
	for _, id := range kept {
		set[id] = true
	}
	return set
}

// catalogWorkflowsFor returns the catalog workflows for ids in selection order, skipping
// kept ids (which the catalog does not supply) and ids the catalog does not know.
func catalogWorkflowsFor(lookup func(string) (domain.Workflow, bool), ids, kept []string) []domain.Workflow {
	keptIDs := keptSet(kept)
	workflows := make([]domain.Workflow, 0, len(ids))
	for _, id := range ids {
		if keptIDs[id] {
			continue
		}
		if wf, ok := lookup(id); ok {
			workflows = append(workflows, wf)
		}
	}
	return workflows
}

// deployedWithoutKept returns deployed minus the kept ids, so a kept workflow found in the
// deployed orchestrator is neither drift nor a removal.
func deployedWithoutKept(deployed domain.DeployedWorkflows, kept []string) domain.DeployedWorkflows {
	if len(kept) == 0 || len(deployed) == 0 {
		return deployed
	}
	keptIDs := keptSet(kept)
	out := make(domain.DeployedWorkflows, 0, len(deployed))
	for _, d := range deployed {
		if !keptIDs[d.ID] {
			out = append(out, d)
		}
	}
	return out
}
