package screens_test

// input_width_test.go verifies that every single-line input in the Deploy screens renders
// within its real width, keeps the cursor and the text around it visible for a value wider
// than the width, and follows a terminal resize after creation.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// wideInputValue has 62 distinct characters, so any visible slice identifies its position.
const wideInputValue = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

const (
	inputHeadMark = "abcde"
	inputTailMark = "VWXYZ"
)

// inputUnderTest is a single-line input reachable through a screen's public API.
type inputUnderTest struct {
	send   func(tea.Msg)
	view   func() string
	resize func(width int)
}

// inputCase builds an input that is ready for typing, created at the given width.
type inputCase struct {
	name  string
	build func(width int) inputUnderTest
}

func inputCases() []inputCase {
	cases := []inputCase{
		{"stack catalogue folder", buildStackInput},
		{"model custom entry", buildModelCustomInput},
		{"text prompt", buildTextPromptInput},
		{"workspace resized before path kind", buildWorkspaceRebuiltAfterResize},
	}
	for _, k := range []struct {
		name string
		kind screens.PathKind
	}{
		{"directory", screens.PathKindDirectory},
		{"file", screens.PathKindFile},
		{"transform workspace", screens.PathKindTransformWorkspace},
	} {
		kind := k.kind
		cases = append(cases, inputCase{"workspace " + k.name, func(w int) inputUnderTest {
			return workspaceInput(screens.NewWorkspaceScreen(w, 24, plainStyles()), kind)
		}})
	}
	return cases
}

func workspaceInput(s *screens.WorkspaceScreen, kind screens.PathKind) inputUnderTest {
	s.SetPathKind(kind)
	return inputUnderTest{
		send:   func(m tea.Msg) { s.Update(m) },
		view:   s.View,
		resize: func(w int) { s.Resize(w, 24) },
	}
}

func buildWorkspaceRebuiltAfterResize(w int) inputUnderTest {
	s := screens.NewWorkspaceScreen(w+40, 24, plainStyles())
	s.Resize(w, 24)
	return workspaceInput(s, screens.PathKindFile)
}

func buildStackInput(w int) inputUnderTest {
	s := screens.NewStackScreen(stackconfig.Description{IncludeDefault: true}, w, 24, plainStyles())
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	return inputUnderTest{
		send:   func(m tea.Msg) { s.Update(m) },
		view:   s.View,
		resize: func(w int) { s.Resize(w, 24) },
	}
}

func buildModelCustomInput(w int) inputUnderTest {
	q := tierModelQuestion([]string{"m1"}, false, false, true, "")
	s := screens.NewModelSelectScreen(q, w, 24, plainStyles())
	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return inputUnderTest{
		send:   func(m tea.Msg) { s.Update(m) },
		view:   s.View,
		resize: func(w int) { s.Resize(w, 24) },
	}
}

func buildTextPromptInput(w int) inputUnderTest {
	s := screens.NewTextPromptScreen(domain.TextQuestion{}, w, 24, plainStyles())
	return inputUnderTest{
		send:   func(m tea.Msg) { s.Update(m) },
		view:   s.View,
		resize: func(w int) { s.Resize(w, 24) },
	}
}

func typeValue(in inputUnderTest, text string) {
	for _, r := range text {
		in.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// inputLineWith returns the view line that contains mark, failing when no line does.
func inputLineWith(t *testing.T, view, mark string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, mark) {
			return line
		}
	}
	t.Fatalf("no line of the view shows %q:\n%s", mark, view)
	return ""
}

func assertInputLineWithin(t *testing.T, view, mark string, width int) {
	t.Helper()
	line := inputLineWith(t, view, mark)
	if got := lipgloss.Width(line); got > width {
		t.Fatalf("input line width = %d, want <= %d: %q", got, width, line)
	}
}

func TestSingleLineInputs_WideValue_ShowsTailWithinWidth(t *testing.T) {
	for _, c := range inputCases() {
		t.Run(c.name, func(t *testing.T) {
			in := c.build(40)

			typeValue(in, wideInputValue)

			assertInputLineWithin(t, in.view(), inputTailMark, 40)
		})
	}
}

func TestSingleLineInputs_WideValue_CursorAtStart_ShowsHeadNotTail(t *testing.T) {
	for _, c := range inputCases() {
		t.Run(c.name, func(t *testing.T) {
			in := c.build(40)
			typeValue(in, wideInputValue)

			in.send(tea.KeyMsg{Type: tea.KeyHome})

			view := in.view()
			assertInputLineWithin(t, view, inputHeadMark, 40)
			if strings.Contains(view, inputTailMark) {
				t.Errorf("tail is shown although the cursor is at the start of a value wider than the input:\n%s", view)
			}
		})
	}
}

func TestSingleLineInputs_ResizeNarrower_UsesNewWidth(t *testing.T) {
	for _, c := range inputCases() {
		t.Run(c.name, func(t *testing.T) {
			in := c.build(40)
			typeValue(in, wideInputValue)

			in.resize(25)

			assertInputLineWithin(t, in.view(), inputTailMark, 25)
		})
	}
}

func TestSingleLineInputs_ResizeWider_ShowsMoreOfValue(t *testing.T) {
	for _, c := range inputCases() {
		t.Run(c.name, func(t *testing.T) {
			in := c.build(25)
			typeValue(in, wideInputValue)

			in.resize(100)

			assertInputLineWithin(t, in.view(), wideInputValue, 100)
		})
	}
}

func TestSingleLineInputs_TypingAfterResize_StillShowsTailWithinNewWidth(t *testing.T) {
	for _, c := range inputCases() {
		t.Run(c.name, func(t *testing.T) {
			in := c.build(80)
			in.resize(25)

			typeValue(in, wideInputValue)

			assertInputLineWithin(t, in.view(), inputTailMark, 25)
		})
	}
}
