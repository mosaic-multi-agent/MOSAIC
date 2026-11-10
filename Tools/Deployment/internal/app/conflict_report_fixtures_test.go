package app_test

// conflict_report_fixtures_test.go holds the fixtures shared by the tests of the catalog-stack
// conflict report in the write paths: a real two-layer stack in which a user layer shadows the
// default layer's agent, deployed orchestrator files that embed workflows, and one table of
// the write operations. Nothing in this file is a test itself.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// crUserID and crDefaultID are the catalog ids of the two fixture layers.
const (
	crUserID    = "user-a"
	crDefaultID = "mosaic-default"
)

// crAgentKey is the agent both layers supply, so the user layer shadows the default layer's.
// It is the agent the shared minimal workflow routes to.
const crAgentKey = "test-runner"

// crOtherAgentKey is an agent that no layer shadows; workflows that route only to it are
// never affected by the shadowed agent.
const crOtherAgentKey = "other-agent"

// crOtherAgent is the stub catalog entry of crOtherAgentKey.
var crOtherAgent = domain.Agent{Key: crOtherAgentKey, NumericID: "9", Version: "1.0", Name: "Other", Role: domain.RoleWorker}

// crAgentFile returns a generic subagent source with the given version.
func crAgentFile(key, version string) string {
	return "---\nid: \"" + key + "-id\"\nversion: \"" + version + "\"\nname: " + key + "\n" +
		"description: Test agent.\nrole: subagent\nrequired_skills: []\n---\nBody.\n"
}

// crWriteAgent writes a subagent source into a catalog root.
func crWriteAgent(t *testing.T, root, key, version string) {
	t.Helper()
	writeSysFile(t, filepath.Join(root, "Subagents", "general", key+".md"), crAgentFile(key, version))
}

// crWriteOrchestratorPair writes both orchestrator source files into a catalog root.
func crWriteOrchestratorPair(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"orchestrator", "orchestrator-script"} {
		writeSysFile(t, filepath.Join(root, "Orchestrator", name+".md"),
			"---\nversion: \"1.0.0\"\nname: "+name+"\ndescription: Orchestrator.\nrole: orchestrator\n---\nBody.\n")
	}
}

// crDeclareOverrides rewrites the identity of a layer so that it declares overridesBody (YAML
// lines indented by two spaces under "overrides:").
func crDeclareOverrides(t *testing.T, root, id, overridesBody string) {
	t.Helper()
	writeSysFile(t, catalogpaths.IdentityFile(root),
		"id: "+id+"\nname: Fixture "+id+"\nversion: \"1.0.0\"\noverrides:\n"+overridesBody)
}

// crStackOpts tunes the fixture stack and the stub catalog behind it.
type crStackOpts struct {
	// declareAgent makes the user layer declare its override of the shared agent.
	declareAgent bool
	// orchestratorPair makes both layers supply the orchestrator pair.
	orchestratorPair bool
	// workflows are extra workflows added to the stub catalog.
	workflows []domain.Workflow
}

// crWorkflow returns a catalog workflow that routes to the given agents.
func crWorkflow(id string, agents ...string) domain.Workflow {
	return domain.Workflow{ID: id, Name: id, Description: id, Hint: id, Version: "1.0", Category: "Build", ReferencedAgents: agents}
}

// crLayers builds the two-layer fixture stack on disk and returns the MOSAIC root and the
// layers, highest first: the user layer sits above the default layer and both supply
// crAgentKey. The default layer alone supplies the protocol and the bundle, so the only shadow
// records are the ones opts asks for.
func crLayers(t *testing.T, opts crStackOpts) (string, []stack.Layer) {
	t.Helper()
	mosaic := newSysMosaicRoot(t, sysLayer{id: crDefaultID, protocolVersion: "1.0", bundleVersion: "1.0.0"})
	user := newSysUserLayer(t, sysLayer{id: crUserID})
	crWriteAgent(t, filepath.Join(mosaic, catalogpaths.CatalogDirName), crAgentKey, "1.0")
	crWriteAgent(t, user, crAgentKey, "2.0")
	if opts.orchestratorPair {
		crWriteOrchestratorPair(t, filepath.Join(mosaic, catalogpaths.CatalogDirName))
		crWriteOrchestratorPair(t, user)
	}
	if opts.declareAgent {
		crDeclareOverrides(t, user, crUserID, "  agents: ["+crAgentKey+"]\n")
	}
	return mosaic, []stack.Layer{{Root: user}, defaultStackLayer(mosaic)}
}

// crDeps returns deps whose catalog is the app's stub catalog with the resolution of the
// fixture stack. The stub catalog carries the minimal workflow "quick-fix" (routes to
// crAgentKey), a script orchestrator, and the workflows of opts.
func crDeps(t *testing.T, stub *interactiontest.Stub, opts crStackOpts) (app.Deps, string) {
	t.Helper()
	deps, ws := newBaseDeps(t, stub)
	cat := deps.Catalog.(*stubCatalog)
	cat.orchestratorScript = domain.Agent{Key: "orchestrator-script", Version: "1.0", Name: "Orchestrator Script", Role: domain.RoleOrchestrator}
	cat.orchestratorScriptOK = true
	cat.agents = append(cat.agents, crOtherAgent)
	cat.workflows = append(cat.workflows, opts.workflows...)
	mosaic, layers := crLayers(t, opts)
	useResolvedStack(t, &deps, mosaic, layers...)
	return deps, ws
}

// crSpyExecutor records whether the executor ran and whether it was asked for a dry run.
// onExecute, when set, runs inside Execute so a test can change the workspace the way a real
// write would.
type crSpyExecutor struct {
	called    bool
	dryRun    bool
	onExecute func()
}

func (e *crSpyExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.called = true
	e.dryRun = req.DryRun
	if e.onExecute != nil {
		e.onExecute()
	}
	return deploy.ExecResult{}, nil
}

// crDeployedOrchestrator returns a deployed orchestrator file that embeds the given workflows
// as managed regions, the shape a real deploy writes.
func crDeployedOrchestrator(workflowIDs ...string) []byte {
	body := "---\nversion: \"1.0\"\n---\n\n<Identity type=\"core\">\n<AvailableWorkflows type=\"managed\">\n"
	for _, id := range workflowIDs {
		body += fmt.Sprintf("<Workflow type=\"managed\" name=%q version=\"1.0\">\nBody of %s.\n</Workflow>\n", id, id)
	}
	return []byte(body + "</AvailableWorkflows>\n</Identity>\n")
}

// crWriteDeployed writes a deployed orchestrator-role file into the workspace.
func crWriteDeployed(t *testing.T, ws, fileName string, workflowIDs ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, fileName), crDeployedOrchestrator(workflowIDs...), 0o644); err != nil {
		t.Fatalf("write deployed %s: %v", fileName, err)
	}
}

// crFlags are the run options the operation table passes through.
type crFlags struct {
	autoConfirm bool
	dryRun      bool
}

// crOperation is one write-planning operation of the service, driven with fixed selections.
type crOperation struct {
	name string
	// selectsQuickFix is true when the operation deploys the workflow "quick-fix" itself.
	selectsQuickFix bool
	run             func(ctx context.Context, svc app.Service, ws string, f crFlags) (domain.RunSummary, error)
}

// crOperations lists deploy, update, workflows, agents and hooks.
func crOperations() []crOperation {
	return []crOperation{
		{name: "deploy", selectsQuickFix: true, run: func(ctx context.Context, svc app.Service, ws string, f crFlags) (domain.RunSummary, error) {
			return svc.DeployNew(ctx, app.DeployRequest{HarnessID: "stub-harness", WorkspacePath: ws,
				WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun})
		}},
		{name: "update", run: func(ctx context.Context, svc app.Service, ws string, f crFlags) (domain.RunSummary, error) {
			return svc.Update(ctx, app.UpdateRequest{HarnessID: "stub-harness", WorkspacePath: ws,
				AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun})
		}},
		{name: "workflows", selectsQuickFix: true, run: func(ctx context.Context, svc app.Service, ws string, f crFlags) (domain.RunSummary, error) {
			return svc.UpdateWorkflows(ctx, app.WorkflowUpdateRequest{HarnessID: "stub-harness", WorkspacePath: ws,
				WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun})
		}},
		{name: "agents", run: func(ctx context.Context, svc app.Service, ws string, f crFlags) (domain.RunSummary, error) {
			return svc.DeployAgents(ctx, app.DeployAgentsRequest{HarnessID: "stub-harness", WorkspacePath: ws,
				SubagentIDs: []string{crAgentKey}, TierModels: map[domain.Tier]string{"HIGH": "model-a"},
				AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun})
		}},
		{name: "hooks", run: func(ctx context.Context, svc app.Service, ws string, f crFlags) (domain.RunSummary, error) {
			return svc.DeployHooks(ctx, app.DeployHooksRequest{HarnessID: "stub-harness", WorkspacePath: ws,
				HookIDs: []string{}, AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun})
		}},
	}
}

// crLineFor returns the single report line for item and fails the test when there is not
// exactly one.
func crLineFor(t *testing.T, report domain.ConflictReport, item domain.CatalogItem) domain.ConflictLine {
	t.Helper()
	var found []domain.ConflictLine
	for _, l := range report.Lines {
		if l.Item == item {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one report line for %+v, got %d (report: %+v)", item, len(found), report.Lines)
	}
	return found[0]
}

// crAgentItem is the report item of the shared agent.
func crAgentItem() domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: crAgentKey}
}

// crSnapshotDir returns every file under dir (relative path to content), so a test can prove a
// workspace is byte-identical before and after a run.
func crSnapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(dir, p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}
