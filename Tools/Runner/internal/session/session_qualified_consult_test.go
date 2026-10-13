package session_test

// Consulted dispatches of mode-qualified rows through a session. One agent
// (build-review) fills two rows told apart by qualifiers; the consultant names
// the bare agent id and the row. Covered: Mode 1 dispatches of every row,
// Mode 2 and 3 consultations after a harness failure, and a Mode 1 resume of a
// run interrupted after the consulted dispatch of the second qualified row.

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// Row indices (zero-based) of qualified-rows-orch.md.
const (
	qrPlanner     = 0
	qrTestsReview = 1 // build-review(tests)
	qrImplementer = 2
	qrImplReview  = 3 // build-review(impl)
)

// ===== Mode 1 =====

func TestSession_QualifiedRows_Orchestrated_ConsultedDispatchUsesBareAgentOnTheNamedRow(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{})
	r.consultant.queueDispatch("planner", "plan it", qrPlanner)
	r.consultant.queueDispatch("build-review", "review the tests", qrTestsReview)
	r.consultant.queueDispatch("implementer", "implement it", qrImplementer)
	r.consultant.queueDispatch("build-review", "review the implementation", qrImplReview)
	r.consultant.queueStop("done")
	r.queue("planner", "planner#1", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
	r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#4", domain.StatusSUCCESS)

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	invs := r.f.Invocations()
	if len(invs) != 4 {
		t.Fatalf("want 4 consulted dispatches, got %d", len(invs))
	}
	for i, want := range []struct{ agent, instance, task string }{
		{"planner", "planner#1", "plan it"},
		{"build-review", "build-review#2", "review the tests"},
		{"implementer", "implementer#3", "implement it"},
		{"build-review", "build-review#4", "review the implementation"},
	} {
		if invs[i].Agent.Identifier != want.agent || invs[i].Request.AgentInstanceID != want.instance {
			t.Errorf("invocation %d: want agent %q instance %q, got agent %q instance %q",
				i, want.agent, want.instance, invs[i].Agent.Identifier, invs[i].Request.AgentInstanceID)
		}
		if invs[i].Request.TaskDescription != want.task {
			t.Errorf("invocation %d: want the consultant's task description %q, got %q",
				i, want.task, invs[i].Request.TaskDescription)
		}
	}
	requireRecordedRows(t, r.store, 1, 2, 3, 4)
}

// The consultant may jump back to the first qualified row after the second:
// the row it names is the row that runs, whichever row holds the bare id first.
func TestSession_QualifiedRows_Orchestrated_BackwardJumpToFirstQualifiedRowRunsThatRow(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{})
	r.consultant.queueDispatch("build-review", "review the implementation", qrImplReview)
	r.consultant.queueDispatch("build-review", "review the tests again", qrTestsReview)
	r.consultant.queueStop("done")
	r.queue("build-review", "build-review#1", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#2", domain.StatusSUCCESS)

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireRecordedRows(t, r.store, 4, 2)
}

// ===== Modes 2 and 3 =====

// After the harness fails the second qualified row for the whole attempt
// budget, the engine consults; the consultant's dispatch names the bare agent
// id and that row, and the row runs as a consulted dispatch.
func TestSession_QualifiedRows_AutoModes_ConsultedDispatchOfSecondQualifiedRow(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			r := newQualifiedRun(t, qualifiedOptions{})
			r.queue("planner", "planner#1", domain.StatusSUCCESS)
			r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
			r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
			for i := 0; i < engine.E501AttemptLimit; i++ {
				r.f.Queue("build-review", harness.ScriptedEntry{Err: errors.New("simulated harness failure")})
			}
			r.consultant.queueDispatch("build-review", "retry the implementation review", qrImplReview)
			r.consultant.queueStop("done")
			r.queue("build-review", "build-review#8", domain.StatusSUCCESS)

			_, err := r.ses.Start(context.Background(), r.config(mode))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if r.consultant.CallCount == 0 {
				t.Fatal("want a consultation after the failed attempts, got none")
			}
			invs := r.f.Invocations()
			last := invs[len(invs)-1]
			if last.Agent.Identifier != "build-review" || last.Request.TaskDescription != "retry the implementation review" {
				t.Errorf("want the consulted build-review dispatch last, got agent %q task %q",
					last.Agent.Identifier, last.Request.TaskDescription)
			}
			var succeededRows []domain.WorkflowRow
			for _, step := range workflowSteps(r.store) {
				if step.Status == domain.StatusSUCCESS {
					succeededRows = append(succeededRows, step.WorkflowRow)
				}
			}
			if !slices.Equal(succeededRows, []domain.WorkflowRow{1, 2, 3, 4}) {
				t.Errorf("want the consulted dispatch recorded SUCCESS on row 4 after rows 1-3, got rows %v", succeededRows)
			}
		})
	}
}

// ===== Resume in Mode 1 =====

// stateSnapshotConsultant records the Execution Log the store holds at each
// consultation, the artifact the orchestrator would read.
type stateSnapshotConsultant struct {
	inner *scriptedRoutingConsultant
	store *memStore
	logs  [][]domain.ExecutionLogEntry
}

func (c *stateSnapshotConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.logs = append(c.logs, slices.Clone(c.store.state.ExecutionLog))
	return c.inner.ConsultRouting(ctx, req)
}

// A run interrupted after the consulted dispatch of build-review(impl) resumes
// at that exact row: the artifact the next consultation reads ends with the
// entry of row 4 (not row 2, the other row of the bare agent), and the row the
// consultant names runs as row 4 only.
func TestSession_QualifiedRows_Orchestrated_ResumeAfterSecondQualifiedRowKeepsTheExactRow(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{})
	// The interrupted invocation of build-review#4 left its entry in the log;
	// current_state still names the last completed step, implementer#3.
	r.store.state = domain.ArtifactState{
		Workflow:        qualifiedWorkflowID,
		WorkflowVersion: "1.0",
		Task:            "test task",
		GlobalSequence:  4,
		RunSettings:     domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
		CurrentState: domain.CurrentState{
			Phase:      "EXECUTION",
			LastStatus: domain.StatusSUCCESS,
			LastAgent:  "implementer#3",
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			qualifiedStep(1, "planner#1", "PLANNING", 1),
			qualifiedStep(2, "build-review#2", "PLANNING", 2),
			qualifiedStep(3, "implementer#3", "EXECUTION", 3),
			qualifiedStep(4, "build-review#4", "REVIEW", 4),
		},
	}
	r.store.exists = true
	snapshots := &stateSnapshotConsultant{inner: r.consultant, store: r.store}
	r.ses = session.New(session.Deps{
		Harness:  r.f,
		Store:    r.store,
		Routing:  snapshots,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	r.consultant.queueDispatch("build-review", "re-run the implementation review", qrImplReview)
	r.consultant.queueStop("done")
	r.queue("build-review", "build-review#5", domain.StatusSUCCESS)
	cfg := r.config(domain.ExecutionModeOrchestrated)
	markResume(&cfg)

	got, err := r.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(snapshots.logs) == 0 {
		t.Fatal("want a consultation after the resume, got none")
	}
	first := snapshots.logs[0]
	if lastEntry := first[len(first)-1]; lastEntry.WorkflowRow != 4 || lastEntry.Phase != "REVIEW" {
		t.Errorf("want the first consultation to read an artifact ending at row 4 (REVIEW), got row %d phase %q",
			lastEntry.WorkflowRow, lastEntry.Phase)
	}
	invs := r.f.Invocations()
	if len(invs) != 1 || invs[0].Agent.Identifier != "build-review" {
		t.Fatalf("want exactly one build-review dispatch, got %d", len(invs))
	}
	requireRecordedRows(t, r.store, 4)
}
