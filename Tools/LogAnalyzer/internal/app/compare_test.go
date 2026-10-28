package app_test

// Tests for the run comparison entry point of the app layer: it finds both
// named runs in a report and compares them; an unknown run id is reported, not
// guessed.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// compareRun builds a named run with one agent type of the given size.
func compareRun(id string, tokens, nanos int64) domain.RunReport {
	totals := domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(tokens)},
		Money:  domain.CategoryMoney{Total: domain.KnownMoney(domain.Money(nanos)), Complete: true},
	}
	return domain.RunReport{
		Run:      domain.NamedRun(id),
		Totals:   totals,
		Overview: domain.RunOverview{Subagents: domain.GroupFigures{Invocations: 1, Totals: totals}},
		ByAgentType: []domain.AgentTypeGroup{{
			AgentType:  "coder",
			Figures:    domain.GroupFigures{Invocations: 1, Totals: totals},
			Dispatches: []domain.Dispatch{{Totals: totals}},
		}},
	}
}

func compareReport() domain.Report {
	return domain.Report{Runs: []domain.RunReport{
		compareRun("20261028T100000Z-aaaa", 1000, 3_000_000),
		compareRun("20261028T110000Z-bbbb", 2000, 6_000_000),
	}}
}

func TestCompareRuns_ComparesTheTwoNamedRunsOfTheReport(t *testing.T) {
	report := compareReport()

	got, ok := app.CompareRuns(report, "20261028T100000Z-aaaa", "20261028T110000Z-bbbb")

	if !ok {
		t.Fatal("ok = false, want both runs found")
	}
	want := analysis.CompareRuns(report.Runs[0], report.Runs[1])
	if want.A != domain.NamedRun("20261028T100000Z-aaaa") || len(want.Rows) != 1 {
		t.Fatalf("analysis comparison is not populated: %+v", want)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("comparison differs from analysis.CompareRuns of the same runs:\n got %+v\nwant %+v", got, want)
	}
}

func TestCompareRuns_FirstArgumentIsRunA(t *testing.T) {
	got, ok := app.CompareRuns(compareReport(), "20261028T110000Z-bbbb", "20261028T100000Z-aaaa")

	if !ok {
		t.Fatal("ok = false, want both runs found")
	}
	if got.A.ID != "20261028T110000Z-bbbb" || got.B.ID != "20261028T100000Z-aaaa" {
		t.Errorf("runs = %q / %q, want the arguments' order", got.A.ID, got.B.ID)
	}
}

func TestCompareRuns_UnknownRunIdIsReportedAsNotFound(t *testing.T) {
	report := compareReport()

	if _, ok := app.CompareRuns(report, "20261028T100000Z-aaaa", "20261028T120000Z-cccc"); ok {
		t.Error("ok = true for an unknown run B")
	}
	if _, ok := app.CompareRuns(report, "20261028T120000Z-cccc", "20261028T100000Z-aaaa"); ok {
		t.Error("ok = true for an unknown run A")
	}
}

func TestCompareRuns_UnattributableBucketIsNotANamedRun(t *testing.T) {
	report := compareReport()
	bucket := compareRun("", 500, 1_500_000)
	bucket.Run = domain.UnattributableRun()
	report.Unattributable = &bucket

	if _, ok := app.CompareRuns(report, "20261028T100000Z-aaaa", domain.UnknownRunDirName); ok {
		t.Error("ok = true when comparing against the unattributable bucket")
	}
}
