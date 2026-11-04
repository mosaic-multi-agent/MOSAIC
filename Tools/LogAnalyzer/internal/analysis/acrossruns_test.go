package analysis_test

// Tests for the across-runs view (Report.AcrossRuns) over the scenario in
// acrossruns_fixture_test.go.
//
// Rules under test:
//   - Run comparison lists the named runs in report order with their totals,
//     subagent figures, unpriced models and provisional flag.
//   - Per agent type, figures are summed over the named runs that hold at least
//     one invocation of the type; the per-run averages divide by that run count.
//   - Types are ordered ascending with the unknown type ("") last and never dropped.
//   - The unattributable bucket is never folded into the named runs.
//   - Partial data stays labelled: unpriced models, assumed-tier cache writes,
//     open invocations and provisional runs; re-pricing recomputes the view.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestAcrossRuns_RunsAreTheNamedRunsInReportOrder(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	var got, want []string
	for _, s := range report.AcrossRuns.Runs {
		got = append(got, s.Run.ID)
	}
	for _, r := range report.Runs {
		want = append(want, r.Run.ID)
	}

	if !rmEqualStrings(got, want) || !rmEqualStrings(got, []string{arRunA, arRunB, arRunC}) {
		t.Errorf("across-runs runs = %v, report runs = %v, want both %v", got, want, []string{arRunA, arRunB, arRunC})
	}
}

func TestAcrossRuns_RunSummary_CarriesTheRunTotalsAndSubagentFigures(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	for id, wantTotal := range map[string]int64{arRunA: 5_000_000, arRunB: 5_300_000} {
		s := arSummary(t, report.AcrossRuns, id)
		assertMoneyKnown(t, id+" run total", s.Totals.Money.Total, wantTotal)
		if !s.Totals.Money.Complete || s.Provisional {
			t.Errorf("%s: complete = %v, provisional = %v, want true and false", id, s.Totals.Money.Complete, s.Provisional)
		}
	}
	a := arSummary(t, report.AcrossRuns, arRunA)
	if a.Subagents.Invocations != 4 || a.Subagents.MissingUsage != 1 {
		t.Errorf("run a subagent invocations = %d, missing usage = %d, want 4 and 1 (the orchestrator is not counted)",
			a.Subagents.Invocations, a.Subagents.MissingUsage)
	}
	assertMoneyKnown(t, "run a subagent total", a.Subagents.Totals.Money.Total, 4_000_000)
	b := arSummary(t, report.AcrossRuns, arRunB)
	if b.Subagents.Invocations != 2 {
		t.Errorf("run b subagent invocations = %d, want 2", b.Subagents.Invocations)
	}
	assertMoneyKnown(t, "run b subagent total", b.Subagents.Totals.Money.Total, 5_100_000)
}

func TestAcrossRuns_RunSummary_EqualsTheRunReportFigures(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	for _, run := range report.Runs {
		s := arSummary(t, report.AcrossRuns, run.Run.ID)
		if !reflect.DeepEqual(s.Totals, run.Totals) || !reflect.DeepEqual(s.Subagents, run.Overview.Subagents) ||
			!reflect.DeepEqual(s.UnpricedModels, run.UnpricedModels) || s.Provisional != run.Provisional {
			t.Errorf("run %s summary %+v differs from its run report", run.Run.ID, s)
		}
	}
}

func TestAcrossRuns_ProvisionalAndPartialRun_IsLabelledNotDropped(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	c := arSummary(t, report.AcrossRuns, arRunC)

	if !c.Provisional {
		t.Error("run c has no run_end and must be provisional")
	}
	assertMoneyKnown(t, "run c partial total", c.Totals.Money.Total, 825_000)
	if c.Totals.Money.Complete {
		t.Error("run c total is complete, want partial because coder#1 is unpriced")
	}
	assertUnpricedModels(t, "run c", c.UnpricedModels, viewsUnpriced)
	if c.Totals.AssumedTierCacheWrite != 100 {
		t.Errorf("run c assumed-tier cache writes = %d, want 100", c.Totals.AssumedTierCacheWrite)
	}
	if c.Subagents.Invocations != 2 || c.Subagents.Open != 1 {
		t.Errorf("run c subagents = %d invocations, %d open, want 2 and 1", c.Subagents.Invocations, c.Subagents.Open)
	}
}

func TestAcrossRuns_AgentTypesAreOrderedWithUnknownLast(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	var types []string
	for _, g := range report.AcrossRuns.ByAgentType {
		types = append(types, g.AgentType)
	}

	if want := []string{"coder", "tester", ""}; !rmEqualStrings(types, want) {
		t.Errorf("agent types = %q, want %q", types, want)
	}
}

func TestAcrossRuns_CoderType_TotalsAndPerRunAverages(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	coder := arType(t, report.AcrossRuns, "coder")

	if coder.Runs != 3 || coder.ProvisionalRuns != 1 || coder.Figures.Invocations != 5 {
		t.Errorf("coder runs = %d, provisional = %d, invocations = %d, want 3, 1 and 5",
			coder.Runs, coder.ProvisionalRuns, coder.Figures.Invocations)
	}
	assertTokenPresent(t, "coder input", coder.Figures.Totals.Tokens.Input, 7300)
	assertMoneyKnown(t, "coder money", coder.Figures.Totals.Money.Total, 8_500_000)
	if coder.Figures.Totals.Money.Complete {
		t.Error("coder money is complete, want partial because run c's coder is unpriced")
	}
	assertUnpricedModels(t, "coder", coder.Figures.UnpricedModels, viewsUnpriced)
	if coder.Figures.MissingUsage != 1 {
		t.Errorf("coder missing usage = %d, want 1 (coder#4 reported no usage)", coder.Figures.MissingUsage)
	}
	assertRatio(t, "coder invocations per run", coder.PerRunInvocations, 5, 3)
	assertRatio(t, "coder tokens per run", coder.PerRunTokens, 7630, 3)
}

func TestAcrossRuns_CoderType_PerRunMoneyUsesKnownRunsAndIsPartial(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	coder := arType(t, report.AcrossRuns, "coder")

	assertMoneyStats(t, "coder per-run money", coder.PerRunMoney, 2, 3_500_000, 5_000_000, 4_250_000, false)
}

func TestAcrossRuns_TesterType_CompleteMoneyAndLabelsOfItsOpenAssumedTierInvocation(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	tester := arType(t, report.AcrossRuns, "tester")

	if tester.Runs != 2 || tester.ProvisionalRuns != 1 || tester.Figures.Invocations != 2 {
		t.Errorf("tester runs = %d, provisional = %d, invocations = %d, want 2, 1 and 2",
			tester.Runs, tester.ProvisionalRuns, tester.Figures.Invocations)
	}
	assertMoneyKnown(t, "tester money", tester.Figures.Totals.Money.Total, 1_325_000)
	if !tester.Figures.Totals.Money.Complete || len(tester.Figures.UnpricedModels) != 0 {
		t.Errorf("tester complete = %v, unpriced = %v, want complete and none", tester.Figures.Totals.Money.Complete, tester.Figures.UnpricedModels)
	}
	if tester.Figures.Open != 1 || tester.Figures.Totals.AssumedTierCacheWrite != 100 {
		t.Errorf("tester open = %d, assumed = %d, want 1 and 100", tester.Figures.Open, tester.Figures.Totals.AssumedTierCacheWrite)
	}
	assertRatio(t, "tester invocations per run", tester.PerRunInvocations, 2, 2)
	assertRatio(t, "tester tokens per run", tester.PerRunTokens, 1300, 2)
	assertMoneyStats(t, "tester per-run money", tester.PerRunMoney, 2, 500_000, 825_000, 662_500, true)
}

func TestAcrossRuns_InvocationsWithoutAType_FormTheUnknownTypeGroup(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	unknown := arType(t, report.AcrossRuns, "")

	if unknown.Runs != 1 || unknown.ProvisionalRuns != 0 || unknown.Figures.Invocations != 1 {
		t.Errorf("unknown type runs = %d, provisional = %d, invocations = %d, want 1, 0 and 1",
			unknown.Runs, unknown.ProvisionalRuns, unknown.Figures.Invocations)
	}
	assertMoneyKnown(t, "unknown type money", unknown.Figures.Totals.Money.Total, 100_000)
	assertRatio(t, "unknown type tokens per run", unknown.PerRunTokens, 100, 1)
	assertMoneyStats(t, "unknown type per-run money", unknown.PerRunMoney, 1, 100_000, 100_000, 100_000, true)
}

func TestAcrossRuns_UnattributableBucketIsNeverFoldedIn(t *testing.T) {
	report := arReport(t, viewsPricing(t))

	if report.Unattributable == nil {
		t.Fatal("fixture must produce an unattributable bucket")
	}
	if n := len(report.AcrossRuns.Runs); n != 3 {
		t.Errorf("across-runs has %d runs, want the 3 named runs only", n)
	}
	for _, s := range report.AcrossRuns.Runs {
		if s.Run.Kind != domain.RunNamed {
			t.Errorf("across-runs lists non-named run %+v", s.Run)
		}
	}
	for _, g := range report.AcrossRuns.ByAgentType {
		if g.AgentType == "ghost" {
			t.Error("the unattributable bucket's agent type ghost must not appear")
		}
	}
	coder := arType(t, report.AcrossRuns, "coder")
	assertTokenPresent(t, "coder input without the bucket's 99999", coder.Figures.Totals.Tokens.Input, 7300)
}

func TestAcrossRuns_Repricing_RecomputesTheViewAndPerRunAverages(t *testing.T) {
	report := arReport(t, arPricingBoth(t))

	coder := arType(t, report.AcrossRuns, "coder")
	c := arSummary(t, report.AcrossRuns, arRunC)

	assertMoneyStats(t, "coder per-run money", coder.PerRunMoney, 3, 450_000, 5_000_000, 2_983_333, true)
	assertMoneyKnown(t, "coder money", coder.Figures.Totals.Money.Total, 8_950_000)
	if !coder.Figures.Totals.Money.Complete || len(coder.Figures.UnpricedModels) != 0 {
		t.Errorf("coder complete = %v, unpriced = %v, want complete and none", coder.Figures.Totals.Money.Complete, coder.Figures.UnpricedModels)
	}
	assertMoneyKnown(t, "run c total", c.Totals.Money.Total, 1_275_000)
	if !c.Totals.Money.Complete || len(c.UnpricedModels) != 0 || !c.Provisional {
		t.Errorf("run c complete = %v, unpriced = %v, provisional = %v, want complete, none and provisional",
			c.Totals.Money.Complete, c.UnpricedModels, c.Provisional)
	}
}

func TestAcrossRuns_OrchestratorOnlyRun_IsListedWithoutSubagentsAndCountsForNoType(t *testing.T) {
	const runD = "20260104T000000Z-dddd"
	in := arInput()
	in.Streams = append(in.Streams, arOrchestrator(domain.NamedRun(runD), 50))

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	d := arSummary(t, report.AcrossRuns, runD)
	assertMoneyKnown(t, "run d total", d.Totals.Money.Total, 50_000)
	if d.Subagents.Invocations != 0 {
		t.Errorf("run d subagent invocations = %d, want 0", d.Subagents.Invocations)
	}
	if n := len(report.AcrossRuns.Runs); n != 4 {
		t.Errorf("across-runs has %d runs, want 4", n)
	}
	for typ, wantRuns := range map[string]int{"coder": 3, "tester": 2, "": 1} {
		if got := arType(t, report.AcrossRuns, typ).Runs; got != wantRuns {
			t.Errorf("type %q counts %d runs, want %d (run d has no subagents)", typ, got, wantRuns)
		}
	}
}

func TestAcrossRuns_NoNamedRuns_IsEmpty(t *testing.T) {
	in := arInput()
	in.Streams = in.Streams[len(in.Streams)-1:] // the unattributable stream only

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	if len(report.AcrossRuns.Runs) != 0 || len(report.AcrossRuns.ByAgentType) != 0 {
		t.Errorf("across-runs = %+v, want empty when only the unattributable bucket exists", report.AcrossRuns)
	}
}

func TestAcrossRuns_IsDeterministic(t *testing.T) {
	first := arReport(t, viewsPricing(t)).AcrossRuns

	second := arReport(t, viewsPricing(t)).AcrossRuns

	if !reflect.DeepEqual(first, second) {
		t.Errorf("identical input gave different across-runs views:\n%+v\n%+v", first, second)
	}
}
