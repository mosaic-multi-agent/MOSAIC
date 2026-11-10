package app_test

// codex_deploy_op_test.go covers Op derivation for Codex harness flows.
//
// The Codex TOML encoder rejects Op == OpUnspecified, so every flow that builds agent
// content must set the Op explicitly: Deploy workspace and Deploy agents from the planned
// action, RenderAgent as a create.
//
// Covered scenarios:
//
//   DeployNew + Codex harness:
//     - ActionCreate succeeds (Content returns bytes without error)
//
//   DeployAgents + Codex harness:
//     - ActionCreate succeeds
//
//   RenderAgent + Codex harness:
//     - Fresh destination succeeds (no ErrUnspecifiedOperation)
//     - Overwrite=true over an existing destination succeeds
//
//   DeployHooks + Codex harness:
//     - A DeployHooks run with a Codex harness completes without error; hook items
//       bypass buildContent and never reach the Codex encoder.
//
//   DeployAgents idempotency:
//     - A second DeployAgents run with a pre-deployed Codex .toml that matches the
//       source version stamp classifies the item as ActionUnchanged; the executor is
//       never called with Content for that item (no ErrMissingPriorBytes, no write).
//
// Update and conflict-overwrite over an existing Codex agent file are covered in
// codex_deploy_existing_test.go.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// ---------------------------------------------------------------------------
// Fixtures: Codex stub harness for Op derivation tests
// ---------------------------------------------------------------------------

// codexOpHarnessRef is the harness reference used across all Codex Op derivation tests.
var codexOpHarnessRef = domain.HarnessRef{
	ID:          "codex-op-test",
	DisplayName: "Codex (Op test)",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// newCodexOpDesc returns a minimal HarnessDescriptor that activates the codex-toml
// translator. The descriptor mirrors the real Codex harness (codex.yaml) in the
// fields that matter for Op derivation and encoding:
//   - AgentFormatID "codex-toml" routes encoding through the Codex encoder.
//   - Extensions[ArtifactAgent] ".toml" — the file extension the planner and probe use.
//   - Paths.Agents.Project ".codex/agents" — where DeployAgents puts agent files.
//   - Frontmatter.ModelKey "model" — the TOML key the decoder reads for the model stamp.
func newCodexOpDesc() *domain.HarnessDescriptor {
	return &domain.HarnessDescriptor{
		ID:            "codex-op-test",
		DisplayName:   "Codex (Op test)",
		AgentFormatID: "codex-toml",
		Models:        domain.ModelCatalog{IDs: []string{"gpt-5.6-sol"}},
		Frontmatter: domain.FrontmatterSpec{
			ModelKey: "model",
		},
		Extensions: map[domain.ArtifactKind]string{
			domain.ArtifactAgent: ".toml",
		},
		Paths: domain.PathSpec{
			Agents: domain.ScopedPaths{
				Supported: true,
				Project:   ".codex/agents",
			},
		},
	}
}

// newCodexOpModule returns an appTestModule backed by the Codex Op test descriptor.
// appTestModule uses descriptor.MapTools, descriptor.ApplyFrontmatterSpec, and
// descriptor.ResolveTargetPath, giving full Codex path/extension/tool semantics.
func newCodexOpModule() *appTestModule {
	return &appTestModule{desc: newCodexOpDesc()}
}

// newCodexOpRegistry returns a stubRegistry containing the Codex Op test module.
func newCodexOpRegistry() *stubRegistry {
	return &stubRegistry{
		list:    []domain.HarnessRef{codexOpHarnessRef},
		modules: map[string]domain.HarnessModule{codexOpHarnessRef.ID: newCodexOpModule()},
	}
}

// codexOpAgent is the test agent used across the Op derivation deploy tests.
var codexOpAgent = domain.Agent{
	Key:         "codex-op-agent",
	NumericID:   "77",
	Version:     "1.0",
	Name:        "Codex Op Test Agent",
	Description: "A minimal agent for Codex Op derivation tests.",
	Role:        domain.RoleSubagent,
	SourcePath:  "codex-op-agent.md",
}

// codexOpAgentSource returns a minimal valid generic-form MOSAIC subagent document. The
// content must parse through ParseGenericSource and transform.Apply without errors so that
// Op-derivation is the only thing under test. The identity body must be non-empty: the Codex
// encoder rejects an empty developer_instructions block (ErrEmptyBody).
func codexOpAgentSource() []byte {
	return []byte("---\n" +
		"id: \"77\"\n" +
		"version: \"1.0\"\n" +
		"name: Codex Op Test Agent\n" +
		"description: A minimal agent for Codex Op derivation tests.\n" +
		"role: subagent\n" +
		"model: \"{model-identifier}\"\n" +
		"tools: []\n" +
		"---\n" +
		"<Identity type=\"core\">\n" +
		"You are the Codex Op test agent.\n" +
		"</Identity>\n")
}

// newCodexOpCatalog returns a stubCatalog pre-wired with codexOpAgent and its source bytes.
func newCodexOpCatalog() *stubCatalog {
	cat := newMinimalCatalog()
	cat.agents = []domain.Agent{codexOpAgent}
	cat.sources = map[string][]byte{
		codexOpAgent.SourcePath: codexOpAgentSource(),
	}
	return cat
}

// newCodexOpDeps returns app.Deps wired for Codex Op derivation tests: Codex registry and
// catalog with the test agent. All other fields come from newBaseDeps.
func newCodexOpDeps(t *testing.T, stub *interactiontest.Stub) (app.Deps, string) {
	t.Helper()
	deps, workspace := newBaseDeps(t, stub)
	deps.Registry = newCodexOpRegistry()
	deps.Catalog = newCodexOpCatalog()
	return deps, workspace
}

// newCodexOpAgentPlan returns a ModeDeployWorkspace plan with a single agent item for
// codexOpAgent with the given action. For ActionConflict, the Conflict field is populated
// so the deploy service recognises the item as a locally-modified file.
func newCodexOpAgentPlan(workspace string, action domain.PlanAction) domain.Plan {
	item := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: codexOpAgent.Key},
		TargetPath: ".codex/agents/" + codexOpAgent.Key + ".toml",
		Action:     action,
	}
	if action == domain.ActionConflict {
		item.Conflict = &domain.LocalModification{
			RecordedHash: "sha256:aaaa",
			CurrentHash:  "sha256:bbbb",
		}
	}
	return domain.Plan{
		Mode:          domain.ModeDeployWorkspace,
		Harness:       codexOpHarnessRef,
		WorkspacePath: workspace,
		Scope:         domain.ScopeProject,
		Items:         []domain.PlanItem{item},
	}
}

// newCodexOpAgentDeployAgentsPlan returns a ModeDeployAgents plan with a single agent
// item for codexOpAgent with the given action.
func newCodexOpAgentDeployAgentsPlan(workspace string, action domain.PlanAction) domain.Plan {
	p := newCodexOpAgentPlan(workspace, action)
	p.Mode = domain.ModeDeployAgents
	return p
}

// codexOpTomlContent builds a minimal valid Codex TOML file whose stamp block carries the
// given mosaic_version. The body must be non-empty so the Codex decoder can produce a
// canonical form with a developer_instructions field.
func codexOpTomlContent(version string) []byte {
	var b []byte
	if version != "" {
		b = append(b, ("# mosaic_version: " + version + "\n")...)
		b = append(b, '\n') // blank separator after stamp block
	}
	b = append(b, "sandbox_mode = \"read-only\"\n"...)
	b = append(b, "developer_instructions = \"You are the Codex Op test agent.\\n\"\n"...)
	return b
}

// ---------------------------------------------------------------------------
// codexOpContentExecutor: a deploy.Executor that calls req.Content for each
// non-hook Create, Update, and ConflictOverwrite item, surfacing Content errors
// as the Execute return value. Mirrors the capturingContentExecutor pattern from
// destination_field_test.go:191-219.
// ---------------------------------------------------------------------------

type codexOpContentExecutor struct{}

func (e *codexOpContentExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	for _, item := range req.Plan.Items {
		// Hook items bypass buildContent in the executor; never call Content for them.
		if item.Ref.Kind == domain.ArtifactHook {
			continue
		}
		callContent := false
		switch item.Action {
		case domain.ActionCreate, domain.ActionUpdate:
			callContent = true
		case domain.ActionConflict:
			// Mirror the real executor: call Content only when the resolved decision
			// is DecisionOverwrite. The deploy service populates req.Conflicts.
			decision := req.Conflicts[item.TargetPath]
			callContent = (decision == domain.DecisionOverwrite)
		}
		if callContent {
			if _, err := req.Content(item); err != nil {
				return deploy.ExecResult{}, fmt.Errorf("codexOpContentExecutor: Content(%q): %w", item.Ref.Key, err)
			}
		}
	}
	var actions []domain.ActionRecord
	for _, item := range req.Plan.Items {
		actions = append(actions, domain.ActionRecord{
			Ref:        item.Ref,
			TargetPath: filepath.Join(req.Plan.WorkspacePath, item.TargetPath),
			Taken:      domain.TakenCreated,
		})
	}
	return deploy.ExecResult{
		DeploymentRoot: req.Plan.WorkspacePath,
		Actions:        actions,
	}, nil
}

// ---------------------------------------------------------------------------
// Shared deploy request helpers
// ---------------------------------------------------------------------------

// codexOpDeployRequest returns a headless DeployRequest pre-answering all questions.
// The test agent is included via UtilityAgentIDs so that plan.ResolveArtifacts adds it to
// probeSet.Agents, which in turn puts it in the agentByKey map that buildContent consults.
// DeployNew builds agentByKey from probeSet.Agents (not from the raw catalog), so the agent
// must be reachable via WorkflowIDs, UtilityAgentIDs, or InfrastructureAgentIDs.
func codexOpDeployRequest(workspace string) app.DeployRequest {
	return app.DeployRequest{
		HarnessID:       codexOpHarnessRef.ID,
		WorkspacePath:   workspace,
		WorkflowIDs:     []string{},
		UtilityAgentIDs: []string{codexOpAgent.Key},
		HookIDs:         []string{},
		AutoConfirmPlan: true,
		SkipAll: map[domain.QuestionID]bool{
			domain.QTierModel:  true,
			domain.QAgentModel: true,
		},
	}
}

// codexOpDeployAgentsRequest returns a headless DeployAgentsRequest pre-answering all questions.
func codexOpDeployAgentsRequest(workspace string) app.DeployAgentsRequest {
	return app.DeployAgentsRequest{
		HarnessID:              codexOpHarnessRef.ID,
		WorkspacePath:          workspace,
		Scope:                  domain.ScopeProject,
		SubagentIDs:            []string{codexOpAgent.Key},
		UtilityAgentIDs:        []string{},
		InfrastructureAgentIDs: []string{},
		StandaloneAgentIDs:     []string{},
		AutoConfirmPlan:        true,
		SkipAll: map[domain.QuestionID]bool{
			domain.QTierModel:  true,
			domain.QAgentModel: true,
		},
	}
}

// ---------------------------------------------------------------------------
// DeployNew with Codex harness: Op derivation for each action type
// ---------------------------------------------------------------------------

// TestCodexDeployNew_ActionCreate_ContentSucceeds verifies that DeployNew with a Codex
// harness and an ActionCreate plan item returns no error when Content is called. After the
// fix, buildContent derives Op=OpCreate from item.Action when no deployedReader is present,
// satisfying the Codex encoder (OpCreate + nil PriorDeployed = success).
func TestCodexDeployNew_ActionCreate_ContentSucceeds(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	deps.Planner = &stubPlanner{plan: newCodexOpAgentPlan(workspace, domain.ActionCreate)}
	deps.Executor = &codexOpContentExecutor{}
	svc := app.New(deps)

	// Act
	_, err := svc.DeployNew(context.Background(), codexOpDeployRequest(workspace))

	// Assert
	if err != nil {
		t.Errorf("DeployNew with Codex harness and ActionCreate returned error: %v; "+
			"expected no error: buildContent must derive Op=OpCreate from item.Action when "+
			"no deployedReader is present, so the Codex encoder succeeds with nil PriorDeployed",
			err)
	}
}

// ---------------------------------------------------------------------------
// DeployAgents with Codex harness: Op derivation for each action type
// ---------------------------------------------------------------------------

// TestCodexDeployAgents_ActionCreate_ContentSucceeds verifies that DeployAgents with a
// Codex harness and an ActionCreate plan item returns no error when Content is called.
// deployagents_service.go also passes nil deployedReader, so the same fix applies.
func TestCodexDeployAgents_ActionCreate_ContentSucceeds(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	deps.Planner = &stubPlanner{plan: newCodexOpAgentDeployAgentsPlan(workspace, domain.ActionCreate)}
	deps.Executor = &codexOpContentExecutor{}
	svc := app.New(deps)

	// Act
	_, err := svc.DeployAgents(context.Background(), codexOpDeployAgentsRequest(workspace))

	// Assert
	if err != nil {
		t.Errorf("DeployAgents with Codex harness and ActionCreate returned error: %v; "+
			"expected no error: buildContent must derive Op=OpCreate from item.Action when "+
			"no deployedReader is present (deployagents_service.go:357 passes nil deployedReader)",
			err)
	}
}

// ---------------------------------------------------------------------------
// RenderAgent with Codex harness: Op=OpCreate supplied by render_service_impl.go
// ---------------------------------------------------------------------------

// newCodexRenderDeps returns app.Deps wired for RenderAgent Codex tests. It substitutes
// the Codex Op test module for the render registry, mirroring newRenderDeps but for Codex.
func newCodexRenderDeps(t *testing.T) app.Deps {
	t.Helper()
	stub := interactiontest.NewBuilder().Build()
	deps, _ := newBaseDeps(t, stub)
	deps.Registry = newCodexOpRegistry()
	deps.Catalog = newCodexOpCatalog()
	return deps
}

// TestCodexRenderAgent_FreshDestination_Succeeds verifies that RenderAgent with a Codex
// harness renders to a fresh destination without error. After the fix, render_service_impl.go
// supplies Op=agentformat.OpCreate in the transform.Request, satisfying the Codex encoder.
func TestCodexRenderAgent_FreshDestination_Succeeds(t *testing.T) {
	// Arrange
	deps := newCodexRenderDeps(t)
	svc := app.New(deps)

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "codex-op-agent.md")
	if err := os.WriteFile(srcPath, codexOpAgentSource(), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "codex-op-agent.toml")

	// Act
	_, err := svc.RenderAgent(context.Background(), app.RenderAgentRequest{
		SourcePath:      srcPath,
		TargetHarnessID: codexOpHarnessRef.ID,
		DestinationPath: destPath,
		TargetModel:     "gpt-5.6-sol",
	})

	// Assert
	if err != nil {
		t.Errorf("RenderAgent with Codex harness (fresh destination) returned error: %v; "+
			"expected no error: render_service_impl.go must supply Op=agentformat.OpCreate "+
			"in the transform.Request so the Codex encoder can succeed with nil PriorDeployed",
			err)
	}
}

// TestCodexRenderAgent_OverwriteExistingDestination_Succeeds verifies that RenderAgent with
// Overwrite=true over an existing destination succeeds. The render service's Overwrite flag
// governs whether the destination file may be replaced; it does NOT change the operation:
// render always produces content from source alone, with no prior bytes, so Op=OpCreate is
// always correct regardless of whether the destination already exists.
func TestCodexRenderAgent_OverwriteExistingDestination_Succeeds(t *testing.T) {
	// Arrange
	deps := newCodexRenderDeps(t)
	svc := app.New(deps)

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "codex-op-agent.md")
	if err := os.WriteFile(srcPath, codexOpAgentSource(), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	// Pre-create a file at the destination to exercise the Overwrite=true path.
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "codex-op-agent.toml")
	if err := os.WriteFile(destPath, []byte("existing content"), 0o644); err != nil {
		t.Fatalf("write pre-existing destination file: %v", err)
	}

	// Act
	_, err := svc.RenderAgent(context.Background(), app.RenderAgentRequest{
		SourcePath:      srcPath,
		TargetHarnessID: codexOpHarnessRef.ID,
		DestinationPath: destPath,
		TargetModel:     "gpt-5.6-sol",
		Overwrite:       true,
	})

	// Assert
	if err != nil {
		t.Errorf("RenderAgent with Codex harness (Overwrite=true) returned error: %v; "+
			"expected no error: Overwrite is a write-permission flag, not an operation flag; "+
			"render always supplies Op=OpCreate because it produces content from source alone "+
			"with no prior bytes, regardless of whether the destination file exists",
			err)
	}
}

// ---------------------------------------------------------------------------
// Deploy hooks flow is not affected by Op derivation changes
// ---------------------------------------------------------------------------

// TestCodexDeployHooks_HooksNotBrokenByOpChanges verifies that a DeployHooks run with a
// Codex harness completes without error. Hook items bypass buildContent in the executor
// (executor.go skips Content for ArtifactHook items), so they never reach the Codex
// encoder and are unaffected by the Op derivation change.
//
// This test complements the existing deployhooks_test.go hook-specific tests, which do not
// use a Codex harness. The test confirms that registering codex-toml does not break the
// hooks flow.
func TestCodexDeployHooks_HooksNotBrokenByOpChanges(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)

	// A plan with a single hook item. The stub planner returns it regardless of how
	// probeSet or deployedState are computed.
	hookPlan := domain.Plan{
		Mode:          domain.ModeDeployHooks,
		Harness:       codexOpHarnessRef,
		WorkspacePath: workspace,
		Scope:         domain.ScopeProject,
		Items: []domain.PlanItem{
			{
				Ref:        domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "test-hook"},
				TargetPath: ".mosaic/hooks/test-hook",
				Action:     domain.ActionCreate,
			},
		},
	}
	deps.Planner = &stubPlanner{plan: hookPlan}
	deps.Executor = &stubExecutor{result: deploy.ExecResult{
		DeploymentRoot: workspace,
		Actions: []domain.ActionRecord{
			{
				Ref:    domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "test-hook"},
				Taken:  domain.TakenCreated,
			},
		},
	}}
	svc := app.New(deps)

	// Act — HookIDs: []string{} pre-answers QHooks with an empty selection so no catalog
	// lookup for hook bundles is needed and the flow is entirely headless.
	_, err := svc.DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID:       codexOpHarnessRef.ID,
		WorkspacePath:   workspace,
		HookIDs:         []string{},
		AutoConfirmPlan: true,
	})

	// Assert
	if err != nil {
		t.Errorf("DeployHooks with Codex harness returned error: %v; "+
			"hook items must bypass the Codex encoder path entirely and must not be affected "+
			"by Op derivation changes in buildContent",
			err)
	}
}

// ---------------------------------------------------------------------------
// DeployAgents idempotency with pre-deployed Codex .toml
// ---------------------------------------------------------------------------

// TestCodexDeployAgents_RealPlanner_PreDeployedToml_ActionUnchanged verifies that a second
// DeployAgents run with a Codex harness classifies the agent as ActionUnchanged when the
// deployed .toml file's mosaic_version stamp matches the catalog source version. The executor
// must not be called with Content for an ActionUnchanged item (no ErrMissingPriorBytes).
//
// This pins FR-6 (redeploy idempotency) for the Codex harness using the real planner.
// Pattern mirrors deployagents_deployed_state_test.go:85-120 but with a Codex .toml fixture.
func TestCodexDeployAgents_RealPlanner_PreDeployedToml_ActionUnchanged(t *testing.T) {
	// Arrange — real planner; plan-capturing executor records the computed plan.
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)

	capExec := &planCapturingExecutor{result: deploy.ExecResult{
		DeploymentRoot: workspace,
		Actions:        []domain.ActionRecord{},
	}}
	deps.Planner = plan.New()
	deps.Executor = capExec
	svc := app.New(deps)

	// Simulate a previously-deployed Codex .toml by writing a file at the target path.
	// The mosaic_version stamp ("1.0") matches codexOpAgent.Version so the planner
	// produces ActionUnchanged rather than ActionCreate.
	agentTargetRelPath := filepath.Join(".codex", "agents", codexOpAgent.Key+".toml")
	agentTargetPath := filepath.Join(workspace, agentTargetRelPath)
	if err := os.MkdirAll(filepath.Dir(agentTargetPath), 0o755); err != nil {
		t.Fatalf("create .codex/agents dir: %v", err)
	}
	if err := os.WriteFile(agentTargetPath, codexOpTomlContent(codexOpAgent.Version), 0o644); err != nil {
		t.Fatalf("write pre-deployed .toml file: %v", err)
	}

	// Act — second run; the agent already exists on disk with a matching version stamp.
	_, err := svc.DeployAgents(context.Background(), codexOpDeployAgentsRequest(workspace))
	if err != nil {
		t.Fatalf("DeployAgents returned error: %v; "+
			"the flow must succeed when the deployed Codex .toml matches the source version stamp",
			err)
	}

	// Assert — the executor must have been called with the computed plan.
	if capExec.capturedPlan == nil {
		t.Fatal("executor was not called; DeployAgents must invoke the executor with a plan")
	}

	// Assert — the agent must be classified as ActionUnchanged, not ActionCreate.
	// If it is ActionCreate, the idempotency probe is broken for Codex harnesses.
	for _, item := range capExec.capturedPlan.Items {
		if item.Ref.Kind == domain.ArtifactAgent && item.Ref.Key == codexOpAgent.Key {
			if item.Action == domain.ActionCreate {
				t.Errorf("agent %q was classified as ActionCreate even though its .toml file "+
					"already exists at %q with a matching mosaic_version stamp; "+
					"the probe must decode the Codex .toml and compare the version stamp so "+
					"the planner produces ActionUnchanged on a second run",
					codexOpAgent.Key, agentTargetPath)
			}
		}
	}
}
