package screens_test

// Behaviour of the across-runs view: the runs side by side with their totals,
// the per-agent-type totals and per-run averages, labels for partial and
// provisional data, exclusion of the unattributable bucket, back and scrolling.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func acrossView(report domain.Report, w, h int) string {
	return screens.NewAcrossRunsScreen(report, w, h, plain()).View(w, h)
}

func TestAcrossRuns_IdentifiesItself(t *testing.T) {
	if got := screens.NewAcrossRunsScreen(acrossReport(), wideW, tallH, plain()).ID(); got != screens.ScreenAcrossRuns {
		t.Errorf("across-runs screen ID = %q, want %q", got, screens.ScreenAcrossRuns)
	}
}

func TestAcrossRuns_ListsEveryRunWithItsTotalOnOneRow(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	for id, total := range map[string]string{acrossRunA: "$10.00", acrossRunB: "$6.00", acrossRunC: "$3.21"} {
		if row := rowFor(t, view, id); !strings.Contains(row, total) {
			t.Errorf("row of run %s = %q, want its total %s", id, row, total)
		}
	}
}

func TestAcrossRuns_ListsRunsInTheGivenOrder(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	a, b, c := lineIndex(view, acrossRunA), lineIndex(view, acrossRunB), lineIndex(view, acrossRunC)

	if a < 0 || !(a < b && b < c) {
		t.Errorf("runs must be listed in report order (rows %d, %d, %d)\n%s", a, b, c, view)
	}
}

func TestAcrossRuns_ShowsAgentTypeTotalAndPerRunAverageOnItsRow(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	for typ, amounts := range map[string][]string{
		"coder":  {"$16.00", "$5.33"},
		"tester": {"$2.40", "$1.20"},
	} {
		row := rowFor(t, view, typ)
		for _, want := range amounts {
			if !strings.Contains(row, want) {
				t.Errorf("row of type %s = %q, want %s", typ, row, want)
			}
		}
	}
}

func TestAcrossRuns_ShowsTheInvocationCountOnTheAgentTypeRow(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	if row := rowFor(t, view, "coder"); !strings.Contains(row, "47") {
		t.Errorf("row of type coder = %q, want its invocation count 47", row)
	}
}

func TestAcrossRuns_UnknownTypeIsListedWithItsFigures(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	row := rowFor(t, view, screens.UnknownTypeLabel)

	if !strings.Contains(row, "$0.77") {
		t.Errorf("unknown type row = %q, want its total $0.77", row)
	}
}

func TestAcrossRuns_PartialAndProvisionalDataIsLabelled(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	assertContains(t, view,
		"(partial)",
		screens.ProvisionalLabel,
		screens.AssumedTierLabel,
		screens.OpenLabel,
		screens.MissingUsageLabel,
		"m-unpriced",
	)
	if row := rowFor(t, view, "coder"); !strings.Contains(row, screens.MissingUsageLabel) {
		t.Errorf("row of type coder = %q, want the missing-usage label", row)
	}
	if row := rowFor(t, view, acrossRunC); !strings.Contains(row, "(partial)") {
		t.Errorf("row of the partial run = %q, want a partial marker", row)
	}
	for _, id := range []string{acrossRunA, acrossRunB} {
		if row := rowFor(t, view, id); strings.Contains(row, "(partial)") {
			t.Errorf("row of the complete run %s = %q, must not be marked partial", id, row)
		}
	}
	if row := rowFor(t, view, "tester"); strings.Contains(row, "(partial)") {
		t.Errorf("row of the fully priced type tester = %q, must not be marked partial", row)
	}
}

func TestAcrossRuns_CompleteDataShowsNoIncompletenessLabel(t *testing.T) {
	view := acrossView(acrossCompleteReport(), wideW, tallH)

	assertContains(t, view, acrossRunA, acrossRunB, "$8.00")
	assertNotContains(t, view,
		"(partial)", screens.UnpricedMarker, screens.ProvisionalLabel, screens.AssumedTierLabel, screens.OpenLabel,
		screens.MissingUsageLabel,
	)
}

func TestAcrossRuns_NeverShowsTheUnattributableBucket(t *testing.T) {
	view := acrossView(acrossReport(), wideW, tallH)

	assertContains(t, view, acrossRunA, acrossRunB, acrossRunC, "coder")
	assertNotContains(t, view, "$99.99", screens.UnattributableLabel)
}

func TestAcrossRuns_EscAndQAskToLeave(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		s := screens.NewAcrossRunsScreen(acrossReport(), wideW, tallH, plain())
		press(s, k)

		if !s.Back() {
			t.Errorf("key %q must set Back()", k)
		}
		s.Reset()
		if s.Back() {
			t.Errorf("Reset must clear Back after %q", k)
		}
	}
}

func TestAcrossRuns_HelpListsBack(t *testing.T) {
	help := screens.NewAcrossRunsScreen(acrossReport(), wideW, tallH, plain()).Help()

	if !strings.Contains(help, "esc") {
		t.Errorf("help %q must list esc", help)
	}
}

func TestAcrossRuns_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := screens.NewAcrossRunsScreen(manyAcrossRunsReport(30), wideW, shortH, plain())

	first := s.View(wideW, shortH)
	assertFits(t, first, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertContains(t, first, "run-01")
	assertNotContains(t, first, "run-30")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 120)...)
	last := scrolled.View(wideW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "run-30")
	assertNotContains(t, last, "run-01")

	if back := press(scrolled, repeat("up", 120)...).View(wideW, shortH); back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestAcrossRuns_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := acrossView(manyAcrossRunsReport(30), wideW, 200)

	assertContains(t, view, "run-01", "run-30")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestAcrossRuns_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40

	view := acrossView(acrossReport(), narrow, tallH)

	assertContains(t, view, "coder")
	assertFitsWidth(t, view, narrow)
}

func TestAcrossRuns_ReportWithoutRunsStillRendersAPage(t *testing.T) {
	view := acrossView(domain.Report{Currency: domain.Currency}, wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a report without runs must still render a page")
	}
}
