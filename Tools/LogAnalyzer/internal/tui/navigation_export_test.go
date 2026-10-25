package tui

// navigation_export_test.go verifies how the digest export action is started
// (runs screen, run-detail and the per-run views), how the scope is chosen
// (selected run versus all runs), path entry and cancelling.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

var exportScreenName = screenName(screens.ScreenExport)

func press(m Model, key string) Model {
	updated, _ := pressKey(m, key)
	return asModel(updated)
}

func pressType(m Model, kt tea.KeyType) Model {
	updated, _ := pressKeyType(m, kt)
	return asModel(updated)
}

// openRunDetailOfSecondRun selects the second run (exportRunB) on the runs screen and opens it.
func openRunDetailOfSecondRun(m Model) Model {
	m = press(m, "j")
	return pressType(m, tea.KeyEnter)
}

// replacePath clears the pre-filled path of length n and types p.
func replacePath(m Model, n int, p string) Model {
	for i := 0; i < n; i++ {
		m = pressType(m, tea.KeyBackspace)
	}
	return press(m, p)
}

func requireExportScreen(t *testing.T, m Model) {
	t.Helper()
	if m.screen != exportScreenName {
		t.Fatalf("screen = %q, want the export screen", m.screen)
	}
}

func TestExport_RunsScreen_E_OpensExportForHighlightedRun(t *testing.T) {
	m := newExportModel(t, newTestDigestStore(false))
	m = press(m, "j") // highlight the second run

	m = press(m, "e")

	requireExportScreen(t, m)
	view := m.View()
	for _, want := range []string{"Export run " + exportRunB, exportRunB + ".digest.json", "Digest file path:"} {
		if !strings.Contains(view, want) {
			t.Errorf("export view missing %q; got:\n%s", want, view)
		}
	}
}

func TestExport_RunsScreen_CapitalE_OpensExportForAllRuns(t *testing.T) {
	m := newExportModel(t, newTestDigestStore(false))

	m = press(m, "E")

	requireExportScreen(t, m)
	view := m.View()
	for _, want := range []string{"Export all runs", "runs.digest.json"} {
		if !strings.Contains(view, want) {
			t.Errorf("export view missing %q; got:\n%s", want, view)
		}
	}
}

func TestExport_RunDetail_E_ExportsTheShownRun(t *testing.T) {
	m := openRunDetailOfSecondRun(newExportModel(t, newTestDigestStore(false)))
	if m.screen != screenRunDetail {
		t.Fatalf("setup: screen = %q, want run-detail", m.screen)
	}

	m = press(m, "e")

	requireExportScreen(t, m)
	if view := m.View(); !strings.Contains(view, "Export run "+exportRunB) {
		t.Errorf("export view should name the shown run %s; got:\n%s", exportRunB, view)
	}
}

func TestExport_RunDetail_CapitalE_ExportsAllRuns(t *testing.T) {
	m := openRunDetailOfSecondRun(newExportModel(t, newTestDigestStore(false)))

	m = press(m, "E")

	requireExportScreen(t, m)
	if view := m.View(); !strings.Contains(view, "Export all runs") {
		t.Errorf("export view should offer all runs; got:\n%s", view)
	}
}

func TestExport_PerRunViews_E_ExportsTheShownRun(t *testing.T) {
	views := map[string]string{
		"overview": screens.KeyViewOverview, "agent types": screens.KeyViewAgentTypes,
		"status": screens.KeyViewStatus, "tools": screens.KeyViewTools, "timeline": screens.KeyViewTimeline,
	}
	for name, key := range views {
		t.Run(name, func(t *testing.T) {
			m := openRunDetailOfSecondRun(newExportModel(t, newTestDigestStore(false)))
			m = press(m, key)
			if m.screen == screenRunDetail || m.screen == exportScreenName {
				t.Fatalf("setup: view key %q did not switch view (screen %q)", key, m.screen)
			}

			m = press(m, "e")

			requireExportScreen(t, m)
			if view := m.View(); !strings.Contains(view, "Export run "+exportRunB) {
				t.Errorf("export view should name the shown run; got:\n%s", view)
			}
		})
	}
}

func TestExport_NoSession_KeysDoNothing(t *testing.T) {
	m := newNavModel()
	m.screen = screenRuns
	m.report = makeReport(exportRunA)
	m.runsScreen = screens.NewRunsScreen(m.report, m.width, m.height, stylesFromTheme(m.theme))

	for _, key := range []string{"e", "E"} {
		m2 := press(m, key)
		if m2.screen != screenRuns {
			t.Errorf("key %q without a loaded session: screen = %q, want runs", key, m2.screen)
		}
	}
}

func TestExport_RunsScreen_E_WithoutNamedRun_ShowsNoRunSelected(t *testing.T) {
	store := newTestDigestStore(true)
	store.digest.Aggregate.Runs = nil // only the unattributable bucket
	m := newExportModel(t, store)

	m = press(m, "e")

	if m.screen != screenRuns {
		t.Fatalf("screen = %q, want runs (nothing opened)", m.screen)
	}
	if view := m.View(); !strings.Contains(view, "No run selected") {
		t.Errorf("runs view should say 'No run selected'; got:\n%s", view)
	}
}

func TestExport_PathEntry_Esc_ReturnsToOriginWithCancelStatus(t *testing.T) {
	store := newTestDigestStore(false)
	for _, origin := range []struct {
		name  string
		open  func(Model) Model
		build screenName
	}{
		{"runs", func(m Model) Model { return m }, screenRuns},
		{"run-detail", openRunDetailOfSecondRun, screenRunDetail},
	} {
		t.Run(origin.name, func(t *testing.T) {
			m := origin.open(newExportModel(t, store))
			m = press(m, "e")
			requireExportScreen(t, m)

			m = pressType(m, tea.KeyEsc)

			if m.screen != origin.build {
				t.Fatalf("screen = %q, want %q", m.screen, origin.build)
			}
			view := m.View()
			if !strings.Contains(view, "Export cancelled") {
				t.Errorf("origin view should show 'Export cancelled'; got:\n%s", view)
			}
			if strings.Contains(view, "nothing written") {
				t.Errorf("Esc on path entry should show plain 'Export cancelled'; got:\n%s", view)
			}
			if len(store.writes) != 0 {
				t.Errorf("cancel wrote %d file(s), want none", len(store.writes))
			}
		})
	}
}

func TestExport_StatusClearsOnNextKeyPress(t *testing.T) {
	m := newExportModel(t, newTestDigestStore(false))
	m = press(m, "e")
	m = pressType(m, tea.KeyEsc)
	if !strings.Contains(m.View(), "Export cancelled") {
		t.Fatal("setup: cancel status not shown")
	}

	m = press(m, "j")

	if view := m.View(); strings.Contains(view, "Export cancelled") {
		t.Errorf("status should clear on the next key press; got:\n%s", view)
	}
}

func TestExport_PathEntry_RejectsInvalidPaths(t *testing.T) {
	cases := []struct {
		name, typed, wantErr string
	}{
		{"empty", "", "path cannot be empty"},
		{"wrong suffix", "out.json", "path must end in .digest.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := newTestDigestStore(false)
			m := newExportModel(t, store)
			m = press(m, "E")
			m = replacePath(m, len("runs.digest.json"), c.typed)

			m = pressType(m, tea.KeyEnter)

			requireExportScreen(t, m)
			if !strings.Contains(m.View(), c.wantErr) {
				t.Errorf("view missing %q; got:\n%s", c.wantErr, m.View())
			}
			if len(store.writes) != 0 {
				t.Errorf("invalid path wrote %d file(s)", len(store.writes))
			}
		})
	}
}

func TestExport_PathEntry_TypedPathIsExported(t *testing.T) {
	store := newTestDigestStore(false)
	m := newExportModel(t, store)
	m = press(m, "E")
	m = replacePath(m, len("runs.digest.json"), "/exports/custom.digest.json")

	updated, cmd := pressKeyType(m, tea.KeyEnter)
	m = asModel(updated)
	if m.screen != screenLoading {
		t.Errorf("while exporting, screen = %q, want loading", m.screen)
	}
	m = runCmd(t, m, cmd)

	if len(store.writes) != 1 || store.writes[0].path != "/exports/custom.digest.json" {
		t.Fatalf("writes = %+v, want one write to /exports/custom.digest.json", store.writes)
	}
}
