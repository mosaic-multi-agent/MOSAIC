package app_test

// tools_stamp_crossflow_test.go verifies that every flow that stamps a tool_mappings_version
// computes the same stamp from the same harness descriptor and configuration: the stamp each
// flow hands to the planner or writes into an agent is the tool settings stamp of the harness
// (its tool spec plus its tool_destinations), so an agent written by one flow is not stale for
// another and no update loop starts.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
)

// tcDestination is the config tool_destinations entry of every cross-flow run; the stamp must
// carry it besides the descriptor's tool spec.
func tcDestination() domain.ToolMapping { return tdMain("web", "fetch") }

// tcStamp calls config.ToolSettingsStamp for the harness id with the given user configuration
// and fails the test, instead of aborting the whole test binary, when the call panics.
func tcStamp(t *testing.T, id string, spec domain.ToolSpec, user config.ToolDestinationsByHarness) string {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ToolSettingsStamp panicked: %v", r)
		}
	}()
	return config.ToolSettingsStamp(id, spec, nil, user)
}

// tcExpected is the stamp the flows must compute for a harness with spec and tcDestination.
func tcExpected(t *testing.T, spec domain.ToolSpec) string {
	t.Helper()
	user := config.ToolDestinationsByHarness{tdHarnessID: {tcDestination()}}
	return tcStamp(t, tdHarnessID, spec, user)
}

func newTcEnv(t *testing.T) *tdEnv {
	t.Helper()
	env := newTdEnv(t)
	env.setUserDestinations(tcDestination())
	return env
}

// tcFlow runs one planner-driven flow of env with the harness whose tool spec is spec.
type tcFlow struct {
	name string
	run  func(env *tdEnv, spec domain.ToolSpec) *tdRun
}

func tcFlows() []tcFlow {
	return []tcFlow{
		{"deploy", func(e *tdEnv, s domain.ToolSpec) *tdRun { return e.deploy(s) }},
		{"agents", (*tdEnv).agentsFlow},
		{"update", func(e *tdEnv, s domain.ToolSpec) *tdRun { e.deploy(s); return e.update(s) }},
		{"workflows", func(e *tdEnv, s domain.ToolSpec) *tdRun { e.deploy(s); return e.workflowsFlow(s) }},
		{"hooks", (*tdEnv).hooksFlow},
	}
}

// agentsFlow runs the agents-only flow.
func (e *tdEnv) agentsFlow(spec domain.ToolSpec) *tdRun {
	e.t.Helper()
	deps, run := e.deps(tdRegistry(spec))
	_, run.err = app.New(deps).DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: tdHarnessID, WorkspacePath: e.ws, Scope: domain.ScopeProject, AutoConfirmPlan: true,
		SubagentIDs: []string{tdAgentKey}, UtilityAgentIDs: []string{}, StandaloneAgentIDs: []string{},
		InfrastructureAgentIDs: []string{}, AgentModels: tdModels(), SkipAll: tdSkip,
	})
	if run.err != nil {
		e.t.Fatalf("agents flow: %v", run.err)
	}
	return run
}

// workflowsFlow runs the workflow-only update flow.
func (e *tdEnv) workflowsFlow(spec domain.ToolSpec) *tdRun {
	e.t.Helper()
	deps, run := e.deps(tdRegistry(spec))
	_, run.err = app.New(deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: tdHarnessID, WorkspacePath: e.ws, WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: true,
		SkipAll: tdSkip, ConflictDefault: domain.DecisionOverwrite,
	})
	if run.err != nil {
		e.t.Fatalf("workflows flow: %v", run.err)
	}
	return run
}

// hooksFlow runs the hooks-only flow.
func (e *tdEnv) hooksFlow(spec domain.ToolSpec) *tdRun {
	e.t.Helper()
	deps, run := e.deps(tdRegistry(spec))
	_, run.err = app.New(deps).DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID: tdHarnessID, WorkspacePath: e.ws, Scope: domain.ScopeProject, HookIDs: []string{minimalHookBundle.Key},
		AutoConfirmPlan: true, SkipAll: tdSkip, ConflictDefault: domain.DecisionOverwrite,
	})
	if run.err != nil {
		e.t.Fatalf("hooks flow: %v", run.err)
	}
	return run
}

func TestStampFlows_PlannerInput_IsTheToolSettingsStampOfDescriptorAndConfig(t *testing.T) {
	for _, spec := range []struct {
		name string
		spec domain.ToolSpec
	}{{"before", tdSpecBefore()}, {"after", tdSpecAfter()}} {
		for _, flow := range tcFlows() {
			t.Run(flow.name+"/"+spec.name, func(t *testing.T) {
				env := newTcEnv(t)

				run := flow.run(env, spec.spec)

				if len(run.planner.inputs) == 0 {
					t.Fatal("the flow built no plan")
				}
				want := tcExpected(t, spec.spec)
				for _, in := range run.planner.inputs {
					if in.ToolMappingsVersion != want {
						t.Errorf("plan input stamp = %q, want %q", in.ToolMappingsVersion, want)
					}
				}
			})
		}
	}
}

func TestStampFlows_DescriptorChange_ChangesStampOfEveryFlow(t *testing.T) {
	for _, flow := range tcFlows() {
		t.Run(flow.name, func(t *testing.T) {
			before := flow.run(newTcEnv(t), tdSpecBefore()).planner.inputs
			after := flow.run(newTcEnv(t), tdSpecAfter()).planner.inputs

			if len(before) == 0 || len(after) == 0 {
				t.Fatal("the flow built no plan")
			}
			if before[0].ToolMappingsVersion == after[0].ToolMappingsVersion {
				t.Errorf("the stamp %q did not change with the descriptor tool settings", after[0].ToolMappingsVersion)
			}
		})
	}
}

func TestStampFlows_AgentFileWrittenByDeployAgentsAndUpdate_CarriesTheSameStamp(t *testing.T) {
	for _, flow := range []tcFlow{
		{"deploy", func(e *tdEnv, s domain.ToolSpec) *tdRun { return e.deploy(s) }},
		{"agents", (*tdEnv).agentsFlow},
		{"update", func(e *tdEnv, s domain.ToolSpec) *tdRun { e.deploy(tdSpecBefore()); return e.update(s) }},
	} {
		t.Run(flow.name, func(t *testing.T) {
			env := newTcEnv(t)

			flow.run(env, tdSpecAfter())

			if got, want := tdStampOf(env.written()), tcExpected(t, tdSpecAfter()); got != want {
				t.Errorf("stamp written by %s = %q, want %q", flow.name, got, want)
			}
		})
	}
}

func TestStampFlows_AgentWrittenByEachFlow_IsUnchangedForFollowingUpdate(t *testing.T) {
	for _, flow := range []tcFlow{
		{"deploy", func(e *tdEnv, s domain.ToolSpec) *tdRun { return e.deploy(s) }},
		{"agents", (*tdEnv).agentsFlow},
		{"workflows", func(e *tdEnv, s domain.ToolSpec) *tdRun { e.deploy(s); return e.workflowsFlow(s) }},
		{"hooks", func(e *tdEnv, s domain.ToolSpec) *tdRun { e.deploy(s); return e.hooksFlow(s) }},
	} {
		t.Run(flow.name, func(t *testing.T) {
			env := newTcEnv(t)
			flow.run(env, tdSpecAfter())
			// An agent-only run needs the orchestrator that requires the agent to be updated.
			if flow.name == "agents" {
				env.deploy(tdSpecAfter())
			}

			run := env.update(tdSpecAfter())

			run.requireAction(t, domain.ActionUnchanged)
		})
	}
}

func TestStampFlows_RenderedAgent_CarriesTheToolSettingsStampOfDescriptorAndConfig(t *testing.T) {
	env := newTcEnv(t)
	deps, _ := env.deps(tdRegistry(tdSpecAfter()))
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())
	dest := filepath.Join(dir, "out.md")

	_, err := app.New(deps).RenderAgent(context.Background(), app.RenderAgentRequest{
		SourcePath: src, TargetHarnessID: tdHarnessID, DestinationPath: dest, TargetModel: "model-a",
	})

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read rendered agent: %v", err)
	}
	if got, want := tdStampOf(string(b)), tcExpected(t, tdSpecAfter()); got != want {
		t.Errorf("rendered stamp = %q, want %q", got, want)
	}
}

func TestStampFlows_TransformedAgent_CarriesTheToolSettingsStampAndIsNotStaleForUpdate(t *testing.T) {
	r, spy := trFollowRun(t)
	want := tcStamp(t, hOpen, r.module(hOpen).Descriptor().Tools, nil)

	got := trTransformThenUpdate(t, r, spy, domain.DecisionOverwrite)

	if stamp := tdStampOf(r.read(r.dest("subagent"))); stamp != want {
		t.Errorf("stamp written by the transform = %q, want %q", stamp, want)
	}
	it := trPlanItem(t, got.Plan.Items, domain.ArtifactAgent, "subagent")
	if reasonWithPrefix(it, tdReasonStamp) != "" {
		t.Errorf("update plans the transformed agent as %q (%s), want no tool_mappings_version change", it.Action, reasonText(it))
	}
}
