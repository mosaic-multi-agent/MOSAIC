package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/pathutil"
	"mosaic-common/tui/widgets"
)

// SourceScreen prompts the user to enter a path to a logs root or single run folder.
//
// Navigation contract:
//   - Enter on a non-empty path -> Done() == true, Path() returns the path.
//   - Esc -> Back() == true.
type SourceScreen struct {
	input   *widgets.TextInput
	width   int
	height  int
	styles  Styles
	version string
}

// NewSourceScreen creates the log-source path entry screen.
func NewSourceScreen(width, height int, styles Styles) *SourceScreen {
	inputStyles := textInputStyles(styles)
	input := widgets.NewTextInput(
		"Log source path:",
		"/path/to/OrchestrationLogs  or  /path/to/run-folder  or  /path/to/run.digest.json",
		width,
		inputStyles,
	)
	input.SetValidate(validateNonEmpty)
	return &SourceScreen{
		input:  input,
		width:  width,
		height: height,
		styles: styles,
	}
}

func validateNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errEmptyPath
	}
	return nil
}

// errEmptyPath is a sentinel error for the empty path validation.
var errEmptyPath = errString("path cannot be empty")

type errString string

func (e errString) Error() string { return string(e) }

// SetVersion sets the tool version string displayed in the screen title.
// Call before the first View() for the version to appear on entry.
func (s *SourceScreen) SetVersion(v string) {
	s.version = v
}

// ID implements Screen.
func (s *SourceScreen) ID() ScreenID { return ScreenSource }

// Init returns the command required to start cursor blinking.
func (s *SourceScreen) Init() tea.Cmd { return s.input.Init() }

// Update processes a key message and delegates to the text input widget.
func (s *SourceScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	cmd := s.input.Update(msg)
	return s, cmd
}

// View renders the source entry screen.
func (s *SourceScreen) View(width, height int) string {
	s.width = width
	s.height = height
	s.input.Resize(width)

	titleText := "Log Analyzer -- Select Source"
	if s.version != "" {
		titleText = "Log Analyzer v" + s.version + " -- Select Source"
	}
	body := []string{s.styles.Subtitle.Render("Enter the path to your logs root, a single run folder, or a digest.")}
	for _, l := range WrapText(sourceGuidance, width) {
		body = append(body, s.styles.Muted.Render(l))
	}
	body = append(body, strings.Split(s.input.View(), "\n")...)

	// The text input owns every printable key, so the page offers no help overlay.
	return RenderPage(Page{Title: titleText, Keys: sourceKeys}, body, width, height, s.styles)
}

// sourceGuidance says what the path entry accepts.
const sourceGuidance = "Accepts: OrchestrationLogs/ directory, a single run folder (e.g. 20260101T120000Z-abcd), a .digest.json file, or a directory containing digests"

// sourceKeys are the short help keys of the source page.
var sourceKeys = []tuicommon.HelpEntry{
	{Key: "enter", Desc: "confirm"}, {Key: "esc", Desc: "skip"}, {Key: "ctrl+c", Desc: "quit"},
}

// Help returns the key hints string for this screen.
func (s *SourceScreen) Help() string { return leadHelp("", sourceKeys) }

// Done reports whether the user confirmed a valid path.
func (s *SourceScreen) Done() bool { return s.input.Done() }

// Back reports whether the user pressed Esc.
func (s *SourceScreen) Back() bool { return s.input.Back() }

// Path returns the entered path, normalized. Only valid when Done() is true.
func (s *SourceScreen) Path() string { return pathutil.NormalizeInput(s.input.Value()) }

// Reset clears the done and back flags.
func (s *SourceScreen) Reset() { s.input.Reset() }
