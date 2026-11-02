package screens_test

// The no-data page is a calm, informational state: the source path and the
// explanation stay as text under the page frame, nothing in it reads as an error,
// and every line stays inside the terminal.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const noDataPath = "/test/OrchestrationLogs"

func newNoData(path string, width, height int) *screens.NoDataScreen {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: path}
	return screens.NewNoDataScreen(src, width, height, paletteStyles())
}

func TestNoDataPage_ShowsTheSourcePathAndBothExplanationLinesAsText(t *testing.T) {
	colourOff(t)

	view := stripStyling(newNoData(noDataPath, 120, 24).View(120, 24))

	assertContains(t, view, "Source: "+noDataPath,
		"The logs directory is empty", "This is a normal state when the tool has not yet been run")
}

func TestNoDataPage_ExplanationComesAfterTheSourcePath(t *testing.T) {
	colourOff(t)

	view := stripStyling(newNoData(noDataPath, 120, 24).View(120, 24))

	if lineIndex(view, "Source: ") >= lineIndex(view, "The logs directory is empty") {
		t.Errorf("the source path must come before the explanation\n%s", view)
	}
}

func TestNoDataPage_NeverUsesTheBadRoleOrTheWordError(t *testing.T) {
	colourOn(t)

	view := newNoData(noDataPath, 120, 24).View(120, 24)

	assertNotContains(t, view, rolePrefix(bad))
	assertNotContains(t, strings.ToLower(stripStyling(view)), "error")
}

func TestNoDataPage_LongSourcePathStaysInsideNarrowTerminals(t *testing.T) {
	colourOff(t)
	long := "/" + strings.Repeat("very-long-directory/", 20) + "OrchestrationLogs"

	assertPageWithinEverySize(t, newNoData(long, 80, 24))
}

func TestNoDataPage_ShortPathIsShownIntactAtEightyColumns(t *testing.T) {
	colourOff(t)

	view := stripStyling(newNoData(noDataPath, 80, 24).View(80, 24))

	assertContains(t, view, "Source: "+noDataPath)
}

func TestNoDataPage_KeysStillWorkWithoutTheOverlay(t *testing.T) {
	for key, check := range map[string]func(*screens.NoDataScreen) bool{
		"s": (*screens.NoDataScreen).Done, "enter": (*screens.NoDataScreen).Done,
		"esc": (*screens.NoDataScreen).Back, "q": (*screens.NoDataScreen).Back,
	} {
		s := newNoData(noDataPath, 80, 24)

		press(s, key)

		if !check(s) {
			t.Errorf("key %q lost its action on the no-data page", key)
		}
	}
}
