package plan_test

// resolve_unknown_requested_agent_test.go verifies the error for agents requested directly
// (not through a workflow): it says the requested agent was not found, lists the searched
// catalogs, carries no workflow wording and still matches plan.ErrUnknownAgent. An agent
// referenced by a workflow keeps its own wording.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

func TestResolveArtifactsFrom_UnknownDirectlyRequestedAgent_ReportsRequestedAgentAndSearchedCatalogs(t *testing.T) {
	cases := []struct {
		name string
		sel  plan.Selection
		id   string
		kind string
	}{
		{"subagent", plan.Selection{SubagentIDs: []string{"team-linter"}}, "team-linter", "subagent"},
		{"utility", plan.Selection{UtilityAgentIDs: []string{"util-missing"}}, "util-missing", "utility"},
		{"infrastructure", plan.Selection{InfrastructureAgentIDs: []string{"infra-missing"}}, "infra-missing", "infrastructure"},
		{"standalone", plan.Selection{StandaloneAgentIDs: []string{"solo-missing"}}, "solo-missing", "standalone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cat := layeredCatalogWithOrchestrator(
				testLayer("team-extras", "/fake/team", false),
				testLayer("mosaic-default", "/fake/mosaic/Catalog", true),
			)

			_, err := plan.ResolveArtifactsFrom(cat, tc.sel)

			if !errors.Is(err, plan.ErrUnknownAgent) {
				t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
			}
			msg := err.Error()
			if !strings.Contains(msg, "requested") || !strings.Contains(msg, tc.kind) ||
				!strings.Contains(msg, tc.id) || !strings.Contains(msg, "not found") {
				t.Errorf("error %q must say the requested %s %q was not found", msg, tc.kind, tc.id)
			}
			if strings.Contains(msg, "workflow") {
				t.Errorf("error %q must not carry workflow wording for a direct request", msg)
			}
			for _, l := range cat.layers {
				if !strings.Contains(msg, l.Identity.ID) {
					t.Errorf("error %q does not name the searched catalog %q", msg, l.Identity.ID)
				}
			}
		})
	}
}

func TestResolveArtifactsFrom_UnknownDirectlyRequestedAgent_PlainCatalogNamesRoot(t *testing.T) {
	cat := &fakeCatalog{root: "/fake/plain-catalogue", orchestrator: makeOrchestrator()}

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{SubagentIDs: []string{"team-linter"}})

	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
	if !strings.Contains(err.Error(), "/fake/plain-catalogue") {
		t.Errorf("error %q must name the catalog root", err.Error())
	}
}

func TestResolveArtifactsFrom_UnknownDirectlyRequestedAgent_ListsCatalogsHighestFirst(t *testing.T) {
	cat := layeredCatalogWithOrchestrator(
		testLayer("first-layer", "/fake/one", false),
		testLayer("second-layer", "/fake/two", false),
	)

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{SubagentIDs: []string{"team-linter"}})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	first, second := strings.Index(err.Error(), "first-layer"), strings.Index(err.Error(), "second-layer")
	if first < 0 || second < 0 || first > second {
		t.Errorf("error %q must list first-layer before second-layer", err.Error())
	}
}

func TestResolveArtifactsFrom_UnknownAgentReferencedByWorkflow_KeepsWorkflowWording(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workflows:    []domain.Workflow{makeWorkflow("broken-wf", "ghost-agent")},
	}

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"broken-wf"}})

	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
	want := `workflow references an agent that does not exist: "ghost-agent" referenced by workflow "broken-wf"`
	if err.Error() != want {
		t.Errorf("error = %q, want unchanged wording %q", err.Error(), want)
	}
}
