package screens_test

// The findings overlay inside a run, over every per-run view (dispatches,
// overview, by type, by status, tools, timeline, context and the session
// screen): f opens it, f and esc close it, it takes every key while open, and it
// shows the findings of the run only.

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// findingsView is what every per-run view offers for the findings overlay.
type findingsView interface {
	screens.RunView
	FindingsOpen() bool
	FindingsScope() screens.FindingsScope
	SetRunFindings([]domain.Finding)
}

const viewW, viewH = 200, 60

// findingsViewCase builds one per-run view of the fixture run.
type findingsViewCase struct {
	name string
	make func(t *testing.T, w, h int) findingsView
}

func findingsViewCases() []findingsViewCase {
	var cases []findingsViewCase
	for _, fv := range allFrameViews() {
		cases = append(cases, findingsViewCase{string(fv.id), func(t *testing.T, w, h int) findingsView {
			colourOff(t)
			return fv.make(fv.run, w, h, paletteStyles()).(findingsView)
		}})
	}
	return append(cases,
		findingsViewCase{"context", func(t *testing.T, w, h int) findingsView {
			colourOff(t)
			return screens.NewContextScreen(contextRun(), w, h, paletteStyles())
		}},
		findingsViewCase{"session", func(t *testing.T, w, h int) findingsView {
			return ctxDrill(t, contextRun(), "builder#1", w, h)
		}},
	)
}

// viewFindingsRun is the run every per-run fixture view shows.
func viewFindingsRun() domain.RunRef { return overviewRun().Run }

// otherRun is a run whose findings must never show in the view of viewFindingsRun.
func otherRun() domain.RunRef { return domain.NamedRun("20260102T120000Z-bbbb") }

// ownQuality is the quality summary of a report with findings of the shown run and of other runs.
func ownQuality() domain.QualitySummary {
	return domain.NewQualitySummary(findingsFor(viewFindingsRun(), otherRun()))
}

// withFindings gives the view the findings of its own run, as the model does.
func withFindings(v findingsView) findingsView {
	v.SetRunFindings(screens.RunFindingsOf(ownQuality(), viewFindingsRun()))
	return v
}

func TestRunFindings_EveryPerRunViewOffersTheOverlay(t *testing.T) {
	var _ findingsView = (*screens.RunDetailScreen)(nil)
	var _ findingsView = (*screens.OverviewScreen)(nil)
	var _ findingsView = (*screens.AgentTypesScreen)(nil)
	var _ findingsView = (*screens.StatusScreen)(nil)
	var _ findingsView = (*screens.ToolsScreen)(nil)
	var _ findingsView = (*screens.TimelineScreen)(nil)
	var _ findingsView = (*screens.ContextScreen)(nil)
	var _ findingsView = (*screens.SessionScreen)(nil)
}

func TestRunFindings_FOpensTheOverlayInPlaceOfTheBodyAndKeepsTheFrame(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))
			before := v.View(viewW, viewH)

			overlay := press(v, screens.KeyFindings).View(viewW, viewH)

			if !v.FindingsOpen() {
				t.Fatal("f must open the findings overlay")
			}
			if overlay == before {
				t.Fatal("f must change the view")
			}
			lines := framedLines(t, overlay)
			assertContains(t, lines[0], viewFindingsRun().Label())
			assertWithinBounds(t, overlay, viewW, viewH)
		})
	}
}

func TestRunFindings_RowsCoverTheRunsOwnFindingsOnly(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))

			overlay := press(v, screens.KeyFindings).View(viewW, viewH)

			assertRow(t, overlay, domain.FindingCrossStreamRecord, "warning", "2", "")
			assertRow(t, overlay, domain.FindingMalformedLine, "warning", "2", "")
			assertRow(t, overlay, domain.FindingMissingModel, "info", "1", "")
			assertRowOrder(t, overlay, domain.FindingCrossStreamRecord, domain.FindingMalformedLine, domain.FindingMissingModel)
			plain := stripStyling(overlay)
			assertNotContains(t, plain, string(domain.FindingInvalidPricingEntry), string(futureKind))
			assertNoFindingDetails(t, overlay)
		})
	}
}

func TestRunFindings_ScopeIsTheSliceTheViewWasGiven(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))

			scope := v.FindingsScope()

			want := screens.RunFindingsOf(ownQuality(), viewFindingsRun())
			if len(want) != 5 {
				t.Fatalf("setup: the run has %d findings, want 5", len(want))
			}
			if !reflect.DeepEqual(scope.Findings, want) {
				t.Errorf("scope covers %d findings, want the %d of the run", len(scope.Findings), len(want))
			}
			if scope.Label != viewFindingsRun().Label() {
				t.Errorf("Label = %q, want the run label %q", scope.Label, viewFindingsRun().Label())
			}
			if got := screens.RunQualityOf(ownQuality(), viewFindingsRun()).Findings; len(scope.Findings) != got {
				t.Errorf("scope has %d findings, the header badge counts %d", len(scope.Findings), got)
			}
		})
	}
}

func TestRunFindings_ReplacingTheFindingsRefreshesAnOpenOverlay(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))
			press(v, screens.KeyFindings)

			v.SetRunFindings([]domain.Finding{findingOf(domain.FindingMissingModel, domain.SeverityInfo, viewFindingsRun())})

			overlay := stripStyling(v.View(viewW, viewH))
			assertContains(t, overlay, string(domain.FindingMissingModel))
			assertNotContains(t, overlay, string(domain.FindingMalformedLine))
			if !v.FindingsOpen() {
				t.Error("a reprice must leave the overlay open")
			}
		})
	}
}

func TestRunFindings_NoFindingsShowsASingleNoFindingsLine(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := c.make(t, viewW, viewH)

			overlay := stripStyling(press(v, screens.KeyFindings).View(viewW, viewH))

			assertContains(t, overlay, "no findings")
		})
	}
}

func TestRunFindings_FAndEscCloseItAndRestoreTheView(t *testing.T) {
	for _, c := range findingsViewCases() {
		for _, closer := range []string{screens.KeyFindings, "esc"} {
			t.Run(c.name+"/"+closer, func(t *testing.T) {
				v := withFindings(c.make(t, viewW, viewH))
				before := v.View(viewW, viewH)

				open := press(v, screens.KeyFindings).View(viewW, viewH)
				if !v.FindingsOpen() || open == before {
					t.Fatal("f must open the overlay and change the view before it can be closed")
				}
				after := press(v, closer).View(viewW, viewH)

				if v.FindingsOpen() {
					t.Error("the overlay must be closed")
				}
				if after != before {
					t.Errorf("must restore the view\n%s\nvs\n%s", after, before)
				}
				if v.Back() {
					t.Error("closing the overlay must not leave the view")
				}
			})
		}
	}
}

func TestRunFindings_EscLeavesTheViewOnlyAfterTheOverlayIsClosed(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))

			press(v, screens.KeyFindings, "esc")
			if v.Back() || v.FindingsOpen() {
				t.Fatalf("the first esc must close the overlay only (back=%v open=%v)", v.Back(), v.FindingsOpen())
			}
			press(v, "esc")

			if !v.Back() {
				t.Error("esc after closing the overlay must leave the view")
			}
		})
	}
}

func TestRunFindings_EveryOtherKeyIsSwallowedWhileOpen(t *testing.T) {
	keys := []string{
		screens.KeyViewDispatches, screens.KeyViewOverview, screens.KeyViewAgentTypes, screens.KeyViewStatus,
		screens.KeyViewTools, screens.KeyViewTimeline, screens.KeyViewContext, "q", "enter", screens.KeySort,
		screens.KeyToggleView, screens.KeyGroup, screens.KeyMarkUserTool, screens.KeyExportRun, screens.KeyExportAll,
		screens.KeyHelp, "s", "a", "p",
	}
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			for _, k := range keys {
				v := withFindings(c.make(t, viewW, viewH))
				before := v.View(viewW, viewH)
				open := press(v, screens.KeyFindings).View(viewW, viewH)

				press(v, k)

				if !v.FindingsOpen() {
					t.Errorf("%q must not close the overlay", k)
				}
				if _, requested := v.Requested(); requested || v.Back() {
					t.Errorf("%q must not request another view or leave the view while the overlay is open", k)
				}
				if got := v.View(viewW, viewH); got != open {
					t.Errorf("%q must change nothing while the overlay is open", k)
				}
				if after := press(v, screens.KeyFindings).View(viewW, viewH); after != before {
					t.Errorf("%q leaked through the overlay into the view", k)
				}
			}
		})
	}
}

func TestRunFindings_HelpOverlayWinsWhileOpenAndFIsConsumedByIt(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))
			before := v.View(viewW, viewH)
			help := openOverlay(v, viewW, viewH)

			press(v, screens.KeyFindings)

			if v.FindingsOpen() {
				t.Error("f while the help overlay is open must not open the findings overlay")
			}
			if got := v.View(viewW, viewH); got != help {
				t.Error("f must change nothing while the help overlay is open")
			}
			if after := press(v, screens.KeyHelp).View(viewW, viewH); after != before {
				t.Error("closing the help overlay must show the view again")
			}
		})
	}
}

func TestRunFindings_QuestionMarkWhileOpenDoesNotOpenTheHelpOverlay(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))
			open := press(v, screens.KeyFindings).View(viewW, viewH)

			if got := press(v, screens.KeyHelp).View(viewW, viewH); got != open {
				t.Error("? must do nothing inside the findings overlay")
			}
		})
	}
}

func TestRunFindings_ScrollsAMoreRowsThanFitOnAShortTerminalWithinBounds(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := c.make(t, 120, 14)
			v.SetRunFindings(manyKindFindings(30, viewFindingsRun()))

			first := press(v, screens.KeyFindings).View(120, 14)
			assertWithinBounds(t, first, 120, 14)
			assertContains(t, stripStyling(first), "kind_00")
			assertNotContains(t, stripStyling(first), "kind_29")

			bottom := press(v, repeat("down", 100)...).View(120, 14)
			assertWithinBounds(t, bottom, 120, 14)
			assertContains(t, stripStyling(bottom), "kind_29")

			if top := press(v, repeat("up", 100)...).View(120, 14); !strings.Contains(stripStyling(top), "kind_00") {
				t.Errorf("up must scroll back to the first row\n%s", top)
			}
		})
	}
}

func TestRunFindings_ShortHelpListsFWhileClosedAndTheClosingKeysWhileOpen(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, viewH))

			if line := footerLine(v.View(viewW, viewH)); !hasEntry(line, "f findings") {
				t.Errorf("footer %q must list the entry %q", line, "f findings")
			}
			if help := v.Help(); !hasEntry(help, "f findings") {
				t.Errorf("Help() = %q must list %q", help, "f findings")
			}

			line := footerLine(press(v, screens.KeyFindings).View(viewW, viewH))
			assertContains(t, line, "f/esc close", "scroll")
			assertNotContains(t, line, "f findings")
		})
	}
}

func TestRunFindings_HelpOverlayListsTheFindingsKey(t *testing.T) {
	for _, c := range findingsViewCases() {
		t.Run(c.name, func(t *testing.T) {
			v := withFindings(c.make(t, viewW, legendOverlayH))

			overlay := strings.Join(overlayLines(openOverlay(v, viewW, legendOverlayH)), "\n")

			if !regexp.MustCompile(`(?m)^\s+f\s+.*(?i:finding)`).MatchString(overlay) {
				t.Errorf("the help overlay must list the findings key\n%s", overlay)
			}
		})
	}
}

func TestFindingsKey_DoesNotCollideWithAnyPerRunViewKey(t *testing.T) {
	for _, k := range []string{
		screens.KeyViewDispatches, screens.KeyViewOverview, screens.KeyViewAgentTypes, screens.KeyViewStatus,
		screens.KeyViewTools, screens.KeyViewTimeline, screens.KeyViewContext, screens.KeySort, screens.KeyToggleView,
		screens.KeyGroup, screens.KeyMarkUserTool, screens.KeyHelp, screens.KeyExportRun, screens.KeyExportAll,
		"q", "up", "down", "j", "k", "enter", "esc",
	} {
		if screens.KeyFindings == k {
			t.Errorf("the findings key collides with %q", k)
		}
	}
}
