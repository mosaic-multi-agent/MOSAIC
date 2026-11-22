package runconfig

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/tui/widgets"
	"mosaic-run/internal/tui/screens"
)

const (
	seedOptionalLabel    = "Seed input path (optional):"
	seedRequiredLabel    = "Seed input path (required):"
	seedOptionalSubtitle = "Enter an optional path to a file or directory to copy into the run folder before the first dispatch. Leave blank to start with no seeded artifacts."
	seedPlaceholder      = "/path/to/seed/file-or-directory"
)

// SeedInputScreen prompts the user for a seed-input path (a file or a
// directory) to copy into the run folder before the first dispatch. The seed is
// optional unless the selected workflow's first agent declares input artifacts
// (see SetRequiredInputs).
//
// Navigation contract:
//   - Optional seed: Enter -> Done() == true. SeedInput() returns the trimmed
//     entered value, which may be the empty string: blank is a legal
//     confirmation.
//   - Required seed: Enter with a value -> Done() == true. The first Enter on a
//     blank value shows a warning naming the missing inputs and leaves
//     Done() == false; a second Enter on the still-blank value confirms with an
//     empty SeedInput(). Any change of the value, Reset, Esc or
//     SetRequiredInputs restarts the confirmation.
//   - Esc -> Back() == true.
//
// The screen performs no filesystem validation. Existence, collision, and
// destination refusals are decided at session start.
type SeedInputScreen struct {
	input    *widgets.TextInput
	width    int
	height   int
	styles   screens.Styles
	required []string // inputs the first agent reads; empty means optional
	warned   bool     // the blank-required warning is showing
}

// NewSeedInputScreen creates the seed-input entry screen in its optional state.
func NewSeedInputScreen(width, height int, styles screens.Styles) *SeedInputScreen {
	s := &SeedInputScreen{width: width, height: height, styles: styles}
	s.input = s.newInput("")
	return s
}

// newInput builds the text input with the label for the current state, holding value.
func (s *SeedInputScreen) newInput(value string) *widgets.TextInput {
	label := seedOptionalLabel
	if len(s.required) > 0 {
		label = seedRequiredLabel
	}
	input := widgets.NewTextInput(label, seedPlaceholder, s.width, widgets.TextInputStyles{
		Label:  s.styles.Subtitle,
		Input:  s.styles.Body,
		ErrMsg: s.styles.Error,
	})
	// No SetValidate call: the blank-required confirmation is handled in Update.
	if value != "" {
		input.SetValue(value)
	}
	return input
}

// Update processes a key message and delegates to the embedded text input. On a
// required seed, Enter on a blank value is intercepted to warn first and only
// confirm when pressed again.
func (s *SeedInputScreen) Update(msg tea.Msg) tea.Cmd {
	before := s.input.Value()
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter && len(s.required) > 0 {
		if s.SeedInput() == "" && !s.warned {
			s.warned = true
			return nil
		}
	}
	cmd := s.input.Update(msg)
	if s.input.Value() != before || s.input.Back() {
		s.warned = false
	}
	return cmd
}

// View renders the seed-input screen. Returns a non-empty string containing
// the screen title, subtitle, input field, any blank-required warning, and help line.
func (s *SeedInputScreen) View() string {
	title := s.styles.Title.Width(s.width).Render("Seed Input")
	subtitle := s.styles.Subtitle.Width(s.width).Render(s.subtitleText())
	border := s.styles.Border.Width(s.width).Render(strings.Repeat("─", s.width))
	inputView := s.input.View()
	if s.warned {
		warning := "No seed input given, but the first agent reads " + s.requiredList() +
			". Press Enter again to start without it, or enter a path."
		inputView += "\n" + s.styles.Error.Width(s.width).Render(warning)
	}
	help := s.styles.Help.Width(s.width).Render("enter confirm  esc back  ctrl+c quit")
	return strings.Join([]string{title, subtitle, border, inputView, border, help}, "\n")
}

func (s *SeedInputScreen) subtitleText() string {
	if len(s.required) == 0 {
		return seedOptionalSubtitle
	}
	return "This workflow's first agent reads " + s.requiredList() +
		". Enter the path to a file or directory to copy into the run folder before the first dispatch."
}

func (s *SeedInputScreen) requiredList() string { return strings.Join(s.required, ", ") }

// Done reports whether the user confirmed the entered path. For an optional
// seed an empty path is a legal confirmation; for a required seed it needs the
// second Enter.
func (s *SeedInputScreen) Done() bool { return s.input.Done() }

// Back reports whether the user pressed Esc.
func (s *SeedInputScreen) Back() bool { return s.input.Back() }

// SeedInput returns the entered path, normalised (whitespace-trimmed,
// surrounding double quotes stripped). Empty string means the
// user chose not to seed. Only meaningful when Done() is true.
func (s *SeedInputScreen) SeedInput() string { return screens.NormalizePath(s.input.Value()) }

// Reset clears both the Done and Back flags and any pending blank confirmation
// before the screen is re-entered.
func (s *SeedInputScreen) Reset() {
	s.input.Reset()
	s.warned = false
}

// InputInit returns the command required to start cursor blinking.
// Returned by the root model when transitioning into this screen.
func (s *SeedInputScreen) InputInit() tea.Cmd { return s.input.Init() }

// Resize updates the screen dimensions and forwards the width to the text input.
func (s *SeedInputScreen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.input.Resize(width)
}

// SetRequiredInputs sets the seed requirement of the selected workflow.
// Empty or nil: optional wording, exactly today's text. Non-empty: required
// wording naming the inputs. The entered value is kept. Any pending blank
// confirmation is cleared.
func (s *SeedInputScreen) SetRequiredInputs(inputs []string) {
	s.required = append([]string(nil), inputs...)
	s.warned = false
	s.input = s.newInput(s.input.Value())
}
