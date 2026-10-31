package tui

// input_width_routing_test.go verifies, through the root model, that a terminal resize
// reaches every single-line input (not only its label): the workspace path, the catalogue
// folder path and the AskText and custom-model question overlays render within the new
// width and keep the end of a wide value visible.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-deploy/internal/domain"
)

const (
	rootWideValue = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rootTailMark  = "VWXYZ"
)

func typeIntoRoot(m *rootModel, text string) {
	for _, r := range text {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func resizeRoot(m *rootModel, width int) {
	m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
}

func assertTailLineWithin(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, rootTailMark) {
			continue
		}
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("input line width = %d, want <= %d: %q", got, width, line)
		}
		return
	}
	t.Fatalf("the end of the value is not visible:\n%s", view)
}

func TestRootModel_ResizeNarrower_WorkspacePathInputUsesNewWidth(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toPath(t)
	resizeRoot(f.m, 120)
	typeIntoRoot(f.m, rootWideValue)

	resizeRoot(f.m, 30)

	assertTailLineWithin(t, f.m.View(), 30)
}

func TestRootModel_ResizeNarrower_StackFolderInputUsesNewWidth(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)
	f.typeText("a")
	resizeRoot(f.m, 120)
	f.typeText(rootWideValue)

	resizeRoot(f.m, 30)

	assertTailLineWithin(t, f.m.View(), 30)
}

func TestRootModel_ResizeNarrower_AskTextOverlayInputUsesNewWidth(t *testing.T) {
	m := newRoutingModel()
	m.Update(buildTextMsg(domain.QCustomTool))
	resizeRoot(m, 120)
	typeIntoRoot(m, rootWideValue)

	resizeRoot(m, 30)

	assertTailLineWithin(t, m.View(), 30)
}

func TestRootModel_ResizeNarrower_CustomModelInputUsesNewWidth(t *testing.T) {
	m := newRoutingModel()
	qMsg := buildSelectOneMsg(domain.QTierModel, []domain.Option{{ID: "gpt-4o", Label: "gpt-4o"}})
	qMsg.choiceQ.AllowCustom = true
	m.Update(qMsg)
	resizeRoot(m, 120)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeIntoRoot(m, rootWideValue)

	resizeRoot(m, 30)

	assertTailLineWithin(t, m.View(), 30)
}

func TestRootModel_ResizeWider_AskTextOverlayShowsWholeValue(t *testing.T) {
	m := newRoutingModel()
	m.Update(buildTextMsg(domain.QCustomTool))
	resizeRoot(m, 30)
	typeIntoRoot(m, rootWideValue)

	resizeRoot(m, 120)

	if !strings.Contains(m.View(), rootWideValue) {
		t.Errorf("whole value is not visible after widening:\n%s", m.View())
	}
}
