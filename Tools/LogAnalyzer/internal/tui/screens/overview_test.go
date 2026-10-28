package screens_test

// Behaviour of the run overview screen that holds in both versions (visual and
// figures-only): figures, incompleteness labels, scrolling and key requests.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func lineWith(view, needle string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}

func TestOverview_ShowsRunTotalInBothVersions(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			assertContains(t, v.view(overviewRun(), tallH), "$10.00")
		})
	}
}

func TestOverview_FiguresOnlyShowsOrchestratorAndSubagentShareAndAmount(t *testing.T) {
	view := overviewFiguresView(overviewRun(), tallH)

	assertContains(t, view, "40.0%", "60.0%", "$4.00", "$6.00")
	assertContains(t, strings.ToLower(view), "orchestrator", "subagent")
}

func TestOverview_FiguresOnlyShowsEveryTokenCategoryWithTokensAndMoney(t *testing.T) {
	view := overviewFiguresView(overviewRun(), tallH)

	for _, cat := range domain.BillableCategories() {
		assertContains(t, view, cat.Label())
	}
	assertContains(t, view, "1,000", "3,000", "500", "200")
	assertContains(t, view, "$1.11", "$2.22", "$3.33", "$3.34")
}

func TestOverview_VisualNamesEveryTokenCategory(t *testing.T) {
	view := strings.ToLower(overviewView(overviewRun(), tallH))

	for _, cat := range domain.BillableCategories() {
		assertContains(t, view, strings.ToLower(cat.Label()))
	}
}

func TestOverview_ShowsFiguresPerModelInBothVersions(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(overviewRun(), tallH)

			assertContains(t, view, "model-a", "$6.50", "model-b", "$2.75", screens.UnknownModelLabel, "$0.75")
		})
	}
}

func TestOverview_ShowsCacheHitRatioInBothVersions(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(overviewRun(), tallH)

			assertContains(t, view, "66.7%")
			assertContains(t, strings.ToLower(view), "cache hit")
		})
	}
}

func TestOverview_UndefinedRatiosShowAbsentMarkerNotZeroPercent(t *testing.T) {
	run := overviewRun()
	run.Overview.OrchestratorShare = domain.Ratio{}
	run.Overview.SubagentShare = domain.Ratio{}
	run.Overview.CacheHitRatio = domain.Ratio{}

	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(run, tallH)

			line := firstLineMatching(view, `(?i)cache hit`)
			if strings.Contains(line, "%") {
				t.Errorf("cache hit line %q must show no percentage", line)
			}
			if window := windowAround(view, `(?i)cache hit`, 1); !strings.Contains(window, screens.AbsentMarker) {
				t.Errorf("cache hit figure must show the absent marker\n%s", window)
			}
		})
	}
	figures := overviewFiguresView(run, tallH)
	for _, pattern := range []string{`(?i)orchestrator`, `(?i)subagent`} {
		line := firstLineMatching(figures, pattern)
		if !strings.Contains(line, screens.AbsentMarker) || strings.Contains(line, "%") {
			t.Errorf("%s line %q must show the absent marker and no percentage", pattern, line)
		}
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

	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(run, tallH)

			assertContains(t, view, "$5.00 (partial)")
			if line := lineWith(view, "model-b"); !strings.Contains(line, screens.UnpricedMarker) {
				t.Errorf("model-b line %q must carry %q", line, screens.UnpricedMarker)
			}
		})
	}
}

func TestOverview_CompleteRunHasNoIncompletenessLabels(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(overviewRun(), tallH)

			assertNotContains(t, view, "(partial)", screens.UnpricedMarker, screens.AssumedTierLabel,
				screens.MissingUsageLabel)
		})
	}
}

func TestOverview_LabelsAssumedTierCacheWrites(t *testing.T) {
	run := overviewRun()
	run.Overview.Totals.AssumedTierCacheWrite = 12345
	run.Totals = run.Overview.Totals

	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(run, tallH)

			if line := lineWith(view, "12,345"); !strings.Contains(line, screens.AssumedTierLabel) {
				t.Errorf("assumed-tier count line %q must carry %q", line, screens.AssumedTierLabel)
			}
		})
	}
}

// The word "open" also names a share of the outcome split, so the open
// invocations label is recognised by the extra occurrence it adds.
func TestOverview_LabelsMissingUsageAndOpenInvocations(t *testing.T) {
	run := overviewRun()
	run.Overview.Subagents.MissingUsage = 1
	run.Overview.Subagents.Open = 2

	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(run, tallH)

			assertContains(t, view, screens.MissingUsageLabel)
			if with, without := countOf(view, screens.OpenLabel), countOf(v.view(overviewRun(), tallH), screens.OpenLabel); with <= without {
				t.Errorf("open invocations must add a %q label (%d occurrences, %d without)", screens.OpenLabel, with, without)
			}
		})
	}
}

func TestOverview_ShortTerminalScrollsToEveryModelAndShowsHint(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			s := v.open(manyModelsRun(20), wideW, shortH)

			first := s.View(wideW, shortH)
			assertFits(t, first, shortH)
			if !scrollHintRE.MatchString(first) {
				t.Errorf("short terminal must show a scroll hint\n%s", first)
			}
			all := scrollThrough(s, wideW, shortH, 300)
			for i := 1; i <= 20; i++ {
				if name := fmt.Sprintf("model-%02d", i); !strings.Contains(all, name) {
					t.Errorf("scrolling never reaches %s", name)
				}
			}
			assertFits(t, s.View(wideW, shortH), shortH)

			back := press(s, repeat("up", 300)...).View(wideW, shortH)
			if back != first {
				t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
			}
		})
	}
}

func TestOverview_TallTerminalShowsEveryModelWithoutHint(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(manyModelsRun(20), 200)

			assertContains(t, view, "model-01", "model-20")
			if scrollHintRE.MatchString(view) {
				t.Errorf("no scroll hint expected when everything fits\n%s", view)
			}
		})
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
	for _, v := range overviewVersions() {
		for _, c := range cases {
			s := v.open(overviewRun(), wideW, tallH).(screens.RunView)
			press(s, c.key)

			got, ok := s.Requested()
			if !ok || got != c.want {
				t.Errorf("%s, key %q: Requested() = (%q, %v), want (%q, true)", v.name, c.key, got, ok, c.want)
			}
			s.Reset()
			if _, ok := s.Requested(); ok {
				t.Errorf("%s, key %q: Reset must clear the request", v.name, c.key)
			}
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

func TestOverview_HelpHintsAtTheOverviewListingViewKeysAndBack(t *testing.T) {
	s := screens.NewOverviewScreen(overviewRun(), wideW, tallH, plain())

	assertContains(t, s.Help(), screens.KeyHelp)
	assertOverlayListsViewKeys(t, s, screens.KeyViewDispatches, screens.KeyViewAgentTypes)
}
