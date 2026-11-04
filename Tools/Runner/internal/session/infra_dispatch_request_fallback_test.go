package session

// The task description of a commit-class ITERATION_END dispatch names the
// recorded commit branch also when the ended iteration or its artifacts cannot
// be resolved and the description falls back to the default "triggered by" form.

import (
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
)

// fallbackBranches are a MOSAIC-owned and a user-own branch; the second one
// contains "/" and "-" so a hard-coded branch form cannot satisfy the test.
var fallbackBranches = []string{
	"mosaic/run/20261022T170341Z-d869",
	"PBI/Regulated-Current/TriggerPath",
}

// fallbackStep is the completed review row of the fixture's grouped block.
func fallbackStep(recorded string) domain.CompletedStep {
	return domain.CompletedStep{
		Seq: 2, AgentInstance: "tests-review#2", Phase: "EXECUTION", Stage: recorded,
		WorkflowRow: domain.WorkflowRowFromIndex(ivRowReview), Status: domain.StatusSUCCESS,
	}
}

// fallbackRequest builds the ITERATION_END request of a commit-class agent.
func fallbackRequest(t *testing.T, branch, recorded string, aw domain.AdmittedWorkflow) domain.ProtocolRequest {
	t.Helper()
	b := triggerBoundaries{
		Workflow: aw, Stages: ivSchedules(), RowIndex: ivRowReview, Mode: domain.ExecutionModeAutoReview,
	}
	agent := domain.DeclaredInfraAgent{Name: "commit-manager", Class: "commit"}
	runFolder := filepath.Join(t.TempDir(), "run")

	return buildInfraRequest(agent, []string{domain.InfraTriggerIterationEnd}, "commit-manager#9",
		"run-1", runFolder, branch, fallbackStep(recorded), b)
}

func assertFallbackDescription(t *testing.T, req domain.ProtocolRequest, branch string) {
	t.Helper()
	want := "infrastructure agent dispatch: commit-manager, triggered by ITERATION_END; commit branch: " + branch
	if req.TaskDescription != want {
		t.Errorf("TaskDescription = %q, want %q", req.TaskDescription, want)
	}
	if len(req.InputArtifacts) != 0 {
		t.Errorf("InputArtifacts = %v, want none in the fallback form", req.InputArtifacts)
	}
}

func TestBuildInfraRequest_CommitIterationEndIterationUnresolvable_FallbackNamesBranch(t *testing.T) {
	for _, branch := range fallbackBranches {
		t.Run(branch, func(t *testing.T) {
			// The pre-3.0 spelling "{Group}.{ID}" resolves no iteration.
			req := fallbackRequest(t, branch, "Test.A", ivAdmitted(t))

			assertFallbackDescription(t, req, branch)
		})
	}
}

func TestBuildInfraRequest_CommitIterationEndArtifactCandidatesFail_FallbackNamesBranch(t *testing.T) {
	for _, branch := range fallbackBranches {
		t.Run(branch, func(t *testing.T) {
			aw := ivAdmitted(t)
			// A block whose rows lie outside the table: the iteration resolves,
			// its artifact candidates do not.
			blocks := append([]domain.AdmittedBlock(nil), aw.Blocks...)
			blocks[0].EndRow = len(aw.Table.Rows) + 10
			aw.Blocks = blocks

			req := fallbackRequest(t, branch, "A.Test", aw)

			assertFallbackDescription(t, req, branch)
		})
	}
}
