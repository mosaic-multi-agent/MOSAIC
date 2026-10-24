package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/tui/pathutil"
	"mosaic-common/tui/widgets"

	"mosaic-log-analyzer/internal/domain"
)

// Contract constants for the digest export action.
const ScreenExport ScreenID = "export"

const (
	KeyExportRun = "e" // export the selected / shown run
	KeyExportAll = "E" // export all runs
)

// exportKeysHelp is the help fragment that lists the export keys.
const exportKeysHelp = KeyExportRun + " export run  " + KeyExportAll + " export all"

// DefaultAllRunsDigestName is the pre-filled file name when exporting all runs.
const DefaultAllRunsDigestName = "runs" + domain.DigestFileSuffix

// ExportAction is what the export screen asks its host to do next.
type ExportAction uint8

const (
	ExportNone      ExportAction = iota // nothing requested yet
	ExportSubmit                        // a valid path was entered; export without overwriting
	ExportOverwrite                     // the user confirmed replacing the existing file
	ExportCancel                        // the user left the path entry
	ExportDecline                       // the user declined to overwrite
)

// ExportScreen asks for the digest file path and, when the file already
// exists, for confirmation before overwriting.
//
// Navigation contract:
//   - Enter on a valid path -> Action() == ExportSubmit, Path() returns it.
//   - Esc on path entry     -> Action() == ExportCancel.
//   - After AskOverwrite: y -> ExportOverwrite; n or Esc -> ExportDecline.
type ExportScreen struct {
	runID   string // "" = all runs
	input   *widgets.TextInput
	styles  Styles
	confirm bool
	action  ExportAction
	path    string
}

// NewExportScreen creates the export screen for one run (runID) or, with an
// empty runID, for all runs. The path is pre-filled with a default file name.
func NewExportScreen(runID string, width int, styles Styles) *ExportScreen {
	input := widgets.NewTextInput("Digest file path:", "path"+domain.DigestFileSuffix, width, widgets.TextInputStyles{
		Label:  styles.Subtitle,
		Input:  styles.Body,
		ErrMsg: styles.Error,
	})
	input.SetValidate(validateDigestPath)
	if runID == "" {
		input.SetValue(DefaultAllRunsDigestName)
	} else {
		input.SetValue(runID + domain.DigestFileSuffix)
	}
	return &ExportScreen{runID: runID, input: input, styles: styles}
}

func validateDigestPath(s string) error {
	p := pathutil.NormalizeInput(s)
	if p == "" {
		return errEmptyPath
	}
	if !domain.IsDigestFileName(p) {
		return errString("path must end in " + domain.DigestFileSuffix)
	}
	return nil
}

// ID implements Screen.
func (s *ExportScreen) ID() ScreenID { return ScreenExport }

// Init implements Screen.
func (s *ExportScreen) Init() tea.Cmd { return s.input.Init() }

// Update implements Screen.
func (s *ExportScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if s.confirm {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "Y":
				s.action = ExportOverwrite
			case "n", "N", "esc":
				s.action = ExportDecline
			}
		}
		return s, nil
	}
	cmd := s.input.Update(msg)
	switch {
	case s.input.Back():
		s.action = ExportCancel
	case s.input.Done():
		s.path = pathutil.NormalizeInput(s.input.Value())
		s.action = ExportSubmit
	}
	return s, cmd
}

// View implements Screen.
func (s *ExportScreen) View(width, height int) string {
	s.input.Resize(width)
	sep := s.styles.Border.Width(width).Render(strings.Repeat("-", width))
	title := s.styles.Title.Width(width).Render("Log Analyzer -- Export digest")
	subtitle := s.styles.Subtitle.Width(width).Render(s.heading())
	body := s.input.View()
	if s.confirm {
		body = s.styles.Warning.Width(width).Render(s.path + " already exists. Overwrite? (y/n)")
	}
	help := s.styles.Help.Width(width).Render(s.Help())
	return strings.Join([]string{title, subtitle, sep, "", body, "", sep, help}, "\n")
}

func (s *ExportScreen) heading() string {
	if s.runID == "" {
		return "Export all runs"
	}
	return "Export run " + s.runID
}

// Help implements Screen.
func (s *ExportScreen) Help() string {
	if s.confirm {
		return "y overwrite  n cancel  esc cancel  ctrl+c quit"
	}
	return "enter export  esc cancel  ctrl+c quit"
}

// AskOverwrite switches to the overwrite confirmation for the submitted path.
func (s *ExportScreen) AskOverwrite() {
	s.confirm = true
	s.action = ExportNone
}

// Action reports what the user asked for since the last Reset.
func (s *ExportScreen) Action() ExportAction { return s.action }

// Path returns the entered, normalized path; valid once Action is ExportSubmit.
func (s *ExportScreen) Path() string { return s.path }

// RunID returns the run being exported, or "" for all runs.
func (s *ExportScreen) RunID() string { return s.runID }

// Reset clears the requested action.
func (s *ExportScreen) Reset() {
	s.action = ExportNone
	s.input.Reset()
}
