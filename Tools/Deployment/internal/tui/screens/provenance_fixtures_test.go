package screens_test

// provenance_fixtures_test.go holds the fixtures shared by the catalog-provenance tests of the
// selection screens: one driver per screen that builds the screen with three items, hands it a
// label map and returns what the user sees once the items are listed. Nothing in this file is a
// test itself.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// provWidth and provHeight are wide enough that a list row is never truncated, so a label
// placed next to an item is always visible.
const (
	provWidth  = 160
	provHeight = 40
)

// provScreen drives one selection screen. keys are the ids the label map is keyed by (agent
// keys, workflow ids, hook keys); marks are the texts that identify each item's row in the view.
type provScreen struct {
	name string
	keys [3]string
	// marks[i] is a text that appears on the row of item i and on no other row.
	marks [3]string
	// view builds the screen at the given size, hands it labels (also when nil) and returns
	// the rendered view of every list level that shows the three items.
	view func(width, height int, labels screens.ProvenanceLabels) string
}

// provSend feeds one special key to a screen's Update.
func provSend(update func(tea.Msg) tea.Cmd, k tea.KeyType) {
	update(tea.KeyMsg{Type: k})
}

func provScreens() []provScreen {
	return []provScreen{
		provWorkflowScreen(),
		provDeployAgentScreen(),
		provUtilityScreen(),
		provStandaloneScreen(),
		provInfrastructureScreen(),
		provHookScreen(),
	}
}

func provAgents(keys, names [3]string) []domain.Agent {
	out := make([]domain.Agent, 3)
	for i := range out {
		out[i] = domain.Agent{Key: keys[i], Name: names[i]}
	}
	return out
}

func provWorkflowScreen() provScreen {
	keys := [3]string{"prov-wf-a", "prov-wf-b", "prov-wf-c"}
	return provScreen{
		name: "workflows", keys: keys, marks: [3]string{"[prov-wf-a]", "[prov-wf-b]", "[prov-wf-c]"},
		view: func(w, h int, labels screens.ProvenanceLabels) string {
			cats := []domain.WorkflowCategory{{Name: "Build", Workflows: []domain.Workflow{
				{ID: keys[0], Name: "Flow One"}, {ID: keys[1], Name: "Flow Two"}, {ID: keys[2], Name: "Flow Three"},
			}}}
			s := screens.NewWorkflowBrowserScreen(cats, w, h, plainStyles(), "")
			s.SetProvenance(labels)
			provSend(s.Update, tea.KeyRight) // open the folder
			return s.View()
		},
	}
}

// provDeployAgentScreen puts two agents in an outer folder and the third in a folder nested
// inside it, so the nested level is covered too.
func provDeployAgentScreen() provScreen {
	keys := [3]string{"prov-tree-a", "prov-tree-b", "prov-tree-c"}
	names := [3]string{"Tree Alpha", "Tree Beta", "Tree Gamma"}
	return provScreen{
		name: "deploy-agents", keys: keys, marks: names,
		view: func(w, h int, labels screens.ProvenanceLabels) string {
			agents := provAgents(keys, names)
			root := &screens.AgentTreeNode{Children: []*screens.AgentTreeNode{{
				Name:     "Outer",
				Agents:   agents[:2],
				Children: []*screens.AgentTreeNode{{Name: "Inner", Agents: agents[2:]}},
			}}}
			s := screens.NewDeployAgentScreen(root, w, h, plainStyles(), "")
			s.SetProvenance(labels)
			provSend(s.Update, tea.KeyRight) // enter Outer: Inner folder row, then its own agents
			outer := s.View()
			provSend(s.Update, tea.KeyRight) // cursor is on the Inner folder row: enter it
			return outer + "\n" + s.View()
		},
	}
}

func provUtilityScreen() provScreen {
	keys := [3]string{"prov-util-a", "prov-util-b", "prov-util-c"}
	names := [3]string{"Util Alpha", "Util Beta", "Util Gamma"}
	return provScreen{
		name: "utility-agents", keys: keys, marks: names,
		view: func(w, h int, labels screens.ProvenanceLabels) string {
			s := screens.NewUtilityAgentScreen(provAgents(keys, names), w, h, plainStyles(), "")
			s.SetProvenance(labels)
			return s.View()
		},
	}
}

func provStandaloneScreen() provScreen {
	keys := [3]string{"prov-solo-a", "prov-solo-b", "prov-solo-c"}
	names := [3]string{"Solo Alpha", "Solo Beta", "Solo Gamma"}
	return provScreen{
		name: "standalone-agents", keys: keys, marks: names,
		view: func(w, h int, labels screens.ProvenanceLabels) string {
			cats := []screens.AgentCategory{{Name: "Solos", Agents: provAgents(keys, names)}}
			s := screens.NewStandaloneAgentScreen(cats, w, h, plainStyles(), "")
			s.SetProvenance(labels)
			return s.View()
		},
	}
}

// provInfrastructureScreen lists both orchestrator agent keys and one further agent.
func provInfrastructureScreen() provScreen {
	keys := [3]string{"orchestrator", "orchestrator-script", "prov-infra-x"}
	names := [3]string{"Orchestrator Primary", "Orchestrator Variant", "Infra Extra"}
	return provScreen{
		name: "infrastructure-agents", keys: keys, marks: names,
		view: func(w, h int, labels screens.ProvenanceLabels) string {
			s := screens.NewInfrastructureAgentScreen(provAgents(keys, names), w, h, plainStyles())
			s.SetProvenance(labels)
			return s.View()
		},
	}
}

func provHookScreen() provScreen {
	keys := [3]string{"prov-hook-a", "prov-hook-b", "prov-hook-c"}
	return provScreen{
		name: "hooks", keys: keys, marks: keys,
		view: func(w, h int, labels screens.ProvenanceLabels) string {
			bundles := []domain.HookBundle{{Key: keys[0]}, {Key: keys[1]}, {Key: keys[2]}}
			s := screens.NewHookScreen(bundles, true, w, h, plainStyles(), "")
			s.SetProvenance(labels)
			return s.View()
		},
	}
}

// provRow returns the list-column part of the first line of view whose list column contains
// mark, failing the test when none does. The detail pane right of the separator is left out, so
// a detail title that repeats the focused item's name never stands in for a list row.
func provRow(t *testing.T, view, mark string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		list, _, _ := strings.Cut(line, "│")
		if strings.Contains(list, mark) {
			return list
		}
	}
	t.Fatalf("no row shows %q; view:\n%s", mark, view)
	return ""
}
