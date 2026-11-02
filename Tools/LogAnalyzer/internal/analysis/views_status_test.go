package analysis_test

// Tests for the by-status view (RunReport.ByStatus) over the shared scenario in
// views_scenario_test.go.
//
// Rules under test:
//   - Cost and dispatch count are grouped by returned status code, per run and
//     per agent type within the run.
//   - Order: enum codes in enum order, then other codes verbatim ascending, then
//     "ended without a status code", then "open".
//   - Invocations without a status code and open invocations are two separate
//     groups, never merged into a real status or into each other.
//   - Partial figures stay marked on the group they belong to.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

type wantGroup struct {
	kind        domain.StatusKind
	code        string
	invocations int
}

func assertStatusGroups(t *testing.T, name string, got []domain.StatusGroup, want []wantGroup) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d groups %+v, want %d", name, len(got), got, len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Kind != w.kind || g.Code != w.code || g.Figures.Invocations != w.invocations {
			t.Errorf("%s[%d] = kind %v code %q invocations %d, want kind %v code %q invocations %d",
				name, i, g.Kind, g.Code, g.Figures.Invocations, w.kind, w.code, w.invocations)
		}
	}
}

func requireStatusGroup(t *testing.T, groups []domain.StatusGroup, kind domain.StatusKind, code string) domain.StatusGroup {
	t.Helper()
	for _, g := range groups {
		if g.Kind == kind && g.Code == code {
			return g
		}
	}
	t.Fatalf("no status group kind %v code %q in %+v", kind, code, groups)
	return domain.StatusGroup{}
}

func TestByStatus_Run_GroupsInEnumOrderThenOtherThenNoneThenOpen(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	assertStatusGroups(t, "run", run.ByStatus.Run, []wantGroup{
		{domain.StatusReported, "SUCCESS", 2},
		{domain.StatusReported, "BLOCKED", 1},
		{domain.StatusReported, "WEIRD", 1},
		{domain.StatusNone, "", 1},
		{domain.StatusOpen, "", 1},
	})
}

func TestByStatus_NoStatusAndOpen_AreSeparateGroupsWithOwnFigures(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	none := requireStatusGroup(t, run.ByStatus.Run, domain.StatusNone, "")
	open := requireStatusGroup(t, run.ByStatus.Run, domain.StatusOpen, "")

	if none.Figures.MissingUsage != 1 || none.Figures.Open != 0 {
		t.Errorf("no-status figures = %+v, want 1 missing usage and 0 open", none.Figures)
	}
	assertMoneyState(t, "no-status total", none.Figures.Totals.Money.Total, domain.MoneyNoData)
	if open.Figures.Open != 1 || open.Figures.MissingUsage != 0 {
		t.Errorf("open figures = %+v, want 1 open and 0 missing usage", open.Figures)
	}
	assertMoneyKnown(t, "open total", open.Figures.Totals.Money.Total, 350_000)
	if open.Figures.Totals.AssumedTierCacheWrite != 200 {
		t.Errorf("open assumed-tier tokens = %d, want 200", open.Figures.Totals.AssumedTierCacheWrite)
	}
}

func TestByStatus_ReportedGroups_CarryCostAndCount(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	success := requireStatusGroup(t, run.ByStatus.Run, domain.StatusReported, "SUCCESS")
	blocked := requireStatusGroup(t, run.ByStatus.Run, domain.StatusReported, "BLOCKED")

	assertMoneyKnown(t, "SUCCESS total", success.Figures.Totals.Money.Total, 5_425_000)
	if !success.Figures.Totals.Money.Complete {
		t.Error("SUCCESS total is partial, want complete")
	}
	assertMoneyState(t, "BLOCKED total", blocked.Figures.Totals.Money.Total, domain.MoneyUnpriced)
	assertUnpricedModels(t, "BLOCKED", blocked.Figures.UnpricedModels, viewsUnpriced)
}

func TestByStatus_CodeOutsideTheEnum_IsShownAsLogged(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	weird := requireStatusGroup(t, run.ByStatus.Run, domain.StatusReported, "WEIRD")

	assertMoneyKnown(t, "WEIRD total", weird.Figures.Totals.Money.Total, 10_000)
}

func TestByStatus_PerAgentType_GroupsStatusesWithinEachType(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	byType := run.ByStatus.ByAgentType

	var types []string
	for _, a := range byType {
		types = append(types, a.AgentType)
	}
	if !rmEqualStrings(types, []string{"coder", "reviewer", "tester", ""}) {
		t.Fatalf("agent types = %q, want coder, reviewer, tester, unknown last", types)
	}
	assertStatusGroups(t, "coder", byType[0].Groups, []wantGroup{
		{domain.StatusReported, "SUCCESS", 2},
		{domain.StatusReported, "BLOCKED", 1},
	})
	assertStatusGroups(t, "reviewer", byType[1].Groups, []wantGroup{{domain.StatusNone, "", 1}})
	assertStatusGroups(t, "tester", byType[2].Groups, []wantGroup{{domain.StatusOpen, "", 1}})
	assertStatusGroups(t, "unknown type", byType[3].Groups, []wantGroup{{domain.StatusReported, "WEIRD", 1}})
}

func TestByStatus_EmptyGroupsAreOmitted(t *testing.T) {
	in := oneAgentStream(testRunRef, "coder#1",
		rmInvStart("coder#1", "coder", rmAt(1)),
		rmInvEnd("coder#1", "SUCCESS", viewsModel, rmAt(2)))

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	assertStatusGroups(t, "run", requireRunReport(t, report, testRunID).ByStatus.Run,
		[]wantGroup{{domain.StatusReported, "SUCCESS", 1}})
}
