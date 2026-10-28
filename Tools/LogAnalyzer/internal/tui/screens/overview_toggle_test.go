package screens_test

// The toggle between the visual and the figures-only Overview: the key, what the
// figures-only version shows (every figure as text), and that both versions stay
// within the terminal with every marker still in the text.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestOverviewToggle_KeyIsTheLetterVAndCollidesWithNoExistingKey(t *testing.T) {
	if screens.KeyToggleView != "v" {
		t.Errorf("KeyToggleView = %q, want v", screens.KeyToggleView)
	}
	for _, k := range []string{"1", "2", "3", "4", "5", "6", "a", "s", "p", "q", "e", "E", "up", "down", "j", "k",
		"enter", "esc", screens.KeyHelp, screens.KeySort} {
		if k == screens.KeyToggleView {
			t.Errorf("the toggle key collides with %q", k)
		}
	}
}

func TestOverviewToggle_StartsVisualSwitchesToFiguresOnlyAndBack(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(analysesRun(), wideW, overviewTallH, paletteStyles())

	visual := s.View(wideW, overviewTallH)
	press(s, screens.KeyToggleView)
	figures := s.View(wideW, overviewTallH)
	press(s, screens.KeyToggleView)
	again := s.View(wideW, overviewTallH)

	assertContains(t, visual, "▓")
	assertNotContains(t, figures, "▓", "▒", "░")
	if figures == visual {
		t.Error("the toggle must change the view")
	}
	if again != visual {
		t.Errorf("toggling twice must restore the visual version\n%s\nvs\n%s", again, visual)
	}
}

func TestOverviewToggle_IsNotARequestToLeaveOrToSwitchViews(t *testing.T) {
	s := screens.NewOverviewScreen(analysesRun(), wideW, tallH, paletteStyles())

	press(s, screens.KeyToggleView)

	if _, ok := s.Requested(); ok || s.Back() {
		t.Error("the toggle must stay on the Overview")
	}
}

func TestOverviewToggle_KeyIsListedInTheHelpOverlay(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(analysesRun(), 120, tallH, paletteStyles())

	overlay := strings.Join(overlayLines(openOverlay(s, 120, tallH)), "\n")

	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeyToggleView) + `\s+\S`).MatchString(overlay) {
		t.Errorf("the help overlay must list the toggle key %q\n%s", screens.KeyToggleView, overlay)
	}
}

func TestOverviewFiguresOnly_ShowsEveryOverviewFigureAsText(t *testing.T) {
	colourOff(t)
	view := overviewFiguresView(withUnattributed(analysesRun()), overviewTallH)

	assertContains(t, view,
		// run total, shares, unattributed
		"$10.00", "$4.00", "$6.00", "40.0%", "60.0%", "16.7%", "$2.00",
		// token categories with money, reasoning inside output
		"1,000", "3,000", "500", "200", "123", "$1.11", "$2.22", "$3.33", "$3.34",
		// models
		"model-a", "$6.50", "model-b", "$2.75", screens.UnknownModelLabel, "$0.75",
		// cache: hit ratio, net benefit, unread writes
		"66.7%", "$12.34", "777", "$4.56",
		// wall time durations and counted tools
		screens.FormatDuration(25*time.Minute, true), screens.FormatDuration(66*time.Minute, true),
		screens.FormatDuration(19*time.Minute, true), screens.FormatDuration(10*time.Minute, true),
		"AskUserQuestion", "mcp__contact-user__ask_user_questions")
	assertFields(t, view, "17", "6", "4", "27", "5", "2")
	assertMatches(t, view, `(?i)permission notifications cannot be paired`, `(?i)mcp contact tools may be missing`)
	if !hasOpenEndedCount(view, "3") {
		t.Errorf("the figures-only version must count the open-ended waits\n%s", view)
	}
}

func TestOverviewFiguresOnly_NamesTheShareOfEachOutcomeAndWallTimeCategory(t *testing.T) {
	colourOff(t)

	view := strings.ToLower(overviewFiguresView(analysesRun(), overviewTallH))

	assertContains(t, view, "success", "other", "waiting", "subagents", "orchestrator only", "between sessions")
}

func TestOverviewFiguresOnly_LabelsUnreadCacheWritesAsReportedData(t *testing.T) {
	colourOff(t)

	window := windowAround(overviewFiguresView(analysesRun(), overviewTallH), unreadWritePattern, 2)

	assertContains(t, strings.ToLower(window), "reported data")
	assertContains(t, window, "777", "$4.56")
}

func TestOverviewFiguresOnly_NegativeBenefitIsBadAndAbsentFiguresShowTheMarker(t *testing.T) {
	colourOn(t)
	run := negativeBenefit()
	run.Cache.UnreadWriteTokens = domain.AbsentTokens()

	view := overviewFiguresView(run, overviewTallH)

	assertNegativeBenefitInRole(t, rawLineMatching(t, view, negativeBenefitPattern), "signal-bad")
	assertContains(t, windowAround(view, unreadWritePattern, 2), screens.AbsentMarker)
}

// markerRun carries every marker the Overview shows: partial total, unpriced
// model and unread cost, assumed tier, missing usage, open invocations and
// absent figures.
func markerRun() domain.RunReport {
	run := analysesRun()
	run.Provisional = true
	run.UnpricedModels = []domain.PricingKey{{Model: "model-b"}}
	run.Overview.Totals.Money.Total = known(500)
	run.Overview.Totals.Money.Complete = false
	run.Overview.Totals.AssumedTierCacheWrite = 4321
	run.Overview.Subagents.MissingUsage = 1
	run.Overview.Subagents.Open = 2
	run.Overview.OrchestratorShare = domain.Ratio{}
	run.Overview.ByModel[1].Priced = false
	run.Overview.ByModel[1].Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	run.Cache.NetBenefit = domain.UnpricedMoney()
	run.Cache.UnreadWriteCost = domain.UnpricedMoney()
	run.Cache.UnreadWriteTokens = domain.AbsentTokens()
	run.Totals = run.Overview.Totals
	return run
}

func TestOverview_WithColourOffEveryMarkerRemainsAsTextInBothVersions(t *testing.T) {
	colourOff(t)
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(markerRun(), overviewTallH)

			if view != stripStyling(view) {
				t.Errorf("colour off must leave no escape sequences: %q", view)
			}
			assertContains(t, view, screens.ProvisionalLabel, screens.UnpricedMarker, "(partial)",
				screens.AssumedTierLabel, screens.MissingUsageLabel, screens.OpenLabel, screens.AbsentMarker)
			assertNotContains(t, view, "$0.00")
		})
	}
}

func TestOverview_NothingIsDrawnPastTheTerminalInEitherVersion(t *testing.T) {
	colourOff(t)
	for _, v := range overviewVersions() {
		for _, width := range []int{120, 100, 80, 60, 40, 30} {
			for _, height := range []int{5, 8, 12, 24, 40} {
				s := v.open(withUnattributed(markerRun()), width, height)

				assertWithinBounds(t, s.View(width, height), width, height)
				press(s, repeat("down", 7)...)
				assertWithinBounds(t, s.View(width, height), width, height)
			}
		}
	}
}

func TestOverview_IsReadableAtEightyColumnsAndTwentyFourRowsByScrolling(t *testing.T) {
	colourOff(t)
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			s := v.open(analysesRun(), 80, 24)

			first := s.View(80, 24)
			all := scrollThrough(s, 80, 24, 120)

			assertWithinBounds(t, first, 80, 24)
			if !scrollHintRE.MatchString(first) {
				t.Errorf("a body taller than the terminal must show the scroll hint\n%s", first)
			}
			assertContains(t, all, "66.7%", "model-a", "$6.50", screens.FormatDuration(25*time.Minute, true))
			assertMatches(t, all, `(?i)cannot be paired`, `(?i)may be missing`)
		})
	}
}
