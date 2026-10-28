package screens_test

// Behaviour of the by-agent-type list: figures per type, incompleteness
// labels, ordering, scrolling and key requests. Drill-down behaviour is in
// agenttype_drill_test.go.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func typesView(run domain.RunReport, h int) string {
	return screens.NewAgentTypesScreen(run, tableW, h, plain()).View(tableW, h)
}

func TestAgentTypes_ShowsFiguresPerType(t *testing.T) {
	view := typesView(agentTypesRun(), tallH)

	// builder: 12 invocations, total / average / min / max.
	assertContains(t, view, "builder", "12", "$60.25", "$5.02", "$1.37", "$9.64")
	// reviewer: 15 invocations, total / average / min / max.
	assertContains(t, view, "reviewer", "15", "$20.50", "$1.36", "$0.45", "$3.88")
}

func TestAgentTypes_ListsTypesInGivenOrderWithUnknownTypeLabelled(t *testing.T) {
	view := typesView(agentTypesRun(), tallH)

	b, r, u := strings.Index(view, "builder"), strings.Index(view, "reviewer"), strings.Index(view, screens.UnknownTypeLabel)
	if b < 0 || r < 0 || u < 0 {
		t.Fatalf("builder, reviewer and %q must all be listed\n%s", screens.UnknownTypeLabel, view)
	}
	if b >= r || r >= u {
		t.Errorf("types must appear in order builder, reviewer, unknown (got %d, %d, %d)", b, r, u)
	}
	assertContains(t, view, "21", "$7.77")
}

func TestAgentTypes_CompleteRunHasNoIncompletenessLabels(t *testing.T) {
	view := typesView(agentTypesRun(), tallH)

	assertNotContains(t, view, "(partial)", screens.UnpricedMarker, screens.AssumedTierLabel,
		screens.MissingUsageLabel, screens.OpenLabel)
}

func TestAgentTypes_PartialStatisticsAreMarkedPartial(t *testing.T) {
	run := agentTypesRun()
	run.ByAgentType[0].PerInvocation.Complete = false // the group total itself stays complete

	view := typesView(run, tallH)

	assertContains(t, view, "(partial)")
}

func TestAgentTypes_UnpricedGroupShowsUnpricedMarkerAndAbsentStatistics(t *testing.T) {
	run := agentTypesRun()
	g := &run.ByAgentType[1]
	g.Figures.Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	g.Figures.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	g.PerInvocation = domain.MoneyStats{Min: domain.NoMoneyData(), Max: domain.NoMoneyData(), Average: domain.NoMoneyData()}

	view := typesView(run, tallH)

	assertContains(t, view, screens.UnpricedMarker, screens.AbsentMarker)
	assertNotContains(t, view, "$20.50")
}

func TestAgentTypes_LabelsAssumedTierCacheWrites(t *testing.T) {
	run := agentTypesRun()
	run.ByAgentType[0].Figures.Totals.AssumedTierCacheWrite = 4321

	view := typesView(run, tallH)

	if line := lineWith(view, "4,321"); !strings.Contains(line, screens.AssumedTierLabel) {
		t.Errorf("assumed-tier count line %q must carry %q", line, screens.AssumedTierLabel)
	}
}

func TestAgentTypes_LabelsMissingUsageAndOpenInvocations(t *testing.T) {
	run := agentTypesRun()
	run.ByAgentType[1].Figures.MissingUsage = 1
	run.ByAgentType[1].Figures.Open = 2

	view := typesView(run, tallH)

	assertContains(t, view, screens.MissingUsageLabel, screens.OpenLabel)
}

func TestAgentTypes_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := screens.NewAgentTypesScreen(manyTypesRun(25), wideW, shortH, plain())

	first := s.View(wideW, shortH)
	assertFits(t, first, shortH)
	assertContains(t, first, "type-01")
	assertNotContains(t, first, "type-25")
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}

	var cur screens.Screen = s
	cur = press(cur, repeat("down", 30)...)
	last := cur.View(wideW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "type-25")
	assertNotContains(t, last, "type-01")
}

func TestAgentTypes_EnterDrillsIntoTheTypeUnderTheCursorAfterScrolling(t *testing.T) {
	s := screens.NewAgentTypesScreen(manyTypesRun(25), wideW, shortH, plain())

	press(s, append(repeat("down", 3), "enter")...)

	got, ok := s.DrilledType()
	if !ok || got != "type-04" {
		t.Errorf("DrilledType() = (%q, %v), want (\"type-04\", true)", got, ok)
	}
}

func TestAgentTypes_ViewKeysRequestSiblingViews(t *testing.T) {
	cases := []struct {
		key  string
		want screens.ScreenID
	}{
		{screens.KeyViewDispatches, screens.ScreenRunDetail},
		{screens.KeyViewOverview, screens.ScreenOverview},
	}
	for _, c := range cases {
		s := screens.NewAgentTypesScreen(agentTypesRun(), wideW, tallH, plain())
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

func TestAgentTypes_OwnViewKeyRequestsNothing(t *testing.T) {
	s := screens.NewAgentTypesScreen(agentTypesRun(), wideW, tallH, plain())
	press(s, screens.KeyViewAgentTypes)

	if _, ok := s.Requested(); ok {
		t.Error("pressing the key of the current view must not request a switch")
	}
}

func TestAgentTypes_EscAndQAskToLeave(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		s := screens.NewAgentTypesScreen(agentTypesRun(), wideW, tallH, plain())
		press(s, k)

		if !s.Back() {
			t.Errorf("key %q must set Back()", k)
		}
	}
}

func TestAgentTypes_HelpHintsAtTheOverviewListingViewKeysAndBack(t *testing.T) {
	s := screens.NewAgentTypesScreen(agentTypesRun(), wideW, tallH, plain())

	assertContains(t, s.Help(), screens.KeyHelp)
	assertOverlayListsViewKeys(t, s, screens.KeyViewDispatches, screens.KeyViewOverview)
}

func TestAgentTypes_RunWithoutSubagentsRendersAndCannotDrillDown(t *testing.T) {
	run := overviewRun() // no ByAgentType
	s := screens.NewAgentTypesScreen(run, wideW, tallH, plain())

	view := s.View(wideW, tallH)
	press(s, "enter")

	if strings.TrimSpace(view) == "" {
		t.Error("view of a run without subagents must not be blank")
	}
	if _, ok := s.DrilledType(); ok {
		t.Error("Enter on an empty list must not drill down")
	}
}

func TestRunDetail_ViewKeysRequestPerRunViews(t *testing.T) {
	cases := []struct {
		key  string
		want screens.ScreenID
	}{
		{screens.KeyViewOverview, screens.ScreenOverview},
		{screens.KeyViewAgentTypes, screens.ScreenAgentTypes},
	}
	for _, c := range cases {
		s := screens.NewRunDetailScreen(overviewRun(), wideW, tallH, plain())
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

func TestRunDetail_OwnViewKeyRequestsNothing(t *testing.T) {
	s := screens.NewRunDetailScreen(overviewRun(), wideW, tallH, plain())
	press(s, screens.KeyViewDispatches)

	if _, ok := s.Requested(); ok {
		t.Error("pressing the key of the current view must not request a switch")
	}
}

func TestRunDetail_HelpHintsAtTheOverviewListingViewKeys(t *testing.T) {
	s := screens.NewRunDetailScreen(overviewRun(), wideW, tallH, plain())

	assertContains(t, s.Help(), screens.KeyHelp)
	assertOverlayListsViewKeys(t, s, screens.KeyViewOverview, screens.KeyViewAgentTypes)
}
