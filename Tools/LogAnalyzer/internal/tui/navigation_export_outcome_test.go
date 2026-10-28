package tui

// navigation_export_outcome_test.go verifies what the export action reports:
// success with the number of runs written, overwrite confirmation, declining,
// errors, and the help lines that list the action.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// submitExport presses Enter on the export screen and runs the resulting command.
func submitExport(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := pressKeyType(m, tea.KeyEnter)
	return runCmd(t, asModel(updated), cmd)
}

func runIDsOf(d domain.Digest) []string {
	var ids []string
	for _, r := range d.Aggregate.Runs {
		ids = append(ids, r.Run.ID)
	}
	return ids
}

func TestExport_SelectedRun_WritesOnlyThatRunAndReportsOne(t *testing.T) {
	store := newTestDigestStore(true)
	m := newExportModel(t, store)
	m = press(press(m, "j"), "e")

	m = submitExport(t, m)

	if m.screen != screenRuns {
		t.Fatalf("screen = %q, want runs", m.screen)
	}
	if len(store.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(store.writes))
	}
	w := store.writes[0]
	if !strings.HasSuffix(w.path, exportRunB+".digest.json") {
		t.Errorf("written path = %q, want the pre-filled %s.digest.json", w.path, exportRunB)
	}
	if ids := runIDsOf(w.d); len(ids) != 1 || ids[0] != exportRunB {
		t.Errorf("written runs = %v, want only %s", ids, exportRunB)
	}
	if w.d.Aggregate.Unattributable != nil {
		t.Error("exporting one run must not include the unattributable bucket")
	}
	if w.overwrite {
		t.Error("first attempt must not request overwrite")
	}
	view := m.View()
	if !strings.Contains(view, "Exported 1 run(s) to ") || strings.Contains(view, "unattributable") {
		t.Errorf("status should report one run without the bucket; got:\n%s", view)
	}
}

func TestExport_AllRuns_WritesEveryRunAndReportsCount(t *testing.T) {
	store := newTestDigestStore(false)
	m := newExportModel(t, store)
	m = press(m, "E")

	m = submitExport(t, m)

	if len(store.writes) != 1 || len(store.writes[0].d.Aggregate.Runs) != 2 {
		t.Fatalf("writes = %+v, want one digest with 2 runs", store.writes)
	}
	if view := m.View(); !strings.Contains(view, "Exported 2 run(s) to ") {
		t.Errorf("status should report 2 runs; got:\n%s", view)
	}
}

func TestExport_AllRuns_WithBucket_StatusMentionsBucket(t *testing.T) {
	store := newTestDigestStore(true)
	m := newExportModel(t, store)
	m = press(m, "E")

	m = submitExport(t, m)

	if view := m.View(); !strings.Contains(view, "Exported 2 run(s) to ") ||
		!strings.Contains(view, "(including the unattributable bucket)") {
		t.Errorf("status should mention the bucket; got:\n%s", view)
	}
}

func TestExport_ExistingFile_AsksBeforeOverwriting(t *testing.T) {
	store := newTestDigestStore(false)
	store.allExist = true
	m := newExportModel(t, store)
	m = press(m, "E")

	m = submitExport(t, m)

	requireExportScreen(t, m)
	if view := m.View(); !strings.Contains(view, "already exists. Overwrite? (y/n)") {
		t.Errorf("view should ask for overwrite confirmation; got:\n%s", view)
	}
	if len(store.writes) != 0 {
		t.Errorf("nothing may be written before confirmation; writes = %d", len(store.writes))
	}
}

func TestExport_ExistingFile_ConfirmReplaces(t *testing.T) {
	store := newTestDigestStore(false)
	store.allExist = true
	m := newExportModel(t, store)
	m = press(m, "E")
	m = submitExport(t, m)

	updated, cmd := pressKey(m, "y")
	m = runCmd(t, asModel(updated), cmd)

	if len(store.writes) != 1 || !store.writes[0].overwrite {
		t.Fatalf("writes = %+v, want exactly one write with overwrite", store.writes)
	}
	if m.screen != screenRuns {
		t.Errorf("screen = %q, want runs", m.screen)
	}
	if view := m.View(); !strings.Contains(view, "Exported 2 run(s) to ") {
		t.Errorf("status should report success after overwrite; got:\n%s", view)
	}
}

func TestExport_ExistingFile_DeclineWritesNothing(t *testing.T) {
	for _, decline := range []struct {
		name string
		do   func(Model) Model
	}{
		{"n", func(m Model) Model { return press(m, "n") }},
		{"esc", func(m Model) Model { return pressType(m, tea.KeyEsc) }},
	} {
		t.Run(decline.name, func(t *testing.T) {
			store := newTestDigestStore(false)
			store.allExist = true
			m := newExportModel(t, store)
			m = press(m, "E")
			m = submitExport(t, m)

			m = decline.do(m)

			if m.screen != screenRuns {
				t.Fatalf("screen = %q, want runs", m.screen)
			}
			if view := m.View(); !strings.Contains(view, "Export cancelled; nothing written") {
				t.Errorf("status should say nothing was written; got:\n%s", view)
			}
			if len(store.writes) != 0 {
				t.Errorf("declined overwrite wrote %d file(s)", len(store.writes))
			}
		})
	}
}

func TestExport_WriteError_IsShownAndUiSurvives(t *testing.T) {
	store := newTestDigestStore(false)
	store.writeErr = errors.New("disk is full")
	m := newExportModel(t, store)
	m = press(m, "E")

	m = submitExport(t, m)

	if m.screen != screenRuns {
		t.Fatalf("screen = %q, want runs (the UI must not quit or switch to the error screen)", m.screen)
	}
	view := m.View()
	if !strings.Contains(view, "Export failed: ") || !strings.Contains(view, "disk is full") {
		t.Errorf("status should show the error; got:\n%s", view)
	}
}

func TestExport_RunsHelp_ListsExportKeys(t *testing.T) {
	want := "a across runs  e export run  E export all  s source  p pricing  esc quit  ctrl+c quit"
	m := newExportModel(t, newTestDigestStore(false))

	if help := m.runsScreen.Help(); !strings.Contains(help, want) {
		t.Errorf("runs Help() = %q, want it to contain %q", help, want)
	}
	if view := m.View(); !strings.Contains(view, want) {
		t.Errorf("runs help bar does not list the export keys; got:\n%s", view)
	}
}

func TestExport_PerRunViews_HelpOverlayListsExportKeys(t *testing.T) {
	m := newExportModel(t, newTestDigestStore(false))
	run, ok := m.report.FindRun(exportRunA)
	if !ok {
		t.Fatal("setup: run missing from report")
	}
	st := plainStyles()
	views := map[string]screens.Screen{
		"run detail":  screens.NewRunDetailScreen(run, 200, 24, st),
		"overview":    screens.NewOverviewScreen(run, 200, 24, st),
		"agent types": screens.NewAgentTypesScreen(run, 200, 24, st),
		"status":      screens.NewStatusScreen(run, 200, 24, st),
		"tools":       screens.NewToolsScreen(run, 200, 24, st),
		"timeline":    screens.NewTimelineScreen(run, 200, 24, st),
	}
	for name, v := range views {
		t.Run(name, func(t *testing.T) {
			if help := v.Help(); !strings.Contains(help, screens.KeyHelp) {
				t.Errorf("Help() = %q, want it to hint at %q", help, screens.KeyHelp)
			}
			overlay := helpOverlayText(v, 200, 60)
			assertListsKey(t, overlay, screens.KeyExportRun)
			assertListsKey(t, overlay, screens.KeyExportAll)
			if !strings.Contains(strings.ToLower(overlay), "export") {
				t.Errorf("the export entries must say what they export\n%s", overlay)
			}
		})
	}
}
