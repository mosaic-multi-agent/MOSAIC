package claudecode

import (
	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/defmodel"
)

var _ domain.DefinitionModelReader = (*Adapter)(nil)

// DefinitionModel implements domain.DefinitionModelReader: the deployment tool
// writes this harness's model under the frontmatter key "model".
func (a *Adapter) DefinitionModel(path string) (domain.DefinitionModelField, error) {
	return defmodel.Read(path)
}

// SubjectAgentsDir implements domain.SubjectPlacementReader: the subject is
// launched with `--agent <identity>`, which this harness resolves by `name:`
// among the project-level agents.
func (a *Adapter) SubjectAgentsDir() string { return AgentsRelDir }

var _ domain.SubjectPlacementReader = (*Adapter)(nil)
