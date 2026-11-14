package session_test

// The repair loop: the Runner verifies the artifact after each repair
// consultation. An accepted repair continues the run with the returned reply, a
// rejected one is discarded and asked again once, and a second rejection stops
// the run with the damage report and the copy paths, the artifact untouched.

import (
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_AcceptedRepairWithDispatch_IsCarriedOutWithTheNextSequenceAfterTheRepairedGlobalSequence(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndSetSequence(5), reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("finished")},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if want := []string{"agent-a#1", "agent-b#6"}; !slices.Equal(invokedInstances(rig.f), want) {
		t.Errorf("want the accepted dispatch numbered after the repaired global_sequence %v, got %v", want, invokedInstances(rig.f))
	}
	state := readLog(t, rig.run.artifactPath)
	if len(state.ExecutionLog) != 2 || state.ExecutionLog[1].Seq != 6 {
		t.Errorf("want the dispatched step recorded as row 6, got %+v", state.ExecutionLog)
	}
	if n := len(rig.user.asked); n != 0 {
		t.Errorf("want the run to continue without asking the user, got %d questions", n)
	}
}

func TestSession_AcceptedRepairWithStop_IsAConsultantStop(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("workflow is done")},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if got.StopReason != "workflow is done" {
		t.Errorf("want the repair reply's stop reason, got %q", got.StopReason)
	}
	if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want no dispatch after the stop, got %v", invokedAgents(rig.f))
	}
}

func TestSession_RepairChangingARow_IsRejectedItsDispatchDiscardedAndAskedAgainWithAFreshReport(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRewriteRow, reply: dispatchReply("agent-a", 0)},
		{edit: fixLogAndRestoreRow, reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("finished")},
	}, nil)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(rig.f.Invocations(), "agent-a"); n != 1 {
		t.Errorf("want the rejected repair's valid dispatch discarded (agent-a invoked once), got %d", n)
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the second repair's dispatch carried out once, got %d", n)
	}
	first, second := requireDamageReport(t, rig.cons.request(2)), requireDamageReport(t, rig.cons.request(3))
	if slices.Equal(problemStrings(first.Problems), problemStrings(second.Problems)) {
		t.Errorf("want a fresh report for the second repair consultation, got the same problems %v", problemStrings(first.Problems))
	}
	if !strings.Contains(strings.Join(problemStrings(second.Problems), "\n"), "ExecutionLog") {
		t.Errorf("want the second report to name the Execution Log row problem, got %v", problemStrings(second.Problems))
	}
}

func TestSession_SecondRejectedRepair_StopsWithTheReportAndCopyPathsAndLeavesTheArtifact(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLogAndRewriteRow, reply: dispatchReply("agent-b", 1)},
		{reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("must not be reached")},
	}, nil)

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedRejected || unrepaired.Consultations != 2 {
		t.Errorf("want reason rejected after 2 consultations, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if rig.cons.calls() != 4 {
		t.Errorf("want exactly 4 consultations (normal, damaging, 2 repairs), got %d", rig.cons.calls())
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want no dispatch from a rejected repair, got %d agent-b invocations", n)
	}
	if unrepaired.DamagedCopy == "" || unrepaired.LastGoodCopy == "" || unrepaired.Artifact != rig.run.artifactPath {
		t.Errorf("want the artifact, damaged copy and last-good copy paths, got %+v", unrepaired)
	}
	for _, path := range []string{unrepaired.DamagedCopy, unrepaired.LastGoodCopy, rig.run.artifactPath} {
		if !strings.Contains(got.Message, path) {
			t.Errorf("want the outcome message to name %q, got %q", path, got.Message)
		}
	}
	if got, want := readText(t, rig.run.artifactPath), rig.cons.leftAfter(3); got != want {
		t.Error("want the artifact exactly as the orchestrator last wrote it")
	}
	if got, want := readText(t, unrepaired.DamagedCopy), rig.cons.leftAfter(1); got != want {
		t.Error("want the original damaged copy kept, holding the artifact as the damaging consultation left it")
	}
}

func TestSession_RepairWithoutLastGoodCopy_IsStillMadeAndNamesNone(t *testing.T) {
	rig := newRepairRig(t, nil, func(r *repairRig) {
		r.cons.steps = []repairStep{
			firstStep(),
			{edit: damageAndDropLastGood(r.evidDir), reply: dispatchReply("agent-b", 1)},
			{edit: fixLog, reply: dispatchReply("agent-b", 1)},
			{reply: stopReply("finished")},
		}
	})

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if report := requireDamageReport(t, rig.cons.request(2)); report.LastGoodCopy != "" {
		t.Errorf("want no last-good path in the report, got %q", report.LastGoodCopy)
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the repair accepted on the other checks (agent-b dispatched once), got %d", n)
	}
}

func TestSession_DamagedCopyIsSavedBeforeTheFirstRepairConsultation(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("done")},
	}, nil)

	rig.start(t)

	if got := rig.cons.copiesAtCall(1); len(got) != 0 {
		t.Errorf("want no damaged copy before the damage exists, got %v", got)
	}
	copies := rig.cons.copiesAtCall(2)
	if len(copies) != 1 {
		t.Fatalf("want the damaged copy on disk when the repair consultation begins, got %v", copies)
	}
	if got, want := readText(t, rig.evidDir+"/"+copies[0]), rig.cons.leftAfter(1); got != want {
		t.Error("want the copy to hold the artifact as the damaging consultation left it")
	}
}

func problemStrings(problems []domain.ArtifactDamageProblem) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = p.String()
	}
	return out
}
