package screens

// provenance.go declares the catalog-provenance labels of the selection screens. Each selection
// screen shows, next to every item, the catalog the item comes from; the labels reach it through
// the SetProvenance method of the screen. A screen that never received labels shows none.

import (
	"strings"

	"mosaic-deploy/internal/tui/widgets"
)

// unknownProvenance is the label of an item whose catalog is not known.
const unknownProvenance = "unknown"

// ProvenanceLabels maps option ids (agent keys, workflow ids, hook keys) to a short catalog
// label. A label is the supplying layer's identity id, for example "mosaic-default". A nil map,
// or an id missing from the map, renders as "unknown".
type ProvenanceLabels map[string]string

// provenanceView holds the labels of one screen. Until set is called the screen shows no
// provenance; afterwards every item shows its label, or "unknown".
type provenanceView struct {
	on     bool
	labels ProvenanceLabels
}

func (p *provenanceView) set(labels ProvenanceLabels) {
	p.on = true
	p.labels = labels
}

// apply appends the catalog label to every selectable item. Folder and heading rows carry no
// catalog and are left as they are.
func (p provenanceView) apply(items []widgets.ListItem) []widgets.ListItem {
	if !p.on {
		return items
	}
	for i, it := range items {
		if it.Disabled || strings.HasPrefix(it.ID, "dir:") || strings.HasPrefix(it.ID, "heading:") {
			continue
		}
		label := p.labels[it.ID]
		if label == "" {
			label = unknownProvenance
		}
		items[i].Label = it.Label + "  <" + label + ">"
	}
	return items
}

// multiSelectStyles returns the multi-select styles the selection screens share.
func multiSelectStyles(st Styles) widgets.MultiSelectStyles {
	return widgets.MultiSelectStyles{
		Normal: st.Body, Selected: st.Selected, Checked: st.Checked, Disabled: st.Muted,
		Cursor: "▶", CheckOn: "[x]", CheckOff: "[ ]",
	}
}

// relabeled rebuilds a multi-select with new items, keeping the checked items and the cursor.
func relabeled(old *widgets.MultiSelect, items []widgets.ListItem, st Styles, height, width int) *widgets.MultiSelect {
	ms := widgets.NewMultiSelect(items, height, width, multiSelectStyles(st))
	for _, id := range old.SelectedIDs() {
		ms.SetChecked(id, true)
	}
	ms.SetCursorIndex(old.CursorIndex())
	return ms
}

// SetProvenance hands the catalog labels of the workflows to the screen.
func (s *WorkflowBrowserScreen) SetProvenance(labels ProvenanceLabels) {
	s.prov.set(labels)
	if s.level == wfNavWorkflows {
		s.syncCurrentFolder()
		s.enterCategory(s.currentCatIdx)
	}
}

// SetProvenance hands the catalog labels of the agents, including those of the agent tree.
func (s *DeployAgentScreen) SetProvenance(labels ProvenanceLabels) {
	s.prov.set(labels)
	node := s.currentNode()
	if node == nil {
		return
	}
	s.syncCurrentLevel()
	idx := s.currentCursorIndex()
	s.buildLevelWidget(node)
	if idx < 0 {
		return
	}
	if s.levelList != nil {
		s.levelList.SetCursorIndex(idx)
	} else if s.levelMultisel != nil {
		s.levelMultisel.SetCursorIndex(idx)
		s.updateDetail()
	}
}

// SetProvenance hands the catalog labels of the utility agents to the screen.
func (s *UtilityAgentScreen) SetProvenance(labels ProvenanceLabels) {
	s.prov.set(labels)
	items := s.prov.apply(makeAgentItems(s.agents))
	s.multisel = relabeled(s.multisel, items, s.styles, uaListHeight(s.height), uaListWidth(s.width))
}

// SetProvenance hands the catalog labels of the standalone agents to the screen.
func (s *StandaloneAgentScreen) SetProvenance(labels ProvenanceLabels) {
	s.prov.set(labels)
	items, _ := buildStandaloneItems(s.categories)
	s.multisel = relabeled(s.multisel, s.prov.apply(items), s.styles, uaListHeight(s.height), uaListWidth(s.width))
}

// SetProvenance hands the catalog labels of the infrastructure agents to the screen.
func (s *InfrastructureAgentScreen) SetProvenance(labels ProvenanceLabels) {
	s.prov.set(labels)
	items := s.prov.apply(makeInfraAgentItems(s.agents))
	s.multisel = relabeled(s.multisel, items, s.styles, iaListHeight(s.height), uaListWidth(s.width))
}

// SetProvenance hands the catalog labels of the hook bundles to the screen.
func (s *HookScreen) SetProvenance(labels ProvenanceLabels) {
	s.prov.set(labels)
	items := s.prov.apply(makeHookItems(s.hooks))
	s.multisel = relabeled(s.multisel, items, s.styles, hookListHeight(s.height), hookListWidth(s.width))
}
