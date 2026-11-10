package tui

// Tests for the assumed-tier disclosure on the runs list and run-detail
// screens: cache writes priced at the 5m fallback because their request
// carried no tier split are labelled wherever unpriced models are labelled,
// and nothing is added when no tokens were priced at an assumed tier.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const assumedViewWidth, assumedViewHeight = 120, 40

func assumedRunsReport(allRuns, unattributable int64) domain.Report {
	report := domain.Report{
		Runs: []domain.RunReport{{
			Run:          domain.NamedRun("20260101T120000Z-abcd"),
			Totals:       domain.Totals{AssumedTierCacheWrite: allRuns},
			Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		}},
		AllRuns:  domain.Totals{AssumedTierCacheWrite: allRuns},
		Currency: domain.Currency,
		Quality:  domain.NewQualitySummary(nil),
	}
	if unattributable > 0 {
		report.Unattributable = &domain.RunReport{
			Run:          domain.UnattributableRun(),
			Totals:       domain.Totals{AssumedTierCacheWrite: unattributable},
			Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		}
	}
	return report
}

func runsView(report domain.Report) string {
	s := screens.NewRunsScreen(report, assumedViewWidth, assumedViewHeight, plainStyles())
	return s.View(assumedViewWidth, assumedViewHeight)
}

// assumedDetailWidth leaves room for every column of the Dispatches table, so
// the labels of a row are not cut.
const assumedDetailWidth = 240

func detailView(run domain.RunReport) string {
	s := screens.NewRunDetailScreen(run, assumedDetailWidth, assumedViewHeight, plainStyles())
	return s.View(assumedDetailWidth, assumedViewHeight)
}

func lineContaining(view, needle string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func assertLabelledCount(t *testing.T, view, count string) {
	t.Helper()
	line := lineContaining(view, count)
	if line == "" || !strings.Contains(strings.ToLower(line), "assumed") {
		t.Errorf("no line labels %s as assumed tier; view:\n%s", count, view)
	}
}

func TestRunsScreen_AssumedTierTokensInNamedRuns_ShowsLabelledCount(t *testing.T) {
	view := runsView(assumedRunsReport(5_555, 0))

	assertLabelledCount(t, view, "5,555")
}

func TestRunsScreen_AssumedTierTokensOnlyInUnattributable_ShowsLabelledCount(t *testing.T) {
	view := runsView(assumedRunsReport(0, 3_333))

	assertLabelledCount(t, view, "3,333")
}

func TestRunsScreen_AssumedTierInNamedRunsAndUnattributable_ShowsSummedCount(t *testing.T) {
	view := runsView(assumedRunsReport(4_321, 8_765))

	assertLabelledCount(t, view, "13,086")
}

func TestRunsScreen_NoAssumedTierTokens_ShowsNoAssumedLabel(t *testing.T) {
	view := runsView(assumedRunsReport(0, 0))

	if strings.Contains(strings.ToLower(view), "assumed") {
		t.Errorf("runs screen mentions an assumed tier although none was used; view:\n%s", view)
	}
}

func TestRunDetailScreen_RunWithAssumedTierTokens_ShowsLabelledCount(t *testing.T) {
	run := domain.RunReport{
		Run:          domain.NamedRun("20260101T120000Z-abcd"),
		Totals:       domain.Totals{AssumedTierCacheWrite: 9_876},
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
	}

	assertLabelledCount(t, detailView(run), "9,876")
}

func TestRunDetailScreen_AffectedActor_IsLabelledAndOthersAreNot(t *testing.T) {
	run := domain.RunReport{
		Run:          domain.NamedRun("20260101T120000Z-abcd"),
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		Agents: []domain.ActorReport{
			{Actor: domain.AgentInstance("Alpha#1"), Totals: domain.Totals{AssumedTierCacheWrite: 2_222}},
			{Actor: domain.AgentInstance("Beta#2")},
		},
	}
	run.ByAgentType = scrollDispatchesOf(run.Agents)

	view := detailView(run)

	if alpha := lineContaining(view, "Alpha#1"); !strings.Contains(strings.ToLower(alpha), "assumed") {
		t.Errorf("affected actor label line %q does not mention the assumed tier; view:\n%s", alpha, view)
	}
	if beta := lineContaining(view, "Beta#2"); strings.Contains(strings.ToLower(beta), "assumed") {
		t.Errorf("unaffected actor label line %q mentions the assumed tier", beta)
	}
}

func TestRunDetailScreen_NoAssumedTierTokens_ShowsNoAssumedLabel(t *testing.T) {
	run := domain.RunReport{
		Run:            domain.NamedRun("20260101T120000Z-abcd"),
		Orchestrator:   domain.ActorReport{Actor: domain.Orchestrator()},
		Agents:         []domain.ActorReport{{Actor: domain.AgentInstance("Alpha#1")}},
		UnpricedModels: []domain.PricingKey{{Model: "claude-mystery-model"}},
	}

	view := detailView(run)

	if strings.Contains(strings.ToLower(view), "assumed") {
		t.Errorf("run-detail screen mentions an assumed tier although none was used; view:\n%s", view)
	}
}
