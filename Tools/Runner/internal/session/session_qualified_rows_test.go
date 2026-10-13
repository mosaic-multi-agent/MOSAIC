package session_test

// Tests for mode-qualified rows through a session in the auto modes: one agent
// (build-review) fills two rows, told apart by the qualifiers (tests) and
// (impl). The session resolves, dispatches, logs, resumes and routes findings
// by the bare agent id and keeps every decision on the exact row.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const qualifiedWorkflowID = "qualified-rows"

// qualifiedRun bundles one session over the qualified-rows workflow.
type qualifiedRun struct {
	ses        session.Session
	f          *harness.MockAdapter
	store      *memStore
	consultant *scriptedRoutingConsultant
	orchPath   string
}

// qualifiedOptions varies the qualified-rows fixture.
type qualifiedOptions struct {
	// implFindings replaces the On Findings target of the build-review(impl)
	// row (the fixture names implementer). Empty keeps the fixture.
	implFindings string
	// omitBuildReviewFile leaves build-review.md out of the agent directory.
	omitBuildReviewFile bool
}

// newQualifiedRun builds a session over the qualified-rows workflow with agent
// files for planner, implementer and (unless omitted) the single
// build-review.md that serves both qualified rows.
func newQualifiedRun(t *testing.T, opts qualifiedOptions) qualifiedRun {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "qualified-rows-orch.md")
	if opts.implFindings != "" {
		content, err := os.ReadFile(orchPath)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		patched := strings.Replace(string(content),
			"| COMPLETE | implementer |", "| COMPLETE | "+opts.implFindings+" |", 1)
		if patched == string(content) {
			t.Fatal("fixture error: the build-review(impl) On Findings cell was not found")
		}
		if err := os.WriteFile(filepath.Join(dir, "orchestrator.md"), []byte(patched), 0600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	writeAgentFile(t, dir, "planner")
	writeAgentFile(t, dir, "implementer")
	if !opts.omitBuildReviewFile {
		writeAgentFile(t, dir, "build-review")
	}

	r := qualifiedRun{
		f:          harness.NewMockAdapter(),
		store:      &memStore{},
		consultant: &scriptedRoutingConsultant{},
		orchPath:   orchPath,
	}
	r.ses = session.New(session.Deps{
		Harness:  r.f,
		Store:    r.store,
		Routing:  r.consultant,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	return r
}

func (r qualifiedRun) config(mode domain.ExecutionMode) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: r.orchPath,
		WorkflowID:           qualifiedWorkflowID,
		Task:                 "test task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: mode},
	}
}

// queue scripts one response for the bare agent id.
func (r qualifiedRun) queue(agent, instance string, status domain.StatusCode) {
	r.f.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: instance,
		StatusCode:      status,
		StatusMessage:   "scripted",
	}})
}

// qualifiedFocus is the task-description paragraph that carries a qualifier.
func qualifiedFocus(qualifier string) string {
	return engine.QualifierFocusPrefix + " " + qualifier
}

// requireDispatchStep asserts invocation i went to the bare agent id under the
// instance id, with the qualifier focus (or none when qualifier is empty).
func requireDispatchStep(t *testing.T, invs []harness.Invocation, i int, agent, instance, qualifier string) {
	t.Helper()
	if i >= len(invs) {
		t.Fatalf("want an invocation at position %d (%s), got only %d", i, instance, len(invs))
	}
	inv := invs[i]
	if inv.Agent.Identifier != agent || inv.Request.AgentInstanceID != instance {
		t.Errorf("invocation %d: want agent %q instance %q, got agent %q instance %q",
			i, agent, instance, inv.Agent.Identifier, inv.Request.AgentInstanceID)
	}
	task := inv.Request.TaskDescription
	if qualifier == "" {
		if strings.Contains(task, engine.QualifierFocusPrefix) {
			t.Errorf("invocation %d (%s) has no qualifier but its task carries a focus: %q", i, instance, task)
		}
		return
	}
	if !strings.Contains(task, qualifiedFocus(qualifier)) {
		t.Errorf("invocation %d (%s): want focus %q in task description, got %q", i, instance, qualifier, task)
	}
}

// requireRecordedRows asserts the workflow rows (1-based) the store recorded,
// in order.
func requireRecordedRows(t *testing.T, store *memStore, want ...domain.WorkflowRow) {
	t.Helper()
	var got []domain.WorkflowRow
	for _, step := range store.Applied {
		got = append(got, step.WorkflowRow)
	}
	if len(got) != len(want) {
		t.Fatalf("recorded rows: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recorded rows: want %v, got %v", want, got)
		}
	}
}

// ===== Dispatch in Modes 2 and 3 =====

// TestSession_QualifiedRows_AutoModes_ResolveAndDispatchBareAgentWithFocus
// verifies that a workflow repeating build-review through two qualifiers runs
// to completion with one build-review.md: both rows are dispatched as agent
// build-review with instance ids build-review#N and their own qualifier focus,
// in table order, and no consultation happens.
func TestSession_QualifiedRows_AutoModes_ResolveAndDispatchBareAgentWithFocus(t *testing.T) {
	modes := map[string]domain.ExecutionMode{
		"auto":        domain.ExecutionModeAuto,
		"auto-review": domain.ExecutionModeAutoReview,
	}
	for name, mode := range modes {
		t.Run(name, func(t *testing.T) {
			r := newQualifiedRun(t, qualifiedOptions{})
			r.queue("planner", "planner#1", domain.StatusSUCCESS)
			r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
			r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
			r.queue("build-review", "build-review#4", domain.StatusSUCCESS)

			got, err := r.ses.Start(context.Background(), r.config(mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			invs := r.f.Invocations()
			if len(invs) != 4 {
				t.Fatalf("want 4 dispatches in table order, got %d", len(invs))
			}
			requireDispatchStep(t, invs, 0, "planner", "planner#1", "")
			requireDispatchStep(t, invs, 1, "build-review", "build-review#2", "tests")
			requireDispatchStep(t, invs, 2, "implementer", "implementer#3", "")
			requireDispatchStep(t, invs, 3, "build-review", "build-review#4", "impl")
			requireRecordedRows(t, r.store, 1, 2, 3, 4)
			if r.consultant.CallCount != 0 {
				t.Errorf("want no consultation for an all-SUCCESS qualified run, got %d", r.consultant.CallCount)
			}
		})
	}
}

// ===== Findings route-back =====

// TestSession_QualifiedRows_AutoReview_FindingsRouteBackLandsOnNamedRowAbove
// verifies that when build-review(impl) returns COMPLETED_NEEDS_ACTION and its
// On Findings names implementer, the engine's route-back dispatches
// implementer, never the build-review(tests) row, and then re-runs
// build-review(impl) with its own focus.
func TestSession_QualifiedRows_AutoReview_FindingsRouteBackLandsOnNamedRowAbove(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{})
	r.queue("planner", "planner#1", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
	r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#4", domain.StatusCOMPLETED_NEEDS_ACTION)
	r.queue("implementer", "implementer#5", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#6", domain.StatusSUCCESS)

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 6 {
		t.Fatalf("want 6 dispatches (route-back repeats implementer and build-review(impl)), got %d", len(invs))
	}
	requireDispatchStep(t, invs, 4, "implementer", "implementer#5", "")
	requireDispatchStep(t, invs, 5, "build-review", "build-review#6", "impl")
	requireRecordedRows(t, r.store, 1, 2, 3, 4, 3, 4)
	if r.consultant.CallCount != 0 {
		t.Errorf("want the unambiguous On Findings auto-routed without consultation, got %d consultations",
			r.consultant.CallCount)
	}
}

// TestSession_QualifiedRows_AutoReview_FindingsRouteBackToOtherQualifiedRow
// verifies that an On Findings entry naming the build-review(tests) cell lands
// on that exact row, dispatched with the tests focus. The implementer row after
// it had already finished, so the step into it is a finished-work consultation
// and the orchestrator re-runs it; build-review(impl) never finished (its only
// result was findings), so the engine continues to it without consulting.
func TestSession_QualifiedRows_AutoReview_FindingsRouteBackToOtherQualifiedRow(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{implFindings: "build-review(tests)"})
	r.queue("planner", "planner#1", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
	r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#4", domain.StatusCOMPLETED_NEEDS_ACTION)
	r.queue("build-review", "build-review#5", domain.StatusSUCCESS)
	r.queue("implementer", "implementer#6", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#7", domain.StatusSUCCESS)
	r.consultant.queueDispatch("implementer", "rerun implementer", 2)

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 7 {
		t.Fatalf("want 7 dispatches, got %d", len(invs))
	}
	requireDispatchStep(t, invs, 4, "build-review", "build-review#5", "tests")
	requireDispatchStep(t, invs, 5, "implementer", "implementer#6", "")
	requireDispatchStep(t, invs, 6, "build-review", "build-review#7", "impl")
	requireRecordedRows(t, r.store, 1, 2, 3, 4, 2, 3, 4)
	if len(r.consultant.Requests) != 1 {
		t.Fatalf("want one finished-work consultation (implementer finished, build-review(impl) did not), got %d", len(r.consultant.Requests))
	}
	requireFinishedWorkConsultation(t, r.consultant.Requests[0], 2)
}

// ===== Resume =====

// qualifiedStep builds one recorded execution log entry of the qualified-rows
// run; row is the 1-based workflow row (0 for infrastructure).
func qualifiedStep(seq int, instance, phase string, row int) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{
		Seq:         seq,
		Agent:       instance,
		Phase:       phase,
		WorkflowRow: domain.WorkflowRow(row),
		Status:      domain.StatusSUCCESS,
	}
}

// seedQualifiedResume stores an interrupted run: the log, and a current state
// that still names lastAgent as the last completed workflow step.
func seedQualifiedResume(r qualifiedRun, lastAgent, lastPhase string, log ...domain.ExecutionLogEntry) {
	r.store.state = domain.ArtifactState{
		Workflow:        qualifiedWorkflowID,
		WorkflowVersion: "1.0",
		Task:            "test task",
		GlobalSequence:  len(log),
		RunSettings:     domain.RunSettings{Mode: domain.ExecutionModeAuto},
		CurrentState: domain.CurrentState{
			Phase:      lastPhase,
			LastStatus: domain.StatusSUCCESS,
			LastAgent:  lastAgent,
		},
		ExecutionLog: log,
	}
	r.store.exists = true
}

// TestSession_QualifiedRows_Resume_InterruptedAfterInfraEntry_RerunsExactRow
// verifies the mid-invocation rewind over qualified rows: the interrupted row
// (implementer) is preceded by an infrastructure entry, which itself follows
// build-review(tests). The qualified row's log entry must be recognised as a
// routing-table entry, so the rewind stops at it and the run re-dispatches
// implementer. A rewind that treats it as foreign would fall back to planner
// and dispatch build-review(tests) a second time. The implementer row already
// has a SUCCESS entry, so the resume consults the orchestrator, which re-runs
// that exact row.
func TestSession_QualifiedRows_Resume_InterruptedAfterInfraEntry_RerunsExactRow(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{})
	infra := qualifiedStep(3, "checkpoint-manager-git#3", "PLANNING", int(domain.NoWorkflowRow))
	seedQualifiedResume(r, "build-review#2", "PLANNING",
		qualifiedStep(1, "planner#1", "PLANNING", 1),
		qualifiedStep(2, "build-review#2", "PLANNING", 2),
		infra,
		qualifiedStep(4, "implementer#4", "EXECUTION", 3),
	)
	r.queue("implementer", "implementer#5", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#6", domain.StatusSUCCESS)
	r.consultant.queueDispatch("implementer", "rerun the interrupted implementer row", 2)
	cfg := r.config(domain.ExecutionModeAuto)
	markResume(&cfg)

	got, err := r.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 2 {
		t.Fatalf("want implementer re-dispatched then build-review(impl): 2 dispatches, got %d", len(invs))
	}
	requireDispatchStep(t, invs, 0, "implementer", "implementer#5", "")
	requireDispatchStep(t, invs, 1, "build-review", "build-review#6", "impl")
	if len(r.consultant.Requests) != 1 {
		t.Fatalf("want one finished-work consultation for the already finished implementer row, got %d", len(r.consultant.Requests))
	}
	requireFinishedWorkConsultation(t, r.consultant.Requests[0], 2)
}

// TestSession_QualifiedRows_Resume_InterruptedQualifiedRow_RerunsRowByRecordedRow
// verifies that an interrupted build-review(impl) row is re-dispatched with its
// own focus, picked by the recorded WorkflowRow, not the earlier row that holds
// the same bare agent id. The row already has a SUCCESS entry, so the resume
// consults the orchestrator, which re-runs that exact row.
func TestSession_QualifiedRows_Resume_InterruptedQualifiedRow_RerunsRowByRecordedRow(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{})
	seedQualifiedResume(r, "implementer#3", "EXECUTION",
		qualifiedStep(1, "planner#1", "PLANNING", 1),
		qualifiedStep(2, "build-review#2", "PLANNING", 2),
		qualifiedStep(3, "implementer#3", "EXECUTION", 3),
		qualifiedStep(4, "build-review#4", "REVIEW", 4),
	)
	r.queue("build-review", "build-review#5", domain.StatusSUCCESS)
	r.consultant.queueDispatch("build-review", qualifiedFocus("impl"), 3)
	cfg := r.config(domain.ExecutionModeAuto)
	markResume(&cfg)

	got, err := r.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 1 {
		t.Fatalf("want only the interrupted row re-dispatched, got %d dispatches", len(invs))
	}
	requireDispatchStep(t, invs, 0, "build-review", "build-review#5", "impl")
	requireRecordedRows(t, r.store, 4)
	if len(r.consultant.Requests) != 1 {
		t.Fatalf("want one finished-work consultation for the already finished row, got %d", len(r.consultant.Requests))
	}
	requireFinishedWorkConsultation(t, r.consultant.Requests[0], 3)
}

// ===== Missing agent file =====

// TestSession_QualifiedRows_MissingAgentFile_RefusalNamesBareAgentId verifies
// that when build-review.md is absent the pre-dispatch refusal names the agent
// by its bare id, not by a whole qualified cell, and nothing is dispatched.
func TestSession_QualifiedRows_MissingAgentFile_RefusalNamesBareAgentId(t *testing.T) {
	r := newQualifiedRun(t, qualifiedOptions{omitBuildReviewFile: true})

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAuto))

	message := requireRefused(t, got, err)
	if !strings.Contains(message, "build-review") {
		t.Errorf("want the refusal to name the missing agent build-review, got %q", message)
	}
	for _, cell := range []string{"build-review(tests)", "build-review(impl)"} {
		if strings.Contains(message, cell) {
			t.Errorf("the refusal must name the bare id, not the whole cell %q: %q", cell, message)
		}
	}
	if n := len(r.f.Invocations()); n != 0 {
		t.Errorf("want no dispatch after a pre-dispatch refusal, got %d", n)
	}
}
