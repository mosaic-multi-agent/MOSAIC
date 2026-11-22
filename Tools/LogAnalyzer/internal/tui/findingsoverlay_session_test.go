package tui

// findingsoverlay_session_test.go verifies the findings overlay on the Session
// screen at model level: it shows only the findings of the run the session
// belongs to, and swallows the export-all and app-level keys while open (e starts the findings export).

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// openSessionOfSecondRunWithFindings opens the Session screen of the second run
// of a model whose report carries findings in both runs.
func openSessionOfSecondRunWithFindings(t *testing.T) (Model, *tuiFakeDigestStore) {
	t.Helper()
	store := newTestDigestStore(false)
	m := newExportModel(t, store)
	m.report.Quality = findingsReportFor(exportRunA, exportRunB).Quality
	m = send(openRunDetailOfSecondRun(m), "enter")
	assertOn(t, m, screens.ScreenSession)
	return m, store
}

func openSessionOverlay(t *testing.T) (Model, *tuiFakeDigestStore) {
	t.Helper()
	m, store := openSessionOfSecondRunWithFindings(t)
	m = press(m, screens.KeyFindings)
	view, ok := m.runView.(findingsOverlay)
	if !ok || !view.FindingsOpen() {
		t.Fatal("f on the Session screen must open the findings overlay")
	}
	return m, store
}

func TestFindingsOverlay_Session_ShowsOnlyTheShownRunsFindings(t *testing.T) {
	m, _ := openSessionOverlay(t)

	scope := m.runView.(findingsOverlay).FindingsScope()

	if scope.Label != exportRunB {
		t.Errorf("scope label = %q, want %q", scope.Label, exportRunB)
	}
	if want := screens.RunFindingsOf(m.report.Quality, domain.NamedRun(exportRunB)); len(want) != 2 || len(scope.Findings) != len(want) {
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
}

func TestFindingsOverlay_Session_ExportAndAppKeysDoNothingWhileOpen(t *testing.T) {
	for _, key := range []string{screens.KeyExportAll, "s", screens.KeyAcrossRuns, "p"} {
		t.Run(key, func(t *testing.T) {
			m, store := openSessionOverlay(t)
			shown := m.screen

			m = press(m, key)

			if m.screen != shown {
				t.Errorf("%q moved the screen from %q to %q while the overlay is open", key, shown, m.screen)
			}
			if strings.Contains(m.View(), digestPrompt) {
				t.Error("a digest export started while the overlay is open")
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

func TestFindingsOverlay_Session_ExportWorksOnceItIsClosed(t *testing.T) {
	m, _ := openSessionOverlay(t)
	m = press(m, screens.KeyFindings)

	m = press(m, screens.KeyExportRun)

	requireExportScreen(t, m)
}
