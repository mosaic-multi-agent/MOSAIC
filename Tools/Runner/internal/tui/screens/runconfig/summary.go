package runconfig

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens"
)

// SummaryScreen is the read-only summary of a resumed run's recorded settings,
// shown as the last screen before the session starts. Enter confirms; esc goes
// back to the previous screen.
type SummaryScreen struct {
	lines     []string
	width     int
	height    int
	styles    screens.Styles
	confirmed bool
	back      bool
}

// NewSummaryScreen creates the summary screen. lines is the shared summary text
// (resumecheck.Summary.Lines()), shown verbatim.
func NewSummaryScreen(lines []string, width, height int, styles screens.Styles) *SummaryScreen {
	return &SummaryScreen{lines: lines, width: width, height: height, styles: styles}
}

// Update handles a key message: enter confirms, esc goes back.
func (s *SummaryScreen) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch keyMsg.Type {
	case tea.KeyEnter:
		s.confirmed = true
	case tea.KeyEsc:
		s.back = true
	}
	return nil
}

// View renders the title, the lines verbatim and the help line.
func (s *SummaryScreen) View() string {
	title := s.styles.Title.Render("Resume: recorded settings")
	border := s.styles.Border.Render(strings.Repeat("\u2500", max(s.width, 1)))
	help := s.styles.Help.Render("enter  start run   esc  back")
	var body strings.Builder
	for _, line := range s.lines {
		body.WriteString(s.styles.Body.Render(line) + "\n")
	}
	return strings.Join([]string{title, border, body.String(), border, help}, "\n")
}

// Confirmed reports whether the user confirmed the summary.
func (s *SummaryScreen) Confirmed() bool { return s.confirmed }

// Back reports whether the user asked to go back.
func (s *SummaryScreen) Back() bool { return s.back }

// Reset clears Confirmed and Back.
func (s *SummaryScreen) Reset() {
	s.confirmed = false
	s.back = false
}

// Resize updates the screen dimensions.
func (s *SummaryScreen) Resize(width, height int) {
	s.width = width
	s.height = height
}
