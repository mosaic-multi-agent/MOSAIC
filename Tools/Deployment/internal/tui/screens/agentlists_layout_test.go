package screens_test

// agentlists_layout_test.go verifies the terminal-height contract of the utility,
// infrastructure and standalone agent screens: the view fits the terminal however many agents
// are selected (also on a 40 column terminal), the title stays the first line, the list scrolls
// with the cursor row rendered, and a short list leaves no tall empty area.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func layoutAgents(n int) []domain.Agent {
	agents := make([]domain.Agent, n)
	for i := range agents {
		agents[i] = domain.Agent{Key: layoutRowKey(i), Name: layoutRowName(i), Description: layoutDescription()}
	}
	return agents
}

func utilityLayout() layoutScreen {
	return layoutScreen{
		name: "utility-agents", title: "Select Utility Agents", topMarks: []string{layoutRowName(0)},
		build: func(w, h, n int, withProv bool) layoutDriver {
			s := screens.NewUtilityAgentScreen(layoutAgents(n), w, h, plainStyles(), "")
			if withProv {
				s.SetProvenance(layoutLabels(n))
			}
			return s
		},
	}
}

func infrastructureLayout() layoutScreen {
	return layoutScreen{
		name: "infrastructure-agents", title: "Select Infrastructure Agents", topMarks: []string{layoutRowName(0)},
		build: func(w, h, n int, withProv bool) layoutDriver {
			s := screens.NewInfrastructureAgentScreen(layoutAgents(n), w, h, plainStyles())
			if withProv {
				s.SetProvenance(layoutLabels(n))
			}
			return s
		},
	}
}

// standaloneLayout allows one extra content line for the category heading row.
func standaloneLayout() layoutScreen {
	return layoutScreen{
		name: "standalone-agents", title: "Select Standalone Agents", slack: 1,
		topMarks: []string{layoutRowName(0), "Solos"},
		build: func(w, h, n int, withProv bool) layoutDriver {
			cats := []screens.AgentCategory{{Name: "Solos", Agents: layoutAgents(n)}}
			s := screens.NewStandaloneAgentScreen(cats, w, h, plainStyles(), "")
			if withProv {
				s.SetProvenance(layoutLabels(n))
			}
			return s
		},
	}
}

func layoutAgentScreens() []layoutScreen {
	return []layoutScreen{utilityLayout(), infrastructureLayout(), standaloneLayout()}
}

func TestAgentListScreensLayout_FitWhateverIsSelected(t *testing.T) {
	for _, sc := range layoutAgentScreens() {
		t.Run(sc.name, func(t *testing.T) { layoutRunFitsWhateverSelected(t, sc) })
	}
}

func TestAgentListScreensLayout_CursorStaysVisibleWhileScrolling(t *testing.T) {
	for _, sc := range layoutAgentScreens() {
		t.Run(sc.name, func(t *testing.T) { layoutRunCursorStaysVisible(t, sc) })
	}
}

func TestAgentListScreensLayout_ResizeKeepsFit(t *testing.T) {
	for _, sc := range layoutAgentScreens() {
		t.Run(sc.name, func(t *testing.T) { layoutRunResizeKeepsFit(t, sc) })
	}
}

func TestAgentListScreensLayout_ProvenanceLabelsKeepFit(t *testing.T) {
	for _, sc := range layoutAgentScreens() {
		t.Run(sc.name, func(t *testing.T) { layoutRunProvenanceKeepsFit(t, sc) })
	}
}

func TestAgentListScreensLayout_ShortListIsNotPadded(t *testing.T) {
	for _, sc := range layoutAgentScreens() {
		t.Run(sc.name, func(t *testing.T) { layoutRunShortListIsNotPadded(t, sc) })
	}
}
