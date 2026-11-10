package tui

// provenance_root_test.go verifies that the root model shows, next to every option of the
// selection questions, the catalog it comes from: for each option it asks the app service for
// the supplying layer of a catalog item of the question's kind, labels the option with the
// layer's catalog id, and renders anything it cannot establish as "unknown". Questions outside
// the selection set get no labels. Tests are in package tui so they can drive the root model.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
)

// provService answers the provenance method from lookup. It stands in for the app service.
type provService struct {
	stubRoutingService
	lookup func(item domain.CatalogItem) (domain.CatalogLayer, bool)
}

func (s *provService) CatalogOf(item domain.CatalogItem) (domain.CatalogLayer, bool) {
	return s.lookup(item)
}

func provLayer(id string) domain.CatalogLayer {
	return domain.CatalogLayer{Identity: domain.CatalogIdentity{ID: id, Version: "1.0.0"}}
}

// provSrcLabel is the label an option with the given id must show when the service answers
// per item through perItemLookup.
func provSrcLabel(id string) string { return "src-" + id }

const provWrongKindLabel = "wrong-kind"

// perItemLookup answers a layer named after the option id for an item of the wanted kind, and a
// layer named provWrongKindLabel for the same key under any other kind, so a lookup with the
// wrong kind is visible on screen.
func perItemLookup(want domain.CatalogItemKind) func(domain.CatalogItem) (domain.CatalogLayer, bool) {
	return func(item domain.CatalogItem) (domain.CatalogLayer, bool) {
		if item.Kind == want {
			return provLayer(provSrcLabel(item.Key)), true
		}
		return provLayer(provWrongKindLabel), true
	}
}

// provQuestion describes one selection question of the root model.
type provQuestion struct {
	name string
	id   domain.QuestionID
	kind domain.CatalogItemKind
	// options are the question's options; marks[i] identifies the row of options[i].
	options []domain.Option
	marks   []string
	// open lists the keys that bring the option rows into view (folder navigation).
	open []tea.KeyType
}

func provQuestions() []provQuestion {
	return []provQuestion{
		{
			name: "workflows", id: domain.QWorkflows, kind: domain.CatalogItemWorkflow,
			options: []domain.Option{
				{ID: "wf-one", Label: "Flow One", Group: "Build"},
				{ID: "wf-two", Label: "Flow Two", Group: "Build"},
			},
			marks: []string{"[wf-one]", "[wf-two]"},
			open:  []tea.KeyType{tea.KeyRight},
		},
		{
			name: "deploy-agents", id: domain.QDeployAgents, kind: domain.CatalogItemAgent,
			options: []domain.Option{
				{ID: "da-one", Label: "Deploy One", Group: "FolderA"},
				{ID: "da-two", Label: "Deploy Two", Group: "FolderA"},
			},
			marks: []string{"Deploy One", "Deploy Two"},
			open:  []tea.KeyType{tea.KeyRight},
		},
		{
			name: "utility-agents", id: domain.QUtilityAgents, kind: domain.CatalogItemAgent,
			options: []domain.Option{{ID: "ut-one", Label: "Util One"}, {ID: "ut-two", Label: "Util Two"}},
			marks:   []string{"Util One", "Util Two"},
		},
		{
			name: "standalone-agents", id: domain.QStandaloneAgents, kind: domain.CatalogItemAgent,
			options: []domain.Option{
				{ID: "so-one", Label: "Solo One", Group: "Solos"},
				{ID: "so-two", Label: "Solo Two", Group: "Solos"},
			},
			marks: []string{"Solo One", "Solo Two"},
		},
		{
			name: "infrastructure-agents", id: domain.QInfrastructureAgents, kind: domain.CatalogItemAgent,
			options: []domain.Option{
				{ID: "orchestrator", Label: "Orchestrator Primary"},
				{ID: "orchestrator-script", Label: "Orchestrator Variant"},
				{ID: "infra-extra", Label: "Infra Extra"},
			},
			marks: []string{"Orchestrator Primary", "Orchestrator Variant", "Infra Extra"},
		},
		{
			name: "hooks", id: domain.QHooks, kind: domain.CatalogItemHook,
			options: []domain.Option{{ID: "hk-one"}, {ID: "hk-two"}},
			marks:   []string{"hk-one", "hk-two"},
		},
	}
}

// provModel returns a root model with a wide window, in the state where questions arrive.
func provModel(svc *provService) *rootModel {
	m := newRootModel(context.Background(), svc, Options{Theme: DefaultTheme()})
	m.screen = screenRunning
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return m
}

// provAsk sends q to the model, opens the folders that hold its rows and returns the view.
func provAsk(m *rootModel, q provQuestion) string {
	m.Update(buildSelectManyMsg(q.id, q.options))
	for _, k := range q.open {
		m.Update(tea.KeyMsg{Type: k})
	}
	return m.View()
}

// provListRow returns the list-column part of the first line whose list column contains mark.
func provListRow(t *testing.T, view, mark string) string {
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

// TestRootModel_SelectionQuestions_LabelEveryOptionWithItsCatalog verifies, for every selection
// question, that each option shows the catalog id the service gave for a catalog item of the
// question's kind with the option id as key.
func TestRootModel_SelectionQuestions_LabelEveryOptionWithItsCatalog(t *testing.T) {
	for _, q := range provQuestions() {
		t.Run(q.name, func(t *testing.T) {
			m := provModel(&provService{lookup: perItemLookup(q.kind)})

			view := provAsk(m, q)

			for i, opt := range q.options {
				row := provListRow(t, view, q.marks[i])
				if !strings.Contains(row, provSrcLabel(opt.ID)) {
					t.Errorf("row of %q does not show its catalog %q:\n%s", opt.ID, provSrcLabel(opt.ID), row)
				}
				if strings.Contains(row, provWrongKindLabel) {
					t.Errorf("row of %q was labelled from an item of the wrong kind:\n%s", opt.ID, row)
				}
			}
		})
	}
}

// TestRootModel_InfrastructureQuestion_BothOrchestratorAgentsShowThePairLayer verifies that the
// orchestrator and the script orchestrator, which the service answers with the layer of the
// pair, both show that layer next to another infrastructure agent's own layer.
func TestRootModel_InfrastructureQuestion_BothOrchestratorAgentsShowThePairLayer(t *testing.T) {
	q := provQuestions()[4]
	m := provModel(&provService{lookup: func(item domain.CatalogItem) (domain.CatalogLayer, bool) {
		if item.Kind != domain.CatalogItemAgent {
			return domain.CatalogLayer{}, false
		}
		if item.Key == "orchestrator" || item.Key == "orchestrator-script" {
			return provLayer("layer-pair"), true
		}
		return provLayer("layer-other"), true
	}})

	view := provAsk(m, q)

	for _, mark := range []string{"Orchestrator Primary", "Orchestrator Variant"} {
		if row := provListRow(t, view, mark); !strings.Contains(row, "layer-pair") {
			t.Errorf("row of %q does not show the pair's layer:\n%s", mark, row)
		}
	}
	if row := provListRow(t, view, "Infra Extra"); !strings.Contains(row, "layer-other") {
		t.Errorf("row of the other agent does not show its own layer:\n%s", row)
	}
}

// TestRootModel_UnresolvedProvenance_RendersUnknownOnEveryQuestion verifies that an option
// renders as unknown when the service does not find its item and when it answers a layer with
// an empty catalog id.
func TestRootModel_UnresolvedProvenance_RendersUnknownOnEveryQuestion(t *testing.T) {
	answers := map[string]func(domain.CatalogItem) (domain.CatalogLayer, bool){
		"not-found": func(domain.CatalogItem) (domain.CatalogLayer, bool) { return domain.CatalogLayer{}, false },
		"empty-id": func(domain.CatalogItem) (domain.CatalogLayer, bool) {
			return domain.CatalogLayer{Root: "/somewhere"}, true
		},
	}
	for name, lookup := range answers {
		for _, q := range provQuestions() {
			t.Run(name+"/"+q.name, func(t *testing.T) {
				m := provModel(&provService{lookup: lookup})

				view := provAsk(m, q)

				for i, opt := range q.options {
					if row := provListRow(t, view, q.marks[i]); !strings.Contains(row, "unknown") {
						t.Errorf("row of %q does not show \"unknown\":\n%s", opt.ID, row)
					}
				}
			})
		}
	}
}

// TestRootModel_QuestionOutsideTheSelectionSet_GetsNoLabels verifies that a multi-select
// question that is not one of the selection questions shows no catalog label, however the
// service would answer.
func TestRootModel_QuestionOutsideTheSelectionSet_GetsNoLabels(t *testing.T) {
	m := provModel(&provService{lookup: func(item domain.CatalogItem) (domain.CatalogLayer, bool) {
		return provLayer("must-not-show"), true
	}})

	m.Update(buildSelectManyMsg("some-other-question", []domain.Option{{ID: "opt-one", Label: "Option One"}}))
	view := m.View()

	if !strings.Contains(view, "Option One") {
		t.Fatalf("the question is not on screen:\n%s", view)
	}
	if strings.Contains(view, "must-not-show") || strings.Contains(view, "unknown") {
		t.Errorf("a question outside the selection set must carry no catalog label:\n%s", view)
	}
}
