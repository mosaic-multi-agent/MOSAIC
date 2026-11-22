package session_test

// Tests of Esc at the first step of a manual dialogue opened by the in-run
// manual-dispatch request: the request is cancelled, the decision is made as if
// it had never been made (the replaced engine dispatch runs, or the orchestrator
// is consulted), the progress screen is told and the cancel is logged. The
// dialogues that are not opened by the request keep the resumable stop.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// requireCancelNotice asserts exactly one cancel notice was sent, at info level.
func requireCancelNotice(t *testing.T, rig *inRunRig) {
	t.Helper()
	got := rig.notes.manualNotices()
	if len(got) != 1 {
		t.Fatalf("want one cancel notice, got %+v", got)
	}
	if got[0].Level != interaction.NoticeInfo || !strings.Contains(got[0].Message, manualCancelFragment) {
		t.Errorf("want an info notice mentioning %q, got %+v", manualCancelFragment, got[0])
	}
}

// requireCancelLogged asserts exactly one cancelled event with the decision.
func requireCancelLogged(t *testing.T, rig *inRunRig, decision string) sessionLogEntry {
	t.Helper()
	entries := rig.cancelledEntries()
	if len(entries) != 1 {
		t.Fatalf("want one %s event, got %d (events %v)", domain.EventSessionRequestCancelled, len(entries), rig.logger.allEvents())
	}
	e := entries[0]
	requireFields(t, e, map[string]string{"request": string(domain.InRunRequestManualDispatch), "decision": decision})
	return e
}

// requireNoCancelTraces asserts nothing was reported as a cancelled request.
func requireNoCancelTraces(t *testing.T, rig *inRunRig) {
	t.Helper()
	if n := len(rig.notes.manualNotices()); n != 0 {
		t.Errorf("want no cancel notice, got %d", n)
	}
	if n := len(rig.cancelledEntries()); n != 0 {
		t.Errorf("want no cancel event, got %d", n)
	}
}

// requireAbandonedStop asserts the resumable stop of a cancelled dialogue.
func requireAbandonedStop(t *testing.T, got domain.RunOutcome, err error) {
	t.Helper()
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	var ce *domain.ConsultationError
	if !errors.As(got.Cause, &ce) || ce.Failure != domain.ConsultFailUserAbandoned {
		t.Errorf("want cause %q, got %v", domain.ConsultFailUserAbandoned, got.Cause)
	}
}

func TestSession_InRunManual_CancelInDialogue_Orchestrated_ConsultsTheOrchestratorForTheSameDecision(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueDispatch("agent-b", "after the cancel", 1)
	consultant.queueStop("done")
	rig := orchestratedRig(t, newManualUser(t), consultant) // unscripted dialogue: Esc
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	var ce *domain.ConsultationError
	if errors.As(got.Cause, &ce) {
		t.Fatalf("want the orchestrator's stop, not a failed dialogue: %v", got.Cause)
	}
	if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want the orchestrator's agent-b after the cancel: %v, got %v", want, invokedAgents(rig.f))
	}
	if consultant.CallCount != 3 {
		t.Fatalf("want the orchestrator consulted at the first decision, after the cancel and after agent-b: got %d calls", consultant.CallCount)
	}
	if req := consultant.Requests[1]; req.UserMessage != nil || req.OfferAskOrchestrator {
		t.Errorf("want the plain consultation of a decision made without the request, got %+v", req)
	}
	if n := rig.user.rowQuestions(); n != 1 {
		t.Errorf("want the dialogue opened once, got %d", n)
	}
	if rig.flag.isPending() {
		t.Error("want the request consumed")
	}
	requireCancelNotice(t, rig)
	requireCancelLogged(t, rig, "routing_consultation")
}

func TestSession_InRunManual_CancelInDialogue_EngineDispatch_RunsTheReplacedStepOnce(t *testing.T) {
	for _, mode := range autoModes {
		for _, withConsultant := range []bool{true, false} {
			name := string(mode) + "/no consultant"
			if withConsultant {
				name = string(mode) + "/with consultant"
			}
			t.Run(name, func(t *testing.T) {
				opts := inRunOptions{fixture: "linear-orch.md", agents: linearAgents}
				consultant := &scriptedRoutingConsultant{}
				if withConsultant {
					opts.consultant = consultant
				}
				rig := newInRunRig(t, newManualUser(t), opts) // unscripted dialogue: Esc
				rig.queueOK("agent-a", 1)
				rig.queueOK("agent-b", 1)
				rig.requestOnceAt("agent-a")

				got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

				requireRunStatus(t, got, err, domain.RunCompleted)
				if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
					t.Errorf("want the replaced agent-b dispatched exactly once: %v, got %v", want, invokedAgents(rig.f))
				}
				if n := len(workflowSteps(rig.store)); n != 2 {
					t.Errorf("want two recorded workflow steps, got %d", n)
				}
				if consultant.CallCount != 0 {
					t.Errorf("want the engine, not the orchestrator, to decide, got %d consultations", consultant.CallCount)
				}
				if n := rig.user.rowQuestions(); n != 1 {
					t.Errorf("want the dialogue opened once and the request not applied again, got %d", n)
				}
				requireCancelNotice(t, rig)
				e := requireCancelLogged(t, rig, "engine_dispatch")
				requireFields(t, e, map[string]string{"row": "2"})
			})
		}
	}
}

func TestSession_InRunManual_CancelInDialogue_Deviation_ConsultsTheOrchestratorAsWithoutTheRequest(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueDispatch("agent-b", "carry on with b", 1)
			rig := orchestratedRig(t, newManualUser(t), consultant) // unscripted dialogue: Esc
			rig.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			_, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want the orchestrator's agent-b after the cancel: %v, got %v", want, invokedAgents(rig.f))
			}
			if consultant.CallCount != 1 {
				t.Errorf("want the orchestrator consulted once for the deviation, got %d", consultant.CallCount)
			}
			if n := rig.user.rowQuestions(); n != 1 {
				t.Errorf("want the dialogue opened once, got %d", n)
			}
			requireCancelNotice(t, rig)
			e := requireCancelLogged(t, rig, "routing_consultation")
			if kind, _ := e.field("kind"); kind == "" {
				t.Error("want the deviation kind in the cancel event")
			}
		})
	}
}

func TestSession_InRunManual_CancelInDialogue_RowQuestionShowsTheEscHint(t *testing.T) {
	user := newManualUser(t)
	rows := cancelRowRecordingDetail(user)
	rig := linearRig(t, user)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !rows.hinted(0) {
		t.Errorf("want the Esc hint on the in-run dialogue, got %q", rows.details)
	}
}

func TestSession_InRunManual_Esc_FailedConsultationFallbackDialogue_KeepsTheResumableStop(t *testing.T) {
	user := newManualUser(t)
	rows := cancelRowRecordingDetail(user)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueError(domain.ConsultFailTransport)
	rig := orchestratedRig(t, user, consultant)
	cfg := baseOrchestratedConfig(rig.orchPath)
	cfg.ManualResolution = true

	got, err := rig.ses.Start(context.Background(), cfg)

	requireAbandonedStop(t, got, err)
	if len(rows.details) != 1 || rows.hinted(0) {
		t.Errorf("want one fallback dialogue without the Esc hint, got %q", rows.details)
	}
	requireNoCancelTraces(t, rig)
}

func TestSession_InRunManual_CancelInDialogue_ThenFailedConsultation_FallbackDialogueIsNotCancellable(t *testing.T) {
	user := newManualUser(t)
	rows := cancelRowRecordingDetail(user)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueError(domain.ConsultFailTransport)
	rig := orchestratedRig(t, user, consultant)
	rig.queueOK("agent-a", 1)
	rig.requestOnceAt("agent-a")
	cfg := baseOrchestratedConfig(rig.orchPath)
	cfg.ManualResolution = true

	got, err := rig.ses.Start(context.Background(), cfg)

	requireAbandonedStop(t, got, err)
	if len(rows.details) != 2 || !rows.hinted(0) || rows.hinted(1) {
		t.Errorf("want the in-run dialogue hinted and the fallback dialogue not, got %q", rows.details)
	}
	if consultant.CallCount != 2 {
		t.Errorf("want the orchestrator consulted after the cancel, got %d calls", consultant.CallCount)
	}
	if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want nothing dispatched after the abandoned fallback: %v, got %v", want, invokedAgents(rig.f))
	}
}

func TestSession_InRunManual_Esc_WithoutRoutingConsultant_KeepsTheResumableStop(t *testing.T) {
	user := newManualUser(t)
	rows := cancelRowRecordingDetail(user)
	rig := newInRunRig(t, user, inRunOptions{fixture: "linear-orch.md", agents: linearAgents})
	rig.queueOK("agent-a", 1)
	rig.flag.request()

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireAbandonedStop(t, got, err)
	if len(rows.details) != 1 || rows.hinted(0) {
		t.Errorf("want one dialogue without the Esc hint, got %q", rows.details)
	}
	if n := len(invokedAgents(rig.f)); n != 0 {
		t.Errorf("want nothing dispatched, got %d invocations", n)
	}
	requireNoCancelTraces(t, rig)
}

func TestSession_InRunManual_Esc_DeviationWithoutRoutingConsultant_KeepsTheResumableStop(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			user := newManualUser(t)
			rows := cancelRowRecordingDetail(user)
			rig := newInRunRig(t, user, inRunOptions{fixture: "linear-orch.md", agents: linearAgents})
			rig.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireAbandonedStop(t, got, err)
			if len(rows.details) != 1 || rows.hinted(0) {
				t.Errorf("want one dialogue without the Esc hint, got %q", rows.details)
			}
			if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want nothing dispatched after the cancel: %v, got %v", want, invokedAgents(rig.f))
			}
			requireNoCancelTraces(t, rig)
		})
	}
}

func TestSession_ManualRouting_Esc_StopScreenOneShot_KeepsTheResumableStopWithoutTheHint(t *testing.T) {
	user := newManualUser(t)
	rows := cancelRowRecordingDetail(user)
	rig := newManualRig(t, user, nil)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireAbandonedStop(t, got, err)
	if len(rows.details) != 1 || strings.Contains(rows.details[0], manualCancelHint) {
		t.Errorf("want one dialogue without the Esc hint, got %q", rows.details)
	}
	if n := len(rig.f.Invocations()); n != 0 {
		t.Errorf("want nothing dispatched, got %d invocations", n)
	}
}
