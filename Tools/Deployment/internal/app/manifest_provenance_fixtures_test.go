package app_test

// manifest_provenance_fixtures_test.go holds the fixtures shared by the tests of manifest
// provenance in the write paths: a two-layer stack in which the default layer supplies the
// protocol and the user layer supplies the bundle, a catalog that answers provenance per item,
// an executor that records every request, and one table of the write operations. Nothing in
// this file is a test itself.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// Identities of the fixture layers, as written by writeSysLayer.
var (
	provDefaultIdentity = domain.CatalogIdentity{ID: crDefaultID, Name: "Fixture " + crDefaultID, Version: "1.0.0"}
	provUserIdentity    = domain.CatalogIdentity{ID: crUserID, Name: "Fixture " + crUserID, Version: "1.0.0"}
)

// Per-item sources: every item comes from a different layer or a recognisable third catalog.
var (
	provAgentSource  = provUserIdentity
	provOrchSource   = provDefaultIdentity
	provHookSource   = domain.CatalogIdentity{ID: "hook-cat", Name: "Hook Catalog", Version: "4.2"}
	provAgentRef     = domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "test-runner"}
	provOrchRef      = domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "orchestrator"}
	provHookRef      = domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "stub-hook"}
	provSkillRef     = domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "stub-skill"}
	provSkillSource  = domain.CatalogIdentity{ID: "skill-cat", Name: "Skill Catalog", Version: "7.1"}
	provExpectedByRf = map[domain.ArtifactRef]domain.CatalogIdentity{
		provAgentRef: provAgentSource,
		provOrchRef:  provOrchSource,
		provHookRef:  provHookSource,
		provSkillRef: provSkillSource,
	}
)

// provCatalog is a stack catalog whose item provenance is a fixed table.
type provCatalog struct {
	resolvedStackCatalog
	byItem map[domain.CatalogItem]domain.CatalogLayer
}

func (c provCatalog) Provenance(i domain.CatalogItem) (domain.CatalogLayer, bool) {
	l, ok := c.byItem[i]
	return l, ok
}

// recordingExecutor records every request it receives.
type recordingExecutor struct {
	reqs []deploy.ExecRequest
}

func (e *recordingExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.reqs = append(e.reqs, req)
	return deploy.ExecResult{DeploymentRoot: req.Plan.WorkspacePath}, nil
}

func (e *recordingExecutor) last(t *testing.T) deploy.ExecRequest {
	t.Helper()
	if len(e.reqs) == 0 {
		t.Fatal("the executor was never called")
	}
	return e.reqs[len(e.reqs)-1]
}

// provPlan plans the agent, the orchestrator and the hook, whatever the operation.
func provPlan(ws string) domain.Plan {
	p := newMinimalPlan(ws)
	p.Items = []domain.PlanItem{
		{Ref: provAgentRef, TargetPath: "test-runner.md", Action: domain.ActionCreate,
			Model: domain.ModelSelection{ModelID: "model-a", Origin: domain.OriginTierMapping}},
		{Ref: provOrchRef, TargetPath: "orchestrator.md", Action: domain.ActionCreate,
			Model: domain.ModelSelection{ModelID: "model-a", Origin: domain.OriginTierMapping}},
		{Ref: provHookRef, TargetPath: "stub-hook", Action: domain.ActionCreate},
		{Ref: provSkillRef, TargetPath: "stub-skill/SKILL.md", Action: domain.ActionCreate},
	}
	return p
}

// provDeps returns deps on a real two-layer stack (protocol from the default layer, bundle from
// the user layer) whose item provenance is the fixed table above, and the recording executor.
func provDeps(t *testing.T, stub *interactiontest.Stub) (app.Deps, string, *recordingExecutor) {
	t.Helper()
	deps, ws := newBaseDeps(t, stub)
	cat := deps.Catalog.(*stubCatalog)
	cat.hooks = []domain.HookBundle{{Key: "stub-hook", Version: "1.0"}}
	cat.skills = []domain.Skill{{Key: "stub-skill", Version: "1.0"}}
	// The skill reaches a deploy through the agent that requires it.
	runner := minimalAgent
	runner.RequiredSkills = []string{"stub-skill"}
	cat.agents = []domain.Agent{runner}
	mosaic := newSysMosaicRoot(t, sysLayer{id: crDefaultID, protocolVersion: "1.0", bundleVersion: "1.0.0"})
	user := newSysUserLayer(t, sysLayer{id: crUserID, bundleVersion: "2.0.0"})
	useResolvedStack(t, &deps, mosaic, stack.Layer{Root: user}, defaultStackLayer(mosaic))
	layerOf := func(id domain.CatalogIdentity) domain.CatalogLayer { return domain.CatalogLayer{Identity: id} }
	deps.Catalog = provCatalog{
		resolvedStackCatalog: deps.Catalog.(resolvedStackCatalog),
		byItem: map[domain.CatalogItem]domain.CatalogLayer{
			{Kind: domain.CatalogItemAgent, Key: "test-runner"}:  layerOf(provAgentSource),
			{Kind: domain.CatalogItemAgent, Key: "orchestrator"}: layerOf(provOrchSource),
			{Kind: domain.CatalogItemHook, Key: "stub-hook"}:     layerOf(provHookSource),
			{Kind: domain.CatalogItemSkill, Key: "stub-skill"}:   layerOf(provSkillSource),
		},
	}
	deps.Planner = &stubPlanner{plan: provPlan(ws)}
	exec := &recordingExecutor{}
	deps.Executor = exec
	return deps, ws, exec
}

// provOperation is one write operation of the service, with the items it stamps and whether it
// is a full run (supplies the protocol and bundle suppliers).
type provOperation struct {
	name    string
	full    bool
	stamped []domain.ArtifactRef
	run     func(ctx context.Context, svc app.Service, ws string) (domain.RunSummary, error)
}

func provOperations() []provOperation {
	return []provOperation{
		{name: "deploy", full: true, stamped: []domain.ArtifactRef{provAgentRef, provOrchRef, provHookRef, provSkillRef},
			run: func(ctx context.Context, svc app.Service, ws string) (domain.RunSummary, error) {
				return svc.DeployNew(ctx, app.DeployRequest{HarnessID: "stub-harness", WorkspacePath: ws,
					WorkflowIDs: []string{"quick-fix"}, HookIDs: []string{"stub-hook"}, AutoConfirmPlan: true})
			}},
		{name: "update", full: true, stamped: []domain.ArtifactRef{provOrchRef},
			run: func(ctx context.Context, svc app.Service, ws string) (domain.RunSummary, error) {
				// Update works on what is deployed, so the agent files must exist.
				if err := provWriteDeployed(ws); err != nil {
					return domain.RunSummary{}, err
				}
				return svc.Update(ctx, app.UpdateRequest{HarnessID: "stub-harness", WorkspacePath: ws, AutoConfirmPlan: true})
			}},
		{name: "workflows", stamped: []domain.ArtifactRef{provOrchRef},
			run: func(ctx context.Context, svc app.Service, ws string) (domain.RunSummary, error) {
				return svc.UpdateWorkflows(ctx, app.WorkflowUpdateRequest{HarnessID: "stub-harness", WorkspacePath: ws,
					WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: true})
			}},
		{name: "agents", stamped: []domain.ArtifactRef{provAgentRef},
			run: func(ctx context.Context, svc app.Service, ws string) (domain.RunSummary, error) {
				return svc.DeployAgents(ctx, app.DeployAgentsRequest{HarnessID: "stub-harness", WorkspacePath: ws,
					SubagentIDs: []string{"test-runner"}, TierModels: map[domain.Tier]string{"HIGH": "model-a"}, AutoConfirmPlan: true})
			}},
		{name: "hooks", stamped: []domain.ArtifactRef{provHookRef},
			run: func(ctx context.Context, svc app.Service, ws string) (domain.RunSummary, error) {
				return svc.DeployHooks(ctx, app.DeployHooksRequest{HarnessID: "stub-harness", WorkspacePath: ws,
					HookIDs: []string{"stub-hook"}, AutoConfirmPlan: true})
			}},
	}
}

// provWriteDeployed puts the agent and orchestrator files of provPlan into the workspace.
func provWriteDeployed(ws string) error {
	if err := os.WriteFile(filepath.Join(ws, "test-runner.md"), []byte("---\nversion: \"0.9\"\n---\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws, "orchestrator.md"), crDeployedOrchestrator(), 0o644)
}

// provStampFor returns the stamp of ref in req, found through the plan item's target path.
func provStampFor(req deploy.ExecRequest, ref domain.ArtifactRef) (domain.VersionStamp, bool) {
	for _, it := range req.Plan.Items {
		if it.Ref == ref {
			s, ok := req.VersionStamps[it.TargetPath]
			return s, ok
		}
	}
	return domain.VersionStamp{}, false
}
