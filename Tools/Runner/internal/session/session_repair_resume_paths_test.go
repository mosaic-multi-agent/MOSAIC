package session_test

// The repair at run start in its surroundings: before pre-consultation, in a
// Retry or continue of the same process, beside a pending manual dispatch, and
// next to the paths that stay as they were (an artifact that reads strictly, the
// pre-3.0 layout and run-identity refusals, a damaged frontmatter).

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

func TestSession_ResumeWithDamage_RepairsBeforePreConsultation(t *testing.T) {
	var pre *scriptedPreConsultant
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
		// After the repair's dispatch the fixture's remaining step is already
		// done, so the run asks the orchestrator once more; it stops there.
		{reply: stopReply("done")},
	}, func(r *repairRig) {
		withDamagedRun(t, startOpts{auto: true})(r)
		pre = &scriptedPreConsultant{}
		r.deps.PreConsult = &tracedPreConsultant{inner: pre, tr: &r.trace.consultTrace}
		r.deps.Routing = &tracedRouting{inner: r.cons, tr: &r.trace.consultTrace, evt: evCall}
	})

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	order := rig.trace.without(evStart, evEnd, evNotice, evRepairJudged)
	if len(order) < 2 || order[0] != evCall || order[1] != evPreCall {
		t.Fatalf("want the repair consultation before the pre-consultation, got %v", order)
	}
	if !pre.Called {
		t.Error("want the pre-consultation made after the repair, it was not called")
	}
	requireDamageReport(t, rig.cons.request(0))
	for i := 1; i < rig.cons.calls(); i++ {
		requireNoDamageReport(t, rig.cons.request(i), "consultation after the repair")
	}
	// The repair names agent-a; a resume position worked out from the repaired
	// log would pick agent-b (the step after row 83), so the first step shows
	// which one was used.
	if got := invokedInstances(rig.f); len(got) == 0 || got[0] != "agent-a#84" {
		t.Errorf("want the repair's dispatch carried out first (agent-a#84), got %v", got)
	}
}

func TestSession_ResumeWithDamageThatCannotBeRepaired_MakesNoPreConsultation(t *testing.T) {
	var pre *scriptedPreConsultant
	rig := newRepairRig(t, []repairStep{
		{err: transportFailure()},
	}, func(r *repairRig) {
		withDamagedRun(t, startOpts{auto: true})(r)
		pre = &scriptedPreConsultant{}
		r.deps.PreConsult = pre
	})

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	if pre.Called {
		t.Error("want no pre-consultation cost spent on a damaged artifact")
	}
}

func TestSession_RetryOfAFailedStartRepair_InTheSameProcess_RepairsTheSameWay(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{err: transportFailure()},
		{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("done")},
	}, withDamagedRun(t, startOpts{}))
	first := rig.start(t)
	if first.Status != domain.RunFailed {
		t.Fatalf("test setup: want the first start to fail on the repair, got %q (%s)", first.Status, first.Message)
	}

	// Retry rebuilds the session in the same process over the same dependencies.
	got, err := session.New(rig.deps).Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	req := rig.cons.request(1)
	requireDamageReport(t, req)
	requireStartTriggersNull(t, req)
	if want := []string{"agent-a#84"}; !slices.Equal(invokedInstances(rig.f), want) {
		t.Errorf("want the accepted dispatch carried out by the retry, got %v", invokedInstances(rig.f))
	}
}

func TestSession_ResumeOfAnArtifactThatReadsStrictly_MakesNoRepairConsultation(t *testing.T) {
	// An earlier, interrupted repair left this artifact valid; it differs from
	// the last-good copy and is accepted as it is.
	rig := newRepairRig(t, []repairStep{
		{reply: stopReply("done")},
	}, withDamagedRun(t, startOpts{edit: func(doc string) string {
		return strings.Replace(repairIncident(t, doc), "Step 5 done", "Step 5 changed", 1)
	}}))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	for i := 0; i < rig.cons.calls(); i++ {
		requireNoDamageReport(t, rig.cons.request(i), "consultation")
	}
	if names := rig.cons.damagedCopyNames(); len(names) != 0 {
		t.Errorf("want no damaged copy saved for a valid artifact, got %v", names)
	}
}

func TestSession_ResumeWithPendingManualDispatch_RepairGoesToTheOrchestratorNotTheManualResolver(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncident, reply: stopReply("done")},
	}, func(r *repairRig) {
		withDamagedRun(t, startOpts{})(r)
		r.cfg.ManualResolution = true
		r.flag.request()
	})

	got := rig.start(t)

	requireDamageReport(t, rig.cons.request(0))
	for _, req := range rig.manual.Requests {
		requireNoDamageReport(t, req, "manual resolver")
	}
	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want the pending manual dispatch not carried out before the repair verdict, got %v", invokedInstances(rig.f))
	}
}

func TestSession_ResumeWithKeptRefusal_IsRefusedWithoutARepairConsultationEvenWhenAlsoDamaged(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(string) string
		check func(*testing.T, domain.RunOutcome)
	}{
		{
			name: "pre-3.0 layout",
			edit: func(doc string) string {
				doc = strings.Replace(doc, "| Iteration |", "| Stage |", 1)
				return strings.Replace(doc, "  iteration: null", "  stage: null", 1)
			},
			check: func(t *testing.T, got domain.RunOutcome) {
				if !errors.Is(got.Cause, artifact.ErrPre30Layout) {
					t.Errorf("want the pre-3.0 layout refusal as the cause, got %v", got.Cause)
				}
			},
		},
		{
			name: "malformed run identity",
			edit: func(doc string) string { return strings.Replace(doc, "run_id: "+testRunID, "run_id: not-a-run-id", 1) },
			check: func(t *testing.T, got domain.RunOutcome) {
				if !strings.Contains(got.Message, "run_id") || !strings.Contains(got.Message, "malformed") {
					t.Errorf("want the refusal to name the malformed run_id, got %q", got.Message)
				}
				var damage *domain.ArtifactDamageError
				if errors.As(got.Cause, &damage) || strings.Contains(got.Message, "damaged") {
					t.Errorf("want the identity refusal alone, not the damage, got %q (cause %v)", got.Message, got.Cause)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rig := newRepairRig(t, []repairStep{{reply: stopReply("must not be asked")}},
				withDamagedRun(t, startOpts{edit: tc.edit}))

			got := rig.start(t)

			requireRefused(t, got, nil)
			tc.check(t, got)
			if rig.cons.calls() != 0 || len(rig.f.Invocations()) != 0 {
				t.Errorf("want no consultation and no dispatch, got %d and %d", rig.cons.calls(), len(rig.f.Invocations()))
			}
		})
	}
}

func TestSession_ResumeWithDamagedFrontmatter_IsRefusedWithTheDamageAndWithoutARepairConsultation(t *testing.T) {
	// The frontends supply an empty workflow for such a run when no last-good
	// copy records it; the session must name the damage rather than the missing
	// workflow. With a workflow in the configuration the run is repaired (see
	// session_repair_resume_frontmatter_test.go).
	rig := newRepairRig(t, []repairStep{{reply: stopReply("must not be asked")}}, func(r *repairRig) {
		withDamagedRun(t, startOpts{edit: func(doc string) string {
			return replaceOnce(t, doc, "type: orchestration-artifact\n", "")
		}})(r)
		r.cfg.WorkflowID = ""
	})

	got := rig.start(t)

	requireRefused(t, got, nil)
	var damage *domain.ArtifactDamageError
	if !errors.As(got.Cause, &damage) {
		t.Errorf("want the damage error as the cause, got %T (%v)", got.Cause, got.Cause)
	}
	if !strings.Contains(strings.ToLower(got.Message), "damage") {
		t.Errorf("want the message to name the damage, got %q", got.Message)
	}
	if rig.cons.calls() != 0 {
		t.Errorf("want no repair consultation without a routing table, got %d", rig.cons.calls())
	}
}
