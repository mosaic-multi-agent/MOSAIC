package session_test

// Tests for the user message that travels with a handed-over routing
// decision: the text entered after "Ask the orchestrator to decide" reaches the
// one orchestrator consultation it precedes, from every place the dialogue
// offers the option, and is neither stored nor sent again.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

const multiLineMessage = "Stop routing back to planning.\nGo to implementation.\n\n  keep   spacing "

// messageStep is one answer to the dialogue's message step.
type messageStep struct {
	esc  bool
	text string
}

// scriptMessages makes u answer the message step with steps in order (the last
// repeats).
func scriptMessages(u *manualUser, steps ...messageStep) {
	n := 0
	u.text[domain.QuestionManualAskMessage] = func(interaction.TextQuestion) interaction.TextAnswer {
		s := steps[min(n, len(steps)-1)]
		n++
		if s.esc {
			return interaction.TextAnswer{Status: interaction.Cancelled}
		}
		return interaction.TextAnswer{Status: interaction.Answered, Text: s.text}
	}
}

func requireUserMessage(t *testing.T, req domain.ConsultationRequest, want *string, label string) {
	t.Helper()
	switch {
	case want == nil && req.UserMessage != nil:
		t.Errorf("%s: want no user message, got %q", label, *req.UserMessage)
	case want != nil && req.UserMessage == nil:
		t.Errorf("%s: want user message %q, got none", label, *want)
	case want != nil && *req.UserMessage != *want:
		t.Errorf("%s: want user message %q verbatim, got %q", label, *want, *req.UserMessage)
	}
}

func msgPtr(s string) *string { return &s }

// messageQuestions counts how often the dialogue's message step was asked.
func messageQuestions(u *manualUser) int {
	n := 0
	for _, id := range u.asked {
		if id == domain.QuestionManualAskMessage {
			n++
		}
	}
	return n
}

// inRunMessageRun hands the decision after agent-a over with the given message
// answers; the orchestrator dispatches agent-b, then stops.
func inRunMessageRun(t *testing.T, steps ...messageStep) (*inRunRig, *scriptedRoutingConsultant, *consultTrace) {
	t.Helper()
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueDispatch("agent-b", "orchestrator's choice", 1)
	consultant.queueStop("all done")
	answers := []string{domain.ManualAskOrchestratorOptionID}
	for range steps[1:] {
		answers = append(answers, domain.ManualAskOrchestratorOptionID)
	}
	rig, _, trace := askRig(t, consultant, answers...)
	scriptMessages(rig.user, steps...)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(consultant.Requests) != 3 {
		t.Fatalf("want 3 consultations (first decision, handed over, last), got %d", len(consultant.Requests))
	}
	return rig, consultant, trace
}

func TestSession_UserMessage_InRunRequest_ReachesOnlyTheHandedOverConsultation(t *testing.T) {
	_, consultant, _ := inRunMessageRun(t, messageStep{text: multiLineMessage})

	requireUserMessage(t, consultant.Requests[0], nil, "consultation before the request")
	requireUserMessage(t, consultant.Requests[1], msgPtr(multiLineMessage), "handed-over consultation")
	requireUserMessage(t, consultant.Requests[2], nil, "next consultation")
}

func TestSession_UserMessage_LeftEmpty_SendsNone(t *testing.T) {
	for name, text := range map[string]string{"empty": "", "whitespace": " \n\t "} {
		t.Run(name, func(t *testing.T) {
			rig, consultant, trace := inRunMessageRun(t, messageStep{text: text})

			if n := messageQuestions(rig.user); n != 1 {
				t.Fatalf("want the message step asked once and left empty, got %d", n)
			}
			requireUserMessage(t, consultant.Requests[1], nil, "handed-over consultation")
			for i, a := range trace.started() {
				if a.HasUserMessage {
					t.Errorf("consultation %d announced with a user message, want none", i+1)
				}
			}
		})
	}
}

func TestSession_UserMessage_EscAtTheMessageStep_ReturnsToTheRowStepAndUsesTheLaterMessage(t *testing.T) {
	rig, consultant, _ := inRunMessageRun(t, messageStep{esc: true}, messageStep{text: "second try"})

	requireUserMessage(t, consultant.Requests[1], msgPtr("second try"), "handed-over consultation")
	if rig.user.rowQuestions() != 2 {
		t.Errorf("want the row step asked again after Esc, got %d row questions", rig.user.rowQuestions())
	}
}

func TestSession_UserMessage_InRunRequestDivertingAnEngineDispatch_CarriesTheMessage(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("orchestrator takes over and stops")
			rig, _, _ := askRig(t, consultant, domain.ManualAskOrchestratorOptionID)
			scriptMessages(rig.user, messageStep{text: multiLineMessage})
			rig.queueOK("agent-a", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if len(consultant.Requests) != 1 {
				t.Fatalf("want one consultation, got %d", len(consultant.Requests))
			}
			requireUserMessage(t, consultant.Requests[0], msgPtr(multiLineMessage), "diverted consultation")
		})
	}
}

func TestSession_UserMessage_StopScreenManualDispatch_CarriesTheMessage(t *testing.T) {
	user := newManualUser(t)
	scriptRow(user, domain.ManualAskOrchestratorOptionID)
	scriptMessages(user, messageStep{text: multiLineMessage})
	rig := newManualRig(t, user, nil, &consultTrace{})

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(rig.consultant.Requests) != 1 {
		t.Fatalf("want one orchestrator consultation, got %d", len(rig.consultant.Requests))
	}
	requireUserMessage(t, rig.consultant.Requests[0], msgPtr(multiLineMessage), "stop-screen consultation")
}

func TestSession_UserMessage_AfterForceStopOfAnAgent_CarriesTheMessage(t *testing.T) {
	user := newManualUser(t)
	scriptRow(user, domain.ManualAskOrchestratorOptionID)
	scriptMessages(user, messageStep{text: multiLineMessage})
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("orchestrator stops")
	ses, cfg := postKillRigWith(t, user, consultant)

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(consultant.Requests) != 1 {
		t.Fatalf("want one orchestrator consultation, got %d", len(consultant.Requests))
	}
	requireUserMessage(t, consultant.Requests[0], msgPtr(multiLineMessage), "post-kill consultation")
}

// A force stop of the handed-over consultation reopens the dialogue; a message
// entered there applies to the consultation it precedes only.
func TestSession_UserMessage_ForceStopOfTheConsultation_ReopenedDialogueCarriesItsOwnMessage(t *testing.T) {
	for name, second := range map[string]*string{"new message": msgPtr("second message"), "left empty": nil} {
		t.Run(name, func(t *testing.T) {
			text := ""
			if second != nil {
				text = *second
			}
			rig, user, _ := askForceRig(t, domain.ForceStopNextManual,
				domain.ManualAskOrchestratorOptionID, domain.ManualAskOrchestratorOptionID)
			scriptMessages(user, messageStep{text: "first message"}, messageStep{text: text})
			rig.inner.queueStop("orchestrator stops")

			got, err := rig.ses.Start(context.Background(), rig.cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			requireUserMessage(t, rig.consultant.blockedRequest(t), msgPtr("first message"), "force-stopped consultation")
			if len(rig.inner.Requests) != 2 {
				t.Fatalf("want the reopened dialogue to lead to one more consultation, got %d answered", len(rig.inner.Requests))
			}
			requireUserMessage(t, rig.inner.Requests[0], nil, "consultation before the request")
			requireUserMessage(t, rig.inner.Requests[1], second, "consultation after the reopened dialogue")
		})
	}
}

func TestSession_UserMessage_DebugLogRecordsTheMessageOncePerCarryingConsultation(t *testing.T) {
	rig, _, _ := inRunMessageRun(t, messageStep{text: multiLineMessage})

	n := 0
	for _, e := range rig.logger.allEvents() {
		if e == domain.EventSessionConsultUserMessage {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want one %q entry (the other consultations carry none), got %d", domain.EventSessionConsultUserMessage, n)
	}
	if got, _ := rig.logger.fieldValue(domain.EventSessionConsultUserMessage, "message"); got != multiLineMessage {
		t.Errorf("want the message text %q in the debug entry, got %q", multiLineMessage, got)
	}
	if got, _ := rig.logger.fieldValue(domain.EventSessionConsultUserMessage, "context"); got != string(domain.ConsultContextRouting) {
		t.Errorf("want context %q, got %q", domain.ConsultContextRouting, got)
	}
}

func TestSession_UserMessage_NoDebugEntryWhenTheStepIsLeftEmpty(t *testing.T) {
	rig, consultant, _ := inRunMessageRun(t, messageStep{text: ""})

	if n := messageQuestions(rig.user); n != 1 {
		t.Fatalf("want the message step asked once and left empty, got %d", n)
	}
	requireUserMessage(t, consultant.Requests[1], nil, "handed-over consultation")
	if rig.logger.eventLogged(domain.EventSessionConsultUserMessage) {
		t.Errorf("want no %q entry without a message", domain.EventSessionConsultUserMessage)
	}
}

func TestSession_UserMessage_OnlyTheCarryingConsultationIsAnnouncedWithIt(t *testing.T) {
	_, _, trace := inRunMessageRun(t, messageStep{text: "please"})

	started := trace.started()
	if len(started) != 3 {
		t.Fatalf("want 3 announced consultations, got %d", len(started))
	}
	for i, want := range []bool{false, true, false} {
		if started[i].HasUserMessage != want {
			t.Errorf("consultation %d: want HasUserMessage=%v, got %v", i+1, want, started[i].HasUserMessage)
		}
	}
}

// A run stopped after the force stop of a message-carrying consultation and
// resumed asks its first consultation without the earlier message.
func TestSession_UserMessage_ResumeAfterAMessageCarryingConsultation_SendsNone(t *testing.T) {
	rig, user, _ := askForceRig(t, domain.ForceStopNextStop, domain.ManualAskOrchestratorOptionID)
	scriptMessages(user, messageStep{text: multiLineMessage})
	first, err := rig.ses.Start(context.Background(), rig.cfg)
	requireRunStatus(t, first, err, domain.RunStopped)
	requireUserMessage(t, rig.consultant.blockedRequest(t), msgPtr(multiLineMessage), "force-stopped consultation")
	resumedConsultant := &scriptedRoutingConsultant{}
	resumedConsultant.queueStop("resumed and done")
	resumed := session.New(session.Deps{
		Harness: rig.f, Store: rig.store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
		Routing: resumedConsultant,
	})
	markResume(&rig.cfg)

	got, err := resumed.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(resumedConsultant.Requests) == 0 {
		t.Fatal("want the resumed run to consult the orchestrator")
	}
	requireUserMessage(t, resumedConsultant.Requests[0], nil, "first consultation after the resume")
}

func TestSession_UserMessage_IsNotStoredInTheRunArtifact(t *testing.T) {
	const marker = "UNIQUE-MARKER-MESSAGE"
	rig, consultant, _ := inRunMessageRun(t, messageStep{text: marker + "\nsecond line"})

	requireUserMessage(t, consultant.Requests[1], msgPtr(marker+"\nsecond line"), "handed-over consultation")
	stored := fmt.Sprintf("%+v %+v", rig.store.state, rig.store.Applied)
	if strings.Contains(stored, marker) {
		t.Errorf("want the message absent from the stored run state and recorded steps, found it in %s", stored)
	}
}
