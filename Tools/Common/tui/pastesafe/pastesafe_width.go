package pastesafe

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// boundedView renders the prompt, a window of the value around the cursor and
// the cursor itself within f.width cells. The textinput's own windowing only
// scrolls when the cursor reaches an edge, so the window is chosen here to keep
// text on both sides of the cursor visible.
func (f *Field) boundedView() string {
	value := []rune(f.model.Value())
	if len(value) == 0 {
		// Empty value: the textinput renders the placeholder; cut it to the width.
		return ansi.Truncate(f.model.View(), f.width, "")
	}

	pos := f.model.Position()
	if pos > len(value) {
		pos = len(value)
	}
	// The cursor sits on a rune of the value, or on an extra blank cell at the end.
	cells := append(append([]rune{}, value...), ' ')
	avail := f.width - lipgloss.Width(f.model.Prompt)
	if avail < 1 {
		avail = 1
	}
	lo, hi := windowAround(cells, pos, avail)

	f.model.Cursor.SetChar(string(cells[pos]))
	var sb strings.Builder
	sb.WriteString(f.model.PromptStyle.Render(f.model.Prompt))
	sb.WriteString(f.model.TextStyle.Render(string(cells[lo:pos])))
	sb.WriteString(f.model.Cursor.View())
	sb.WriteString(f.model.TextStyle.Render(string(cells[pos+1 : hi])))
	return ansi.Truncate(sb.String(), f.width, "")
}

// windowAround returns the half-open range [lo, hi) of cells that contains pos,
// is at most avail terminal cells wide, and extends to both sides of pos as far
// as the width allows.
func windowAround(cells []rune, pos, avail int) (lo, hi int) {
	lo, hi = pos, pos+1
	used := runeWidth(cells[pos])
	for {
		grew := false
		if lo > 0 && used+runeWidth(cells[lo-1]) <= avail {
			lo--
			used += runeWidth(cells[lo])
			grew = true
		}
		if hi < len(cells) && used+runeWidth(cells[hi]) <= avail {
			used += runeWidth(cells[hi])
			hi++
			grew = true
		}
		if !grew {
			return lo, hi
		}
	}
}

func runeWidth(r rune) int { return lipgloss.Width(string(r)) }
