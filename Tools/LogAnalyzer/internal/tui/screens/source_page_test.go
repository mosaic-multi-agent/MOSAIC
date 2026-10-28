package screens_test

// The source page is a page with a text input. The input owns every printable
// key, so the page offers no help overlay and `?` is typed as text; the input's
// own keys are the short help line. The empty-path message is an error and is
// drawn in the bad role.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

func newSource(width, height int) *screens.SourceScreen {
	return screens.NewSourceScreen(width, height, paletteStyles())
}

func TestSourcePage_FrameHoldsTitleRuleSubtitleAndInputKeysOnly(t *testing.T) {
	colourOff(t)

	lines := assertPageLayout(t, newSource(120, 24).View(120, 24), "Log Analyzer -- Select Source",
		"Enter the path to your logs root", "enter", "esc")

	assertNotContains(t, tailOf(lines), "? help")
}

func TestSourcePage_VersionIsPartOfTheTitle(t *testing.T) {
	colourOff(t)
	s := newSource(120, 24)
	s.SetVersion("1.2.3")

	lines := viewLines(s.View(120, 24))

	assertContains(t, headOf(lines), "Log Analyzer", "v1.2.3")
}

func TestSourcePage_KeepsItsInputGuidanceAndDigestHintAsText(t *testing.T) {
	colourOff(t)

	view := stripStyling(newSource(200, 24).View(200, 24))

	assertContains(t, view, "Log source path:", "Accepts:", "digest")
}

func TestSourcePage_QuestionMarkIsTypedAndOpensNoOverlay(t *testing.T) {
	colourOff(t)
	s := newSource(120, 24)
	before := s.View(120, 24)

	press(s, "a", screens.KeyHelp, "b")

	if helpOpenOf(s) {
		t.Error("the help key must not open an overlay while the text input has focus")
	}
	if got := s.Path(); got != "a?b" {
		t.Errorf("typed text = %q, want a?b", got)
	}
	if s.View(120, 24) == before {
		t.Error("the typed text must show on the page")
	}
}

func TestSourcePage_EmptyPathMessageIsDrawnInTheBadRole(t *testing.T) {
	colourOn(t)
	s := newSource(120, 24)

	press(s, "enter")
	view := s.View(120, 24)

	if s.Done() {
		t.Fatal("an empty path must not finish the screen")
	}
	assertDrawnInRole(t, rawLineWith(t, view, "path cannot be empty"), "path cannot be empty", bad)
}

func TestSourcePage_EmptyPathMessageReadsAsTextWithColourOff(t *testing.T) {
	colourOff(t)
	s := newSource(120, 24)

	press(s, "enter")

	assertContains(t, stripStyling(s.View(120, 24)), "path cannot be empty")
}

func TestSourcePage_EnterOnATypedPathFinishesAndEscGoesBack(t *testing.T) {
	s := newSource(120, 24)
	press(s, strings.Split("/logs", "")...)

	press(s, "enter")
	if !s.Done() || s.Path() != "/logs" {
		t.Errorf("Done=%v Path=%q, want done with /logs", s.Done(), s.Path())
	}

	s = newSource(120, 24)
	s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !s.Back() {
		t.Error("esc must still go back")
	}
}

func TestSourcePage_NeverDrawsPastTheTerminal(t *testing.T) {
	colourOff(t)
	s := newSource(80, 24)
	s.SetVersion("10.20.30-with-a-long-suffix")
	press(s, strings.Split(strings.Repeat("/long/path", 20), "")...)

	assertPageWithinEverySize(t, s)
}

func TestSourcePage_ErrorMessageStaysInsideNarrowTerminals(t *testing.T) {
	colourOff(t)
	s := newSource(80, 24)
	press(s, "enter")

	assertPageWithinEverySize(t, s)
}
