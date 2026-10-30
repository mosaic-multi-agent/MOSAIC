package analysis_test

// Tests for the compaction kinds of an actor.
//
// Rules under test:
//   - A detected drop preceded by one or more compaction calls of the same
//     actor (since its previous compaction) is a harness compaction. It carries
//     the number and summed cost of those calls and raises no "no logged
//     compaction near it" finding.
//   - A detected drop with no compaction call of its own actor stays detected
//     only and still raises the finding; compaction calls of another actor
//     never make it a harness compaction.
//   - A drop matched to a logged compaction is shown with it, not as a harness
//     compaction.
//   - Compaction calls that no later compaction follows are unapplied attempts,
//     with count and cost; compaction calls with an unknown time are unapplied.
//   - Prices come from the flat table: 3000 nanodollars per input token.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const hcPerToken = 3000

// hcCompactionCall is a compaction call of the given input size.
func hcCompactionCall(id string, sec int, in int64) domain.InvocationUsage {
	return ctxKinded(ctxCall(id, sec, in, 0, 0, 0), domain.CallCompaction, "summarizeConversationHistory")
}

// hcOrch is an orchestrator spec made of the given calls.
func hcOrch(calls ...domain.InvocationUsage) ctxRunSpec {
	return ctxRunSpec{orchUsage: calls}
}

// hcHarnessSpec: 100k, two compaction calls, then a 60% drop to 40k.
func hcHarnessSpec() ctxRunSpec {
	return hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 30, 2_000),
		hcCompactionCall("c2", 40, 3_000),
		ctxCall("m2", 60, 40_000, 0, 0, 0),
	)
}

func hcRequireOneDrop(t *testing.T, a domain.ActorContext) domain.CompactionMark {
	t.Helper()
	marks := detDetected(a)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	return marks[0]
}

func TestCompactionKinds_DropAfterCompactionCallsOfTheSameActor_IsAHarnessCompaction(t *testing.T) {
	run, _ := detPrice(t, hcHarnessSpec())

	m := hcRequireOneDrop(t, run.Context.Orchestrator)

	if !m.Harness || m.MatchedLogged {
		t.Errorf("mark harness/matched = %v/%v, want true/false", m.Harness, m.MatchedLogged)
	}
	if m.CallCount != 2 {
		t.Errorf("harness compaction calls = %d, want 2", m.CallCount)
	}
	assertMoneyKnown(t, "harness compaction call cost", m.CallCost, 5_000*hcPerToken)
}

func TestCompactionKinds_HarnessCompaction_RaisesNoDetectedButNotLoggedFinding(t *testing.T) {
	run, findings := detPrice(t, hcHarnessSpec())

	hcRequireOneDrop(t, run.Context.Orchestrator)
	detRequireMismatches(t, findings, 0)
}

func TestCompactionKinds_DropWithoutCompactionCalls_IsDetectedOnlyAndStillRaisesTheFinding(t *testing.T) {
	run, findings := detPrice(t, hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		ctxCall("m2", 60, 40_000, 0, 0, 0),
	))

	m := hcRequireOneDrop(t, run.Context.Orchestrator)

	if m.Harness || m.MatchedLogged || m.CallCount != 0 {
		t.Errorf("mark = harness %v matched %v calls %d, want a detected-only drop", m.Harness, m.MatchedLogged, m.CallCount)
	}
	detRequireMismatches(t, findings, 1)
}

func TestCompactionKinds_CompactionCallsOfAnotherActor_DoNotMakeADropAHarnessCompaction(t *testing.T) {
	spec := hcOrch(
		ctxCall("om1", 0, 50_000, 0, 0, 0),
		hcCompactionCall("oc1", 30, 2_000),
		ctxCall("om2", 60, 50_000, 0, 0, 0),
	)
	spec.agents = []ctxAgent{detAgent("worker#1", detPt{0, 100_000}, detPt{60, 40_000})}

	run, findings := detPrice(t, spec)

	m := hcRequireOneDrop(t, ctxRequireAgent(t, run, "worker#1"))
	if m.Harness || m.CallCount != 0 {
		t.Errorf("agent drop harness = %v with %d calls, want detected only", m.Harness, m.CallCount)
	}
	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], detAgentActor("worker#1"))
	if n := ctxRequireAgent(t, run, "worker#1").CompactionCalls.Count; n != 0 {
		t.Errorf("agent compaction calls = %d, want 0", n)
	}
}

func TestCompactionKinds_OnlyCallsSincePreviousCompactionBelongToTheNextHarnessCompaction(t *testing.T) {
	run, _ := detPrice(t, hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 10, 1_000),
		ctxCall("m2", 60, 30_000, 0, 0, 0),
		ctxCall("m3", 100, 100_000, 0, 0, 0),
		hcCompactionCall("c2", 110, 4_000),
		ctxCall("m4", 160, 30_000, 0, 0, 0),
	))

	marks := detDetected(run.Context.Orchestrator)

	if len(marks) != 2 {
		t.Fatalf("detected marks = %d, want 2", len(marks))
	}
	for i, wantNanos := range []int64{1_000 * hcPerToken, 4_000 * hcPerToken} {
		if !marks[i].Harness || marks[i].CallCount != 1 {
			t.Errorf("mark %d harness/calls = %v/%d, want true/1", i, marks[i].Harness, marks[i].CallCount)
		}
		assertMoneyKnown(t, "mark call cost", marks[i].CallCost, wantNanos)
	}
}

func TestCompactionKinds_DropMatchedToALoggedCompaction_IsShownWithItNotAsHarness(t *testing.T) {
	spec := hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 30, 2_000),
		ctxCall("m2", 60, 40_000, 0, 0, 0),
	)
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 50)}

	run, findings := detPrice(t, spec)

	m := hcRequireOneDrop(t, run.Context.Orchestrator)
	if !m.MatchedLogged || m.Harness {
		t.Errorf("mark matched/harness = %v/%v, want true/false", m.MatchedLogged, m.Harness)
	}
	detRequireMismatches(t, findings, 0)
	if n := run.Context.Orchestrator.CompactionCalls.UnappliedCount; n != 0 {
		t.Errorf("unapplied attempts = %d, want 0 (the call precedes a compaction)", n)
	}
}

func TestCompactionKinds_CompactionCallsWithNoLaterDrop_AreUnappliedAttemptsWithCountAndCost(t *testing.T) {
	run, _ := detPrice(t, hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 30, 2_000),
		hcCompactionCall("c2", 40, 3_000),
		ctxCall("m2", 60, 95_000, 0, 0, 0),
	))

	orch := run.Context.Orchestrator

	if n := len(detDetected(orch)); n != 0 {
		t.Fatalf("detected marks = %d, want 0", n)
	}
	got := orch.CompactionCalls
	if got.Count != 2 || got.UnappliedCount != 2 {
		t.Errorf("compaction calls count/unapplied = %d/%d, want 2/2", got.Count, got.UnappliedCount)
	}
	assertMoneyKnown(t, "compaction call cost", got.Cost, 5_000*hcPerToken)
	assertMoneyKnown(t, "unapplied cost", got.UnappliedCost, 5_000*hcPerToken)
	for _, c := range orch.Calls {
		if c.Unapplied != (c.Kind == domain.CallCompaction) {
			t.Errorf("call %s unapplied = %v, want %v", c.RecordID, c.Unapplied, c.Kind == domain.CallCompaction)
		}
	}
}

func TestCompactionKinds_CallAfterTheLastCompaction_IsUnappliedWhileEarlierOnesAreApplied(t *testing.T) {
	run, _ := detPrice(t, hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 30, 2_000),
		ctxCall("m2", 60, 30_000, 0, 0, 0),
		hcCompactionCall("c2", 90, 7_000),
		ctxCall("m3", 120, 31_000, 0, 0, 0),
	))

	got := run.Context.Orchestrator.CompactionCalls

	if got.Count != 2 || got.UnappliedCount != 1 {
		t.Errorf("compaction calls count/unapplied = %d/%d, want 2/1", got.Count, got.UnappliedCount)
	}
	assertMoneyKnown(t, "compaction call cost", got.Cost, 9_000*hcPerToken)
	assertMoneyKnown(t, "unapplied cost", got.UnappliedCost, 7_000*hcPerToken)
	m := hcRequireOneDrop(t, run.Context.Orchestrator)
	if m.CallCount != 1 {
		t.Errorf("harness compaction calls = %d, want only the call before the drop", m.CallCount)
	}
}

func TestCompactionKinds_CompactionCallBeforeALoggedCompaction_IsApplied(t *testing.T) {
	spec := hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 30, 2_000),
		ctxCall("m2", 60, 95_000, 0, 0, 0),
	)
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 50)}

	run, _ := detPrice(t, spec)

	got := run.Context.Orchestrator.CompactionCalls
	if got.Count != 1 || got.UnappliedCount != 0 {
		t.Errorf("compaction calls count/unapplied = %d/%d, want 1/0", got.Count, got.UnappliedCount)
	}
}

func TestCompactionKinds_CompactionCallWithUnknownTime_IsUnapplied(t *testing.T) {
	timeless := hcCompactionCall("c1", 30, 2_000)
	timeless.At = time.Time{}
	run, _ := detPrice(t, hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		timeless,
		ctxCall("m2", 60, 40_000, 0, 0, 0),
	))

	got := run.Context.Orchestrator.CompactionCalls

	if got.UnappliedCount != 1 {
		t.Errorf("unapplied attempts = %d, want 1 (a zero time is never attributed)", got.UnappliedCount)
	}
	if m := hcRequireOneDrop(t, run.Context.Orchestrator); m.Harness {
		t.Error("a compaction call with an unknown time must not make the drop a harness compaction")
	}
}

func TestCompactionKinds_AgentsAreClassifiedIndependentlyOfEachOther(t *testing.T) {
	harness := ctxAgent{id: "a#1", typ: "worker", usage: hcHarnessSpec().orchUsage}
	plain := detAgent("b#1", detPt{0, 100_000}, detPt{60, 40_000})

	run, findings := detPrice(t, ctxRunSpec{agents: []ctxAgent{harness, plain}})

	if m := hcRequireOneDrop(t, ctxRequireAgent(t, run, "a#1")); !m.Harness || m.CallCount != 2 {
		t.Errorf("a#1 mark harness/calls = %v/%d, want true/2", m.Harness, m.CallCount)
	}
	if m := hcRequireOneDrop(t, ctxRequireAgent(t, run, "b#1")); m.Harness {
		t.Error("b#1 drop must be detected only")
	}
	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], detAgentActor("b#1"))
}
