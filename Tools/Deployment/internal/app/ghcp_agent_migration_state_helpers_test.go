package app_test

// ghcp_agent_migration_state_helpers_test.go holds the builders shared by the tests that check
// a migrated GHCP agent keeps the state of its legacy X.md: a planted workspace in the legacy
// and in the current naming, the run table over the four flows (full and subset selections),
// and the comparison of a migrated file with the file a normal run leaves for an agent that was
// already named X.agent.md.

import (
	"context"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// gsPlantedModel is the model planted in every legacy agent. It differs from the parity model,
// so a model that is kept from the deployed file cannot be mistaken for one the run supplies.
const gsPlantedModel = "planted-legacy-model"

// gsOpts adjusts one run: a dry run, a review the user declines, the summary receiver and
// adjustments to the service dependencies.
type gsOpts struct {
	out     *domain.RunSummary
	dry     bool
	decline bool
	deps    []func(*app.Deps)
}

// serviceDeps returns the dependency adjustments of the run; a declined review answers no.
func (o gsOpts) serviceDeps() []func(*app.Deps) {
	deps := o.deps
	if o.decline {
		stub := interactiontest.NewBuilder().AnswerReview(false).Build()
		deps = append(append([]func(*app.Deps){}, deps...), func(d *app.Deps) { d.Interaction = stub })
	}
	return deps
}

// gsRun is one flow with one selection. No run supplies a model, so the model of every agent
// has to come from the deployed file.
type gsRun struct {
	name string
	run  func(t *testing.T, ws, harness string, o gsOpts) error
	// must lists the agents the run writes, so migrates.
	must []string
	// infra is true when the run leaves the infrastructure declarations of orchestrators as
	// they were deployed.
	infra bool
	// workflows is true when the run leaves the workflow set of the orchestrators as deployed.
	workflows bool
}

var (
	gsAllAgents = []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"}
	gsOrchSet   = []string{"orchestrator", "orchestrator-script"}
)

func gsKeep(out *domain.RunSummary, sum domain.RunSummary) {
	if out != nil {
		*out = sum
	}
}

type gsRunFunc = func(*testing.T, string, string, gsOpts) error

// gsRuns returns the flow matrix: update, workflows, agents and deploy, each with a full
// selection and, where the flow has one, a subset selection.
func gsRuns() []gsRun {
	ctx := context.Background()
	deployNew := func(utility []string) gsRunFunc {
		return func(t *testing.T, ws, h string, o gsOpts) error {
			sum, err := parityServiceWith(t, o.serviceDeps()...).DeployNew(ctx, app.DeployRequest{
				HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
				WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: utility,
				AutoConfirmPlan: !o.decline, DryRun: o.dry, ConflictDefault: domain.DecisionSkip})
			gsKeep(o.out, sum)
			return err
		}
	}
	deployAgents := func(sub, util, infra []string) gsRunFunc {
		return func(t *testing.T, ws, h string, o gsOpts) error {
			sum, err := parityServiceWith(t, o.serviceDeps()...).DeployAgents(ctx, app.DeployAgentsRequest{
				HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
				SubagentIDs: sub, UtilityAgentIDs: util, InfrastructureAgentIDs: infra,
				StandaloneAgentIDs: []string{}, AgentModels: map[string]string{gaoInfraKey: parityModel},
				AutoConfirmPlan: !o.decline, DryRun: o.dry,
				ConflictDefault: domain.DecisionSkip})
			gsKeep(o.out, sum)
			return err
		}
	}
	workflows := func(ids []string) gsRunFunc {
		return func(t *testing.T, ws, h string, o gsOpts) error {
			sum, err := parityServiceWith(t, o.serviceDeps()...).UpdateWorkflows(ctx, app.WorkflowUpdateRequest{
				HarnessID: h, WorkspacePath: ws, WorkflowIDs: ids,
				AutoConfirmPlan: !o.decline, DryRun: o.dry, ConflictDefault: domain.DecisionSkip})
			gsKeep(o.out, sum)
			return err
		}
	}
	return []gsRun{
		{name: "update/full", must: gsAllAgents, infra: true, workflows: true,
			run: func(t *testing.T, ws, h string, o gsOpts) error {
				sum, err := parityServiceWith(t, o.serviceDeps()...).Update(ctx, app.UpdateRequest{
					HarnessID: h, WorkspacePath: ws, AutoConfirmPlan: !o.decline, DryRun: o.dry,
					ConflictDefault: domain.DecisionSkip})
				gsKeep(o.out, sum)
				return err
			}},
		{name: "workflows/full", must: gsOrchSet, infra: true, workflows: true,
			run: workflows([]string{"parity-flow"})},
		{name: "workflows/no-workflows", must: gsOrchSet, infra: true,
			run: workflows([]string{})},
		{name: "agents/full", must: gsAllAgents,
			run: deployAgents([]string{"parity-runner"}, []string{"parity-helper"}, []string{gaoInfraKey})},
		{name: "agents/subset", must: []string{"parity-runner"}, workflows: true,
			run: deployAgents([]string{"parity-runner"}, []string{}, []string{})},
		{name: "deploy/full", must: gsAllAgents, infra: true, workflows: true,
			run: deployNew([]string{"parity-helper"})},
		{name: "deploy/subset", must: []string{"parity-runner", "orchestrator", "orchestrator-script"},
			infra: true, workflows: true, run: deployNew([]string{})},
	}
}

// gsPlantedWorkspace builds a workspace whose agents carry the user state of a long-lived
// deployment: sentinels in project and custom regions, hand-set tools, an infrastructure
// declaration in the orchestrators and a planted model. With modern false the agents are
// named X.md as recorded by released versions; with modern true they are named X.agent.md.
// It returns the workspace and the planted bytes of each agent by key.
func gsPlantedWorkspace(t *testing.T, harness string, modern bool) (string, map[string][]byte) {
	t.Helper()
	ws, _ := gamPlantedLegacyWorkspace(t, harness, gamLegacyOpts{})
	m := gamManifest(t, ws)
	for _, a := range parityAgents {
		rel := gamRel(a.key, ".md")
		content := gsPlantModel(t, a.key, string(gamRead(t, ws, rel)))
		gamWrite(t, ws, rel, content)
		for i, e := range m.Entries {
			if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == a.key {
				m.Entries[i].ContentHash = manifest.Hash([]byte(content))
			}
		}
	}
	gamSaveManifest(t, ws, m)
	planted := map[string][]byte{}
	for _, a := range parityAgents {
		planted[a.key] = gamRead(t, ws, gamRel(a.key, ".md"))
	}
	if modern {
		gsRenameToCurrent(t, ws)
	}
	return ws, planted
}

// gsPlantModel replaces the model line of a deployed agent document.
func gsPlantModel(t *testing.T, key, doc string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "model:") {
			lines[i] = "model: " + gsPlantedModel
			return strings.Join(lines, "\n")
		}
	}
	t.Fatalf("%s: deployed file has no model line to replace", key)
	return doc
}

// gsRenameToCurrent renames every agent file and manifest entry from X.md to X.agent.md.
func gsRenameToCurrent(t *testing.T, ws string) {
	t.Helper()
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind != domain.ArtifactAgent || strings.HasSuffix(e.TargetPath, ".agent.md") {
			continue
		}
		current := strings.TrimSuffix(e.TargetPath, ".md") + ".agent.md"
		if err := os.Rename(gamAbs(ws, e.TargetPath), gamAbs(ws, current)); err != nil {
			t.Fatalf("rename %s: %v", e.TargetPath, err)
		}
		m.Entries[i].TargetPath = current
	}
	gamSaveManifest(t, ws, m)
}

// gsWantState lists the planted markers a migrated file has to hold, given what the run keeps.
func gsWantState(r gsRun, planted string) []string {
	var want []string
	markers := []string{gamProjectSentinel, gamCustomSentinel, gamHandSetTools, "model: " + gsPlantedModel}
	if r.infra {
		markers = append(markers, gamInfraSentinel)
	}
	if r.workflows {
		markers = append(markers, "parity-flow")
	}
	for _, mk := range markers {
		if strings.Contains(planted, mk) {
			want = append(want, mk)
		}
	}
	return want
}

// gsAssertNoModelTodo checks no TODO item says an agent of keys has no model.
func gsAssertNoModelTodo(t *testing.T, sum domain.RunSummary, keys []string) {
	t.Helper()
	for _, item := range sum.Todos {
		text := strings.ToLower(item.Subject + " " + item.Owner + " " + item.Detail)
		if !strings.Contains(text, "no model") {
			continue
		}
		for _, k := range keys {
			if strings.Contains(text, k) {
				t.Errorf("TODO records a missing model for %s although its legacy file has one: %+v", k, item)
			}
		}
	}
}
