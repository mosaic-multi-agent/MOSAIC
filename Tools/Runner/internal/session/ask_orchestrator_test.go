package session_test

// Tests for handing a routing decision to the orchestrator from the manual
// dialogue: the option is offered only on user-requested openings with a
// routing consultant wired; choosing it runs an announced consultation for the
// same decision and applies its answer like any orchestrator-routed answer.

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func askRig(t *testing.T, consultant *scriptedRoutingConsultant, answers ...string) (*inRunRig, *rowProbe, *consultTrace) {
	t.Helper()
	user := newManualUser(t)
	probe := scriptRow(user, answers...)
	trace := &consultTrace{}
	rig := newInRunRig(t, user, inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents, consultant: consultant, consultations: trace,
	})
	return rig, probe, trace
}

func TestSession_AskOrchestrator_InRunRequest_RunsAnnouncedConsultationAndAppliesItsDispatch(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueDispatch("agent-b", "orchestrator's choice", 1)
	consultant.queueStop("all done")
	rig, probe, trace := askRig(t, consultant, domain.ManualAskOrchestratorOptionID)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []bool{true}; !slices.Equal(probe.offered, want) {
		t.Errorf("want the option offered at the user-requested dialogue: %v, got %v", want, probe.offered)
	}
	if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Fatalf("want the orchestrator's dispatch applied: %v, got %v", want, invokedAgents(rig.f))
	}
	if got := invocationTask(rig, 1); got != "orchestrator's choice" {
		t.Errorf("want the orchestrator's task on the dispatch, got %q", got)
	}
	if consultant.CallCount != 3 {
		t.Errorf("want the orchestrator asked for the first decision, the handed-over one and the last, got %d calls", consultant.CallCount)
	}
	if s, e := trace.count(evStart), trace.count(evEnd); s != 3 || e != 3 {
		t.Errorf("want every consultation announced (3 starts, 3 ends), got %d and %d", s, e)
	}
	if !onlyRowAsked(rig.user) {
		t.Errorf("want no further dialogue step after choosing the option, got %v", rig.user.asked)
	}
	requireSameDecisionRequest(t, consultant)
}

// requireSameDecisionRequest checks that the handed-over consultation (second
// call) is an ordinary routing consultation for the decision after agent-a: the
// same artifact and schedules as the first consultation, the triggering agent's
// status message, and no manual-dialogue-only offer leaking into the request.
func requireSameDecisionRequest(t *testing.T, c *scriptedRoutingConsultant) {
	t.Helper()
	if len(c.Requests) < 2 {
		t.Fatalf("want at least two consultations, got %d", len(c.Requests))
	}
	first, handed := c.Requests[0], c.Requests[1]
	if handed.Context != domain.ConsultContextRouting {
		t.Errorf("want a routing consultation, got context %q", handed.Context)
	}
	if handed.OrchestrationArtifact == "" || handed.OrchestrationArtifact != first.OrchestrationArtifact {
		t.Errorf("want the run's orchestration artifact %q, got %q", first.OrchestrationArtifact, handed.OrchestrationArtifact)
	}
	if handed.OfferAskOrchestrator {
		t.Error("want the handed-over request not to carry the dialogue-only offer")
	}
	if first.LastStatusMessage != nil {
		t.Errorf("want the first decision to have no triggering message, got %q", *first.LastStatusMessage)
	}
	if handed.LastStatusMessage == nil || *handed.LastStatusMessage != "done" {
		t.Errorf("want the decision after agent-a to carry its status message, got %v", handed.LastStatusMessage)
	}
	if !reflect.DeepEqual(handed.Schedules, first.Schedules) {
		t.Errorf("want the same schedules as the first consultation: %v, got %v", first.Schedules, handed.Schedules)
	}
}

func TestSession_AskOrchestrator_OrchestratorAnswersStop_StopsTheRun(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueStop("orchestrator says enough")
	rig, _, _ := askRig(t, consultant, domain.ManualAskOrchestratorOptionID)
	rig.queueOK("agent-a", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if !strings.Contains(got.Message, "orchestrator says enough") {
		t.Errorf("want the orchestrator's stop reason in the message, got %q", got.Message)
	}
	if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want nothing dispatched after the stop: %v, got %v", want, invokedAgents(rig.f))
	}
	if consultant.CallCount != 2 {
		t.Errorf("want two orchestrator consultations, got %d", consultant.CallCount)
	}
}

func TestSession_AskOrchestrator_ConsultationFails_TakesTheConsultantStopPath(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueError(domain.ConsultFailTransport)
	rig, probe, trace := askRig(t, consultant, domain.ManualAskOrchestratorOptionID)
	rig.queueOK("agent-a", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if !strings.Contains(got.Message, "consultation failed") {
		t.Errorf("want the consultation-failed message, got %q", got.Message)
	}
	if len(probe.offered) != 1 {
		t.Errorf("want no manual fallback without ManualResolution (dialogue opened once), got %v", probe.offered)
	}
	if s, e := trace.count(evStart), trace.count(evEnd); s != 2 || e != 2 {
		t.Errorf("want both consultations announced and ended, got %d and %d", s, e)
	}
}

// A timeout surfaces as a transport failure (harness error, timeout, empty
// reply); the other failure classes take the same path.
func TestSession_AskOrchestrator_ConsultationFailureClasses_TakeTheConsultantStopPath(t *testing.T) {
	for _, failure := range []domain.ConsultationFailure{
		domain.ConsultFailTransport, domain.ConsultFailNoInstruction, domain.ConsultFailMalformedJSON,
	} {
		t.Run(string(failure), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueDispatch("agent-a", "start", 0)
			consultant.queueError(failure)
			rig, probe, _ := askRig(t, consultant, domain.ManualAskOrchestratorOptionID)
			rig.queueOK("agent-a", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if !strings.Contains(got.Message, "consultation failed") {
				t.Errorf("want the consultation-failed message, got %q", got.Message)
			}
			if len(probe.offered) != 1 {
				t.Errorf("want no manual fallback, got dialogue openings %v", probe.offered)
			}
		})
	}
}

func TestSession_AskOrchestrator_ConsultationFailsWithManualResolution_FallbackDoesNotOfferTheOption(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueError(domain.ConsultFailTransport)
	rig, probe, _ := askRig(t, consultant, domain.ManualAskOrchestratorOptionID, domain.ManualStopOptionID)
	rig.queueOK("agent-a", 1)
	rig.requestOnceAt("agent-a")
	cfg := baseOrchestratedConfig(rig.orchPath)
	cfg.ManualResolution = true

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []bool{true, false}; !slices.Equal(probe.offered, want) {
		t.Errorf("want the option offered at the user-requested opening only, not in the fallback: %v, got %v", want, probe.offered)
	}
}

func TestSession_ManualFallbackAfterFailedConsultation_DoesNotOfferTheOption(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueError(domain.ConsultFailTransport)
	rig, probe, _ := askRig(t, consultant, domain.ManualStopOptionID)
	cfg := baseOrchestratedConfig(rig.orchPath)
	cfg.ManualResolution = true

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []bool{false}; !slices.Equal(probe.offered, want) {
		t.Errorf("want the fallback dialogue opened once without the option: %v, got %v", want, probe.offered)
	}
	if consultant.CallCount != 1 {
		t.Errorf("want the failed consultation not repeated, got %d calls", consultant.CallCount)
	}
}

func TestSession_AskOrchestrator_InRunRequestDivertingAnEngineDispatch_OffersAndRunsIt(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("orchestrator takes over and stops")
			rig, probe, trace := askRig(t, consultant, domain.ManualAskOrchestratorOptionID)
			rig.queueOK("agent-a", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if want := []bool{true}; !slices.Equal(probe.offered, want) {
				t.Errorf("want the option offered: %v, got %v", want, probe.offered)
			}
			if consultant.CallCount != 1 || trace.count(evStart) != 1 || trace.count(evEnd) != 1 {
				t.Errorf("want one announced orchestrator consultation, got %d calls, %d starts, %d ends",
					consultant.CallCount, trace.count(evStart), trace.count(evEnd))
			}
			if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want the engine's agent-b not dispatched: %v, got %v", want, invokedAgents(rig.f))
			}
		})
	}
}

func TestSession_AskOrchestrator_NoRoutingConsultantWired_OptionIsNotOffered(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			user := newManualUser(t)
			probe := scriptRow(user, "1")
			user.say(domain.QuestionManualTask, "redo the plan")
			user.acceptDefaultsPlus(domain.QuestionManualInputs)
			user.acceptDefaultsPlus(domain.QuestionManualOutputs)
			user.say(domain.QuestionManualConstraints, "")
			user.pickOne(domain.QuestionManualHITL, domain.ManualHITLDefault)
			rig := linearRig(t, user)
			rig.queueOK("agent-a", 2)
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if want := []bool{false}; !slices.Equal(probe.offered, want) {
				t.Errorf("want the option not offered without a routing consultant: %v, got %v", want, probe.offered)
			}
		})
	}
}

func TestSession_AskOrchestrator_StopScreenManualDispatch_OffersAndRunsIt(t *testing.T) {
	user := newManualUser(t)
	probe := scriptRow(user, domain.ManualAskOrchestratorOptionID)
	trace := &consultTrace{}
	rig := newManualRig(t, user, nil, trace)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []bool{true}; !slices.Equal(probe.offered, want) {
		t.Errorf("want the option offered on the stop screen's manual dispatch: %v, got %v", want, probe.offered)
	}
	if rig.consultant.CallCount != 1 {
		t.Fatalf("want the orchestrator consulted once for the handed-over decision, got %d", rig.consultant.CallCount)
	}
	if !strings.Contains(got.Message, "done") {
		t.Errorf("want the orchestrator's stop applied, got %q", got.Message)
	}
	if trace.count(evStart) != 1 || trace.count(evEnd) != 1 {
		t.Errorf("want the consultation announced once, got %d starts, %d ends", trace.count(evStart), trace.count(evEnd))
	}
	if req := rig.consultant.Requests[0]; req.Context != domain.ConsultContextRouting || req.OrchestrationArtifact == "" || req.OfferAskOrchestrator {
		t.Errorf("want an ordinary routing request for the stop-screen decision, got %+v", req)
	}
}
