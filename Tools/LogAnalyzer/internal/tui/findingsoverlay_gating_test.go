package tui

// findingsoverlay_gating_test.go verifies that the open findings overlay takes
// every key before the app-level hooks see it: on the runs list the source (s),
// across-runs (a) and pricing (p) keys and the export-all key (E) do
// nothing, and so does it in every per-run view (e opens the findings export). With the
// overlay closed the same keys work, which proves the test setup is live.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// findingsOverlay is what both the runs list and every per-run view offer.
type findingsOverlay interface {
	FindingsOpen() bool
	FindingsScope() screens.FindingsScope
}

const digestPrompt = "Digest file path:"

// requireRunsOverlayOpen presses f on the runs list and checks that the overlay opened.
func requireRunsOverlayOpen(t *testing.T, m Model) Model {
	t.Helper()
	m = press(m, screens.KeyFindings)
	if m.runsScreen == nil || !m.runsScreen.FindingsOpen() {
		t.Fatal("f on the runs list must open the findings overlay")
	}
	return m
}

func TestFindingsOverlay_RunsList_AppLevelKeysDoNothingWhileOpen(t *testing.T) {
	for _, key := range []string{"s", screens.KeyAcrossRuns, "p", screens.KeyExportAll} {
		t.Run(key, func(t *testing.T) {
			store := newTestDigestStore(false)
			m := requireRunsOverlayOpen(t, newExportModel(t, store))

			m = press(m, key)

			if m.screen != screenRuns {
				t.Errorf("screen = %q, want the runs list while the overlay is open", m.screen)
			}
			if !m.runsScreen.FindingsOpen() {
				t.Error("the overlay must stay open")
			}
			if strings.Contains(m.View(), digestPrompt) {
				t.Errorf("a digest export started while the overlay is open")
			}
			if len(store.writes) != 0 {
				t.Errorf("%d digest write(s) while the overlay is open", len(store.writes))
			}
		})
	}
}

func TestFindingsOverlay_RunsList_SameKeysWorkOnceItIsClosed(t *testing.T) {
	cases := map[string]screenName{
		"s": screenSource, screens.KeyAcrossRuns: screenAcrossRuns,
		"p": screenPricing, screens.KeyExportRun: screenExport, screens.KeyExportAll: screenExport,
	}
	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			m := requireRunsOverlayOpen(t, newExportModel(t, newTestDigestStore(false)))
			m = press(m, screens.KeyFindings)
			if m.runsScreen.FindingsOpen() {
				t.Fatal("f must close the overlay")
			}

			m = press(m, key)

			if m.screen != want {
				t.Errorf("screen = %q, want %q with the overlay closed", m.screen, want)
			}
		})
	}
}

func TestFindingsOverlay_RunsList_EscClosesTheOverlayAndStaysOnTheList(t *testing.T) {
	m := requireRunsOverlayOpen(t, newExportModel(t, newTestDigestStore(false)))

	m = pressType(m, tea.KeyEsc)

	if m.screen != screenRuns || m.runsScreen.FindingsOpen() {
		t.Errorf("esc: screen = %q open = %v, want the runs list with the overlay closed", m.screen, m.runsScreen.FindingsOpen())
	}
}

// perRunViewKeys switches from the run detail to each per-run view.
func perRunViewKeys() map[string]string {
	return map[string]string{
		"dispatches": screens.KeyViewDispatches, "overview": screens.KeyViewOverview,
		"agent types": screens.KeyViewAgentTypes, "status": screens.KeyViewStatus, "tools": screens.KeyViewTools,
		"timeline": screens.KeyViewTimeline, "context": screens.KeyViewContext,
	}
}

// openPerRunViewWithOverlay opens the second run, switches to the view and presses f.
func openPerRunViewWithOverlay(t *testing.T, key string) (Model, *tuiFakeDigestStore) {
	t.Helper()
	store := newTestDigestStore(false)
	m := openRunDetailOfSecondRun(newExportModel(t, store))
	m = press(m, key)
	if m.screen == exportScreenName {
		t.Fatalf("setup: view key %q did not open a per-run view", key)
	}
	shown := m.screen
	m = press(m, screens.KeyFindings)
	view, ok := m.runView.(findingsOverlay)
	if !ok || !view.FindingsOpen() {
		t.Fatalf("f on the %q view must open the findings overlay", shown)
	}
	return m, store
}

func TestFindingsOverlay_PerRunViews_ExportKeysDoNothingWhileOpen(t *testing.T) {
	for name, viewKey := range perRunViewKeys() {
		for _, key := range []string{screens.KeyExportRun, screens.KeyExportAll} {
			t.Run(name+"/"+key, func(t *testing.T) {
				m, store := openPerRunViewWithOverlay(t, viewKey)
				shown := m.screen

				m = press(m, key)

				if strings.Contains(m.View(), digestPrompt) {
					t.Error("a digest export started while the overlay is open")
				}
				if key == screens.KeyExportAll && m.screen != shown {
					t.Errorf("screen = %q, want %q", m.screen, shown)
				}
				if !m.runView.(findingsOverlay).FindingsOpen() {
					t.Error("the overlay must stay open")
				}
				if len(store.writes) != 0 {
					t.Errorf("%d digest write(s) while the overlay is open", len(store.writes))
				}
			})
		}
	}
}

func TestFindingsOverlay_PerRunViews_OtherAppKeysDoNothingWhileOpen(t *testing.T) {
	for name, viewKey := range perRunViewKeys() {
		t.Run(name, func(t *testing.T) {
			for _, key := range []string{"s", screens.KeyAcrossRuns, "p"} {
				m, _ := openPerRunViewWithOverlay(t, viewKey)
				shown := m.screen
				shownView := m.View()

				m = press(m, key)

				if m.screen != shown {
					t.Errorf("%q moved the screen from %q to %q while the overlay is open", key, shown, m.screen)
				}
				if got := m.View(); got != shownView {
					t.Errorf("%q changed the view while the overlay is open", key)
				}
			}
		})
	}
}

func TestFindingsOverlay_PerRunViews_ExportWorksOnceItIsClosed(t *testing.T) {
	for name, viewKey := range perRunViewKeys() {
		t.Run(name, func(t *testing.T) {
			m, _ := openPerRunViewWithOverlay(t, viewKey)
			m = press(m, screens.KeyFindings)

			m = press(m, screens.KeyExportRun)

			requireExportScreen(t, m)
		})
	}
}

func TestFindingsOverlay_PerRunViews_ViewKeysAreSwallowedWhileOpen(t *testing.T) {
	m, _ := openPerRunViewWithOverlay(t, screens.KeyViewOverview)
	shown := m.screen

	for _, key := range []string{screens.KeyViewTools, screens.KeyViewTimeline, "q"} {
		m = press(m, key)
		if m.screen != shown {
			t.Fatalf("%q changed the screen from %q to %q while the overlay is open", key, shown, m.screen)
		}
	}
}

// findingsReportFor is a report of two named runs with findings in each, in the
// unattributable bucket and tied to no run.
func findingsReportFor(runA, runB string) domain.Report {
	r := makeReport(runA, runB)
	a, b := domain.NamedRun(runA), domain.NamedRun(runB)
	w := domain.SeverityWarning
	r.Quality = domain.NewQualitySummary([]domain.Finding{
		{Kind: domain.FindingCrossStreamRecord, Severity: w, Run: a},
		{Kind: domain.FindingCrossStreamRecord, Severity: w, Run: a},
		{Kind: domain.FindingMalformedLine, Severity: w, Run: b},
		{Kind: domain.FindingMissingModel, Severity: domain.SeverityInfo, Run: b},
		{Kind: domain.FindingInvalidPricingEntry, Severity: w, Run: domain.UnattributableRun()},
		{Kind: domain.FindingInvalidSettingsEntry, Severity: w},
	})
	return r
}

// findingsModel is a model on the runs list over findingsReportFor.
func findingsModel() Model {
	m := newNavModel()
	m.width, m.height = 200, 40
	m.report = findingsReportFor(exportRunA, exportRunB)
	m.runsScreen = screens.NewRunsScreen(m.report, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenRuns
	return m
}

func TestFindingsOverlay_PerRunViews_ShowOnlyTheRunsOwnFindings(t *testing.T) {
	for name, id := range map[string]screens.ScreenID{
		"dispatches": screens.ScreenRunDetail, "overview": screens.ScreenOverview, "agent types": screens.ScreenAgentTypes,
		"status": screens.ScreenStatus, "tools": screens.ScreenTools, "timeline": screens.ScreenTimeline,
		"context": screens.ScreenContext,
	} {
		t.Run(name, func(t *testing.T) {
			m := findingsModel().openRunView(exportRunB, id)

			m = press(m, screens.KeyFindings)

			view, ok := m.runView.(findingsOverlay)
			if !ok || !view.FindingsOpen() {
				t.Fatal("f must open the findings overlay")
			}
			scope := view.FindingsScope()
			if scope.Label != exportRunB {
				t.Errorf("scope label = %q, want %q", scope.Label, exportRunB)
			}
			want := screens.RunFindingsOf(m.report.Quality, domain.NamedRun(exportRunB))
			if len(want) != 2 || len(scope.Findings) != len(want) {
				t.Fatalf("scope has %d findings, want the %d of the run", len(scope.Findings), len(want))
			}
			out := m.runView.View(m.width, m.height)
			for _, k := range []domain.FindingKind{domain.FindingMalformedLine, domain.FindingMissingModel} {
				if !strings.Contains(out, string(k)) {
					t.Errorf("the overlay must show the run's kind %q\n%s", k, out)
				}
			}
			for _, k := range []domain.FindingKind{domain.FindingCrossStreamRecord, domain.FindingInvalidPricingEntry, domain.FindingInvalidSettingsEntry} {
				if strings.Contains(out, string(k)) {
					t.Errorf("the overlay must not show %q, which is not a finding of this run\n%s", k, out)
				}
			}
		})
	}
}

func TestFindingsOverlay_RunsList_ScopeIsEveryFindingOfTheReport(t *testing.T) {
	m := findingsModel()

	m = press(m, screens.KeyFindings)

	scope := m.runsScreen.FindingsScope()
	if scope.Label != "" {
		t.Errorf("scope label = %q, want empty on the runs list", scope.Label)
	}
	if want := len(m.report.Quality.Findings); want != 6 || len(scope.Findings) != want {
		t.Errorf("scope has %d findings, want all %d of the report", len(scope.Findings), want)
	}
}
