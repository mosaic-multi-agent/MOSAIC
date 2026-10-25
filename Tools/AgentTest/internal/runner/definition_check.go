package runner

import "mosaic-agent-test/internal/domain"

// definitionTarget is one deployed definition and the model its tier selected.
type definitionTarget struct {
	agent    string
	path     string
	tier     string
	expected string
	files    []string // catalog file(s) the agent joined to; nil off the catalogue path
}

// failure builds the error for the target with the given problem and found value.
func (t definitionTarget) failure(problem DefinitionModelProblem, found string) *DefinitionModelError {
	return &DefinitionModelError{
		Agent: t.agent, Path: t.path, Tier: t.tier, Expected: t.expected,
		Found: found, CatalogFiles: t.files, Problem: problem,
	}
}

// checkDefinitionModels reads every target definition back through the
// adapter, independently of what the deployment tool reported, and requires
// its model field to equal the expected model. The first failing target in
// order is returned as a *DefinitionModelError. No targets means nothing was
// deployed and nothing is checked.
func checkDefinitionModels(adapter domain.HarnessAdapter, targets []definitionTarget) error {
	if len(targets) == 0 {
		return nil
	}
	reader, ok := adapter.(domain.DefinitionModelReader)
	if !ok {
		return targets[0].failure(DefinitionModelUnsupported, "")
	}
	for _, t := range targets {
		field, err := reader.DefinitionModel(t.path)
		if err != nil {
			return t.failure(DefinitionModelUnreadable, "")
		}
		switch {
		case !field.Present || field.Value == "":
			return t.failure(DefinitionModelMissing, "")
		case field.Value != t.expected:
			return t.failure(DefinitionModelMismatch, field.Value)
		}
	}
	return nil
}
