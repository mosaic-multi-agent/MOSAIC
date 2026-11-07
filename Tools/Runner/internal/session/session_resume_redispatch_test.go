package session_test

// Tests for the resume of a run whose last workflow Execution Log entry is an
// unaccepted attempt (its agent differs from current_state.last_agent), and for
// the resume continuations that must keep going through normal routing.
//
// Coverage:
//
//   - A route-back target that was recorded but not accepted is re-dispatched as
//     recorded (row, iteration, recorded Inputs) instead of routing past it.
//   - An attempt reached by a routing that skipped a row (consultant or manual
//     dispatch) is re-dispatched, not the successor of the accepted position.
//   - A HITL-rejected attempt is re-dispatched with the row's HITL and goes
//     through the HITL gate again; the outcome then routes as in a live run.
//   - A trailing BLOCKED/E501 row and a trailing accepted PARTIALLY_DONE row
//     continue through mechanical retry of the same row.
//   - A crash with no log row for the lost attempt resumes by normal routing.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// newHITLReviewSession builds a session on the hitl-review-orch.md fixture
// (agent-a -> agent-b[HITL, findings back to agent-a] -> agent-c) with the
// given approval reader.
func newHITLReviewSession(t *testing.T, approvals domain.ApprovalReader) (
	ses session.Session, f *harness.MockAdapter, store *memStore, orchPath string,
) {
	t.Helper()
	dir := t.TempDir()
	orchPath = copyOrchestratorFile(t, dir, "hitl-review-orch.md")
	for _, a := range []string{"agent-a", "agent-b", "agent-c"} {
		writeAgentFile(t, dir, a)
	}
	f = harness.NewMockAdapter()
	store = &memStore{}
	ses = session.New(session.Deps{
		Harness:   f,
		Store:     store,
		Approvals: approvals,
		Clock:     fixedClock{t: epoch},
		Interact:  &noopInteraction{},
	})
	return
}

// hitlReviewResume stores a run of the hitl-review workflow in auto-review
// mode whose accepted position is current and whose log is entries.
func hitlReviewResume(store *memStore, orchPath string, current domain.CurrentState, entries ...domain.ExecutionLogEntry) domain.RunConfig {
	store.state = domain.ArtifactState{
		Workflow:        "hitl-review",
		WorkflowVersion: "1.0",
		Task:            "test task",
		GlobalSequence:  entries[len(entries)-1].Seq,
		RunSettings:     domain.RunSettings{Mode: domain.ExecutionModeAutoReview},
		CurrentState:    current,
		ExecutionLog:    entries,
	}
	store.exists = true
	cfg := domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           "hitl-review",
		Task:                 "test task",
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAutoReview},
	}
	markResume(&cfg)
	return cfg
}

func logEntry(seq int, agent string, row int, status domain.StatusCode, inputs string) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{
		Seq: seq, Agent: agent, Phase: "PLANNING", WorkflowRow: domain.WorkflowRow(row),
		Status: status, Inputs: inputs,
	}
}

func queueSuccesses(f *harness.MockAdapter, agent string, n int) {
	queueRepeated(f, agent, n, statusEntry(domain.StatusSUCCESS, "", "done"))
}

// bareInputs returns the dispatched input paths without the run-scoped folder.
func bareInputs(paths []string) []string {
	prefix := domain.RunScopedFolder(testRunID) + "/"
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = strings.TrimPrefix(p, prefix)
	}
	return out
}

func requireInvocationOrder(t *testing.T, f *harness.MockAdapter, want ...string) []harness.Invocation {
	t.Helper()
	invs := f.Invocations()
	var got []string
	for _, inv := range invs {
		got = append(got, inv.Agent.Identifier)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dispatch order: want %v, got %v", want, got)
	}
	return invs
}

func TestSession_Resume_UnacceptedRouteBackTarget_IsRedispatchedWithRecordedInputs(t *testing.T) {
	ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	// agent-b returned findings (accepted); the engine routed back to agent-a,
	// whose attempt #3 was recorded but never accepted.
	cfg := hitlReviewResume(store, orchPath,
		domain.CurrentState{Phase: "PLANNING", LastStatus: domain.StatusCOMPLETED_NEEDS_ACTION, LastAgent: "agent-b#2"},
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "agent-b#2", 2, domain.StatusCOMPLETED_NEEDS_ACTION, "design.md"),
		logEntry(3, "agent-a#3", 1, domain.StatusSUCCESS, "design.md, review.md"),
	)
	queueSuccesses(f, "agent-a", 1)
	queueSuccesses(f, "agent-b", 1)
	queueSuccesses(f, "agent-c", 1)

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := requireInvocationOrder(t, f, "agent-a", "agent-b", "agent-c")
	if id := invs[0].Request.AgentInstanceID; id != "agent-a#4" {
		t.Errorf("want the re-dispatch to take the next sequence (agent-a#4), got %q", id)
	}
	in := bareInputs(invs[0].Request.InputArtifacts)
	if len(in) != 2 || in[0] != "design.md" || in[1] != "review.md" {
		t.Errorf("want the Inputs recorded on the attempt [design.md review.md], got %v", in)
	}
	if len(store.Applied) == 0 || store.Applied[0].WorkflowRow != domain.WorkflowRowFromIndex(0) {
		t.Errorf("want the re-dispatch recorded against the attempt's row 1, got %+v", store.Applied)
	}
}

func TestSession_Resume_AttemptReachedPastARow_IsRedispatchedNotTheAcceptedSuccessor(t *testing.T) {
	ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	// A consultant (or the user) dispatched agent-c directly after agent-a; its
	// attempt was recorded but not accepted. Routing from agent-a's accepted
	// position would dispatch agent-b instead.
	cfg := hitlReviewResume(store, orchPath,
		domain.CurrentState{Phase: "PLANNING", LastStatus: domain.StatusSUCCESS, LastAgent: "agent-a#1"},
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "agent-c#2", 3, domain.StatusSUCCESS, "review.md"),
	)
	queueSuccesses(f, "agent-c", 1)

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := requireInvocationOrder(t, f, "agent-c")
	if in := bareInputs(invs[0].Request.InputArtifacts); len(in) != 1 || in[0] != "review.md" {
		t.Errorf("want the recorded Inputs [review.md], got %v", in)
	}
}

func TestSession_Resume_HITLRejectedAttempt_GoesThroughTheHITLGateAgain(t *testing.T) {
	approvals := &switchingApprovalReader{firstApproval: domain.ApprovalFalse, restApproval: domain.ApprovalTrue}
	ses, f, store, orchPath := newHITLReviewSession(t, approvals)
	// agent-b (HITL row) came back without approval: recorded, not accepted.
	cfg := hitlReviewResume(store, orchPath,
		domain.CurrentState{Phase: "PLANNING", LastStatus: domain.StatusSUCCESS, LastAgent: "agent-a#1"},
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "agent-b#2", 2, domain.StatusPARTIALLY_DONE, "design.md, notes.md"),
	)
	queueSuccesses(f, "agent-b", 2)
	queueSuccesses(f, "agent-c", 1)

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := requireInvocationOrder(t, f, "agent-b", "agent-b", "agent-c")
	if !invs[0].Request.HumanInTheLoop {
		t.Error("want the re-dispatch to carry the row's HITL (true)")
	}
	in := bareInputs(invs[0].Request.InputArtifacts)
	if len(in) != 2 || in[0] != "design.md" || in[1] != "notes.md" {
		t.Errorf("want the recorded Inputs [design.md notes.md] on the re-dispatch, got %v", in)
	}
}

func TestSession_Resume_RedispatchOutcome_RoutesByTheRowLikeALiveRun(t *testing.T) {
	ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	cfg := hitlReviewResume(store, orchPath,
		domain.CurrentState{Phase: "PLANNING", LastStatus: domain.StatusSUCCESS, LastAgent: "agent-a#1"},
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "agent-b#2", 2, domain.StatusPARTIALLY_DONE, "design.md, extra.md"),
	)
	// The re-dispatched reviewer reports findings: auto-review routes back to
	// agent-a, which then flows on to agent-b and agent-c as in a live run.
	f.Queue("agent-b",
		statusEntry(domain.StatusCOMPLETED_NEEDS_ACTION, "", "findings"),
		statusEntry(domain.StatusSUCCESS, "", "approved"))
	queueSuccesses(f, "agent-a", 1)
	queueSuccesses(f, "agent-c", 1)

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := requireInvocationOrder(t, f, "agent-b", "agent-a", "agent-b", "agent-c")
	if in := bareInputs(invs[0].Request.InputArtifacts); len(in) != 2 || in[1] != "extra.md" {
		t.Errorf("want the first dispatch to carry the recorded Inputs [design.md extra.md], got %v", in)
	}
}

// ---- continuations through normal routing ----

func TestSession_Resume_TrailingAcceptedRetryableRow_RetriesTheSameRow(t *testing.T) {
	cases := map[string]seedRow{
		"E501":           {domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE},
		"PARTIALLY_DONE": {domain.StatusPARTIALLY_DONE, ""},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			run := seedFileStoreRun(t, []seedRow{row})
			f := harness.NewMockAdapter()
			queueSuccesses(f, "agent-a", 1)
			queueSuccesses(f, "agent-b", 1)

			got, err := run.newSession(f, nil).Start(context.Background(), run.resumeConfig())

			requireRunStatus(t, got, err, domain.RunCompleted)
			invs := requireInvocationOrder(t, f, "agent-a", "agent-b")
			if id := invs[0].Request.AgentInstanceID; id != "agent-a#2" {
				t.Errorf("want the retry of the same row as agent-a#2, got %q", id)
			}
		})
	}
}

func TestSession_Resume_CrashWithoutLogRow_RoutesNormallyFromLastAcceptedState(t *testing.T) {
	ses, f, store, orchPath := newLinearSession(t)
	// agent-b was dispatched (sequence 2 allocated) but the process died before
	// any row was recorded for it.
	store.state = domain.ArtifactState{
		Workflow:        "linear",
		WorkflowVersion: "1.0",
		Task:            "test task",
		GlobalSequence:  2,
		RunSettings:     domain.RunSettings{Mode: domain.ExecutionModeAuto},
		CurrentState:    domain.CurrentState{Phase: "PLANNING", LastStatus: domain.StatusSUCCESS, LastAgent: "agent-a#1"},
		ExecutionLog:    []domain.ExecutionLogEntry{logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, "")},
	}
	store.exists = true
	queueSuccesses(f, "agent-b", 1)
	cfg := baseLinearConfig(orchPath)
	markResume(&cfg)

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := requireInvocationOrder(t, f, "agent-b")
	if id := invs[0].Request.AgentInstanceID; id != "agent-b#3" {
		t.Errorf("want agent-b#3 (sequence 2 was consumed by the lost attempt), got %q", id)
	}
}
