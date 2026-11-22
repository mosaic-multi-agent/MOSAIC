package screens_test

// deployagents_layout_test.go verifies the terminal-height contract of DeployAgentScreen: the
// whole view fits the terminal however many agents are selected (also on a 40 column terminal),
// the title stays the first line, the list scrolls with the cursor row rendered, and the content
// block is no taller than the rows need.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func deployAgentsLayout() layoutScreen {
	return layoutScreen{
		name:       "deploy-agents",
		title:      "Select Agents",
		hasSummary: true,
		topMarks:   []string{layoutRowName(0)},
		build: func(w, h, n int, withProv bool) layoutDriver {
			agents := make([]domain.Agent, n)
			for i := range agents {
				agents[i] = domain.Agent{Key: layoutRowKey(i), Name: layoutRowName(i), Description: layoutDescription()}
			}
			root := &screens.AgentTreeNode{Children: []*screens.AgentTreeNode{{Name: "Folder", Agents: agents}}}
			s := screens.NewDeployAgentScreen(root, w, h, plainStyles(), "")
			if withProv {
				s.SetProvenance(layoutLabels(n))
			}
			s.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter the folder that lists the agents
			return s
		},
	}
}

func TestDeployAgentScreenLayout_FitsWhateverIsSelected(t *testing.T) {
	layoutRunFitsWhateverSelected(t, deployAgentsLayout())
}

func TestDeployAgentScreenLayout_CursorStaysVisibleWhileScrolling(t *testing.T) {
	layoutRunCursorStaysVisible(t, deployAgentsLayout())
}

func TestDeployAgentScreenLayout_ResizeKeepsFit(t *testing.T) {
	layoutRunResizeKeepsFit(t, deployAgentsLayout())
}

func TestDeployAgentScreenLayout_ProvenanceLabelsKeepFit(t *testing.T) {
	layoutRunProvenanceKeepsFit(t, deployAgentsLayout())
}

func TestDeployAgentScreenLayout_ShortListIsNotPadded(t *testing.T) {
	layoutRunShortListIsNotPadded(t, deployAgentsLayout())
}

func TestDeployAgentScreenLayout_ValidationMessageKeepsFit(t *testing.T) {
	layoutRunValidationFits(t, deployAgentsLayout())
}

// TestDeployAgentScreenLayout_FolderLevelShortListIsNotPadded covers the top level, where the
// rows are folders: two folders must not leave a tall empty area either.
func TestDeployAgentScreenLayout_FolderLevelShortListIsNotPadded(t *testing.T) {
	sc := deployAgentsLayout()
	root := &screens.AgentTreeNode{Children: []*screens.AgentTreeNode{
		{Name: "Folder-A", Agents: []domain.Agent{{Key: "a", Name: "A", Description: layoutDescription()}}},
		{Name: "Folder-B", Agents: []domain.Agent{{Key: "b", Name: "B", Description: layoutDescription()}}},
	}}
	s := screens.NewDeployAgentScreen(root, 80, 40, plainStyles(), "")

	view := s.View()

	if got := contentHeight(view); got < 0 || got > 3 {
		t.Errorf("%s top level with 2 folders: content block is %d lines tall; want at most 3\n%s", sc.name, got, view)
	}
}
