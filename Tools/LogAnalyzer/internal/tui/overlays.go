package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/interaction"
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/pastesafe"
)

// ---------------------------------------------------------------------------
// overlaySelectOne — inline select-one question
// ---------------------------------------------------------------------------

// overlaySelectOne renders a simple interactive select-one question overlay
// on top of the current screen.
type overlaySelectOne struct {
	q      interaction.ChoiceQuestion
	cursor int
	offset int // scroll offset of the option list, clamped when drawn
	done   bool
	back   bool
	theme  tuicommon.Theme
}

func newOverlaySelectOne(q interaction.ChoiceQuestion, theme tuicommon.Theme) *overlaySelectOne {
	o := &overlaySelectOne{q: q, theme: theme}
	o.cursor = defaultCursor(q)
	return o
}

// totalOptions returns the number of options including the optional skip entry.
func (o *overlaySelectOne) totalOptions() int {
	if o.q.AllowSkip {
		return len(o.q.Options) + 1
	}
	return len(o.q.Options)
}

// isSkipEntry reports whether idx refers to the synthetic skip entry.
func (o *overlaySelectOne) isSkipEntry(idx int) bool {
	return o.q.AllowSkip && idx == len(o.q.Options)
}

// update processes a key message. Returns true when the overlay is done.
func (o *overlaySelectOne) update(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	switch key.String() {
	case "up", "k":
		if o.cursor > 0 {
			o.cursor--
		}
	case "down", "j":
		if o.cursor < o.totalOptions()-1 {
			o.cursor++
		}
	case "enter":
		if o.isSkipEntry(o.cursor) {
			o.done = true
		} else if o.cursor >= 0 && o.cursor < len(o.q.Options) {
			if !o.q.Options[o.cursor].Disabled {
				o.done = true
			}
		}
	case "esc":
		o.back = true
	}
	return o.done || o.back
}

// answer constructs the ChoiceAnswer from the current overlay state.
func (o *overlaySelectOne) answer() interaction.ChoiceAnswer {
	if o.back {
		return interaction.ChoiceAnswer{Status: interaction.Cancelled}
	}
	if o.isSkipEntry(o.cursor) {
		return interaction.ChoiceAnswer{Status: interaction.SkippedOne}
	}
	if o.cursor >= 0 && o.cursor < len(o.q.Options) {
		return interaction.ChoiceAnswer{
			Status:   interaction.Answered,
			OptionID: o.q.Options[o.cursor].ID,
		}
	}
	return interaction.ChoiceAnswer{Status: interaction.SkippedOne}
}

// ---------------------------------------------------------------------------
// overlayText — inline text-input question
// ---------------------------------------------------------------------------

// overlayText renders a simple text-input question overlay. Key handling is
// delegated to the shared paste-safe field, so a pasted line break never
// submits the answer.
type overlayText struct {
	q     interaction.TextQuestion
	input *pastesafe.Field
	theme tuicommon.Theme
}

func newOverlayText(q interaction.TextQuestion, theme tuicommon.Theme) *overlayText {
	f := pastesafe.NewField(pastesafe.WithValidate(q.Validate))
	if q.Default != "" {
		f.SetValue(q.Default)
	}
	return &overlayText{
		q:     q,
		input: f,
		theme: theme,
	}
}

// init returns the cursor blink command for the text input.
func (o *overlayText) init() tea.Cmd { return o.input.Init() }

// update processes a message. Returns a Cmd for cursor blinking.
func (o *overlayText) update(msg tea.Msg) tea.Cmd { return o.input.Update(msg) }

// finished reports whether the overlay was submitted or cancelled.
func (o *overlayText) finished() bool { return o.input.Done() || o.input.Back() }

// answer constructs the TextAnswer from the current overlay state.
func (o *overlayText) answer() interaction.TextAnswer {
	if o.input.Back() {
		return interaction.TextAnswer{Status: interaction.Cancelled}
	}
	return interaction.TextAnswer{Status: interaction.Answered, Text: o.input.Value()}
}

// defaultCursor returns the index of the question's default option, or 0 when it
// names no selectable option.
func defaultCursor(q interaction.ChoiceQuestion) int {
	for i, opt := range q.Options {
		if opt.ID == q.DefaultOptionID && !opt.Disabled {
			return i
		}
	}
	return 0
}
