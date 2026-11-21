package app

// promote_batch_projectregions.go asks, once per batch, whether the content of project regions
// is kept or emptied in the promoted generics. The answer applies to every agent of the batch.

import (
	"bytes"

	"mosaic-deploy/internal/domain"
)

// askProjectRegions asks whether project region content is kept or emptied, when at least one
// loaded agent has a project region with content at any depth. It records the answer on the
// run and the result; skipping, or any answer but empty, keeps the content. It reports false
// when the batch is aborted.
func (r *promoteBatchRun) askProjectRegions() bool {
	if !r.anyProjectContent() {
		return true
	}
	ans, ok := r.selectOne(domain.ChoiceQuestion{
		Question: domain.Question{
			ID:              domain.QPromoteProjectRegions,
			Subject:         r.result.AgentsDir,
			Title:           "Keep or empty the content of the project regions in the promoted agents",
			DefaultOptionID: string(ProjectRegionsKeep),
		},
		Options: []domain.Option{
			{ID: string(ProjectRegionsKeep), Label: "Keep the project region content"},
			{ID: string(ProjectRegionsEmpty), Label: "Empty the project regions"},
		},
	})
	if !ok {
		return false
	}
	r.result.ProjectRegions = ProjectRegionsKeep
	if ans.Status == domain.Answered && ans.OptionID == string(ProjectRegionsEmpty) {
		r.result.ProjectRegions = ProjectRegionsEmpty
	}
	return true
}

// anyProjectContent reports whether a loaded, unsettled agent has a project region with
// non-blank content at any nesting depth.
func (r *promoteBatchRun) anyProjectContent() bool {
	for _, e := range r.entries {
		if e.src == nil || e.settled {
			continue
		}
		doc, err := ParseGenericSource(e.src.canonical)
		if err != nil {
			continue
		}
		for _, inj := range doc.Body().Injections() {
			if len(bytes.TrimSpace(inj.Content())) > 0 {
				return true
			}
		}
	}
	return false
}
