// Package resumecheck holds the early resume refusal checks (two tiers) and
// the shared settings summary used by both frontends. The refusal rules and
// texts here are the ones the session applies at run start, so a frontend that
// checks early reports exactly what the session would refuse with.
package resumecheck

import (
	"errors"
	"fmt"
	"os"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// Refusal is an early resume refusal. Error() returns Message unchanged.
type Refusal struct {
	Message string // the refusal text
	Cause   error  // the underlying error, or nil
}

// Error returns the refusal text.
func (r *Refusal) Error() string { return r.Message }

// Unwrap returns Cause.
func (r *Refusal) Unwrap() error { return r.Cause }

// Deployment is what the deployed orchestrator file declares.
type Deployment struct {
	Workflows   []domain.WorkflowRegion
	InfraAgents []domain.DeclaredInfraAgent
}

// NoWorkflowReason is the refusal reason for a resumed run that records no
// workflow. The causes are listed as possibilities because the artifact may be
// missing, unreadable, or present and well-formed while recording no workflow.
func NoWorkflowReason(runID string) string {
	return fmt.Sprintf("run %s records no workflow, so there is nothing to resume it as; "+
		"its artifact may be missing, unreadable, or may record no workflow", runID)
}

// WorkflowGoneReason is the refusal reason for a resumed run whose workflow the
// orchestrator file no longer declares. It names both the missing workflow and
// the run stranded by its absence.
func WorkflowGoneReason(runID, workflowID string) string {
	return fmt.Sprintf("workflow %q for run %s is not declared in the current "+
		"orchestrator file; it may have been renamed or removed", workflowID, runID)
}

// Pre30LayoutReason is the refusal reason for a run artifact written in the
// pre-3.0 layout.
func Pre30LayoutReason(runID string) string {
	return fmt.Sprintf("run %s was written in the pre-3.0 run artifact layout (Stage column or "+
		"current_state.stage) and cannot be resumed", runID)
}

// CheckArtifact is the artifact-only tier: it needs neither a harness choice
// nor the orchestrator file. state and readErr are the result of reading the
// run artifact. A read failure that is not a refusal (for example a permission
// error) is returned unchanged. A workflow version mismatch is not checked.
func CheckArtifact(runID string, state domain.ArtifactState, readErr error) error {
	if readErr != nil {
		return artifactReadRefusal(runID, readErr)
	}
	if state.Workflow == "" {
		return noWorkflowRefusal(runID)
	}
	return nil
}

func artifactReadRefusal(runID string, readErr error) error {
	if errors.Is(readErr, artifact.ErrPre30Layout) {
		refusal := &domain.RefusalError{
			Component: "session",
			Resource:  runID,
			Reason:    Pre30LayoutReason(runID),
			Cause:     readErr,
		}
		return &Refusal{Message: refusal.Error(), Cause: refusal}
	}
	var refusal *domain.RefusalError
	if errors.As(readErr, &refusal) {
		return &Refusal{Message: refusal.Error(), Cause: refusal}
	}
	if errors.Is(readErr, os.ErrNotExist) {
		return noWorkflowRefusal(runID)
	}
	return readErr
}

func noWorkflowRefusal(runID string) error {
	reason := NoWorkflowReason(runID)
	return &Refusal{Message: reason, Cause: &domain.RefusalError{
		Component: "workflow",
		Resource:  runID,
		Reason:    reason,
	}}
}

// CheckDeployment is the deployed-files tier: the recorded settings checked
// against what the deployed orchestrator file declares. It reports the first
// refusal in the order the session applies them. The workflow version is not
// compared and an unrecorded mode is not a refusal here.
func CheckDeployment(runID string, state domain.ArtifactState, d Deployment) error {
	if !declaresWorkflow(d.Workflows, state.Workflow) {
		reason := WorkflowGoneReason(runID, string(state.Workflow))
		return &Refusal{Message: reason, Cause: &domain.RefusalError{
			Component: "workflow",
			Resource:  runID,
			Reason:    reason,
		}}
	}
	if err := domain.ValidateInfraSelections(d.InfraAgents, state.InfraClassSelections); err != nil {
		return &Refusal{Message: err.Error(), Cause: err}
	}
	if err := ValidateOverrides(state.InfrastructureOverrides, d.InfraAgents); err != nil {
		return &Refusal{Message: err.Error(), Cause: err}
	}
	if state.Checkpoints && !HasAgentOfClass(d.InfraAgents, "checkpoint") {
		return &Refusal{Message: NoCheckpointProviderText}
	}
	if state.Commits && !HasAgentOfClass(d.InfraAgents, "commit") {
		return &Refusal{Message: NoCommitAgentText}
	}
	return nil
}

func declaresWorkflow(regions []domain.WorkflowRegion, id domain.WorkflowID) bool {
	for _, r := range regions {
		if r.Info.ID == id {
			return true
		}
	}
	return false
}
