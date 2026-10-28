package tui

// The export status line below a screen takes a signal role by outcome (success
// good, cancelled warning, failure bad), keeps its text, reads as text with
// colour off and never reaches past the terminal.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/tui/screens"
)

// statusLine is the last line of the model's view: the export status.
func statusLine(m Model) string {
	lines := strings.Split(m.View(), "\n")
	return lines[len(lines)-1]
}

func exportedAllRuns(t *testing.T) Model {
	t.Helper()
	m := press(newExportModel(t, newTestDigestStore(false)), "E")
	return submitExport(t, m)
}

func TestExportStatus_SuccessIsDrawnInTheGoodRoleWithItsTextIntact(t *testing.T) {
	colourOn(t)

	line := statusLine(exportedAllRuns(t))

	assertDrawnInRole(t, line, "Exported 2 run(s) to ", tuicommon.RoleSignalGood)
}

func TestExportStatus_CancelledIsDrawnInTheWarningRole(t *testing.T) {
	colourOn(t)
	m := press(newExportModel(t, newTestDigestStore(false)), "e")

	m = pressType(m, tea.KeyEsc)

	assertDrawnInRole(t, statusLine(m), "Export cancelled", tuicommon.RoleSignalWarning)
}

func TestExportStatus_DeclinedOverwriteIsDrawnInTheWarningRole(t *testing.T) {
	colourOn(t)
	store := newTestDigestStore(false)
	store.allExist = true
	m := submitExport(t, press(newExportModel(t, store), "E"))

	m = press(m, "n")

	assertDrawnInRole(t, statusLine(m), "Export cancelled; nothing written", tuicommon.RoleSignalWarning)
}

func TestExportStatus_FailureIsDrawnInTheBadRoleWithTheCauseIntact(t *testing.T) {
	colourOn(t)
	store := newTestDigestStore(false)
	store.writeErr = errors.New("disk is full")
	m := submitExport(t, press(newExportModel(t, store), "E"))

	assertDrawnInRole(t, statusLine(m), "Export failed: ", tuicommon.RoleSignalBad)
	assertContains(t, escapeRE.ReplaceAllString(statusLine(m), ""), "disk is full")
}

func TestExportStatus_ReadsAsPlainTextWithColourOff(t *testing.T) {
	colourOff(t)

	m := exportedAllRuns(t)

	if line := statusLine(m); strings.Contains(line, "\x1b[") || !strings.Contains(line, "Exported 2 run(s) to ") {
		t.Errorf("status must be plain text with colour off: %q", line)
	}
}

func TestExportStatus_LongMessageIsCutToTheWidthAndTheViewKeepsItsHeight(t *testing.T) {
	colourOff(t)
	store := newTestDigestStore(false)
	store.writeErr = errors.New(strings.Repeat("disk is full because of a very long reason ", 10))
	m := press(newExportModel(t, store), "E")
	m.width, m.height = 60, 20

	m = submitExport(t, m)

	assertViewWithin(t, m.View(), 60, 20)
	if !strings.HasPrefix(escapeRE.ReplaceAllString(statusLine(m), ""), "Export failed: ") {
		t.Errorf("the start of the status must survive the cut: %q", statusLine(m))
	}
}

func TestExportStatus_ClearsOnTheNextKeyWhateverItsRole(t *testing.T) {
	colourOn(t)
	m := exportedAllRuns(t)

	m = press(m, "j")

	assertNotContains(t, escapeRE.ReplaceAllString(m.View(), ""), "Exported 2 run(s)")
	assertOn(t, m, screens.ScreenRuns)
}

func assertContains(t *testing.T, view string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(view, w) {
			t.Errorf("view does not contain %q\n%s", w, view)
		}
	}
}

func assertNotContains(t *testing.T, view string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(view, u) {
			t.Errorf("view must not contain %q\n%s", u, view)
		}
	}
}
