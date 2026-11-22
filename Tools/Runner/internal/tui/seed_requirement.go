package tui

import (
	"mosaic-run/internal/workflow"
)

// selectedWorkflowSeedInputs returns the inputs the first agent of the selected
// workflow declares, which decide whether the seed screen asks for a required
// seed. A workflow that is not found or cannot be parsed yields nil (optional
// wording); the session still refuses a bad workflow at run start.
func (m *rootModel) selectedWorkflowSeedInputs() []string {
	for _, wf := range m.workflows {
		if wf.Info.ID != m.selections.workflowID {
			continue
		}
		table, err := workflow.Parse(wf.Content, wf.Info)
		if err != nil {
			return nil
		}
		return table.FirstRowInputs()
	}
	return nil
}
