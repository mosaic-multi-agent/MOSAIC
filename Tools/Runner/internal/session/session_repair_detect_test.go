package session_test

// After every routing consultation the session reads the artifact strictly. A
// consultation that left the artifact damaged - whatever its reply or failure -
// is followed by a repair consultation carrying artifact_damage before any
// further dispatch, and its own reply is not carried out.

import (
	"reflect"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_DamagingConsultation_IsFollowedByARepairConsultationBeforeAnyDispatch(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("repaired and done")},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if rig.cons.calls() != 3 {
		t.Fatalf("want 3 consultations (normal, damaging, repair), got %d", rig.cons.calls())
	}
	report := requireDamageReport(t, rig.cons.request(2))
	if report.LastGoodCopy == "" {
		t.Error("want the last-good copy named: the first step's write left one")
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want the damaging consultation's dispatch discarded, agent-b was invoked %d times", n)
	}
	if got.StopReason != "repaired and done" {
		t.Errorf("want the repair consultation's reply acted on, got stop reason %q", got.StopReason)
	}
}

func TestSession_RepairConsultation_KeepsTheTriggerFieldsOfTheDamagingOne(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("done")},
	}, nil)

	rig.start(t)

	damaging, repair := rig.cons.request(1), rig.cons.request(2)
	requireDamageReport(t, repair)
	if !reflect.DeepEqual(triggerOf(damaging), triggerOf(repair)) {
		t.Errorf("want the repair request to keep the trigger fields %+v, got %+v", triggerOf(damaging), triggerOf(repair))
	}
	if damaging.Context != repair.Context || damaging.OrchestrationArtifact != repair.OrchestrationArtifact {
		t.Errorf("want the same context and artifact path, got %q %q", repair.Context, repair.OrchestrationArtifact)
	}
}

func TestSession_DamagingConsultationThatReplied_Stop_IsRepairedInsteadOfStopping(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: stopReply("give up")},
		{edit: fixLog, reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("finished")},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if got.StopReason != "finished" {
		t.Errorf("want the stop of the damaging consultation discarded, got stop reason %q", got.StopReason)
	}
	requireDamageReport(t, rig.cons.request(2))
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want agent-b dispatched once after the repair, got %d", n)
	}
}

func TestSession_ConsultationFailingAfterDamaging_IsFollowedByARepairConsultation(t *testing.T) {
	failures := map[string]domain.ConsultationFailure{
		"timed out":               domain.ConsultFailTransport,
		"malformed after retries": domain.ConsultFailMalformedJSON,
	}
	for name, failure := range failures {
		t.Run(name, func(t *testing.T) {
			rig := newRepairRig(t, []repairStep{
				firstStep(),
				{edit: damageLog, err: &domain.ConsultationError{Failure: failure, Detail: name}},
				{edit: fixLog, reply: dispatchReply("agent-b", 1)},
				{reply: stopReply("finished")},
			}, nil)

			got := rig.start(t)

			requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
			requireDamageReport(t, rig.cons.request(2))
			if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want %v dispatched with agent-b only after the repair, got %v", want, invokedAgents(rig.f))
			}
		})
	}
}

func TestSession_ConsultationFailingWithoutDamage_EndsTheRunAsBefore(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{err: transportFailure()},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if rig.cons.calls() != 2 {
		t.Errorf("want no repair consultation after a failure that left no damage, got %d consultations", rig.cons.calls())
	}
	for i := 0; i < rig.cons.calls(); i++ {
		requireNoDamageReport(t, rig.cons.request(i), "consultation")
	}
}

func TestSession_ConsultationsWithoutDamage_CarryNoArtifactDamage(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("done")},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if rig.cons.calls() != 3 {
		t.Fatalf("want 3 consultations, got %d", rig.cons.calls())
	}
	for i := 0; i < 3; i++ {
		requireNoDamageReport(t, rig.cons.request(i), "consultation")
	}
}
