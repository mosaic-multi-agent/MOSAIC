package screens_test

// Fixture and helpers of the findings overlay tests: findings of two named runs,
// the unattributable bucket and no run at all, spread over kinds so that every
// count, run count and sort tie-break is distinguishable.
//
// Expected per-kind rows over everything (runs list):
//
//	malformed_line         warning  4  in 2 runs   (a twice, b, unattributable)
//	cross_stream_record    warning  3  in 2 runs
//	invalid_pricing_entry  warning  1  in no run   (tied to no run)
//	future_kind            info     1  in 1 run    (unknown kind: raw name)
//	missing_model          info     1  in 1 run
//
// Expected rows of run a only: cross_stream_record 2, malformed_line 2,
// missing_model 1.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	futureKind  = domain.FindingKind("future_kind")
	secretPath  = "logs/a/secret-file-one.jsonl"
	secretPath2 = "logs/a/secret-file-two.jsonl"
	secretText  = "detail-text-xyz"
)

func findingOf(kind domain.FindingKind, sev domain.FindingSeverity, run domain.RunRef) domain.Finding {
	return domain.Finding{Kind: kind, Severity: sev, Run: run}
}

// findingsFor returns the fixture findings for the named runs a and b.
func findingsFor(a, b domain.RunRef) []domain.Finding {
	w, i := domain.SeverityWarning, domain.SeverityInfo
	return []domain.Finding{
		findingOf(domain.FindingCrossStreamRecord, w, a),
		findingOf(domain.FindingCrossStreamRecord, w, a),
		findingOf(domain.FindingCrossStreamRecord, w, b),
		{Kind: domain.FindingMalformedLine, Severity: w, Run: a, Path: secretPath, Line: 42, Detail: secretText},
		{Kind: domain.FindingMalformedLine, Severity: w, Run: a, Path: secretPath2, Line: 7, Detail: secretText},
		findingOf(domain.FindingMalformedLine, w, b),
		findingOf(domain.FindingMalformedLine, w, domain.UnattributableRun()),
		findingOf(domain.FindingInvalidPricingEntry, w, domain.RunRef{}),
		findingOf(domain.FindingMissingModel, i, a),
		findingOf(futureKind, i, b),
	}
}

// listedFindingsReport is the runs-list fixture with the findings of runIDOne and
// runIDTwo in its quality summary.
func listedFindingsReport() domain.Report {
	r := runsReport()
	r.Quality = domain.NewQualitySummary(findingsFor(domain.NamedRun(runIDOne), domain.NamedRun(runIDTwo)))
	return r
}

func newFindingsRuns(width, height int) *screens.RunsScreen {
	return screens.NewRunsScreen(listedFindingsReport(), width, height, paletteStyles())
}

// rowLine returns the first styling-free line of view that names the kind.
func rowLine(t *testing.T, view string, kind domain.FindingKind) string {
	t.Helper()
	for _, l := range viewLines(view) {
		if strings.Contains(l, string(kind)) {
			return l
		}
	}
	t.Fatalf("no row for kind %q\n%s", kind, stripStyling(view))
	return ""
}

// rowIndex is the line number of the kind's row, or -1.
func rowIndex(view string, kind domain.FindingKind) int {
	for i, l := range viewLines(view) {
		if strings.Contains(l, string(kind)) {
			return i
		}
	}
	return -1
}

// assertRowOrder fails unless every kind has a row and the rows are in the order given.
func assertRowOrder(t *testing.T, view string, kinds ...domain.FindingKind) {
	t.Helper()
	prev := -1
	for _, k := range kinds {
		at := rowIndex(view, k)
		if at < 0 {
			t.Fatalf("no row for kind %q\n%s", k, stripStyling(view))
		}
		if at <= prev {
			t.Errorf("row of %q is at line %d, want it after line %d\n%s", k, at, prev, stripStyling(view))
		}
		prev = at
	}
}

// assertRow checks one row: its severity label, its count as a separate figure,
// its explanation and the "in N runs" text ("" = the row must not carry one).
func assertRow(t *testing.T, view string, kind domain.FindingKind, severity string, count, runsText string) {
	t.Helper()
	line := rowLine(t, view, kind)
	rest := strings.Replace(line, kind.Explanation(), "", 1)
	if !strings.Contains(line, kind.Explanation()) {
		t.Errorf("row of %q lacks its explanation %q\n%s", kind, kind.Explanation(), line)
	}
	if !strings.Contains(line, severity) {
		t.Errorf("row of %q lacks the severity %q\n%s", kind, severity, line)
	}
	if !regexp.MustCompile(`\b` + count + `\b`).MatchString(strings.Replace(rest, runsText, "", 1)) {
		t.Errorf("row of %q lacks the count %s\n%s", kind, count, line)
	}
	inRuns := regexp.MustCompile(`in \d+ runs?`).FindString(rest)
	if inRuns != runsText {
		t.Errorf("row of %q has run text %q, want %q\n%s", kind, inRuns, runsText, line)
	}
}

// assertNoFindingDetails fails when an individual finding's path or detail shows.
func assertNoFindingDetails(t *testing.T, view string) {
	t.Helper()
	assertNotContains(t, stripStyling(view), secretPath, secretPath2, secretText)
}

// manyKindFindings is one warning finding of each of n synthetic kinds, named
// kind_00, kind_01, ...: more rows than a short terminal holds, all with the same
// count and severity so that they are listed by kind name.
func manyKindFindings(n int, run domain.RunRef) []domain.Finding {
	var out []domain.Finding
	for i := 0; i < n; i++ {
		out = append(out, findingOf(domain.FindingKind(fmt.Sprintf("kind_%02d", i)), domain.SeverityWarning, run))
	}
	return out
}
