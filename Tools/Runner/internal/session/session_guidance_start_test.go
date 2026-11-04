package session_test

// Tests for the run-start sequence with standing guidance: the first
// pre-consultation result is saved and offered for Accept/Edit, a resume with
// saved guidance skips pre-consultation and offers Accept/Edit/Regenerate, and
// failure, stop and runs outside auto and auto-review stay as they were.

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
)

const reviewCancelledMessage = "run stopped: guidance review cancelled; resume to continue"

func requireOptions(t *testing.T, q interaction.ChoiceQuestion, want ...string) {
	t.Helper()
	if got := optionIDs(q); !sameStrings(got, want) {
		t.Errorf("want review options %v, got %v", want, got)
	}
}

// ===== First pre-consultation in a run =====

func TestSession_GuidanceFirstRun_SavesPreConsultationResult(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	saves := g.files.savedGuidance()
	if len(saves) != 1 || saves[0] != adviceValue {
		t.Fatalf("want the pre-consultation result saved once as %+v, got %+v", adviceValue, saves)
	}
	if g.files.saves[0].Folder != g.runFolder {
		t.Errorf("want the guidance saved in the run folder %q, got %q", g.runFolder, g.files.saves[0].Folder)
	}
}

func TestSession_GuidanceFirstRun_EmptyResult_IsSavedToo(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if saves := g.files.savedGuidance(); len(saves) != 1 || saves[0] != (domain.Guidance{}) {
		t.Errorf("want one save of the empty result, got %+v", saves)
	}
}

func TestSession_GuidanceFirstRun_OffersAcceptAndEditBeforeFirstDispatch(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if len(g.ask.reviews) != 1 {
		t.Fatalf("want exactly one review question, got %d", len(g.ask.reviews))
	}
	requireOptions(t, g.ask.reviews[0], domain.GuidanceOptionAccept, domain.GuidanceOptionEdit)
	if g.ask.firstReviewDispatches(t) != 0 {
		t.Errorf("want the review before the first dispatch, but %d dispatches had run", g.ask.firstReviewDispatches(t))
	}
}

func TestSession_GuidanceFirstRun_EmptyResult_StillOffersReview(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if len(g.ask.reviews) != 1 {
		t.Fatalf("want an empty result offered for review too, got %d review questions", len(g.ask.reviews))
	}
	if g.ask.firstReviewDispatches(t) != 0 {
		t.Errorf("want the review before the first dispatch, got %d dispatches before it", g.ask.firstReviewDispatches(t))
	}
}

func TestSession_GuidanceReview_DetailShowsBothFields(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = domain.PreConsultationAdvice{TaskDescription: "line one\nline two", Constraints: "be brief"}
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	want := "Appended to task_description:\nline one\nline two\n\nAppended to constraints:\nbe brief"
	if len(g.ask.reviews) != 1 || g.ask.reviews[0].Detail != want {
		t.Errorf("want review detail %q, got %+v", want, g.ask.reviews)
	}
}

func TestSession_GuidanceReview_EmptyFieldShownAsEmptyMarker(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = domain.PreConsultationAdvice{TaskDescription: "only task"}
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	want := "Appended to task_description:\nonly task\n\nAppended to constraints:\n(empty)"
	if len(g.ask.reviews) != 1 || g.ask.reviews[0].Detail != want {
		t.Errorf("want review detail %q, got %+v", want, g.ask.reviews)
	}
}

func TestSession_GuidanceReview_QuestionHasTheGuidanceReviewID(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if len(g.ask.reviews) != 1 || g.ask.reviews[0].ID != domain.QuestionGuidanceReview {
		t.Errorf("want a question with ID %q, got %+v", domain.QuestionGuidanceReview, g.ask.reviews)
	}
}

// ===== Edit =====

func TestSession_GuidanceEdit_ReplacesSavedGuidanceAndAppliesToDispatch(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.ask.queueText(domain.QuestionGuidanceTask, answerText("edited-task-text"))
	g.ask.queueText(domain.QuestionGuidanceConstraints, answerText("edited-constraints-text"))
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := domain.Guidance{TaskDescription: "edited-task-text", Constraints: "edited-constraints-text"}
	file, _ := g.files.fileOf(g.runFolder)
	if file != want {
		t.Errorf("want the saved guidance replaced by %+v, got %+v", want, file)
	}
	for _, agent := range []string{"agent-a", "agent-b"} {
		req := g.requestsFor(agent)[0]
		if !carriesGuidance(req, want) {
			t.Errorf("want %s dispatched with the edited guidance, got task=%q constraints=%q", agent, req.TaskDescription, req.Constraints)
		}
		if !carriesNoGuidance(req, adviceValue) {
			t.Errorf("want the replaced advice gone from %s, got task=%q", agent, req.TaskDescription)
		}
	}
}

func TestSession_GuidanceEdit_AsksTaskThenConstraintsMultilineWithCurrentAsDefault(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if len(g.ask.asks) != 2 {
		t.Fatalf("want two text questions, got %d", len(g.ask.asks))
	}
	first, second := g.ask.asks[0], g.ask.asks[1]
	if first.ID != domain.QuestionGuidanceTask || second.ID != domain.QuestionGuidanceConstraints {
		t.Errorf("want task_description then constraints, got %q then %q", first.ID, second.ID)
	}
	if !first.Multiline || !second.Multiline {
		t.Errorf("want both questions multi-line, got %v and %v", first.Multiline, second.Multiline)
	}
	if first.Default != adviceTask || second.Default != adviceConstraint {
		t.Errorf("want the current fields as defaults, got %q and %q", first.Default, second.Default)
	}
}

func TestSession_GuidanceEdit_UnsupportedAnswerLeavesThatFieldUnchanged(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.ask.queueText(domain.QuestionGuidanceTask, answerText("new-task-only"))
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	want := domain.Guidance{TaskDescription: "new-task-only", Constraints: adviceConstraint}
	if file, _ := g.files.fileOf(g.runFolder); file != want {
		t.Errorf("want %+v saved (constraints unchanged), got %+v", want, file)
	}
}

func TestSession_GuidanceEdit_CancelledQuestionReturnsToReviewSavingNothing(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{
		answerReview(domain.GuidanceOptionEdit),
		answerReview(domain.GuidanceOptionAccept),
	}
	g.ask.queueText(domain.QuestionGuidanceTask, interaction.TextAnswer{Status: interaction.Cancelled})
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(g.ask.reviews) != 2 {
		t.Errorf("want the review asked again after the cancelled edit, got %d review questions", len(g.ask.reviews))
	}
	if saves := g.files.savedGuidance(); len(saves) != 1 || saves[0] != adviceValue {
		t.Errorf("want only the pre-consultation result saved, got %+v", saves)
	}
	if !carriesGuidance(g.firstRequestOf(t, "agent-a"), adviceValue) {
		t.Error("want the dispatch to carry the guidance as it was before the cancelled edit")
	}
}

// ===== Cancelled review =====

func TestSession_GuidanceReview_Cancelled_StopsResumablyBeforeAnyDispatch(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{{Status: interaction.Cancelled}}
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunStopped)
	if got.Message != reviewCancelledMessage {
		t.Errorf("want message %q, got %q", reviewCancelledMessage, got.Message)
	}
	if n := len(g.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch after a cancelled review, got %d", n)
	}
	if file, ok := g.files.fileOf(g.runFolder); !ok || file != adviceValue {
		t.Errorf("want the file left as saved, got %+v found=%v", file, ok)
	}
}

// ===== Resume with saved guidance =====

func TestSession_GuidanceResume_UsesSavedGuidanceWithoutPreConsultation(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.pc.advice = adviceOf(adviceValue)
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), g.linearResume())

	requireRunStatus(t, got, err, domain.RunCompleted)
	if g.pc.Called {
		t.Error("want pre-consultation not run when saved guidance exists")
	}
	req := g.firstRequestOf(t, "agent-b")
	if !carriesGuidance(req, savedGuidanceValue) || !carriesNoGuidance(req, adviceValue) {
		t.Errorf("want the saved guidance applied, got task=%q constraints=%q", req.TaskDescription, req.Constraints)
	}
}

func TestSession_GuidanceResume_LoadsFromTheRunFolder(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueOK("agent-b")

	_, _ = g.ses.Start(context.Background(), g.linearResume())

	if len(g.files.loads) == 0 || g.files.loads[0] != g.runFolder {
		t.Errorf("want the guidance loaded from %q, got %v", g.runFolder, g.files.loads)
	}
}

func TestSession_GuidanceResume_OffersAcceptEditRegenerateWithSavedText(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueOK("agent-b")

	_, _ = g.ses.Start(context.Background(), g.linearResume())

	if len(g.ask.reviews) != 1 {
		t.Fatalf("want one review question, got %d", len(g.ask.reviews))
	}
	requireOptions(t, g.ask.reviews[0], domain.GuidanceOptionAccept, domain.GuidanceOptionEdit, domain.GuidanceOptionRegenerate)
	if !strings.Contains(g.ask.reviews[0].Detail, savedTask) || !strings.Contains(g.ask.reviews[0].Detail, savedConstraints) {
		t.Errorf("want the saved text in the review detail, got %q", g.ask.reviews[0].Detail)
	}
	if g.ask.firstReviewDispatches(t) != 0 {
		t.Errorf("want the review before the first dispatch of the resume, got %d", g.ask.firstReviewDispatches(t))
	}
}

func TestSession_GuidanceResume_EditReplacesSavedFile(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.ask.queueText(domain.QuestionGuidanceTask, answerText("resume-edited-task"))
	g.queueOK("agent-b")

	_, _ = g.ses.Start(context.Background(), g.linearResume())

	want := domain.Guidance{TaskDescription: "resume-edited-task", Constraints: savedConstraints}
	if file, _ := g.files.fileOf(g.runFolder); file != want {
		t.Errorf("want %+v saved, got %+v", want, file)
	}
	if len(g.ask.asks) == 0 || g.ask.asks[0].Default != savedTask {
		t.Errorf("want the saved task text as default, got %+v", g.ask.asks)
	}
	if !carriesGuidance(g.firstRequestOf(t, "agent-b"), want) {
		t.Error("want the dispatch to carry the edited guidance")
	}
}

func TestSession_GuidanceResume_Regenerate_RunsPreConsultationAndOffersResult(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{
		answerReview(domain.GuidanceOptionRegenerate),
		answerReview(domain.GuidanceOptionAccept),
	}
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), g.linearResume())

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !g.pc.Called {
		t.Fatal("want pre-consultation run again on Regenerate")
	}
	if saves := g.files.savedGuidance(); len(saves) != 1 || saves[0] != adviceValue {
		t.Errorf("want the regenerated result saved, got %+v", saves)
	}
	if len(g.ask.reviews) != 2 {
		t.Fatalf("want the regenerated result offered for review, got %d review questions", len(g.ask.reviews))
	}
	requireOptions(t, g.ask.reviews[1], domain.GuidanceOptionAccept, domain.GuidanceOptionEdit)
	if !strings.Contains(g.ask.reviews[1].Detail, adviceTask) {
		t.Errorf("want the regenerated text in the second review, got %q", g.ask.reviews[1].Detail)
	}
	req := g.firstRequestOf(t, "agent-b")
	if !carriesGuidance(req, adviceValue) || !carriesNoGuidance(req, savedGuidanceValue) {
		t.Errorf("want the regenerated guidance applied, got task=%q constraints=%q", req.TaskDescription, req.Constraints)
	}
}

func TestSession_GuidanceResume_RegenerateFailure_EndsStartFailedKeepingSavedFile(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.pc.err = &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "timed out"}
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionRegenerate)}
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), g.linearResume())

	requireStartFailed(t, got, err)
	if file, _ := g.files.fileOf(g.runFolder); file != savedGuidanceValue || len(g.files.saves) != 0 {
		t.Errorf("want the saved file untouched, got %+v with %d saves", file, len(g.files.saves))
	}
	if n := len(g.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch, got %d", n)
	}
	if len(g.ask.reviews) != 1 {
		t.Errorf("want the saved guidance offered for review before Regenerate, got %d review questions", len(g.ask.reviews))
	}
}

func TestSession_GuidanceResume_RegenerateStop_EndsStoppedByConsultantKeepingSavedFile(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.pc.advice = domain.PreConsultationAdvice{Stop: &domain.StopInstruction{Reason: "not now"}}
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionRegenerate)}
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), g.linearResume())

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if !got.PreConsultationStop {
		t.Error("want PreConsultationStop set")
	}
	if file, _ := g.files.fileOf(g.runFolder); file != savedGuidanceValue || len(g.files.saves) != 0 {
		t.Errorf("want the saved file untouched, got %+v with %d saves", file, len(g.files.saves))
	}
	if len(g.ask.reviews) != 1 {
		t.Errorf("want the saved guidance offered for review before Regenerate, got %d review questions", len(g.ask.reviews))
	}
}

// ===== Failure and stop of the first pre-consultation =====

func TestSession_GuidanceFirstRun_PreConsultationFailure_SavesNothingAndAsksNothing(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.err = &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "timed out"}
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireStartFailed(t, got, err)
	if len(g.files.saves) != 0 || len(g.ask.reviews) != 0 {
		t.Errorf("want no save and no review, got %d saves and %d reviews", len(g.files.saves), len(g.ask.reviews))
	}
}

func TestSession_GuidanceFirstRun_PreConsultationStop_SavesNothingAndAsksNothing(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = domain.PreConsultationAdvice{Stop: &domain.StopInstruction{Reason: "not now"}}
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if !got.PreConsultationStop {
		t.Error("want PreConsultationStop set")
	}
	if len(g.files.saves) != 0 || len(g.ask.reviews) != 0 {
		t.Errorf("want no save and no review, got %d saves and %d reviews", len(g.files.saves), len(g.ask.reviews))
	}
}

func TestSession_GuidanceFirstRun_FailureThenResume_RunsPreConsultationAgain(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.err = &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "timed out"}
	cfg := g.autoConfig("linear")
	first, firstErr := g.ses.Start(context.Background(), cfg)
	requireStartFailed(t, first, firstErr)
	g.pc.err, g.pc.Called, g.pc.advice = nil, false, adviceOf(adviceValue)
	g.queueLinear()
	markResume(&cfg)

	got, err := g.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !g.pc.Called {
		t.Error("want pre-consultation run again because the failure left no saved guidance")
	}
	if saves := g.files.savedGuidance(); len(saves) != 1 || saves[0] != adviceValue {
		t.Errorf("want the result saved once the resume succeeded, got %+v", saves)
	}
}

// ===== Store errors =====

func TestSession_GuidanceLoadError_EndsStartFailedBeforeAnyDispatch(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.loadErr = errString("RunnerGuidance.txt: line 3: unexpected text")
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), g.linearResume())

	msg := requireStartFailed(t, got, err)
	if !strings.Contains(msg, "unexpected text") {
		t.Errorf("want the load error in the message, got %q", msg)
	}
	if n := len(g.adapter.Invocations()); n != 0 || g.pc.Called {
		t.Errorf("want no dispatch and no pre-consultation, got %d dispatches, called=%v", n, g.pc.Called)
	}
}

func TestSession_GuidanceSaveError_EndsStartFailedBeforeAnyDispatch(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.saveErr = errString("RunnerGuidance.txt: disk full")
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	msg := requireStartFailed(t, got, err)
	if !strings.Contains(msg, "disk full") {
		t.Errorf("want the save error in the message, got %q", msg)
	}
	if n := len(g.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch, got %d", n)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
