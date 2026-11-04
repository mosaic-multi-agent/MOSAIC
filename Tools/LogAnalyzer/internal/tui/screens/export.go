package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
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
	pageChrome
}

// NewExportScreen creates the export screen for one run (runID) or, with an
// empty runID, for all runs. The path is pre-filled with a default file name.
func NewExportScreen(runID string, width int, styles Styles) *ExportScreen {
	input := widgets.NewTextInput("Digest file path:", "path"+domain.DigestFileSuffix, width, textInputStyles(styles))
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
		if key, ok := msg.(tea.KeyMsg); ok && !s.help.handle(key.String()) {
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
	body := []string{s.styles.Subtitle.Render(s.heading())}
	page := Page{Title: exportTitle, Keys: exportPathKeys}
	if !s.confirm {
		body = append(body, strings.Split(s.input.View(), "\n")...)
		return RenderPage(page, body, width, height, s.styles)
	}
	warning := s.styles.Theme.Style(tuicommon.RoleSignalWarning)
	for _, l := range WrapText(s.path+" already exists. Overwrite? (y/n)", width) {
		body = append(body, warning.Render(l))
	}
	page.Keys, page.Help = exportConfirmKeys, true
	return s.render(page, exportOverlayGroups(), body, width, height, s.styles)
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
		return leadHelp("", exportConfirmKeys)
	}
	return leadHelp("", exportPathKeys)
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
