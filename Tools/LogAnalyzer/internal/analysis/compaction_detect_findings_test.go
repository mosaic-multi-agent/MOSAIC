package analysis_test

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const (
	detNoLoggedNear = "no logged compaction near it"
	detNoDropNear   = "no context drop near it"

	// detOrchStreamWording marks the finding for a subagent drop matched by a
	// compaction logged on the orchestrator stream.
	detOrchStreamWording = "orchestrator stream"
)

// detDropSpec has one 60% drop between 300 s and 360 s; calls at 0 s and 600 s
// bracket it so a logged compaction before or after the drop can be checked.
func detDropSpec() ctxRunSpec {
	return detOrch(detPt{0, 200000}, detPt{300, 200000}, detPt{360, 80000}, detPt{600, 80000})
}

func TestCompactionMismatch_DropWithNoLoggedCompaction_RaisesFindingNamingRunAndActor(t *testing.T) {
	// Arrange
	spec := detOrch(detPt{0, 200000}, detPt{60, 80000})

	// Act
	_, findings := detPrice(t, spec)

	// Assert
	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], domain.Orchestrator())
	detAssertDetailContains(t, got[0], "60%", detNoLoggedNear)
	if got[0].Severity != domain.SeverityWarning {
		t.Errorf("severity = %v, want warning", got[0].Severity)
	}
	if domain.NewQualitySummary(got).Incomplete() {
		t.Error("a compaction mismatch must not mark the data incomplete")
	}
}

func TestCompactionMismatch_LoggedCompactionWithNoDrop_RaisesFindingNamingRunAndActor(t *testing.T) {
	spec := detOrch(detPt{0, 100000}, detPt{60, 100000}, detPt{120, 100000})
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 90)}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], domain.Orchestrator())
	detAssertDetailContains(t, got[0], detNoDropNear)
}

func TestCompactionMismatch_LoggedAndDetectedThatMatch_RaiseNoFinding(t *testing.T) {
	spec := detDropSpec()
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 330)}

	run, findings := detPrice(t, spec)

	detRequireCounts(t, run.Context.Orchestrator, 1, 1)
	detRequireMismatches(t, findings, 0)
}

func TestCompactionMismatch_NoCompactionsAndNoDrops_RaiseNoFinding(t *testing.T) {
	_, findings := detPrice(t, detOrch(detPt{0, 100000}, detPt{60, 110000}))

	detRequireMismatches(t, findings, 0)
}

func TestCompactionMismatch_NearWindowIsTwoMinutesEitherSideOfTheDropCalls(t *testing.T) {
	// The drop lies between 300 s and 360 s, so the window is [180 s, 480 s].
	cases := []struct {
		name     string
		sec      int
		wantNear bool
	}{
		{"at lower edge", 180, true},
		{"just below lower edge", 179, false},
		{"at upper edge", 480, true},
		{"just above upper edge", 481, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := detDropSpec()
			spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), tc.sec)}

			run, findings := detPrice(t, spec)

			want := 2 // unmatched drop and unmatched logged compaction
			if tc.wantNear {
				want = 0
			}
			detRequireCounts(t, run.Context.Orchestrator, 1, 1)
			detRequireMismatches(t, findings, want)
		})
	}
}

func TestCompactionMismatch_PairedCompactionNearWhenAnyPartOfItsSpanIsInTheWindow(t *testing.T) {
	// Window [180 s, 480 s]. A span ending at 190 s reaches in; one ending at
	// 179 s does not, even though both start before the window.
	orch := domain.Orchestrator()
	cases := []struct {
		name    string
		endSec  int
		wantNum int
	}{
		{"span reaches window", 190, 0},
		{"span ends just before window", 179, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := detDropSpec()
			spec.compactions = []domain.Compaction{detLoggedPaired(orch, 100, tc.endSec)}

			run, findings := detPrice(t, spec)

			detRequireCounts(t, run.Context.Orchestrator, 1, 1)
			detRequireMismatches(t, findings, tc.wantNum)
		})
	}
}

func TestCompactionMismatch_ClaudeCodeShape_PreAndPostThirtyEightSecondsApart_RaisesNoFinding(t *testing.T) {
	// The pre half sits between the two calls that bracket the context drop
	// (0 s and 50 s); the post half lands after the second of them, 38 s later.
	spec := detOrch(detPt{0, 180000}, detPt{50, 60000}, detPt{160, 65000})
	spec.compactions = []domain.Compaction{detLoggedPaired(domain.Orchestrator(), 40, 78)}

	run, findings := detPrice(t, spec)

	detRequireMismatches(t, findings, 0)
	o := run.Context.Orchestrator
	if o.LoggedCount() != 1 || o.DetectedCount() != 1 {
		t.Errorf("logged/detected = %d/%d, want 1/1", o.LoggedCount(), o.DetectedCount())
	}
}

func TestCompactionMismatch_ZeroAtCompaction_IsNotCheckedAndMatchesNothing(t *testing.T) {
	spec := detOrch(detPt{0, 100000}, detPt{60, 100000}, detPt{120, 100000})
	spec.compactions = []domain.Compaction{{Actor: domain.Orchestrator(), Shape: domain.CompactionSingle}}

	_, findings := detPrice(t, spec)

	detRequireMismatches(t, findings, 0)
}

func TestCompactionMismatch_ZeroAtCompaction_DoesNotMatchADrop(t *testing.T) {
	spec := detOrch(detPt{0, 200000}, detPt{60, 80000})
	spec.compactions = []domain.Compaction{{Actor: domain.Orchestrator(), Shape: domain.CompactionSingle}}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertDetailContains(t, got[0], detNoLoggedNear)
}

func TestCompactionMismatch_LoggedBeforeFirstCallOrAfterLastCall_IsNotChecked(t *testing.T) {
	orch := domain.Orchestrator()
	cases := map[string]int{
		"no call strictly before": 0,
		"no call strictly after":  120,
	}
	for name, sec := range cases {
		t.Run(name, func(t *testing.T) {
			spec := detOrch(detPt{0, 100000}, detPt{60, 100000}, detPt{120, 100000})
			spec.compactions = []domain.Compaction{detLogged(orch, sec)}

			_, findings := detPrice(t, spec)

			detRequireMismatches(t, findings, 0)
		})
	}
}

func TestCompactionMismatch_PairedCompactionWithNoCallAfterItsSpanEnd_IsNotChecked(t *testing.T) {
	spec := detOrch(detPt{0, 100000}, detPt{60, 100000})
	spec.compactions = []domain.Compaction{detLoggedPaired(domain.Orchestrator(), 30, 90)}

	_, findings := detPrice(t, spec)

	detRequireMismatches(t, findings, 0)
}

func TestCompactionMismatch_MatchingIsOneToOne_SecondLoggedCompactionNearOneDropRaisesFinding(t *testing.T) {
	orch := domain.Orchestrator()
	spec := detDropSpec()
	spec.compactions = []domain.Compaction{detLogged(orch, 320), detLogged(orch, 340)}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertDetailContains(t, got[0], detNoDropNear)
}

func TestCompactionMismatch_MatchingIsOneToOne_SecondDropNearOneLoggedCompactionRaisesFinding(t *testing.T) {
	// Two drops (0-60 s and 120-180 s); the logged compaction at 90 s is near
	// both and matches only the earlier one.
	spec := detOrch(detPt{0, 300000}, detPt{60, 100000}, detPt{120, 300000}, detPt{180, 100000})
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 90)}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertDetailContains(t, got[0], detNoLoggedNear)
}

func TestCompactionMismatch_SubagentCompactionOnItsOwnStream_MatchesItsOwnDrop(t *testing.T) {
	agentRef := detAgentActor("ag-1")
	spec := ctxRunSpec{
		orchUsage:   detCalls("o", detPt{0, 100000}, detPt{60, 100000}),
		agents:      []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
		compactions: []domain.Compaction{detLogged(agentRef, 30)},
	}

	run, findings := detPrice(t, spec)

	agent, ok := run.Context.Find("ag-1")
	if !ok {
		t.Fatal("no context figures for ag-1")
	}
	detRequireCounts(t, agent, 1, 1)
	detRequireMismatches(t, findings, 0)
}

func TestCompactionMismatch_SubagentLoggedCompactionOnItsOwnStreamWithNoDrop_NamesTheSubagent(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage:   detCalls("o", detPt{0, 100000}, detPt{60, 100000}),
		agents:      []ctxAgent{detAgent("ag-1", detPt{0, 100000}, detPt{30, 100000}, detPt{60, 100000})},
		compactions: []domain.Compaction{detLogged(detAgentActor("ag-1"), 30)},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], detAgentActor("ag-1"))
	detAssertDetailContains(t, got[0], detNoDropNear)
}

func TestCompactionMismatch_SubagentDropWithNoLoggedCompaction_NamesTheSubagent(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage: detCalls("o", detPt{0, 100000}, detPt{60, 100000}),
		agents:    []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], detAgentActor("ag-1"))
	detAssertDetailContains(t, got[0], detNoLoggedNear)
}

func TestCompactionMismatch_OrchestratorStreamCompactionMatchingSubagentDrop_RaisesOneFindingForTheSubagent(t *testing.T) {
	// The orchestrator has no drop; the compaction logged on its stream falls
	// next to a drop of the subagent.
	spec := ctxRunSpec{
		orchUsage:   detCalls("o", detPt{0, 100000}, detPt{100, 100000}),
		agents:      []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 30)},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], detAgentActor("ag-1"))
	detAssertDetailContains(t, got[0], "ag-1", detOrchStreamWording)
	detAssertNoGenericWording(t, got[0])
}

func TestCompactionMismatch_OrchestratorStreamCompactionWithNoNearDropAnywhere_RaisesFindingForTheOrchestrator(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage:   detCalls("o", detPt{0, 100000}, detPt{100, 100000}),
		agents:      []ctxAgent{detAgent("ag-1", detPt{0, 100000}, detPt{60, 100000})},
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 30)},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], domain.Orchestrator())
	detAssertDetailContains(t, got[0], detNoDropNear)
}

func TestCompactionMismatch_OrchestratorStreamCompactionNextToSubagentDropOutsideTheWindow_RaisesTwoFindings(t *testing.T) {
	// The subagent drop lies between 600 s and 660 s, window [480 s, 780 s];
	// the compaction at 30 s is far outside it.
	spec := ctxRunSpec{
		orchUsage:   detCalls("o", detPt{0, 100000}, detPt{100, 100000}),
		agents:      []ctxAgent{detAgent("ag-1", detPt{600, 200000}, detPt{660, 60000}, detPt{700, 60000})},
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 30)},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 2)
	byKind := map[domain.ActorKind]domain.Finding{}
	for _, f := range got {
		byKind[f.Actor.Kind] = f
	}
	detAssertDetailContains(t, byKind[domain.ActorOrchestrator], detNoDropNear)
	detAssertDetailContains(t, byKind[domain.ActorAgentInstance], detNoLoggedNear)
}

func TestCompactionMismatch_OrchestratorCompactionMatchesOrchestratorDropFirst_SubagentDropStaysUnmatched(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage:   detCalls("o", detPt{0, 200000}, detPt{60, 80000}),
		agents:      []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 30)},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], detAgentActor("ag-1"))
	detAssertDetailContains(t, got[0], detNoLoggedNear)
}

func TestCompactionMismatch_OrchestratorCompactionTakesTheEarliestSubagentDrop(t *testing.T) {
	// The earliest drop in time (ag-2, 0-60 s) belongs to the later instance id,
	// so instance-id order and time order disagree. Both drops are near 50 s.
	spec := ctxRunSpec{
		orchUsage: detCalls("o", detPt{0, 100000}, detPt{100, 100000}),
		agents: []ctxAgent{
			detAgent("ag-1", detPt{20, 200000}, detPt{80, 60000}),
			detAgent("ag-2", detPt{0, 200000}, detPt{60, 60000}),
		},
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 50)},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 2)
	byInstance := map[domain.AgentInstanceID]domain.Finding{}
	for _, f := range got {
		byInstance[f.Actor.Instance] = f
	}
	detAssertDetailContains(t, byInstance["ag-2"], "ag-2", detOrchStreamWording)
	detAssertNoGenericWording(t, byInstance["ag-2"])
	detAssertDetailContains(t, byInstance["ag-1"], detNoLoggedNear)
}

func TestCompactionMismatch_SubagentDropMatchedByItsOwnCompaction_IsUnavailableToOrchestratorStreamCompaction(t *testing.T) {
	// ag-1's only drop is matched by its own logged compaction; the orchestrator
	// stream compaction near the same drop has no orchestrator drop and no other
	// free subagent drop to match.
	spec := ctxRunSpec{
		orchUsage: detCalls("o", detPt{0, 100000}, detPt{100, 100000}),
		agents:    []ctxAgent{detAgent("ag-1", detPt{0, 200000}, detPt{60, 60000})},
		compactions: []domain.Compaction{
			detLogged(detAgentActor("ag-1"), 30),
			detLogged(domain.Orchestrator(), 40),
		},
	}

	_, findings := detPrice(t, spec)

	got := detRequireMismatches(t, findings, 1)
	detAssertRunActor(t, got[0], domain.Orchestrator())
	detAssertDetailContains(t, got[0], detNoDropNear)
}

func TestCompactionMismatch_CustomThreshold_ChangesWhichDropsNeedALoggedCompaction(t *testing.T) {
	// A 40% drop with a logged compaction near it: matched at threshold 30,
	// but at threshold 50 there is no drop, so the logged compaction is flagged.
	spec := detOrch(detPt{0, 100000}, detPt{60, 60000}, detPt{120, 60000})
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 30)}
	strict := domain.DefaultAnalysisSettings()
	strict.CompactionDropPercent = 50

	_, defaultFindings := detPrice(t, spec)
	_, strictFindings := detPriceSettings(t, spec, strict)

	detRequireMismatches(t, defaultFindings, 0)
	got := detRequireMismatches(t, strictFindings, 1)
	detAssertDetailContains(t, got[0], detNoDropNear)
}

func TestCompactionMismatch_FindingsNameTheRunTheyBelongTo(t *testing.T) {
	// Two runs; only the second has a drop without a logged compaction.
	first := ctxAggregate(detOrch(detPt{0, 100000}, detPt{60, 100000}))
	second := ctxAggregate(detOrch(detPt{0, 200000}, detPt{60, 80000}))
	second.Runs[0].Run = domain.NamedRun("20261028T110000Z-bbbb")
	agg := domain.Aggregate{Runs: []domain.RunAggregate{first.Runs[0], second.Runs[0]}}

	_, findings := detPriceAll(t, agg, domain.DefaultAnalysisSettings())

	got := detRequireMismatches(t, findings, 1)
	if got[0].Run != domain.NamedRun("20261028T110000Z-bbbb") {
		t.Errorf("finding run = %+v, want 20261028T110000Z-bbbb", got[0].Run)
	}
	if got[0].Actor.Kind != domain.ActorOrchestrator {
		t.Errorf("finding actor kind = %v, want orchestrator", got[0].Actor.Kind)
	}
}
