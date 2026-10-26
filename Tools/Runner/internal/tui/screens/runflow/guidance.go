package runflow

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui/widgets"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens"
)

const (
	guidanceTitleText      = "Standing guidance"
	guidanceTaskLabelText  = "Appended to task_description"
	guidanceConstraintText = "Appended to constraints"
	guidanceHelpText       = "tab/shift+tab switch field  ctrl+s save  esc cancel"

	// guidanceChromeLines is the number of screen lines the title, the two
	// fields' labels and hint lines, and the help line take besides the text areas.
	guidanceChromeLines = 8
	guidanceMinAreaRows = 3
)

// GuidanceScreen shows the run's standing guidance in two multi-line fields and
// lets the user edit it. It does not save anything itself: the root model reads
// Saved and hands the text to the session.
type GuidanceScreen struct {
	task        *widgets.TextArea
	constraints *widgets.TextArea
	focus       int // 0 task, 1 constraints
	saved       bool
	closed      bool
	errMsg      string
	width       int
	height      int
	styles      screens.Styles
}

// NewGuidanceScreen creates the screen pre-filled from g, with the task field focused.
func NewGuidanceScreen(g domain.Guidance, width, height int, styles screens.Styles) *GuidanceScreen {
	areaStyles := widgets.TextAreaStyles{
		Label:  styles.Subtitle,
		Input:  lipgloss.NewStyle(),
		ErrMsg: styles.Error,
		Help:   styles.Help,
	}
	rows := guidanceAreaRows(height)
	s := &GuidanceScreen{
		task:        widgets.NewTextArea(guidanceTaskLabelText, width, rows, areaStyles),
		constraints: widgets.NewTextArea(guidanceConstraintText, width, rows, areaStyles),
		width:       width,
		height:      height,
		styles:      styles,
	}
	s.task.SetValue(g.TaskDescription)
	s.constraints.SetValue(g.Constraints)
	s.task.Focus()
	return s
}

func guidanceAreaRows(height int) int {
	rows := (height - guidanceChromeLines) / 2
	if rows < guidanceMinAreaRows {
		rows = guidanceMinAreaRows
	}
	return rows
}

// Init returns the cursor blink command of the focused field.
func (s *GuidanceScreen) Init() tea.Cmd { return s.task.Init() }

// Update processes a message.
func (s *GuidanceScreen) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok && !key.Paste {
		switch key.Type {
		case tea.KeyTab:
			s.moveFocus(1)
			return nil
		case tea.KeyShiftTab:
			s.moveFocus(0)
			return nil
		}
	}
	area := s.focused()
	cmd := area.Update(msg)
	if area.Back() {
		area.Reset()
		s.closed = true
	} else if area.Done() {
		area.Reset()
		s.saved = true
		s.errMsg = ""
	}
	return cmd
}

func (s *GuidanceScreen) focused() *widgets.TextArea {
	if s.focus == 0 {
		return s.task
	}
	return s.constraints
}

func (s *GuidanceScreen) moveFocus(to int) {
	if to == s.focus {
		return
	}
	s.focused().Blur()
	s.focus = to
	s.focused().Focus()
}

// View renders the screen.
func (s *GuidanceScreen) View() string {
	parts := []string{
		s.styles.Title.Width(s.width).Render(guidanceTitleText),
		s.task.View(),
		s.constraints.View(),
	}
	if s.errMsg != "" {
		parts = append(parts, s.styles.Error.Width(s.width).Render(s.errMsg))
	}
	parts = append(parts, s.styles.Help.Width(s.width).Render(guidanceHelpText))
	return strings.Join(parts, "\n")
}

// Resize changes the visible size; the text is kept.
func (s *GuidanceScreen) Resize(width, height int) {
	s.width, s.height = width, height
	rows := guidanceAreaRows(height)
	s.task.Resize(width, rows)
	s.constraints.Resize(width, rows)
}

// Saved returns the edited guidance and true from the user's ctrl+s until the
// next SetError.
func (s *GuidanceScreen) Saved() (domain.Guidance, bool) {
	if !s.saved {
		return domain.Guidance{}, false
	}
	return domain.Guidance{TaskDescription: s.task.Value(), Constraints: s.constraints.Value()}, true
}

// Closed reports that the user left without saving.
func (s *GuidanceScreen) Closed() bool { return s.closed }

// SetError shows a save error and clears Saved; the edited text is kept.
func (s *GuidanceScreen) SetError(msg string) {
	s.errMsg = msg
	s.saved = false
}
