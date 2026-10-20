package app

import (
	"errors"
	"fmt"

	"mosaic-deploy/internal/plan"
)

// withRemovedLayerHint adds, to an unknown-workflow error naming a workflow that is deployed in
// the workspace, that the workflow may have come from a layer no longer in the stack. Any other
// error is returned unchanged.
func withRemovedLayerHint(err error, deployedWorkflowIDs []string) error {
	var unknown *plan.UnknownWorkflowError
	if !errors.As(err, &unknown) {
		return err
	}
	for _, id := range deployedWorkflowIDs {
		if id == unknown.WorkflowID {
			return fmt.Errorf("%w; the workflow is deployed in this workspace and may have come from a catalog layer that is no longer in the stack", err)
		}
	}
	return err
}
