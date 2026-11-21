package tui

// entry_flow.go routes the entry screens in the root model. The order is mode -> harness ->
// path -> stack (see stack_flow.go) -> running. A session launched with a preset harness skips
// the harness screen. Esc returns to the previous entry screen keeping the choices made so far;
// Esc on the mode screen exits the TUI.

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// updateMode handles input on the mode screen, the first entry screen.
func (m *rootModel) updateMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.modeScreen.Update(msg)
	if m.modeScreen.Back() {
		// Nothing to go back to from the first screen: exit the TUI.
		m.modeScreen.Reset()
		return m, tea.Quit
	}
	if m.modeScreen.Done() {
		m.selections.mode = m.modeScreen.SelectedMode()
		m.modeScreen.Reset()
		if m.presetHarness != "" || m.asksPathFirst() {
			return m.enterPath()
		}
		m.enterHarness()
		return m, nil
	}
	return m, cmd
}

// enterHarness shows the harness screen worded for the chosen mode, with the harness chosen
// earlier (or the one the previous run used) highlighted.
func (m *rootModel) enterHarness() {
	highlight := m.selections.harnessID
	if highlight == "" {
		highlight = m.lastHarness
	}
	m.harnessScreen.Configure(screens.HarnessScreenConfig{
		Role:        screens.HarnessRoleForMode(m.selections.mode),
		Harnesses:   m.svc.ListHarnesses(),
		HighlightID: highlight,
	})
	m.screen = screenHarness
}

// updateHarness handles input on the harness screen, the second entry screen.
func (m *rootModel) updateHarness(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.targetShown {
		return m.updateTarget(msg)
	}
	cmd := m.harnessScreen.Update(msg)
	if m.harnessScreen.Back() {
		m.harnessScreen.Reset()
		if m.asksPathFirst() {
			m.screen = screenWorkspace
			return m, m.wsScreen.InputInit()
		}
		m.screen = screenMode
		return m, nil
	}
	if m.harnessScreen.Done() {
		m.selections.harnessID = m.harnessScreen.SelectedID()
		m.lastHarness = m.selections.harnessID
		m.harnessScreen.Reset()
		if m.asksPathFirst() {
			return m.afterSource()
		}
		return m.enterPath()
	}
	return m, cmd
}

// afterSource continues from a chosen source harness in a path-first mode: Transform asks the
// target harness on its own screen, every other mode goes on to the stack step.
func (m *rootModel) afterSource() (tea.Model, tea.Cmd) {
	if m.selections.mode == domain.ModeTransformHarness {
		m.enterTarget()
		return m, nil
	}
	return m.afterPath()
}

// enterTarget shows the harness screen worded as the harness to transform to. It offers the full
// registry list with the source disabled, highlighting the target chosen earlier or on the
// previous run.
func (m *rootModel) enterTarget() {
	all := m.svc.ListHarnesses()
	sourceName := m.selections.harnessID
	for _, h := range all {
		if h.ID == m.selections.harnessID && h.DisplayName != "" {
			sourceName = h.DisplayName
		}
	}
	highlight := m.selections.targetID
	if highlight == "" {
		highlight = m.lastTarget
	}
	m.harnessScreen.Configure(screens.HarnessScreenConfig{
		Role:           screens.HarnessRoleTransformTarget,
		Harnesses:      all,
		SourceName:     sourceName,
		DisabledID:     m.selections.harnessID,
		DisabledReason: "already the transform source",
		HighlightID:    highlight,
	})
	m.targetShown = true
	m.screen = screenHarness
}

// updateTarget handles input on the target harness screen. Esc returns to the source screen, or
// to the path screen when the source was chosen automatically or preset.
func (m *rootModel) updateTarget(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.harnessScreen.Update(msg)
	if m.harnessScreen.Back() {
		m.harnessScreen.Reset()
		m.targetShown = false
		if m.harnessShown {
			m.showNarrowedHarness(m.offeredHarnesses(m.selections.workspacePath))
			return m, nil
		}
		m.screen = screenWorkspace
		return m, m.wsScreen.InputInit()
	}
	if m.harnessScreen.Done() {
		m.selections.targetID = m.harnessScreen.SelectedID()
		m.lastTarget = m.selections.targetID
		m.harnessScreen.Reset()
		m.targetShown = false
		return m.afterPath()
	}
	return m, cmd
}

// asksPathFirst reports whether the chosen mode asks the path before the harness: the modes
// that work on an existing workspace, whose manifest narrows the harness choice.
func (m *rootModel) asksPathFirst() bool {
	switch m.selections.mode {
	case domain.ModeUpdateWorkspace, domain.ModeUpdateWorkflows, domain.ModeTransformHarness:
		return true
	}
	return false
}

// offeredHarnesses returns the harnesses the harness screen offers for the workspace at path:
// the full list when its manifest records no harness, otherwise the recorded ones. A recorded
// harness the registry does not list is offered disabled, after the registry's, in ascending
// ID order.
func (m *rootModel) offeredHarnesses(path string) []domain.HarnessRef {
	all := m.svc.ListHarnesses()
	recorded := m.svc.RecordedHarnesses(path)
	if len(recorded) == 0 {
		return all
	}
	isRecorded := make(map[string]bool, len(recorded))
	for _, id := range recorded {
		isRecorded[id] = true
	}
	var offered []domain.HarnessRef
	listed := make(map[string]bool, len(all))
	for _, h := range all {
		listed[h.ID] = true
		if isRecorded[h.ID] {
			offered = append(offered, h)
		}
	}
	unlisted := make([]string, 0, len(recorded))
	for _, id := range recorded {
		if !listed[id] {
			unlisted = append(unlisted, id)
		}
	}
	sort.Strings(unlisted)
	for _, id := range unlisted {
		offered = append(offered, domain.HarnessRef{
			ID:             id,
			Usable:         false,
			UnusableReason: "recorded in the workspace manifest, but not available in this session",
		})
	}
	return offered
}

// showNarrowedHarness shows the harness screen with the given harnesses, highlighting the
// harness chosen earlier.
func (m *rootModel) showNarrowedHarness(harnesses []domain.HarnessRef) {
	highlight := m.selections.harnessID
	if highlight == "" {
		highlight = m.lastHarness
	}
	m.harnessScreen.Configure(screens.HarnessScreenConfig{
		Role:        screens.HarnessRoleForMode(m.selections.mode),
		Harnesses:   harnesses,
		HighlightID: highlight,
	})
	m.harnessShown = true
	m.screen = screenHarness
}

// afterPathFirstPath continues a path-first mode from a confirmed path: it applies a preset
// harness (refusing one the manifest does not record), chooses a single usable recorded harness
// automatically, or shows the harness screen.
func (m *rootModel) afterPathFirstPath(path string) (tea.Model, tea.Cmd) {
	m.harnessShown = false
	if m.presetHarness != "" {
		recorded := m.svc.RecordedHarnesses(path)
		if len(recorded) > 0 && !slices.Contains(recorded, m.presetHarness) {
			sorted := slices.Clone(recorded)
			sort.Strings(sorted)
			m.wsScreen.SetExternalError(fmt.Sprintf(
				"harness %q is not recorded in the workspace manifest at %s; recorded harnesses: %s",
				m.presetHarness, path, strings.Join(sorted, ", ")))
			return m, m.wsScreen.InputInit()
		}
		m.selections.workspacePath = path
		m.selections.harnessID = m.presetHarness
		return m.afterSource()
	}
	m.selections.workspacePath = path
	recorded := m.svc.RecordedHarnesses(path)
	offered := m.offeredHarnesses(path)
	if len(recorded) > 0 && len(offered) == 1 && offered[0].Usable {
		m.selections.harnessID = offered[0].ID
		m.lastHarness = offered[0].ID
		return m.afterSource()
	}
	m.showNarrowedHarness(offered)
	return m, nil
}

// enterPath shows the path screen configured for the chosen mode.
func (m *rootModel) enterPath() (tea.Model, tea.Cmd) {
	// Set the path kind on every transition so that back-navigation followed by a different
	// mode selection always applies the correct kind.
	switch m.selections.mode {
	case domain.ModePromoteToGeneric:
		m.wsScreen.SetPathKind(screens.PathKindAgentsFolder)
	case domain.ModeTransformHarness:
		m.wsScreen.SetPathKind(screens.PathKindTransformWorkspace)
	default:
		m.wsScreen.SetPathKind(screens.PathKindDirectory)
	}
	m.screen = screenWorkspace
	// Start the text input cursor blink.
	return m, m.wsScreen.InputInit()
}

// updateWorkspace handles input on the path screen, the third entry screen.
func (m *rootModel) updateWorkspace(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.wsScreen.Update(msg)
	if m.wsScreen.Back() {
		m.wsScreen.Reset()
		if m.presetHarness != "" || m.asksPathFirst() {
			// The harness screen was skipped or comes later; the previous entry screen is the
			// mode screen.
			m.screen = screenMode
			return m, nil
		}
		m.enterHarness()
		return m, nil
	}
	if m.wsScreen.Done() {
		path := m.wsScreen.WorkspacePath()
		m.wsScreen.Reset()
		if m.asksPathFirst() {
			return m.afterPathFirstPath(path)
		}
		m.selections.workspacePath = path
		return m.afterPath()
	}
	return m, cmd
}
