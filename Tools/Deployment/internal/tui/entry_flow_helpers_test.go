package tui

// entry_flow_helpers_test.go holds the one place that knows the order of the entry screens and
// how to drive them with the keyboard (choosing the mode, the harness and confirming the path).
// Tests that need a model at a given point of the entry flow call these helpers instead of
// pressing the keys themselves, so a change to the entry order is made here only.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
)

// entryModes lists the modes in the order the mode screen offers them.
var entryModes = []domain.RunMode{
	domain.ModeDeployWorkspace,
	domain.ModeUpdateWorkspace,
	domain.ModeUpdateWorkflows,
	domain.ModeDeployAgents,
	domain.ModeDeployHooks,
	domain.ModePromoteToGeneric,
	domain.ModeTransformHarness,
}

// pressKey sends one key to the model and returns the command it produced.
func pressKey(m *rootModel, keyType tea.KeyType) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: keyType})
	return cmd
}

// pressEnter sends Enter to the model.
func pressEnter(m *rootModel) tea.Cmd { return pressKey(m, tea.KeyEnter) }

// pressEsc sends Esc to the model.
func pressEsc(m *rootModel) tea.Cmd { return pressKey(m, tea.KeyEsc) }

// pressTimes sends the same key n times.
func pressTimes(m *rootModel, keyType tea.KeyType, n int) {
	for i := 0; i < n; i++ {
		pressKey(m, keyType)
	}
}

// requireScreen fails the test unless the model shows want.
func requireScreen(t *testing.T, m *rootModel, want screenID) {
	t.Helper()
	if m.screen != want {
		t.Fatalf("screen = %v; want %v", m.screen, want)
	}
}

// moveListTo puts the cursor of the list on the screen on item index (counting from the first
// item) by going to the top and then down. Lists do not wrap.
func moveListTo(m *rootModel, index int) {
	pressTimes(m, tea.KeyUp, 20)
	pressTimes(m, tea.KeyDown, index)
}

// pickMode chooses mode on the mode screen with Enter.
func pickMode(t *testing.T, m *rootModel, mode domain.RunMode) {
	t.Helper()
	requireScreen(t, m, screenMode)
	for i, candidate := range entryModes {
		if candidate == mode {
			moveListTo(m, i)
			pressEnter(m)
			if m.selections.mode != mode {
				t.Fatalf("selections.mode = %q after choosing it; want %q", m.selections.mode, mode)
			}
			return
		}
	}
	t.Fatalf("mode %q is not offered by the mode screen", mode)
}

// pathFirstModes are the modes whose path screen comes before the harness screen, because the
// harness screen offers only the harnesses the workspace at the path records.
var pathFirstModes = []domain.RunMode{
	domain.ModeUpdateWorkspace,
	domain.ModeUpdateWorkflows,
	domain.ModeTransformHarness,
}

// harnessFirstModes are the modes whose harness screen comes before the path screen.
var harnessFirstModes = []domain.RunMode{
	domain.ModeDeployWorkspace,
	domain.ModeDeployAgents,
	domain.ModeDeployHooks,
	domain.ModePromoteToGeneric,
}

// isPathFirst reports whether mode asks the path before the harness.
func isPathFirst(mode domain.RunMode) bool {
	for _, candidate := range pathFirstModes {
		if candidate == mode {
			return true
		}
	}
	return false
}

// pickHarness chooses the harness with id on the harness screen with Enter and returns the
// command that Enter produced. An empty id takes the first harness the service lists.
func pickHarness(t *testing.T, m *rootModel, id string) tea.Cmd {
	t.Helper()
	requireScreen(t, m, screenHarness)
	harnesses := m.svc.ListHarnesses()
	index := 0
	if id != "" {
		index = -1
		for i, h := range harnesses {
			if h.ID == id {
				index = i
			}
		}
		if index < 0 {
			t.Fatalf("harness %q is not listed by the service", id)
		}
	}
	moveListTo(m, index)
	cmd := pressEnter(m)
	if id != "" && m.selections.harnessID != id {
		t.Fatalf("selections.harnessID = %q after choosing it; want %q", m.selections.harnessID, id)
	}
	return cmd
}

// driveToPath drives a model that shows the mode screen through the entry screens up to the path
// screen. In the harness-first modes it chooses harnessID on the way (empty = the first listed
// harness) unless the session has a preset harness; in the path-first modes the path screen
// follows the mode screen directly. The model must end on the path screen.
func driveToPath(t *testing.T, m *rootModel, mode domain.RunMode, harnessID string) {
	t.Helper()
	pickMode(t, m, mode)
	if !isPathFirst(mode) && m.presetHarness == "" {
		pickHarness(t, m, harnessID)
	}
	requireScreen(t, m, screenWorkspace)
}

// stackAfterHarnessModes are the path-first modes whose harness screen is followed directly by
// the stack screen (Transform asks the target harness in between).
var stackAfterHarnessModes = []domain.RunMode{
	domain.ModeUpdateWorkspace,
	domain.ModeUpdateWorkflows,
}

// flowTargetHarness is a second usable harness, so a Transform run has a target besides the source.
var flowTargetHarness = domain.HarnessRef{
	ID:          "flow-target",
	DisplayName: "Flow Target Harness",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// onTargetScreen reports whether the model shows the Transform target harness screen: the harness
// screen worded as the harness to transform to.
func onTargetScreen(m *rootModel) bool {
	return m.screen == screenHarness &&
		strings.Contains(strings.ToLower(collapseSpaces(m.View())), "harness to transform to")
}

// highlightedHarness returns the ID of the harness the harness screen has its cursor on.
func highlightedHarness(m *rootModel) string { return m.harnessScreen.SelectedID() }

// pickTarget chooses the harness with id on the Transform target screen with Enter and returns the
// command that Enter produced. An empty id takes the first listed usable harness that is not the
// source.
func pickTarget(t *testing.T, m *rootModel, id string) tea.Cmd {
	t.Helper()
	if !onTargetScreen(m) {
		t.Fatalf("not on the target harness screen (screen %v):\n%s", m.screen, m.View())
	}
	index := -1
	for i, h := range m.svc.ListHarnesses() {
		if (id == "" && h.Usable && h.ID != m.selections.harnessID) || (id != "" && h.ID == id) {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("no harness %q to choose as the target", id)
	}
	moveListTo(m, index)
	return pressEnter(m)
}

// pickTargetIfAsked chooses the first usable non-source harness when the target screen is shown.
func pickTargetIfAsked(t *testing.T, m *rootModel) tea.Cmd {
	t.Helper()
	if onTargetScreen(m) {
		return pickTarget(t, m, "")
	}
	return nil
}

// confirmPath confirms the path on the path screen (the model is built with a pre-filled path).
// In the path-first modes it also chooses harnessID (empty = the first listed harness) when the
// harness screen follows the path, and in Transform the first usable non-source harness when the
// target screen follows. It returns the command of the last key it pressed.
func confirmPath(t *testing.T, m *rootModel, harnessID string) tea.Cmd {
	t.Helper()
	requireScreen(t, m, screenWorkspace)
	cmd := pressEnter(m)
	if isPathFirst(m.selections.mode) && m.screen == screenHarness && !onTargetScreen(m) {
		cmd = pickHarness(t, m, harnessID)
	}
	if c := pickTargetIfAsked(t, m); c != nil {
		cmd = c
	}
	return cmd
}

// driveEntry drives the entry screens like driveToPath and then confirms the path with
// confirmPath. It returns the command of the last key pressed.
func driveEntry(t *testing.T, m *rootModel, mode domain.RunMode, harnessID string) tea.Cmd {
	t.Helper()
	driveToPath(t, m, mode, harnessID)
	return confirmPath(t, m, harnessID)
}

// driveToRunning drives the entry screens for mode with the first harness and requires the
// model to be running afterwards. It returns the service command.
func driveToRunning(t *testing.T, m *rootModel, mode domain.RunMode) tea.Cmd {
	t.Helper()
	cmd := driveEntry(t, m, mode, "")
	requireScreen(t, m, screenRunning)
	return cmd
}
