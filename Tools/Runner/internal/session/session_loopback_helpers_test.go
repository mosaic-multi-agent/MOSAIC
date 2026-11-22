package session_test

// Shared rig for the loop-back injection tests: runs over the run-scope
// reviewer pair of hitl-review-orch.md (agent-a -> agent-b, findings back to
// agent-a) and over the staged reviewer pair of consult-defaults-orch.md
// (implementer -> build-review, findings back to implementer), starting from a
// recorded position, and compares the inputs the dispatched requests list.

import (
	"context"
	"sort"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// hitlPosition is a recorded position of a hitl-review run.
type hitlPosition struct {
	current domain.CurrentState
	entries []domain.ExecutionLogEntry
}

// hitlAfterCreator is the position right after agent-a was accepted.
func hitlAfterCreator() hitlPosition {
	return hitlPosition{
		current: domain.CurrentState{Phase: "PLANNING", LastStatus: domain.StatusSUCCESS, LastAgent: "agent-a#1"},
		entries: []domain.ExecutionLogEntry{logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, "")},
	}
}

// hitlAfterReviewer is the position right after the reviewer agent-b was
// accepted with status (a graceful stop armed after it leaves exactly this).
func hitlAfterReviewer(status domain.StatusCode) hitlPosition {
	return hitlPosition{
		current: domain.CurrentState{Phase: "PLANNING", LastStatus: status, LastAgent: "agent-b#2"},
		entries: []domain.ExecutionLogEntry{
			logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
			logEntry(2, "agent-b#2", 2, status, "design.md"),
		},
	}
}

// hitlCreatorRegistry is the Artifacts registry after agent-a produced design.md,
// with the run-folder prefix as recorded in the artifact.
func hitlCreatorRegistry() []domain.ArtifactRegistryEntry {
	return []domain.ArtifactRegistryEntry{{
		Artifact:  domain.RunScopedFolder(testRunID) + "/design.md",
		CreatedIn: "PLANNING",
		CreatedBy: "agent-a#1",
	}}
}

// resumeHITLReview resumes a hitl-review run at pos with the given registry
// recorded, scripts the harness, runs it and returns the harness for request
// inspection.
func resumeHITLReview(t *testing.T, pos hitlPosition, registry []domain.ArtifactRegistryEntry,
	script func(f *harness.MockAdapter)) *harness.MockAdapter {
	t.Helper()
	ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	cfg := hitlReviewResume(store, orchPath, pos.current, pos.entries...)
	store.state.ArtifactRegistry = registry
	script(f)
	if _, err := ses.Start(context.Background(), cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return f
}

// scriptReviewerFindingsThenApproval scripts the creator and the reviewer to
// succeed, except that the reviewer reports findings on its first dispatch.
func scriptReviewerFindingsThenApproval(f *harness.MockAdapter) {
	f.Queue("agent-b",
		statusEntry(domain.StatusCOMPLETED_NEEDS_ACTION, "", "findings"),
		statusEntry(domain.StatusSUCCESS, "", "approved"))
	queueSuccesses(f, "agent-a", 2)
	queueSuccesses(f, "agent-c", 1)
}

// scriptAllSucceed scripts every agent of hitl-review to succeed generously.
func scriptAllSucceed(f *harness.MockAdapter) {
	queueSuccesses(f, "agent-a", 2)
	queueSuccesses(f, "agent-b", 2)
	queueSuccesses(f, "agent-c", 1)
}

// scriptReviewerPartialThenSuccess scripts the reviewer to report
// PARTIALLY_DONE on its first dispatch and succeed afterwards.
func scriptReviewerPartialThenSuccess(f *harness.MockAdapter) {
	f.Queue("agent-b",
		statusEntry(domain.StatusPARTIALLY_DONE, "", "half"),
		statusEntry(domain.StatusSUCCESS, "", "done"))
	queueSuccesses(f, "agent-a", 1)
	queueSuccesses(f, "agent-c", 1)
}

// blockSeed is the artifact state of a consult-defaults run (staged Stage block)
// whose last accepted step is agent at the 1-based row, in the Implementation
// group of the given plan iteration, with that step's status.
func blockSeed(agent string, row domain.WorkflowRow, iteration string, status domain.StatusCode,
	registry []domain.ArtifactRegistryEntry) domain.ArtifactState {
	seed := rowStageSeed(agent, row, "EXECUTION", iteration+".Implementation")
	seed.CurrentState.LastStatus = status
	seed.ExecutionLog[0].Status = status
	seed.ArtifactRegistry = registry
	return seed
}

// blockCreatorRegistry is the registry after the implementer produced
// Stage-<iteration>/impl.md, with the run-folder prefix as recorded.
func blockCreatorRegistry(iteration string) []domain.ArtifactRegistryEntry {
	return []domain.ArtifactRegistryEntry{{
		Artifact:  domain.RunScopedFolder(testRunID) + "/Stage-" + iteration + "/impl.md",
		CreatedIn: "EXECUTION." + iteration + ".Implementation",
		CreatedBy: "implementer#2",
	}}
}

// resumeBlockRun resumes a consult-defaults run in auto-review mode from seed,
// scripts the harness, runs it and returns the harness.
func resumeBlockRun(t *testing.T, seed domain.ArtifactState, script func(f *harness.MockAdapter)) *harness.MockAdapter {
	t.Helper()
	rig := newDefaultsRig(t, &scriptedRoutingConsultant{}, domain.ExecutionModeAutoReview, seed)
	script(rig.f)
	if _, err := rig.ses.Start(context.Background(), rig.cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return rig.f
}

// scriptBuildReviewFindings scripts the build-review to report findings on its
// first dispatch, and the implementer to succeed.
func scriptBuildReviewFindings(f *harness.MockAdapter) {
	f.Queue("build-review",
		statusEntry(domain.StatusCOMPLETED_NEEDS_ACTION, "", "findings"),
		statusEntry(domain.StatusSUCCESS, "", "approved"))
	queueSuccesses(f, "implementer", 2)
}

// scriptBlockSucceed scripts the implementer and the build-review to succeed.
func scriptBlockSucceed(f *harness.MockAdapter) {
	queueSuccesses(f, "implementer", 2)
	queueSuccesses(f, "build-review", 2)
}

// scriptBuildReviewPartial scripts the build-review to report PARTIALLY_DONE on
// its first dispatch and succeed afterwards.
func scriptBuildReviewPartial(f *harness.MockAdapter) {
	f.Queue("build-review",
		statusEntry(domain.StatusPARTIALLY_DONE, "", "half"),
		statusEntry(domain.StatusSUCCESS, "", "done"))
	queueSuccesses(f, "implementer", 1)
}

// sortedInputs returns the request's input paths without the run folder
// prefix, sorted.
func sortedInputs(req domain.ProtocolRequest) []string {
	out := bare(req, req.InputArtifacts)
	sort.Strings(out)
	return out
}

// requireInputs fails unless the request lists exactly the wanted bare paths
// (order ignored, each once).
func requireInputs(t *testing.T, req domain.ProtocolRequest, want ...string) {
	t.Helper()
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	got := sortedInputs(req)
	if strings.Join(got, ",") != strings.Join(wantSorted, ",") {
		t.Errorf("%s: want input artifacts %v, got %v", req.AgentInstanceID, wantSorted, got)
	}
}

// requireNthRequest returns the n-th (zero-based) request to agent, failing the
// test when the agent was dispatched fewer times.
func requireNthRequest(t *testing.T, f *harness.MockAdapter, agent string, n int) domain.ProtocolRequest {
	t.Helper()
	reqs := requestsTo(f, agent)
	if len(reqs) <= n {
		t.Fatalf("want dispatch #%d of %s, got only %d dispatch(es)", n+1, agent, len(reqs))
	}
	return reqs[n]
}
