package app_test

// tools_descriptor_fixtures_test.go holds the fixtures of the tests that a change of a harness's
// tool settings, in the descriptor or in the configuration, reaches the deployed agents: a
// harness module whose tools field is rendered from a tool-local descriptor spec, the spec of a
// Claude Code-like harness before and after it dropped its user-interaction tool, and a
// workspace that is deployed to and updated again with the real planner, executor and manifest
// store. Nothing in this file is a test itself.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
	"mosaic-deploy/internal/todo"
)

const (
	tdHarnessID    = "stub-harness"
	tdAgentKey     = "test-runner"
	tdAgentPath    = tdAgentKey + ".md"
	tdRemovedTool  = "AskUserQuestion"
	tdStampKeyLine = "mosaic_tool_mappings_version:"
	tdReasonStamp  = "tool_mappings_version changed from"
)

// tdAgentSource is the generic source of the agent: its catalog tools are file_read, file_edit
// and user_interaction. It never changes between the runs of a test.
var tdAgentSource = strings.Replace(strings.Replace(tuCatalogSource, "name: plan-review", "name: test-runner", 1),
	"tools: [gen_read, gen_edit]", "tools: [file_read, file_edit, user_interaction]", 1) +
	"\n<CommunicationProtocol type=\"managed\">\n</CommunicationProtocol>\n"

// tdModule renders the tools field of an agent from the tool spec of its descriptor.
type tdModule struct {
	*stubHarnessModule
}

func (m tdModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	return descriptor.MapTools(&m.descriptor, req)
}

func tdMain(generic string, names ...string) domain.ToolMapping {
	return domain.ToolMapping{Generic: generic, Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: names}}}
}

// tdSpecBefore is the list-shaped tool spec of a harness that maps user_interaction to
// AskUserQuestion.
func tdSpecBefore() domain.ToolSpec {
	return domain.ToolSpec{
		Shape: domain.ShapeList,
		Universe: []domain.HarnessTool{
			{Name: "Read"}, {Name: "Edit"}, {Name: tdRemovedTool},
		},
		Mappings: []domain.ToolMapping{
			tdMain("file_read", "Read"), tdMain("file_edit", "Edit"), tdMain("user_interaction", tdRemovedTool),
		},
		PlaceholderExpansion: []string{"Read", "Edit", tdRemovedTool},
	}
}

// tdSpecAfter is the same harness once it dropped the user_interaction mapping and the
// placeholder expansion of AskUserQuestion; the tool stays in the universe as denied.
func tdSpecAfter() domain.ToolSpec {
	return domain.ToolSpec{
		Shape: domain.ShapeList,
		Universe: []domain.HarnessTool{
			{Name: "Read"}, {Name: "Edit"}, {Name: tdRemovedTool, Unused: domain.Deny},
		},
		Mappings:             []domain.ToolMapping{tdMain("file_read", "Read"), tdMain("file_edit", "Edit")},
		PlaceholderExpansion: []string{"Read", "Edit"},
	}
}

// tdRegistry returns a registry whose only harness renders tools from spec.
func tdRegistry(spec domain.ToolSpec) *stubRegistry {
	base := newMinimalModule()
	base.descriptor.Frontmatter.ToolsKey = "tools"
	base.descriptor.Frontmatter.ModelKey = "model"
	base.descriptor.Tools = spec
	return tuRegistryOf(tdModule{base})
}

// tdEnv is one workspace with the configuration stores that every run of a test shares.
type tdEnv struct {
	t    *testing.T
	ws   string
	user *spyUserConfig
	tool *stubToolConfig
}

func newTdEnv(t *testing.T) *tdEnv {
	t.Helper()
	return &tdEnv{t: t, ws: t.TempDir(), user: &spyUserConfig{}, tool: &stubToolConfig{cfg: config.DefaultToolConfig()}}
}

// tdRun is what one run of the environment shows.
type tdRun struct {
	review  *tuReview
	rec     *injectionRecorder
	planner *tdInputPlanner
	err     error
}

// tdInputPlanner runs the real planner and records the input of every plan it builds.
type tdInputPlanner struct {
	inner  plan.Planner
	inputs []plan.Input
}

func (p *tdInputPlanner) Build(ctx context.Context, in plan.Input) (domain.Plan, error) {
	p.inputs = append(p.inputs, in)
	return p.inner.Build(ctx, in)
}

// item returns the plan item of the agent, from the plan the run showed for review or else from
// the plan it handed to the executor; false when the run planned no such item.
func (r *tdRun) item() (domain.PlanItem, bool) {
	var plans [][]domain.PlanItem
	for _, p := range r.review.plans {
		plans = append(plans, p.Items)
	}
	if r.rec.called {
		plans = append(plans, r.rec.req.Plan.Items)
	}
	for _, items := range plans {
		for _, it := range items {
			if it.TargetPath == tdAgentPath {
				return it, true
			}
		}
	}
	return domain.PlanItem{}, false
}

func (e *tdEnv) deps(reg *stubRegistry) (app.Deps, *tdRun) {
	review := &tuReview{Stub: interactiontest.NewBuilder().AnswerReview(true).Build()}
	deps, _ := newBaseDeps(e.t, review.Stub)
	deps.Interaction = review
	cat := scriptContentCatalog("src/orchestrator-script.md", "src/orchestrator.md")
	agent := cat.agents[0]
	agent.Key, agent.Version, agent.SourcePath = tdAgentKey, "1.0.0", "src/"+tdAgentKey+".md"
	cat.agents = []domain.Agent{agent}
	cat.hooks = []domain.HookBundle{minimalHookBundle}
	cat.sources[agent.SourcePath] = []byte(tdAgentSource)
	cat.sections = map[string][]byte{"quick-fix": []byte("<Workflow type=\"managed\" name=\"quick-fix\" version=\"1.0\">\nQuick fix.\n</Workflow>\n")}
	deps.Catalog = cat
	planner := &tdInputPlanner{inner: plan.New()}
	deps.Planner = planner
	deps.Registry = reg
	deps.Manifest = manifest.NewStore()
	deps.Todo = todo.NewCollector()
	rec := &injectionRecorder{inner: deploy.NewExecutor(deps.Manifest, deps.Logger, deps.Todo)}
	deps.Executor = rec
	deps.ToolConfig = e.tool
	deps.UserConfig = e.user
	return deps, &tdRun{review: review, rec: rec, planner: planner}
}

var tdSkip = map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true, domain.QCustomTool: true}

// tdModels names the model of every agent a run of the environment writes.
func tdModels() map[string]string {
	return map[string]string{tdAgentKey: "model-a", "orchestrator": "model-a", "orchestrator-script": "model-a"}
}

// deploy writes the agent, with the orchestrator that requires it, for the harness whose tool
// spec is spec.
func (e *tdEnv) deploy(spec domain.ToolSpec) *tdRun {
	e.t.Helper()
	deps, run := e.deps(tdRegistry(spec))
	_, run.err = app.New(deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID: tdHarnessID, WorkspacePath: e.ws, Scope: domain.ScopeProject, AutoConfirmPlan: true,
		WorkflowIDs: []string{"quick-fix"}, UtilityAgentIDs: []string{}, InfrastructureAgentIDs: []string{},
		HookIDs: []string{}, AgentModels: tdModels(), SkipAll: tdSkip, ConflictDefault: domain.DecisionOverwrite,
	})
	if run.err != nil {
		e.t.Fatalf("deploy agent: %v", run.err)
	}
	return run
}

// update runs the update flow with the harness whose tool spec is spec.
func (e *tdEnv) update(spec domain.ToolSpec) *tdRun {
	e.t.Helper()
	deps, run := e.deps(tdRegistry(spec))
	_, run.err = app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: tdHarnessID, WorkspacePath: e.ws, AutoConfirmPlan: true, SkipAll: tdSkip,
		AgentModels: tdModels(), ConflictDefault: domain.DecisionOverwrite,
	})
	if run.err != nil {
		e.t.Fatalf("update: %v", run.err)
	}
	return run
}

// written returns the agent file as it is on disk.
func (e *tdEnv) written() string {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.ws, tdAgentPath))
	if err != nil {
		e.t.Fatalf("read %s: %v", tdAgentPath, err)
	}
	return string(b)
}

// requireAction fails the test unless the run planned the agent with the given action.
func (r *tdRun) requireAction(t *testing.T, want domain.PlanAction) domain.PlanItem {
	t.Helper()
	it, ok := r.item()
	if !ok {
		t.Fatalf("the run planned no item for %s", tdAgentPath)
	}
	if it.Action != want {
		t.Fatalf("Action = %q (reasons %q), want %q", it.Action, it.Reasons, want)
	}
	return it
}

var tdStampValue = regexp.MustCompile(`(?m)^` + tdStampKeyLine + `\s*"?([^"\s]*)"?\s*$`)

// stampOf returns the tool-mappings stamp written in a deployed file, "" when absent.
func tdStampOf(content string) string {
	if m := tdStampValue.FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return ""
}

// tdSetUserDestinations declares tool_destinations for the harness in the user configuration.
func (e *tdEnv) setUserDestinations(m ...domain.ToolMapping) {
	e.user.cfg.ToolDestinations = config.ToolDestinationsByHarness{tdHarnessID: m}
}
