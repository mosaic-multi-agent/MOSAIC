package screens_test

// Behaviour of the run overview screen: figures, incompleteness labels,
// scrolling and key requests.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func overviewView(run domain.RunReport, h int) string {
	return screens.NewOverviewScreen(run, wideW, h, plain()).View(wideW, h)
}

func lineWith(view, needle string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}

func TestOverview_ShowsRunTotal(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	assertContains(t, view, "$10.00")
}

func TestOverview_ShowsOrchestratorAndSubagentShare(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	assertContains(t, view, "40.0%", "60.0%", "$4.00", "$6.00")
	assertContains(t, strings.ToLower(view), "orchestrator", "subagent")
}

func TestOverview_ShowsEveryTokenCategoryWithTokensAndMoney(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	for _, cat := range domain.BillableCategories() {
		assertContains(t, view, cat.Label())
	}
	assertContains(t, view, "1,000", "3,000", "500", "200")
	assertContains(t, view, "$1.11", "$2.22", "$3.33", "$3.34")
}

func TestOverview_ShowsFiguresPerModel(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	assertContains(t, view, "model-a", "$6.50", "model-b", "$2.75", screens.UnknownModelLabel, "$0.75")
}

func TestOverview_ShowsCacheHitRatio(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	assertContains(t, view, "66.7%")
	assertContains(t, strings.ToLower(view), "cache hit")
}

func TestOverview_UndefinedRatiosShowAbsentMarkerNotZeroPercent(t *testing.T) {
	run := overviewRun()
	run.Overview.OrchestratorShare = domain.Ratio{}
	run.Overview.SubagentShare = domain.Ratio{}
	run.Overview.CacheHitRatio = domain.Ratio{}

	view := overviewView(run, tallH)

	assertNotContains(t, view, "%")
	assertContains(t, view, screens.AbsentMarker)
	line := lineWith(strings.ToLower(view), "cache hit")
	if !strings.Contains(line, screens.AbsentMarker) {
		t.Errorf("cache hit line %q must show the absent marker", line)
	}
}

func TestOverview_UnpricedModelMarksFiguresPartial(t *testing.T) {
	run := overviewRun()
	run.UnpricedModels = []domain.PricingKey{{Model: "model-b"}}
	run.Overview.Totals.Money.Total = known(500)
	run.Overview.Totals.Money.Complete = false
	run.Totals = run.Overview.Totals
	run.Overview.ByModel[1] = domain.ModelFigures{
		Model: "model-b", Priced: false, Requests: 2,
		Totals: domain.Totals{Money: domain.CategoryMoney{Total: domain.UnpricedMoney()}},
	}

	view := overviewView(run, tallH)

	assertContains(t, view, "$5.00 (partial)")
	if line := lineWith(view, "model-b"); !strings.Contains(line, screens.UnpricedMarker) {
		t.Errorf("model-b line %q must carry %q", line, screens.UnpricedMarker)
	}
}

func TestOverview_CompleteRunHasNoIncompletenessLabels(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	assertNotContains(t, view, "(partial)", screens.UnpricedMarker, screens.AssumedTierLabel,
		screens.MissingUsageLabel, screens.OpenLabel)
}

func TestOverview_LabelsAssumedTierCacheWrites(t *testing.T) {
	run := overviewRun()
	run.Overview.Totals.AssumedTierCacheWrite = 12345
	run.Totals = run.Overview.Totals

	view := overviewView(run, tallH)

	if line := lineWith(view, "12,345"); !strings.Contains(line, screens.AssumedTierLabel) {
		t.Errorf("assumed-tier count line %q must carry %q", line, screens.AssumedTierLabel)
	}
}

func TestOverview_LabelsMissingUsageAndOpenInvocations(t *testing.T) {
	run := overviewRun()
	run.Overview.Subagents.MissingUsage = 1
	run.Overview.Subagents.Open = 2

	view := overviewView(run, tallH)

	assertContains(t, view, screens.MissingUsageLabel, screens.OpenLabel)
}

func TestOverview_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	run := manyModelsRun(20)
	s := screens.NewOverviewScreen(run, wideW, shortH, plain())

	first := s.View(wideW, shortH)
	assertFits(t, first, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertNotContains(t, first, "model-20")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 80)...)
	last := scrolled.View(wideW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "model-20")
	assertNotContains(t, last, "model-01")

	back := press(scrolled, repeat("up", 80)...).View(wideW, shortH)
	if back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestOverview_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := overviewView(manyModelsRun(20), 200)

	assertContains(t, view, "model-01", "model-20")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestOverview_ViewKeysRequestSiblingViews(t *testing.T) {
	cases := []struct {
		key  string
		want screens.ScreenID
	}{
		{screens.KeyViewDispatches, screens.ScreenRunDetail},
		{screens.KeyViewAgentTypes, screens.ScreenAgentTypes},
	}
	for _, c := range cases {
		s := screens.NewOverviewScreen(overviewRun(), wideW, tallH, plain())
		press(s, c.key)

		got, ok := s.Requested()
		if !ok || got != c.want {
			t.Errorf("key %q: Requested() = (%q, %v), want (%q, true)", c.key, got, ok, c.want)
		}
		s.Reset()
		if _, ok := s.Requested(); ok {
			t.Errorf("key %q: Reset must clear the request", c.key)
		}
	}
}

func TestOverview_OwnViewKeyRequestsNothing(t *testing.T) {
	s := screens.NewOverviewScreen(overviewRun(), wideW, tallH, plain())
	press(s, screens.KeyViewOverview)

	if _, ok := s.Requested(); ok {
		t.Error("pressing the key of the current view must not request a switch")
	}
}

func TestOverview_EscAndQAskToLeave(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		s := screens.NewOverviewScreen(overviewRun(), wideW, tallH, plain())
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

func TestOverview_HelpListsViewKeysAndBack(t *testing.T) {
	help := screens.NewOverviewScreen(overviewRun(), wideW, tallH, plain()).Help()

	assertContains(t, help, screens.KeyViewDispatches, screens.KeyViewAgentTypes, "esc")
}
