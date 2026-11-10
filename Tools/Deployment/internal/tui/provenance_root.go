package tui

// provenance_root.go gives the selection screens the catalog each option comes from. For every
// option of a selection question the root model asks the app service which stack layer supplies
// a catalog item of the question's kind, and hands the layers' catalog ids to the screen as
// labels. It also wires the standalone-agent screen, which has no other overlay.

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// provenanceKinds maps each selection question that shows catalog provenance to the kind of
// catalog item its options are. Questions not listed get no labels.
var provenanceKinds = map[domain.QuestionID]domain.CatalogItemKind{
	domain.QWorkflows:            domain.CatalogItemWorkflow,
	domain.QDeployAgents:         domain.CatalogItemAgent,
	domain.QUtilityAgents:        domain.CatalogItemAgent,
	domain.QStandaloneAgents:     domain.CatalogItemAgent,
	domain.QInfrastructureAgents: domain.CatalogItemAgent,
	domain.QHooks:                domain.CatalogItemHook,
}

// provenanceLabels returns the label of every option the service can attribute to a layer with
// a catalog id. An option without an entry renders as unknown.
func (m *rootModel) provenanceLabels(kind domain.CatalogItemKind, opts []domain.Option) screens.ProvenanceLabels {
	labels := make(screens.ProvenanceLabels, len(opts))
	for _, opt := range opts {
		layer, ok := m.svc.CatalogOf(domain.CatalogItem{Kind: kind, Key: opt.ID})
		if ok && layer.Identity.ID != "" {
			labels[opt.ID] = layer.Identity.ID
		}
	}
	return labels
}

// applyProvenance labels the overlay just built for question q. It does nothing for a question
// outside provenanceKinds.
func (m *rootModel) applyProvenance(q domain.ChoiceQuestion) {
	kind, ok := provenanceKinds[q.ID]
	if !ok {
		return
	}
	labels := m.provenanceLabels(kind, q.Options)
	switch {
	case q.ID == domain.QWorkflows && m.workflowOverlay != nil:
		m.workflowOverlay.SetProvenance(labels)
	case q.ID == domain.QDeployAgents && m.deployAgentOverlay != nil:
		m.deployAgentOverlay.SetProvenance(labels)
	case q.ID == domain.QUtilityAgents && m.agentOverlay != nil:
		m.agentOverlay.SetProvenance(labels)
	case q.ID == domain.QStandaloneAgents && m.standaloneOverlay != nil:
		m.standaloneOverlay.SetProvenance(labels)
	case q.ID == domain.QInfrastructureAgents && m.infraAgentOverlay != nil:
		m.infraAgentOverlay.SetProvenance(labels)
	case q.ID == domain.QHooks && m.hookOverlay != nil:
		m.hookOverlay.SetProvenance(labels)
	}
}

// updateStandaloneOverlay routes a message to the standalone-agent overlay, if one is open, and
// reports whether it handled the message.
func (m *rootModel) updateStandaloneOverlay(msg tea.Msg) (tea.Cmd, bool) {
	if m.standaloneOverlay == nil {
		return nil, false
	}
	cmd := m.standaloneOverlay.Update(msg)
	if m.standaloneOverlay.Done() {
		m.dispatchMultiChoiceAnswer(m.standaloneOverlay.SelectedIDs())
		m.standaloneOverlay = nil
		m.screen = screenRunning
	} else if m.standaloneOverlay.Back() {
		m.dispatchMultiChoiceCancelled()
		m.standaloneOverlay = nil
		m.screen = screenRunning
	}
	return cmd, true
}

// optionsToAgentCategories groups the flat option slice of the standalone-agent question into
// categories by Group, in order of first appearance, with the ungrouped agents last.
func optionsToAgentCategories(opts []domain.Option) []screens.AgentCategory {
	var cats []screens.AgentCategory
	index := map[string]int{}
	for _, opt := range opts {
		i, ok := index[opt.Group]
		if !ok {
			i = len(cats)
			index[opt.Group] = i
			cats = append(cats, screens.AgentCategory{Name: opt.Group})
		}
		cats[i].Agents = append(cats[i].Agents, domain.Agent{Key: opt.ID, Name: opt.Label, Description: opt.Description})
	}
	if i, ok := index[""]; ok && i != len(cats)-1 {
		last := cats[i]
		cats = append(append(cats[:i], cats[i+1:]...), last)
	}
	return cats
}
