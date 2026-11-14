package screens_test

// While the findings overlay is open its short help lists the export key between
// scrolling and closing: "up/down scroll", "e export", "f/esc close".

import (
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func assertOverlayHelp(t *testing.T, footer string) {
	t.Helper()
	entries := footerEntries(footer)
	index := func(want string) int {
		for i, e := range entries {
			if e == want {
				return i
			}
		}
		return -1
	}
	scroll, export, closeAt := index("↑/↓ scroll"), index("e export"), index("f/esc close")
	if scroll < 0 || export < 0 || closeAt < 0 {
		t.Fatalf("footer %q must list the scroll, export and close entries", footer)
	}
	if !(scroll < export && export < closeAt) {
		t.Errorf("footer %q: want the order scroll, export, close", footer)
	}
}

func TestRunsFindingsOverlay_ShortHelpListsTheExportKey(t *testing.T) {
	colourOff(t)
	s := newFindingsRuns(wideW, tallH)
	press(s, screens.KeyFindings)

	assertOverlayHelp(t, footerLine(s.View(wideW, tallH)))
}

func TestRunViewFindingsOverlay_ShortHelpListsTheExportKey(t *testing.T) {
	for _, c := range footerCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.view(t)
			press(s, screens.KeyFindings)

			assertOverlayHelp(t, footerLine(s.View(wideW, tallH)))
		})
	}
}
