package session_test

// Tests for the consultation the session enters when the next step of the
// run has already finished: the request it sends, live and on resume, what it
// consumes, and the execution of the orchestrator's dispatch afterwards. The
// run is the qualified-rows workflow routed back to its first review row.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// queueWithMessage scripts one response for the bare agent id with the given
// status message.
func (r qualifiedRun) queueWithMessage(agent, instance string, status domain.StatusCode, message string) {
	r.f.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: instance,
		StatusCode:      status,
		StatusMessage:   message,
	}})
}

// routedBackRun scripts a run in which build-review(impl) reports findings
// that name build-review(tests). The re-run of build-review(tests) (instance
// build-review#5) succeeds with statusMessage; the implementer row after it
// had finished, so the session consults. The orchestrator re-runs the
// implementer row.
func routedBackRun(t *testing.T, statusMessage string) qualifiedRun {
	t.Helper()
	r := newQualifiedRun(t, qualifiedOptions{implFindings: "build-review(tests)"})
	r.queue("planner", "planner#1", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
	r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#4", domain.StatusCOMPLETED_NEEDS_ACTION)
	r.queueWithMessage("build-review", "build-review#5", domain.StatusSUCCESS, statusMessage)
	r.queue("implementer", "implementer#6", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#7", domain.StatusSUCCESS)
	r.consultant.queueDispatch("implementer", "rerun the implementer row", 2)
	return r
}

// resumedFinishedRun is a run stopped after build-review(impl) had finished
// while its last completed step was the implementer row: the resume's next
// step already finished. The orchestrator re-runs that row.
func resumedFinishedRun(t *testing.T) (qualifiedRun, domain.RunConfig) {
	t.Helper()
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
	return r, cfg
}

func TestSession_FinishedWork_Live_RequestCarriesFullStatusMessageAndNoErrorReason(t *testing.T) {
	message := strings.Repeat("reviewed the tests against the plan; ", 120) + "END"
	r := routedBackRun(t, message)

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(r.consultant.Requests) != 1 {
		t.Fatalf("want exactly one consultation, got %d", len(r.consultant.Requests))
	}
	req := r.consultant.Requests[0]
	requireFinishedWorkConsultation(t, req, 2)
	if req.LastStatusMessage == nil || *req.LastStatusMessage != message {
		t.Errorf("last_status_message: want the full %d-character message of the step that succeeded, got %v",
			len(message), req.LastStatusMessage)
	}
	if req.LastErrorReason != nil {
		t.Errorf("last_error_reason: want null after a SUCCESS, got %q", *req.LastErrorReason)
	}
}

func TestSession_FinishedWork_Live_ConsultationConsumesNoSeqAndWritesNoLogRow(t *testing.T) {
	r := routedBackRun(t, "re-checked")

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 7 {
		t.Fatalf("want 7 dispatches, got %d", len(invs))
	}
	// The dispatch after the consultation takes the very next Seq.
	requireDispatchStep(t, invs, 5, "implementer", "implementer#6", "")
	var seqs []int
	for _, e := range r.store.state.ExecutionLog {
		seqs = append(seqs, e.Seq)
	}
	if want := []int{1, 2, 3, 4, 5, 6, 7}; !slices.Equal(seqs, want) {
		t.Errorf("Execution Log Seq values: want %v (no row for the consultation), got %v", want, seqs)
	}
	requireRecordedRows(t, r.store, 1, 2, 3, 4, 2, 3, 4)
}

func TestSession_FinishedWork_Live_ConsultantDispatchOfFinishedStepRunsWithoutTrippingAntiLoop(t *testing.T) {
	r := routedBackRun(t, "re-checked")

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 7 {
		t.Fatalf("want the consultant's dispatch executed and the run completed in 7 dispatches, got %d", len(invs))
	}
	if r.consultant.CallCount != 1 {
		t.Errorf("want one consultation, the guard must not escalate to another, got %d", r.consultant.CallCount)
	}
}

func TestSession_FinishedWork_Resume_RequestHasNullStatusMessageAndErrorReason(t *testing.T) {
	r, cfg := resumedFinishedRun(t)

	got, err := r.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(r.consultant.Requests) != 1 {
		t.Fatalf("want exactly one consultation, got %d", len(r.consultant.Requests))
	}
	req := r.consultant.Requests[0]
	requireFinishedWorkConsultation(t, req, 3)
	if req.LastStatusMessage != nil {
		t.Errorf("last_status_message: want null on resume, got %q", *req.LastStatusMessage)
	}
	if req.LastErrorReason != nil {
		t.Errorf("last_error_reason: want null on resume, got %q", *req.LastErrorReason)
	}
}

func TestSession_FinishedWork_Resume_ConsultationConsumesNoSeqAndWritesNoLogRow(t *testing.T) {
	r, cfg := resumedFinishedRun(t)

	got, err := r.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.f.Invocations()
	if len(invs) != 1 {
		t.Fatalf("want the consultant's dispatch executed once, got %d dispatches", len(invs))
	}
	requireDispatchStep(t, invs, 0, "build-review", "build-review#5", "impl")
	if n := len(r.store.state.ExecutionLog); n != 5 {
		t.Errorf("Execution Log: want the 4 resumed rows plus the one dispatched row, got %d", n)
	}
	requireRecordedRows(t, r.store, 4)
}

func TestSession_FinishedWork_ManualDispatchArmed_IsResolvedByTheManualResolver(t *testing.T) {
	r, cfg := resumedFinishedRun(t)
	user := newManualUser(t).pickOne(domain.QuestionManualRow, "4").
		say(domain.QuestionManualTask, "re-run the impl review").
		acceptDefaultsPlus(domain.QuestionManualInputs).
		acceptDefaultsPlus(domain.QuestionManualOutputs).
		say(domain.QuestionManualConstraints, "").
		pickOne(domain.QuestionManualHITL, domain.ManualHITLDefault)
	r.ses = session.New(session.Deps{
		Harness:  r.f,
		Store:    r.store,
		Routing:  r.consultant,
		Manual:   &deviation.ManualResolver{Interact: user},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	cfg.ManualDispatch = true

	got, err := r.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !slices.Contains(user.asked, interaction.QuestionID(domain.QuestionManualRow)) {
		t.Errorf("want the manual dialogue asked for the finished-work consultation, asked %v", user.asked)
	}
	if r.consultant.CallCount != 0 {
		t.Errorf("want the armed manual dispatch to take the place of the orchestrator, got %d consultations", r.consultant.CallCount)
	}
	invs := r.f.Invocations()
	if len(invs) != 1 {
		t.Fatalf("want the manually chosen step dispatched once, got %d dispatches", len(invs))
	}
	if inv := invs[0]; inv.Agent.Identifier != "build-review" || inv.Request.AgentInstanceID != "build-review#5" {
		t.Errorf("want build-review dispatched as build-review#5, got %q as %q", inv.Agent.Identifier, inv.Request.AgentInstanceID)
	}
}
