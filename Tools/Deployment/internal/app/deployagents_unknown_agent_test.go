package app_test

// deployagents_unknown_agent_test.go verifies the error DeployAgents returns for an agent the
// user requested directly that no catalog in the stack supplies: it says the requested agent
// was not found and names the catalogs searched, with no workflow wording.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

func TestDeployAgents_UnknownRequestedSubagent_SaysRequestedAgentNotFoundAndNamesCatalogs(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})

	_, err := app.New(deps).DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, SubagentIDs: []string{"team-linter"},
		TierModels: map[domain.Tier]string{"HIGH": "model-a"}, AutoConfirmPlan: true})

	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "requested") || !strings.Contains(msg, "team-linter") || !strings.Contains(msg, "not found") {
		t.Errorf("error %q must say the requested subagent team-linter was not found", msg)
	}
	if strings.Contains(msg, "workflow") {
		t.Errorf("error %q must not carry workflow wording for a directly requested agent", msg)
	}
	requireSearchedCatalogs(t, err)
}

func TestDeployAgents_UnknownRequestedUtilityAgent_NamesSearchedCatalogs(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})

	_, err := app.New(deps).DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, UtilityAgentIDs: []string{"util-missing"},
		TierModels: map[domain.Tier]string{"HIGH": "model-a"}, AutoConfirmPlan: true})

	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
	if strings.Contains(err.Error(), "workflow") || !strings.Contains(err.Error(), "util-missing") {
		t.Errorf("error %q must name util-missing without workflow wording", err.Error())
	}
	requireSearchedCatalogs(t, err)
}
