package analysis_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func detOrch(pts ...detPt) ctxRunSpec {
	return ctxRunSpec{orchUsage: detCalls("o", pts...)}
}

func TestDetectedCompaction_DropOfExactlyThirtyPercent_IsDetectedWithBeforeAfterFromTheCalls(t *testing.T) {
	// Arrange
	spec := detOrch(detPt{0, 100000}, detPt{60, 70000})

	// Act
	run, _ := detPrice(t, spec)

	// Assert
	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	detAssertMark(t, marks[0], 0, 1, 100000, 70000, ctxAt(60))
	if marks[0].Kind != domain.CompactionDetected {
		t.Errorf("kind = %v, want detected", marks[0].Kind)
	}
}

func TestDetectedCompaction_DropOfTwentyNinePercent_IsNotDetected(t *testing.T) {
	run, _ := detPrice(t, detOrch(detPt{0, 100000}, detPt{60, 71000}))

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_DropJustUnderThirtyPercent_IsNotDetectedButTheExactBoundaryIs(t *testing.T) {
	// 100000 -> 70001 falls 29.999%; 100000 -> 70000 falls exactly 30%. Both runs
	// are priced so the boundary case proves detection happens at all.
	under, _ := detPrice(t, detOrch(detPt{0, 100000}, detPt{60, 70001}))
	exact, _ := detPrice(t, detOrch(detPt{0, 100000}, detPt{60, 70000}))

	if n := exact.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Fatalf("exact 30%% drop detected = %d, want 1", n)
	}
	if n := under.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("drop just under 30%% detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_LargeDropUsesBeforeAndAfterOfThoseTwoCalls(t *testing.T) {
	run, _ := detPrice(t, detOrch(detPt{0, 150000}, detPt{30, 200000}, detPt{90, 80000}, detPt{120, 90000}))

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	detAssertMark(t, marks[0], 1, 2, 200000, 80000, ctxAt(90))
}

func TestDetectedCompaction_IncreaseOrFlatContext_IsNotDetected(t *testing.T) {
	run, _ := detPrice(t, detOrch(detPt{0, 50000}, detPt{30, 50000}, detPt{60, 90000}))

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_ConsecutiveDrops_AreEachDetectedInOrder(t *testing.T) {
	run, _ := detPrice(t, detOrch(detPt{0, 300000}, detPt{60, 150000}, detPt{120, 70000}))

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 2 {
		t.Fatalf("detected marks = %d, want 2", len(marks))
	}
	detAssertMark(t, marks[0], 0, 1, 300000, 150000, ctxAt(60))
	detAssertMark(t, marks[1], 1, 2, 150000, 70000, ctxAt(120))
}

func TestDetectedCompaction_CustomThresholdFromSettings_Applies(t *testing.T) {
	// A 40% drop: below a 50% threshold, above a 10% threshold.
	spec := detOrch(detPt{0, 100000}, detPt{60, 60000})
	high := domain.DefaultAnalysisSettings()
	high.CompactionDropPercent = 50
	low := domain.DefaultAnalysisSettings()
	low.CompactionDropPercent = 10

	highRun, _ := detPriceSettings(t, spec, high)
	lowRun, _ := detPriceSettings(t, spec, low)

	if n := highRun.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("threshold 50: detected = %d, want 0", n)
	}
	if n := lowRun.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Errorf("threshold 10: detected = %d, want 1", n)
	}
}

func TestDetectedCompaction_ThresholdIsInclusiveForCustomPercent(t *testing.T) {
	spec := detOrch(detPt{0, 100000}, detPt{60, 50000})
	s := domain.DefaultAnalysisSettings()
	s.CompactionDropPercent = 50

	run, _ := detPriceSettings(t, spec, s)

	if n := run.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Errorf("exactly 50%% drop at threshold 50: detected = %d, want 1", n)
	}
}

func TestDetectedCompaction_PartialCallOnEitherSide_IsNeverDetected(t *testing.T) {
	cases := map[string][]domain.InvocationUsage{
		"partial before": {ctxInputOnlyCall("p1", 0, 200000, 10), ctxCall("p2", 60, 50000, 0, 0, 10)},
		"partial after":  {ctxCall("p1", 0, 200000, 0, 0, 10), ctxInputOnlyCall("p2", 60, 50000, 10)},
	}
	for name, usage := range cases {
		t.Run(name, func(t *testing.T) {
			run, _ := detPrice(t, ctxRunSpec{orchUsage: usage})

			if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
				t.Errorf("detected = %d, want 0", n)
			}
		})
	}
}

func TestDetectedCompaction_CallWithAbsentContext_IsSkippedAndNeighboursPair(t *testing.T) {
	usage := []domain.InvocationUsage{
		ctxCall("a", 0, 200000, 0, 0, 10),
		detOutputOnlyCall("b", 30, 5),
		ctxCall("c", 60, 80000, 0, 0, 10),
	}

	run, _ := detPrice(t, ctxRunSpec{orchUsage: usage})

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	detAssertMark(t, marks[0], 0, 2, 200000, 80000, ctxAt(60))
}

func TestDetectedCompaction_ZeroContextBefore_IsNotDetected(t *testing.T) {
	run, _ := detPrice(t, detOrch(detPt{0, 0}, detPt{60, 0}))

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_LegacyAggregateEntries_NeverInventADetection(t *testing.T) {
	usage := []domain.InvocationUsage{
		ctxLegacyEntry(0, 200000, 0, 0, 10),
		ctxLegacyEntry(60, 50000, 0, 0, 10),
	}

	run, findings := detPrice(t, ctxRunSpec{orchUsage: usage})

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
	detRequireMismatches(t, findings, 0)
}

func TestDetectedCompaction_ActorWithoutCallsOrWithOneCall_HasNoDetection(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage: detCalls("o", detPt{0, 200000}),
		agents:    []ctxAgent{{id: "ag-empty", typ: "worker"}},
	}

	run, findings := detPrice(t, spec)

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("one-call orchestrator: detected = %d, want 0", n)
	}
	agent := ctxRequireAgent(t, run, "ag-empty")
	if n := agent.DetectedCount(); n != 0 {
		t.Errorf("no-call agent: detected = %d, want 0", n)
	}
	detRequireMismatches(t, findings, 0)
}

func TestDetectedCompaction_SubagentDrop_IsDetectedOnTheSubagentOnly(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage: detCalls("o", detPt{0, 100000}, detPt{60, 110000}),
		agents:    []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
	}

	run, _ := detPrice(t, spec)

	agent := ctxRequireAgent(t, run, "ag-1")
	marks := detDetected(agent)
	if len(marks) != 1 {
		t.Fatalf("agent detected marks = %d, want 1", len(marks))
	}
	detAssertMark(t, marks[0], 0, 1, 200000, 60000, ctxAt(60))
	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("orchestrator detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_LoggedAndDetectedCounts_AreKeptSeparatePerActor(t *testing.T) {
	// Orchestrator: one drop with a logged compaction in it, plus a second
	// logged compaction with no drop. Agent: one drop, nothing logged.
	orch := domain.Orchestrator()
	spec := ctxRunSpec{
		orchUsage: detCalls("o",
			detPt{0, 200000}, detPt{60, 80000}, detPt{300, 85000}, detPt{600, 90000}),
		agents: []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
		compactions: []domain.Compaction{
			detLogged(orch, 30),
			detLogged(orch, 450),
		},
	}

	run, _ := detPrice(t, spec)

	o := run.Context.Orchestrator
	if o.LoggedCount() != 2 || o.DetectedCount() != 1 {
		t.Errorf("orchestrator logged/detected = %d/%d, want 2/1", o.LoggedCount(), o.DetectedCount())
	}
	a := ctxRequireAgent(t, run, "ag-1")
	if a.LoggedCount() != 0 || a.DetectedCount() != 1 {
		t.Errorf("agent logged/detected = %d/%d, want 0/1", a.LoggedCount(), a.DetectedCount())
	}
}

func TestDetectedCompaction_LoggedWithoutDrop_DoesNotCountAsDetected(t *testing.T) {
	orch := domain.Orchestrator()
	spec := detOrch(detPt{0, 100000}, detPt{60, 100000}, detPt{120, 100000})
	spec.compactions = []domain.Compaction{detLogged(orch, 90)}

	run, _ := detPrice(t, spec)

	o := run.Context.Orchestrator
	if o.LoggedCount() != 1 || o.DetectedCount() != 0 {
		t.Errorf("logged/detected = %d/%d, want 1/0", o.LoggedCount(), o.DetectedCount())
	}
}
