package screens_test

// The export page: a page with a path input and, when the file exists, an
// overwrite question. While the input has focus `?` is typed as text; the
// overwrite question has no input, so there the `?` overlay is available. Messages
// keep their text and take signal roles (question: warning, validation: bad).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

const exportRun = "20260101T120000Z-abcd"

func newExport(runID string, width, height int) *screens.ExportScreen {
	return screens.NewExportScreen(runID, width, paletteStyles())
}

// confirming returns an export screen asking whether to overwrite the default
// path of the run.
func confirming(t *testing.T, width, height int) *screens.ExportScreen {
	t.Helper()
	s := newExport(exportRun, width, height)
	press(s, "enter")
	if s.Action() != screens.ExportSubmit {
		t.Fatalf("setup: action = %v, want submit", s.Action())
	}
	s.AskOverwrite()
	return s
}

func clearInput(s *screens.ExportScreen, n int) {
	for i := 0; i < n; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
}

func TestExportPage_PathEntryFrameHoldsTitleRuleHeadingAndKeys(t *testing.T) {
	colourOff(t)

	lines := assertPageLayout(t, newExport(exportRun, 120, 24).View(120, 24),
		"Log Analyzer -- Export digest", "Export run "+exportRun, "enter", "esc")

	assertContains(t, strings.Join(lines, "\n"), "Digest file path:", exportRun+".digest.json")
}

func TestExportPage_AllRunsHeadingAndDefaultNameStayAsText(t *testing.T) {
	colourOff(t)

	lines := assertPageLayout(t, newExport("", 120, 24).View(120, 24),
		"Log Analyzer -- Export digest", "Export all runs", "enter", "esc")

	assertContains(t, strings.Join(lines, "\n"), screens.DefaultAllRunsDigestName)
}

func TestExportPage_QuestionMarkIsTypedWhileThePathInputHasFocus(t *testing.T) {
	colourOff(t)
	s := newExport(exportRun, 120, 24)

	press(s, screens.KeyHelp)
	press(s, "enter")

	if helpOpenOf(s) {
		t.Error("the help key must not open an overlay while the text input has focus")
	}
	if s.Action() == screens.ExportSubmit {
		t.Error("the typed ? ends the path, so the path no longer ends in .digest.json and must be rejected")
	}
	assertContains(t, stripStyling(s.View(120, 24)), "path must end in .digest.json")
}

func TestExportPage_ValidationMessageIsDrawnInTheBadRole(t *testing.T) {
	colourOn(t)
	s := newExport(exportRun, 120, 24)
	clearInput(s, len(exportRun+".digest.json"))

	press(s, "enter")
	view := s.View(120, 24)

	assertDrawnInRole(t, rawLineWith(t, view, "path cannot be empty"), "path cannot be empty", bad)
}

func TestExportPage_OverwriteQuestionKeepsItsTextAndIsDrawnInTheWarningRole(t *testing.T) {
	colourOn(t)
	question := exportRun + ".digest.json already exists. Overwrite? (y/n)"

	view := confirming(t, 160, 24).View(160, 24)

	assertDrawnInRole(t, rawLineWith(t, view, "already exists"), question, warning)
}

func TestExportPage_OverwriteQuestionReadsAsTextWithColourOff(t *testing.T) {
	colourOff(t)

	lines := viewLines(confirming(t, 160, 24).View(160, 24))

	assertContains(t, strings.Join(lines, "\n"), exportRun+".digest.json already exists. Overwrite? (y/n)")
	assertContains(t, tailOf(lines), "y", "n", "esc", "? help")
}

func TestExportPage_OverwriteQuestionOffersTheOverlayWithItsOwnKeys(t *testing.T) {
	colourOff(t)
	s := confirming(t, 120, 40)

	overlay := pageOverlayBody(s, 120, 40)

	if !helpOpenOf(s) {
		t.Fatal("the help key must open the overlay while the overwrite question is shown")
	}
	assertContains(t, overlay, "This screen", "Navigation")
	for _, k := range []string{"y", "n", "esc", screens.KeyHelp} {
		assertOverlayListsKey(t, overlay, k)
	}
}

func TestExportPage_OverlaySwallowsTheAnswerKeysAndEscOnlyClosesIt(t *testing.T) {
	s := confirming(t, 120, 40)
	openOverlay(s, 120, 40)

	press(s, "y", "n")
	press(s, "esc")

	if s.Action() != screens.ExportNone {
		t.Fatalf("action = %v, want none: keys pressed under the overlay must not answer the question", s.Action())
	}
	if helpOpenOf(s) {
		t.Error("esc must close the overlay")
	}
	press(s, "y")
	if s.Action() != screens.ExportOverwrite {
		t.Errorf("after closing the overlay y must confirm, action = %v", s.Action())
	}
}

func TestExportPage_NeverDrawsPastTheTerminalInEitherState(t *testing.T) {
	colourOff(t)
	long := strings.Repeat("run-with-a-very-long-id-", 6)

	assertPageWithinEverySize(t, newExport(long, 80, 24))
	assertPageWithinEverySize(t, confirming(t, 80, 24))
	assertPageWithinEverySize(t, newExport("", 80, 24))
}

func TestExportPage_KeysKeepTheirActions(t *testing.T) {
	s := newExport(exportRun, 120, 24)
	s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if s.Action() != screens.ExportCancel {
		t.Errorf("esc on path entry: action = %v, want cancel", s.Action())
	}

	c := confirming(t, 120, 24)
	press(c, "n")
	if c.Action() != screens.ExportDecline {
		t.Errorf("n on the overwrite question: action = %v, want decline", c.Action())
	}
}
