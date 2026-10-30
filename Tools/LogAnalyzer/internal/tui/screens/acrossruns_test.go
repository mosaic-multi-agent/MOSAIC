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

// acrossView renders the across-runs view with colour off and the application's palette.
func acrossView(t *testing.T, report domain.Report, w, h int) string {
	t.Helper()
	colourOff(t)
	return screens.NewAcrossRunsScreen(report, w, h, paletteStyles()).View(w, h)
}

func TestAcrossRuns_IdentifiesItself(t *testing.T) {
	if got := screens.NewAcrossRunsScreen(acrossReport(), wideW, tallH, paletteStyles()).ID(); got != screens.ScreenAcrossRuns {
		t.Errorf("across-runs screen ID = %q, want %q", got, screens.ScreenAcrossRuns)
	}
}

func TestAcrossRuns_ListsEveryRunWithItsTotalOnOneRow(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	for id, total := range map[string]string{acrossRunA: "$10.00", acrossRunB: "$6.00", acrossRunC: "$3.21"} {
		if row := rowFor(t, view, id); !strings.Contains(row, total) {
			t.Errorf("row of run %s = %q, want its total %s", id, row, total)
		}
	}
}

func TestAcrossRuns_ListsTheMostExpensiveRunFirstByDefault(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	assertAppearInOrder(t, view, []string{acrossRunA, acrossRunB, acrossRunC}, true)
}

func TestAcrossRuns_ShowsAgentTypeTotalAndPerRunAverageOnItsRow(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

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
	view := acrossView(t, acrossReport(), tableW, tallH)

	assertFields(t, rowFor(t, view, "coder"), "47")
}

func TestAcrossRuns_UnknownTypeIsListedWithItsFigures(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	assertContains(t, rowFor(t, view, screens.UnknownTypeLabel), "$0.77")
}

func TestAcrossRuns_PartialAndProvisionalDataIsLabelled(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	assertContains(t, view,
		"(partial)",
		screens.ProvisionalLabel,
		screens.AssumedTierLabel(nil),
		screens.OpenLabel,
		screens.MissingUsageLabel,
		"m-unpriced",
	)
	assertContains(t, rowFor(t, view, "coder"), screens.MissingUsageLabel)
	assertContains(t, rowFor(t, view, acrossRunC), "(partial)")
	for _, id := range []string{acrossRunA, acrossRunB} {
		assertNotContains(t, rowFor(t, view, id), "(partial)")
	}
	assertNotContains(t, rowFor(t, view, "tester"), "(partial)")
}

func TestAcrossRuns_CompleteDataShowsNoIncompletenessLabel(t *testing.T) {
	view := acrossView(t, acrossCompleteReport(), tableW, tallH)

	assertContains(t, view, acrossRunA, acrossRunB, "$8.00")
	for _, needle := range []string{acrossRunA, acrossRunB, "coder"} {
		assertNotContains(t, rowFor(t, view, needle),
			"(partial)", screens.UnpricedMarker, screens.ProvisionalLabel, screens.AssumedTierLabel(nil), screens.OpenLabel,
			screens.MissingUsageLabel,
		)
	}
}

func TestAcrossRuns_NeverShowsTheUnattributableBucket(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	assertContains(t, view, acrossRunA, acrossRunB, acrossRunC, "coder")
	assertNotContains(t, view, "$99.99", screens.UnattributableLabel)
}

func TestAcrossRuns_EscAndQAskToLeave(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		s := screens.NewAcrossRunsScreen(acrossReport(), wideW, tallH, paletteStyles())
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
	help := screens.NewAcrossRunsScreen(acrossReport(), wideW, tallH, paletteStyles()).Help()

	if !strings.Contains(help, "esc") {
		t.Errorf("help %q must list esc", help)
	}
}

func TestAcrossRuns_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	colourOff(t)
	s := screens.NewAcrossRunsScreen(manyAcrossRunsReport(30), wideW, shortH, paletteStyles())

	// Cost sort lists the most expensive run (run-30) first and the cheapest (run-01) last.
	first := s.View(wideW, shortH)
	assertWithinBounds(t, first, wideW, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertContains(t, first, "run-30")
	assertNotContains(t, first, "run-01")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 120)...)
	last := scrolled.View(wideW, shortH)
	assertWithinBounds(t, last, wideW, shortH)
	assertContains(t, last, "run-01")
	assertNotContains(t, last, "run-30")

	if back := press(scrolled, repeat("up", 120)...).View(wideW, shortH); back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestAcrossRuns_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := acrossView(t, manyAcrossRunsReport(30), wideW, 200)

	assertContains(t, view, "run-01", "run-30")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestAcrossRuns_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40

	view := acrossView(t, acrossReport(), narrow, tallH)

	assertNameShown(t, view, "coder", 3)
	assertWithinBounds(t, view, narrow, tallH)
}

func TestAcrossRuns_ReportWithoutRunsStillRendersAPage(t *testing.T) {
	view := acrossView(t, domain.Report{Currency: domain.Currency}, wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a report without runs must still render a page")
	}
	assertContains(t, strings.ToLower(view), "no named runs")
	assertWithinBounds(t, view, wideW, tallH)
}
