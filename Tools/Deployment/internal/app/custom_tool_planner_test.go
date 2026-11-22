package app_test

// custom_tool_planner_test.go verifies, with the real planner, that a tool the user gave a
// server name for during the run is not reported as unmapped in Deploy, Deploy agents and
// Change workflows, while a skipped tool and a tool that was never asked about still are.
// Both the plan gaps (as the executor receives them) and the TODO collector are checked.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// answerAwareToolsModule resolves tools like the real descriptor mapping: a listed tool is
// ToolCustom when a custom name is supplied, ToolSkipped when skipped, else ToolUnmapped.
type answerAwareToolsModule struct {
	*stubHarnessModule
	unmapped map[string]bool
}

func (m *answerAwareToolsModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	resolutions := make([]domain.ToolResolution, len(req.Generic))
	for i, g := range req.Generic {
		outcome := domain.ToolMapped
		if m.unmapped[g] {
			switch {
			case len(req.CustomNames[g]) > 0:
				outcome = domain.ToolCustom
			case req.SkippedTools[g]:
				outcome = domain.ToolSkipped
			default:
				outcome = domain.ToolUnmapped
			}
		}
		resolutions[i] = domain.ToolResolution{Generic: g, Outcome: outcome}
	}
	return domain.ToolResult{Resolutions: resolutions}, nil
}

// customToolRun is a flow setup using the real planner with a plan-capturing executor.
type customToolRun struct {
	deps    app.Deps
	exec    *planCapturingExecutor
	todo    *spyTodo
	stub    *interactiontest.Stub
	workDir string
}

// newCustomToolRun wires the given agents into the catalog, makes every tool named in
// unmapped unmapped for the harness, answers QCustomTool "bash" with a server name and
// leaves every other QCustomTool question unscripted (skipped).
func newCustomToolRun(t *testing.T, agents []domain.Agent, unmapped ...string) *customToolRun {
	t.Helper()
	stub := interactiontest.NewBuilder().
		AnswerText(domain.QCustomTool, "bash", "my-bash-server").
		AnswerReview(true).
		Build()
	deps, workspace := newBaseDeps(t, stub)
	cat := newMinimalCatalog()
	cat.agents = agents
	wf := minimalWorkflow
	wf.ReferencedAgents = nil
	for _, a := range agents {
		wf.ReferencedAgents = append(wf.ReferencedAgents, a.Key)
	}
	cat.workflows = []domain.Workflow{wf}
	cat.categories = []domain.WorkflowCategory{{Name: "Build", Workflows: cat.workflows}}

	set := make(map[string]bool, len(unmapped))
	for _, u := range unmapped {
		set[u] = true
	}
	mod := &answerAwareToolsModule{stubHarnessModule: newMinimalModule(), unmapped: set}
	reg := newMinimalRegistry(nil)
	reg.modules = map[string]domain.HarnessModule{"stub-harness": mod}

	exec := &planCapturingExecutor{result: newMinimalExecResult(workspace)}
	spy := &spyTodo{}
	deps.Catalog, deps.Registry, deps.Planner = cat, reg, plan.New()
	deps.Executor, deps.Todo = exec, spy
	return &customToolRun{deps: deps, exec: exec, todo: spy, stub: stub, workDir: workspace}
}

// toolAgent returns a worker agent declaring the given tools.
func toolAgent(key string, tools ...string) domain.Agent {
	a := minimalAgent
	a.Key, a.Name, a.Tools = key, key, tools
	return a
}

// unmappedSubjects returns the subjects of the unmapped-tool gaps in gaps.
func unmappedSubjects(gaps []domain.Gap) map[string]bool {
	out := map[string]bool{}
	for _, g := range gaps {
		if g.Kind == domain.GapUnmappedTool {
			out[g.Subject] = true
		}
	}
	return out
}

// assertToolGapReporting checks that, in both the plan gaps and the TODO gaps, the tools in gone are
// not reported as unmapped and the tools in still are.
func assertToolGapReporting(t *testing.T, r *customToolRun, gone, still []string) {
	t.Helper()
	if r.exec.capturedPlan == nil {
		t.Fatal("the executor received no plan; the flow did not reach execution")
	}
	places := map[string]map[string]bool{
		"plan gaps": unmappedSubjects(r.exec.capturedPlan.Gaps),
		"TODO gaps": unmappedSubjects(r.todo.gaps),
	}
	for place, got := range places {
		for _, tool := range gone {
			if got[tool] {
				t.Errorf("%s report the tool %q as unmapped although the user named a server for it", place, tool)
			}
		}
		for _, tool := range still {
			if !got[tool] {
				t.Errorf("%s do not report the tool %q as unmapped; it must still be reported", place, tool)
			}
		}
	}
}

// TestDeployNew_RealPlanner_NamedToolNotReportedUnmapped covers Deploy: the named tool is
// not reported, the skipped tool is.
func TestDeployNew_RealPlanner_NamedToolNotReportedUnmapped(t *testing.T) {
	r := newCustomToolRun(t, []domain.Agent{toolAgent("test-runner", "bash", "web-search")}, "bash", "web-search")

	_, err := app.New(r.deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID:       "stub-harness",
		WorkspacePath:   r.workDir,
		Scope:           domain.ScopeProject,
		WorkflowIDs:     []string{"quick-fix"},
		SkipAll:         map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("DeployNew returned error: %v", err)
	}

	assertToolGapReporting(t, r, []string{"bash"}, []string{"web-search"})
}

// TestDeployNew_RealPlanner_PreAnsweredToolNotReportedUnmapped covers a custom tool name
// supplied in the request instead of through the question.
func TestDeployNew_RealPlanner_PreAnsweredToolNotReportedUnmapped(t *testing.T) {
	r := newCustomToolRun(t, []domain.Agent{toolAgent("test-runner", "bash")}, "bash")

	_, err := app.New(r.deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID:       "stub-harness",
		WorkspacePath:   r.workDir,
		Scope:           domain.ScopeProject,
		WorkflowIDs:     []string{"quick-fix"},
		CustomTools:     domain.CustomToolAnswers{"bash": {{Server: "pre-answered-server"}}},
		SkipAll:         map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("DeployNew returned error: %v", err)
	}

	assertToolGapReporting(t, r, []string{"bash"}, nil)
}

// TestDeployAgents_RealPlanner_NamedToolNotReportedUnmapped covers Deploy agents.
func TestDeployAgents_RealPlanner_NamedToolNotReportedUnmapped(t *testing.T) {
	r := newCustomToolRun(t, []domain.Agent{toolAgent("test-runner", "bash", "web-search")}, "bash", "web-search")

	_, err := app.New(r.deps).DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID:              "stub-harness",
		WorkspacePath:          r.workDir,
		Scope:                  domain.ScopeProject,
		SubagentIDs:            []string{"test-runner"},
		UtilityAgentIDs:        []string{},
		InfrastructureAgentIDs: []string{},
		StandaloneAgentIDs:     []string{},
		SkipAll:                map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
		AutoConfirmPlan:        true,
	})
	if err != nil {
		t.Fatalf("DeployAgents returned error: %v", err)
	}

	assertToolGapReporting(t, r, []string{"bash"}, []string{"web-search"})
}

// TestUpdateWorkflows_RealPlanner_NamedToolNotReportedUnmapped covers Change workflows: a
// newly required agent's named tool is not reported, its skipped tool is, and the tool of an
// already deployed stale agent that was never asked about still is.
func TestUpdateWorkflows_RealPlanner_NamedToolNotReportedUnmapped(t *testing.T) {
	r := newCustomToolRun(t, []domain.Agent{
		toolAgent("test-runner", "bash", "web-search"),
		toolAgent("old-agent", "grep"),
	}, "bash", "web-search", "grep")
	staleFile := filepath.Join(r.workDir, "old-agent.md")
	if err := os.WriteFile(staleFile, []byte("---\nversion: 0.1\n---\n# Old Agent\n"), 0o644); err != nil {
		t.Fatalf("setup: write deployed agent: %v", err)
	}

	_, err := app.New(r.deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID:       "stub-harness",
		WorkspacePath:   r.workDir,
		WorkflowIDs:     []string{"quick-fix"},
		SkipAll:         map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("UpdateWorkflows returned error: %v", err)
	}

	if r.stub.WasAsked(domain.QCustomTool, "grep") {
		t.Fatal("setup: the already deployed agent's tool was asked about; it must stay unasked")
	}
	assertToolGapReporting(t, r, []string{"bash"}, []string{"web-search", "grep"})
}
