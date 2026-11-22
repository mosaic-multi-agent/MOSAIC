package tui

// findingsexport_start_test.go verifies how the findings export is started: e inside
// the findings overlay of the runs list or of a per-run view opens the findings
// path prompt (not the digest one), while e outside the overlay still opens the
// digest export, and E and other keys stay swallowed by the open overlay.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// requireFindingsPrompt fails unless the export screen shows the findings prompt
// with the heading and pre-filled name, and not the digest one.
func requireFindingsPrompt(t *testing.T, m Model, heading, name string) {
	t.Helper()
	requireExportScreen(t, m)
	view := m.View()
	for _, want := range []string{heading, name} {
		if !strings.Contains(view, want) {
			t.Errorf("the findings prompt must show %q; got:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Export all runs", ".digest.json", "Export run "} {
		if strings.Contains(view, unwanted) {
			t.Errorf("the findings prompt must not show %q; got:\n%s", unwanted, view)
		}
	}
}

func TestFindingsExport_RunsList_EInTheOverlayOpensTheFindingsPrompt(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{}
	m := onRunsWithOverlay(t, newFindingsFlowModel(t, store, writer))

	m = press(m, screens.KeyExportRun)

	requireFindingsPrompt(t, m, "Export findings", "findings.json")
	if len(store.writes) != 0 || len(writer.writes) != 0 {
		t.Errorf("opening the prompt wrote %d digest(s) and %d findings file(s)", len(store.writes), len(writer.writes))
	}
}

func TestFindingsExport_PerRunViews_EInTheOverlayOpensThePromptOfTheRun(t *testing.T) {
	for name, id := range perRunScreens() {
		t.Run(name, func(t *testing.T) {
			store := newTestDigestStore(false)
			m := inRunWithOverlay(t, newFindingsFlowModel(t, store, &tuiFakeFindingsWriter{}), id)

			m = press(m, screens.KeyExportRun)

			requireFindingsPrompt(t, m, "Export findings of run "+exportRunB, "findings-"+exportRunB+".json")
			if len(store.writes) != 0 {
				t.Errorf("%d digest write(s) while opening the findings prompt", len(store.writes))
			}
		})
	}
}

func TestFindingsExport_CapitalEAndOtherKeysInTheOverlayStaySwallowed(t *testing.T) {
	for _, key := range []string{screens.KeyExportAll, "x", "s", "p", screens.KeyAcrossRuns} {
		t.Run("runs/"+key, func(t *testing.T) {
			m := onRunsWithOverlay(t, newFindingsFlowModel(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}))

			m = press(m, key)

			if m.screen != screenRuns || !m.runsScreen.FindingsOpen() {
				t.Errorf("%q: screen = %q open = %v, want the runs list with the overlay open", key, m.screen, m.runsScreen.FindingsOpen())
			}
		})
		t.Run("run/"+key, func(t *testing.T) {
			m := inRunWithOverlay(t, newFindingsFlowModel(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}), screens.ScreenOverview)
			shown := m.screen

			m = press(m, key)

			if m.screen != shown || !findingsOpen(m.runView) {
				t.Errorf("%q: screen = %q, want %q with the overlay open", key, m.screen, shown)
			}
		})
	}
}

func TestFindingsExport_OutsideTheOverlayEStillStartsTheDigestExport(t *testing.T) {
	m := newFindingsFlowModel(t, newTestDigestStore(false), &tuiFakeFindingsWriter{})
	m = press(m, "j") // highlight the second run

	m = press(m, screens.KeyExportRun)

	requireExportScreen(t, m)
	view := m.View()
	for _, want := range []string{"Digest file path:", ".digest.json", "Export run "} {
		if !strings.Contains(view, want) {
			t.Errorf("e outside the overlay must open the digest prompt showing %q; got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Export findings") {
		t.Errorf("e outside the overlay must not open the findings prompt; got:\n%s", view)
	}
}

func TestFindingsExport_AfterClosingTheOverlayEAndCapitalEStartTheDigestExport(t *testing.T) {
	for key, heading := range map[string]string{screens.KeyExportRun: "Export run ", screens.KeyExportAll: "Export all runs"} {
		t.Run(key, func(t *testing.T) {
			m := onRunsWithOverlay(t, newFindingsFlowModel(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}))
			m = press(m, screens.KeyFindings)

			m = press(m, key)

			requireExportScreen(t, m)
			if view := m.View(); !strings.Contains(view, heading) || strings.Contains(view, "Export findings") {
				t.Errorf("with the overlay closed %q must open the digest prompt (%q); got:\n%s", key, heading, view)
			}
		})
	}
}

func TestFindingsExport_PerRunViews_OutsideTheOverlayEStillStartsTheDigestExport(t *testing.T) {
	m := newFindingsFlowModel(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}).openRunView(exportRunB, screens.ScreenOverview)

	m = press(m, screens.KeyExportRun)

	requireExportScreen(t, m)
	if view := m.View(); !strings.Contains(view, "Export run "+exportRunB) || strings.Contains(view, "Export findings") {
		t.Errorf("e in a run without the overlay must open the digest prompt; got:\n%s", view)
	}
}
