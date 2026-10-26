package session_test

// Tests that every change of the standing guidance is written to the Runner
// log with the new text: loaded, accepted, edited, regenerated and edited
// during the run.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
)

func requireChanges(t *testing.T, g *guidanceRig, want ...string) {
	t.Helper()
	if got := g.guidanceChanges(); !reflect.DeepEqual(got, want) {
		t.Errorf("want guidance changes %v, got %v", want, got)
	}
}

// requireLoggedText asserts that event carries both fields of g in its message.
func requireLoggedText(t *testing.T, event sessionLogEntry, g domain.Guidance) {
	t.Helper()
	for _, text := range []string{g.TaskDescription, g.Constraints, "[task_description]", "[constraints]"} {
		if !strings.Contains(event.Message, text) {
			t.Errorf("want the log message to contain %q, got %q", text, event.Message)
		}
	}
}

func TestSession_GuidanceLog_FirstRunAccepted_LogsAcceptedWithText(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireChanges(t, g, "accepted")
	requireLoggedText(t, g.eventAt(t, 0), adviceValue)
}

func TestSession_GuidanceLog_FirstRunEdited_LogsEditedWithNewText(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.ask.queueText(domain.QuestionGuidanceTask, answerText("edited-task-text"))
	g.ask.queueText(domain.QuestionGuidanceConstraints, answerText("edited-constraints-text"))
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireChanges(t, g, "edited")
	event := g.eventAt(t, 0)
	requireLoggedText(t, event, domain.Guidance{TaskDescription: "edited-task-text", Constraints: "edited-constraints-text"})
	if strings.Contains(event.Message, adviceTask) {
		t.Errorf("want the new text logged, not the replaced advice, got %q", event.Message)
	}
}

func TestSession_GuidanceLog_ResumeAccepted_LogsLoadedThenAccepted(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueOK("agent-b")

	_, _ = g.ses.Start(context.Background(), g.linearResume())

	requireChanges(t, g, "loaded", "accepted")
	for _, e := range g.guidanceEvents() {
		requireLoggedText(t, e, savedGuidanceValue)
	}
}

func TestSession_GuidanceLog_ResumeRegenerated_LogsRegeneratedWithNewText(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{
		answerReview(domain.GuidanceOptionRegenerate),
		answerReview(domain.GuidanceOptionAccept),
	}
	g.queueOK("agent-b")

	_, _ = g.ses.Start(context.Background(), g.linearResume())

	requireChanges(t, g, "loaded", "regenerated", "accepted")
	requireLoggedText(t, g.eventAt(t, 1), adviceValue)
	requireLoggedText(t, g.eventAt(t, 2), adviceValue)
}

func TestSession_GuidanceLog_ResumeEdited_LogsLoadedThenEdited(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.ask.queueText(domain.QuestionGuidanceTask, answerText("resume-edited-task"))
	g.queueOK("agent-b")

	_, _ = g.ses.Start(context.Background(), g.linearResume())

	requireChanges(t, g, "loaded", "edited")
	requireLoggedText(t, g.eventAt(t, 1), domain.Guidance{TaskDescription: "resume-edited-task", Constraints: savedConstraints})
}

func TestSession_GuidanceLog_CannotAsk_LogsAccepted(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.ask.unsupported = true
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireChanges(t, g, "accepted")
	requireLoggedText(t, g.eventAt(t, 0), adviceValue)
}

func TestSession_GuidanceLog_EditedDuringRun_LogsEditedInRunWithNewText(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.onFirstDispatch(func() { _ = updateFrom(g.live, editedInRun) })
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireChanges(t, g, "loaded", "accepted", "edited-in-run")
	requireLoggedText(t, g.eventAt(t, 2), editedInRun)
}

func TestSession_GuidanceLog_RefusedEditDuringRun_LogsNothing(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.onFirstDispatch(func() {
		g.files.saveErr = errString("disk full")
		_ = updateFrom(g.live, editedInRun)
	})
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireChanges(t, g, "loaded", "accepted")
}

func TestSession_GuidanceLog_PreConsultationFailureOrStop_LogsNothing(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.err = &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "timed out"}

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireChanges(t, g)
}
