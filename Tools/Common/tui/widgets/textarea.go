package widgets

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tabPlaceholder stands in for a tab inside the underlying text area, which
// would otherwise turn every tab into four spaces. It is a private-use rune
// that never occurs in real text; Value() maps it back to a tab.
const tabPlaceholder = ''

// TextAreaStyles holds the resolved styles used by a TextArea.
type TextAreaStyles struct {
	Label  lipgloss.Style
	Input  lipgloss.Style
	ErrMsg lipgloss.Style
	Help   lipgloss.Style
}

// DefaultTextAreaStyles returns a plain style set usable in tests.
func DefaultTextAreaStyles() TextAreaStyles {
	return TextAreaStyles{
		Label:  lipgloss.NewStyle(),
		Input:  lipgloss.NewStyle(),
		ErrMsg: lipgloss.NewStyle(),
		Help:   lipgloss.NewStyle(),
	}
}

// TextArea is a multi-line text entry. Enter (typed, or delivered as part of a
// paste) inserts a line break and never submits. ctrl+s submits; esc cancels.
// A key event that is part of a paste never submits or cancels.
type TextArea struct {
	area     textarea.Model
	label    string
	styles   TextAreaStyles
	width    int
	validate func(string) error
	errMsg   string
	done     bool
	back     bool
}

// NewTextArea creates a TextArea with the given label and visible size.
func NewTextArea(label string, width, height int, styles TextAreaStyles) *TextArea {
	area := textarea.New()
	area.Prompt = ""
	area.ShowLineNumbers = false
	area.CharLimit = 0
	area.MaxHeight = 0
	area.MaxWidth = 0
	area.SetWidth(width)
	area.SetHeight(height)
	return &TextArea{area: area, label: label, styles: styles, width: width}
}

// SetValue pre-fills the text and leaves the cursor at its end. Call before the first Update.
func (t *TextArea) SetValue(v string) { t.area.SetValue(toAreaText(v)) }

// SetValidate attaches a validation function used by ctrl+s.
func (t *TextArea) SetValidate(fn func(string) error) { t.validate = fn }

// Init returns the blink command of the underlying text area.
func (t *TextArea) Init() tea.Cmd { return textarea.Blink }

// Update processes a message.
func (t *TextArea) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return t.forward(msg)
	}
	if !t.area.Focused() {
		return nil
	}
	if key.Paste {
		if key.Type == tea.KeyRunes {
			t.area.InsertString(toAreaText(string(key.Runes)))
		} else if key.Type == tea.KeyEnter {
			t.area.InsertString("\n")
		}
		return nil
	}
	switch key.Type {
	case tea.KeyCtrlS:
		t.submit()
		return nil
	case tea.KeyEsc:
		t.back = true
		return nil
	}
	return t.forward(msg)
}

func (t *TextArea) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	t.area, cmd = t.area.Update(msg)
	return cmd
}

func (t *TextArea) submit() {
	if t.validate != nil {
		if err := t.validate(t.Value()); err != nil {
			t.errMsg = err.Error()
			return
		}
	}
	t.errMsg = ""
	t.done = true
}

// View renders label, text area, error and key hint line.
func (t *TextArea) View() string {
	body := strings.ReplaceAll(t.area.View(), string(tabPlaceholder), " ")
	out := t.styles.Label.Width(t.width).Render(t.label) + "\n" + t.styles.Input.Render(body)
	if t.errMsg != "" {
		out += "\n" + t.styles.ErrMsg.Width(t.width).Render(t.errMsg)
	}
	return out + "\n" + t.styles.Help.Render("ctrl+s submit  esc cancel  enter new line")
}

// Value returns the current text, line breaks as "\n".
func (t *TextArea) Value() string {
	return strings.ReplaceAll(t.area.Value(), string(tabPlaceholder), "\t")
}

// ErrMsg returns the current validation error message.
func (t *TextArea) ErrMsg() string { return t.errMsg }

// Done reports whether the user submitted valid text.
func (t *TextArea) Done() bool { return t.done }

// Back reports whether the user pressed esc.
func (t *TextArea) Back() bool { return t.back }

// Reset clears done, back and error flags, keeping the text.
func (t *TextArea) Reset() {
	t.done = false
	t.back = false
	t.errMsg = ""
}

// Resize changes the visible size; it never changes Value().
func (t *TextArea) Resize(width, height int) {
	t.width = width
	t.area.SetWidth(width)
	t.area.SetHeight(height)
}

// Focus focuses the text area.
func (t *TextArea) Focus() tea.Cmd { return t.area.Focus() }

// Blur removes focus.
func (t *TextArea) Blur() { t.area.Blur() }

// toAreaText normalises line breaks to "\n" and protects tabs from the
// underlying text area's tab replacement.
func toAreaText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\t", string(tabPlaceholder))
}
