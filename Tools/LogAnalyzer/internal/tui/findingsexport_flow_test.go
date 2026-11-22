package tui

// findingsexport_flow_test.go verifies the findings export flow after the prompt
// opened: what is written, the one-line status, the overwrite confirmation, that the
// overlay is still open afterwards, and that the digest export is never involved.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// startOnRuns opens the runs-list overlay and its findings prompt.
func startOnRuns(t *testing.T, store *tuiFakeDigestStore, writer *tuiFakeFindingsWriter) Model {
	t.Helper()
	m := press(onRunsWithOverlay(t, newFindingsFlowModel(t, store, writer)), screens.KeyExportRun)
	requireFindingsPrompt(t, m, "Export findings", "findings.json")
	return m
}

// startInRun opens the overlay of run b in the overview and its findings prompt.
func startInRun(t *testing.T, store *tuiFakeDigestStore, writer *tuiFakeFindingsWriter) Model {
	t.Helper()
	m := press(inRunWithOverlay(t, newFindingsFlowModel(t, store, writer), screens.ScreenOverview), screens.KeyExportRun)
	requireFindingsPrompt(t, m, "Export findings of run "+exportRunB, "findings-"+exportRunB+".json")
	return m
}

func requireView(t *testing.T, m Model, wants ...string) {
	t.Helper()
	view := m.View()
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Errorf("view must contain %q; got:\n%s", want, view)
		}
	}
}

func requireNoDigestTraces(t *testing.T, m Model, store *tuiFakeDigestStore) {
	t.Helper()
	if len(store.writes) != 0 {
		t.Errorf("%d digest write(s); the findings export must never export a digest", len(store.writes))
	}
	if view := m.View(); strings.Contains(view, "run(s)") {
		t.Errorf("a digest status line appeared in the findings flow:\n%s", view)
	}
}

func TestFindingsExport_RunsList_SubmitWritesEveryFindingOfTheOverlayAndReportsIt(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{}
	m := startOnRuns(t, store, writer)
	want := m.report.Quality.Findings

	m = submitPath(t, m)

	if len(writer.writes) != 1 {
		t.Fatalf("%d findings write(s), want 1", len(writer.writes))
	}
	w := writer.writes[0]
	if w.path != "findings.json" || w.overwrite {
		t.Errorf("write = %+v, want findings.json without overwrite", w)
	}
	if len(want) != 6 || len(w.findings) != len(want) {
		t.Fatalf("wrote %d findings, want all %d of the overlay's scope", len(w.findings), len(want))
	}
	for i := range want {
		if w.findings[i] != want[i] {
			t.Errorf("finding %d = %+v, want %+v", i, w.findings[i], want[i])
		}
	}
	requireView(t, m, "Exported 6 finding(s) to findings.json")
	requireNoDigestTraces(t, m, store)
}

func TestFindingsExport_RunsList_ReturnsToTheListWithTheOverlayStillOpen(t *testing.T) {
	m := submitPath(t, startOnRuns(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}))

	if m.screen != screenRuns {
		t.Fatalf("screen = %q, want the runs list", m.screen)
	}
	if !m.runsScreen.FindingsOpen() {
		t.Error("the findings overlay must still be open after the export")
	}
}

func TestFindingsExport_InARun_SubmitWritesOnlyTheFindingsOfThatRun(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{}
	m := startInRun(t, store, writer)
	scope := m.runView.(findingsOverlay).FindingsScope()

	m = submitPath(t, m)

	if len(writer.writes) != 1 {
		t.Fatalf("%d findings write(s), want 1", len(writer.writes))
	}
	w := writer.writes[0]
	if w.path != "findings-"+exportRunB+".json" {
		t.Errorf("path = %q, want findings-%s.json", w.path, exportRunB)
	}
	if len(scope.Findings) != 2 || len(w.findings) != 2 {
		t.Fatalf("scope has %d findings and the file %d, want the 2 of the run", len(scope.Findings), len(w.findings))
	}
	for i, f := range w.findings {
		if f != scope.Findings[i] || f.Run != domain.NamedRun(exportRunB) {
			t.Errorf("finding %d = %+v, want the overlay's finding of run b", i, f)
		}
	}
	requireView(t, m, "Exported 2 finding(s) to findings-"+exportRunB+".json")
	if m.screen != screenOverview || !findingsOpen(m.runView) {
		t.Errorf("screen = %q open = %v, want the overview with its overlay open", m.screen, findingsOpen(m.runView))
	}
	requireNoDigestTraces(t, m, store)
}

func TestFindingsExport_PerRunViews_ReturnToTheViewWithTheOverlayOpen(t *testing.T) {
	for name, id := range perRunScreens() {
		t.Run(name, func(t *testing.T) {
			m := press(inRunWithOverlay(t, newFindingsFlowModel(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}), id), screens.KeyExportRun)
			shown := screenName(id)
			requireExportScreen(t, m)

			m = submitPath(t, m)

			if m.screen != shown || !findingsOpen(m.runView) {
				t.Errorf("screen = %q open = %v, want %q with the overlay open", m.screen, findingsOpen(m.runView), shown)
			}
			requireView(t, m, "Exported 2 finding(s)")
		})
	}
}

func TestFindingsExport_EscOnThePromptCancelsAndKeepsTheOverlay(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{}
	m := startOnRuns(t, store, writer)

	m = pressType(m, tea.KeyEsc)

	if m.screen != screenRuns || !m.runsScreen.FindingsOpen() {
		t.Errorf("screen = %q open = %v, want the runs list with the overlay open", m.screen, m.runsScreen.FindingsOpen())
	}
	requireView(t, m, "Export cancelled")
	if len(writer.writes) != 0 {
		t.Errorf("cancelling wrote %d findings file(s)", len(writer.writes))
	}
	requireNoDigestTraces(t, m, store)
}

func TestFindingsExport_ExistingFileAsksBeforeOverwriting(t *testing.T) {
	writer := &tuiFakeFindingsWriter{existing: map[string]bool{"findings.json": true}}
	m := submitPath(t, startOnRuns(t, newTestDigestStore(false), writer))

	requireExportScreen(t, m)
	requireView(t, m, "findings.json", "Overwrite")
	if len(writer.writes) != 0 {
		t.Errorf("%d findings write(s) before the confirmation", len(writer.writes))
	}
}

func TestFindingsExport_ConfirmingOverwriteReplacesTheFile(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{existing: map[string]bool{"findings.json": true}}
	m := submitPath(t, startOnRuns(t, store, writer))

	m = confirmOverwrite(t, m)

	if len(writer.writes) != 1 || !writer.writes[0].overwrite || writer.writes[0].path != "findings.json" {
		t.Fatalf("writes = %+v, want one write of findings.json with overwrite", writer.writes)
	}
	if len(writer.writes[0].findings) != 6 {
		t.Errorf("the confirmed write holds %d findings, want all 6", len(writer.writes[0].findings))
	}
	requireView(t, m, "Exported 6 finding(s) to findings.json")
	if m.screen != screenRuns || !m.runsScreen.FindingsOpen() {
		t.Errorf("screen = %q open = %v, want the runs list with the overlay open", m.screen, m.runsScreen.FindingsOpen())
	}
	requireNoDigestTraces(t, m, store)
}

func TestFindingsExport_DecliningOverwriteWritesNothing(t *testing.T) {
	for name, decline := range map[string]func(Model) Model{
		"n":   func(m Model) Model { return press(m, "n") },
		"esc": func(m Model) Model { return pressType(m, tea.KeyEsc) },
	} {
		t.Run(name, func(t *testing.T) {
			store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{existing: map[string]bool{"findings.json": true}}
			m := submitPath(t, startOnRuns(t, store, writer))

			m = decline(m)

			if len(writer.writes) != 0 {
				t.Errorf("%d findings write(s) after declining", len(writer.writes))
			}
			requireView(t, m, "Export cancelled; nothing written")
			if m.screen != screenRuns || !m.runsScreen.FindingsOpen() {
				t.Errorf("screen = %q open = %v, want the runs list with the overlay open", m.screen, m.runsScreen.FindingsOpen())
			}
			requireNoDigestTraces(t, m, store)
		})
	}
}

func TestFindingsExport_WriteFailureIsShownAsAStatusLine(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{failWith: errDiskFull}
	m := submitPath(t, startOnRuns(t, store, writer))

	requireView(t, m, "Export failed: disk full")
	if m.screen != screenRuns || !m.runsScreen.FindingsOpen() {
		t.Errorf("screen = %q open = %v, want the runs list with the overlay open", m.screen, m.runsScreen.FindingsOpen())
	}
	requireNoDigestTraces(t, m, store)
}

func TestFindingsExport_StatusClearsOnTheNextKeyAndTheOverlayKeepsWorking(t *testing.T) {
	m := submitPath(t, startOnRuns(t, newTestDigestStore(false), &tuiFakeFindingsWriter{}))
	requireView(t, m, "Exported 6 finding(s)")

	m = press(m, "j")

	if view := m.View(); strings.Contains(view, "Exported 6 finding(s)") {
		t.Errorf("the status must clear on the next key; got:\n%s", view)
	}
	if !m.runsScreen.FindingsOpen() {
		t.Error("the overlay must stay open")
	}
}

func TestFindingsExport_DigestExportStillWorksAfterAFindingsExport(t *testing.T) {
	store, writer := newTestDigestStore(false), &tuiFakeFindingsWriter{}
	m := submitPath(t, startOnRuns(t, store, writer))
	m = press(m, screens.KeyFindings) // close the overlay
	m = press(m, "E")
	requireExportScreen(t, m)
	requireView(t, m, "Export all runs", "runs.digest.json")

	m = submitPath(t, m)

	if len(store.writes) != 1 || store.writes[0].path != "runs.digest.json" {
		t.Fatalf("digest writes = %+v, want one write of runs.digest.json", store.writes)
	}
	if len(writer.writes) != 1 {
		t.Errorf("%d findings write(s), want only the earlier one", len(writer.writes))
	}
	requireView(t, m, "run(s)")
}

func TestFindingsExport_OverwriteQuestionOfTheFindingsFlowNeverTouchesTheDigestStore(t *testing.T) {
	store := newTestDigestStore(false)
	store.allExist = true // a digest export of any path would ask or fail on an existing file
	writer := &tuiFakeFindingsWriter{existing: map[string]bool{"findings.json": true}}
	m := submitPath(t, startOnRuns(t, store, writer))
	requireView(t, m, "Overwrite")

	m = confirmOverwrite(t, m)

	if len(writer.writes) != 1 {
		t.Errorf("%d findings write(s), want 1", len(writer.writes))
	}
	requireView(t, m, "Exported 6 finding(s)")
	requireNoDigestTraces(t, m, store)
}
