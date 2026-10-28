package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// NoDataScreen renders the explicit no-data state: a usable source that contains
// no run data. It distinguishes this from an error and from an unusable path.
//
// Navigation contract:
//   - 's' or Enter -> Done() == true (user wants to choose a different source).
//   - Esc or 'q'   -> Back() == true (user wants to quit).
//   - '?' shows the key overlay, which takes every key while it is open.
type NoDataScreen struct {
	source domain.Source
	width  int
	height int
	styles Styles
	done   bool
	back   bool
	pageChrome
}

// NewNoDataScreen creates the no-data state screen for the given source.
func NewNoDataScreen(source domain.Source, width, height int, styles Styles) *NoDataScreen {
	return &NoDataScreen{
		source: source,
		width:  width,
		height: height,
		styles: styles,
	}
}

// ID implements Screen.
func (s *NoDataScreen) ID() ScreenID { return ScreenNoData }

// Init implements Screen (no async work required).
func (s *NoDataScreen) Init() tea.Cmd { return nil }

// Update processes key messages for navigation.
func (s *NoDataScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	k := key.String()
	if s.help.handle(k) {
		return s, nil
	}
	switch k {
	case "s", "enter":
		s.done = true
	case "esc", "q":
		s.back = true
	}
	return s, nil
}

// View renders the no-data state.
func (s *NoDataScreen) View(width, height int) string {
	s.width = width
	s.height = height
	st := s.styles

	body := []string{st.Subtitle.Render("The source exists but contains no run data."), ""}
	for _, l := range WrapText("Source: "+s.source.Path, width) {
		body = append(body, st.Body.Render(l))
	}
	body = append(body, "")
	for _, l := range WrapText(noDataExplanation, width) {
		body = append(body, st.Muted.Render(l))
	}

	page := Page{Title: "Log Analyzer — No Data Found", Keys: noDataKeys, Help: true}
	return s.render(page, noDataOverlayGroups(), body, width, height, st)
}

// Help returns the key hints for this screen.
func (s *NoDataScreen) Help() string { return leadHelp("", noDataKeys) }

// Done reports whether the user wants to choose a different source.
func (s *NoDataScreen) Done() bool { return s.done }

// Back reports whether the user wants to quit.
func (s *NoDataScreen) Back() bool { return s.back }

// Reset clears the done and back flags.
func (s *NoDataScreen) Reset() {
	s.done = false
	s.back = false
}
