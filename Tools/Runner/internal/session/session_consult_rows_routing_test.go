package session_test

// Every orchestrator-agent routing consultation announces its start (with the
// deviation kind where one exists) and its end, in every mode and for every
// kind of consultation, bracketing the consultant call.

import (
	"context"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func TestConsultAnnounce_Orchestrated_EveryConsultationIsBracketedAndEndsBeforeTheDispatch(t *testing.T) {
	rig := newLinearConsultRig(t)
	rig.consult.queueDispatch("agent-a", "first", 0)
	rig.consult.queueDispatch("agent-b", "second", 1)
	rig.consult.queueStop("done")
	successAgentEntries(rig.f, "agent-a", 1)
	successAgentEntries(rig.f, "agent-b", 1)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireBracketed(t, rig.tr, evCall, 3)
	requireBalanced(t, rig.tr)
	requireKindsMatch(t, rig.tr, rig.requests())
	requireKinds(t, rig.tr, "", "", "")
	want := []string{evStart, evCall, evEnd, evInvokePfx + "agent-a", evStart, evCall, evEnd, evInvokePfx + "agent-b",
		evStart, evCall, evEnd}
	if events := rig.tr.without(evNotice); !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestConsultAnnounce_Orchestrated_NoNoticeFallsInsideAConsultation(t *testing.T) {
	rig := newLinearConsultRig(t)
	rig.consult.queueDispatch("agent-a", "first", 0)
	rig.consult.queueStop("done")
	successAgentEntries(rig.f, "agent-a", 1)

	rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath)) //nolint:errcheck

	if rig.tr.count(evStart) != 2 {
		t.Fatalf("scenario did not run as intended: want 2 consultations, got %v", rig.tr.snapshot())
	}
	requireNotOpenAt(t, rig.tr, evNotice, 1)
}

func TestConsultAnnounce_Deviation_HarnessErrorBudgetExhausted_AnnouncesTheKind(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newLinearConsultRig(t)
			rig.consult.queueStop("giving up")
			queueRepeated(rig.f, "agent-a", engine.E501AttemptLimit, failEntry("harness: subprocess timed out"))

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			requireBracketed(t, rig.tr, evCall, 1)
			requireBalanced(t, rig.tr)
			requireKindsMatch(t, rig.tr, rig.requests())
			if got := rig.tr.started()[0].Kind; got == "" {
				t.Error("want the deviation kind announced for the post-exhaustion consultation, got none")
			}
		})
	}
}

func TestConsultAnnounce_Deviation_NonSuccessStatus_AnnouncesTheRequestsKind(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newLinearConsultRig(t)
			rig.consult.queueStop("stop")
			rig.f.Queue("agent-a", failingAgentEntry(domain.ErrorDEPENDENCY_MISSING))

			rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode)) //nolint:errcheck

			requireBracketed(t, rig.tr, evCall, 1)
			requireBalanced(t, rig.tr)
			requireKindsMatch(t, rig.tr, rig.requests())
			if got := rig.tr.started()[0].Kind; got == "" {
				t.Error("want the deviation kind announced for a consultation caused by a deviation, got none")
			}
		})
	}
}

func TestConsultAnnounce_ReviewClass_ConsultationAfterTriggerEvaluation_IsBracketedWithoutKind(t *testing.T) {
	agents := []string{"agent-a", "agent-b", "review-agent-a", "review-agent-b"}
	rig := newConsultRig(t, "review-class-orch.md", agents, nil)
	rig.consult.queueStop("review observed, stopping run")
	rig.f.Queue("agent-a", successEntry("workflow step done"))
	rig.f.Queue("review-agent-a", successEntry("review-a complete"))
	rig.f.Queue("review-agent-b", successEntry("review-b complete"))
	successAgentEntries(rig.f, "agent-b", 1)

	got, err := rig.ses.Start(context.Background(), baseLinearConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
	requireKindsMatch(t, rig.tr, rig.requests())
	requireKinds(t, rig.tr, "")
	events := rig.tr.without(evNotice)
	if i := slices.Index(events, evStart); i < 1 || events[i-1] != evInvokePfx+"review-agent-b" {
		t.Errorf("want the consultation right after the last review agent ran, got %v", events)
	}
}

func TestConsultAnnounce_GuardEscalation_EveryConsultationInTheCycleIsBracketed(t *testing.T) {
	consultant := &sameDispatchConsultant{agent: "agent-b", rowIndex: 1, safetyCap: consultantSafetyCap}
	rig := newLinearConsultRig(t)
	rig.routing.inner = consultant
	queueRepeated(rig.f, "agent-b", 12, failingAgentEntry(domain.ErrorDEPENDENCY_MISSING))

	rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath)) //nolint:errcheck

	if consultant.calls < 2 {
		t.Fatalf("scenario did not run as intended: want the guard cycle to consult repeatedly, got %d", consultant.calls)
	}
	requireBracketed(t, rig.tr, evCall, consultant.calls)
	requireBalanced(t, rig.tr)
	requireKindsMatch(t, rig.tr, rig.requests())
}

func TestConsultAnnounce_ScheduleDefect_ConsultationIsBracketedWithItsKind(t *testing.T) {
	tr := &consultTrace{}
	rig := newBlockRigObserved(t, "audit-plan-block-orch.md", nil, nil, tr, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(3)
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if rig.consult.CallCount != 2 {
		t.Fatalf("scenario did not run as intended: want 2 consultations, got %d", rig.consult.CallCount)
	}
	requireBalanced(t, tr)
	requireKindsMatch(t, tr, rig.consult.Requests)
	requireKinds(t, tr, "", domain.DeviationScheduleDefect)
}

func TestConsultAnnounce_FinishedWork_ConsultationIsBracketedWithItsKind(t *testing.T) {
	tr := &consultTrace{}
	r := newQualifiedRun(t, qualifiedOptions{implFindings: "build-review(tests)", consultations: tr})
	r.queue("planner", "planner#1", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#2", domain.StatusSUCCESS)
	r.queue("implementer", "implementer#3", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#4", domain.StatusCOMPLETED_NEEDS_ACTION)
	r.queueWithMessage("build-review", "build-review#5", domain.StatusSUCCESS, "reviewed")
	r.queue("implementer", "implementer#6", domain.StatusSUCCESS)
	r.queue("build-review", "build-review#7", domain.StatusSUCCESS)
	r.consultant.queueDispatch("implementer", "rerun the implementer row", 2)

	got, err := r.ses.Start(context.Background(), r.config(domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireBalanced(t, tr)
	requireKindsMatch(t, tr, r.consultant.Requests)
	requireKinds(t, tr, domain.DeviationFinishedWork)
}
