package app_test

// tools_update_fixtures_test.go holds the fixtures of the tests of the tools field of an updated
// agent: a harness whose built-in tools are read, edit and bash, an agent whose catalog tools
// are mapped onto them, a previously deployed agent file with a hand-written tools field, and a
// run helper that drives Service.DeployAgents with the real planner and the real executor.
// Nothing in this file is a test itself.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/manifest/manifesttest"
	"mosaic-deploy/internal/plan"
)

const (
	tuAgentKey  = "plan-review"
	tuAgentPath = tuAgentKey + ".md"
)

// tuCatalogSource is the generic source of the agent: its catalog tools are gen_read and
// gen_edit, which the module maps to read and edit.
const tuCatalogSource = `---
id: 99
version: 1.0.0
name: plan-review
description: Reviews plans
model: {model-identifier}
tools: [gen_read, gen_edit]
recommended_tier: HIGH
tier_rationale: minimal
required_skills: []
---

<Identity type="core">
Static prose.
</Identity>
`

// tuModule is a harness module that renders the generic tools gen_read, gen_edit and gen_bash
// as the list-shaped field "tools" holding read, edit and bash.
type tuModule struct {
	*stubHarnessModule
}

var tuMapping = map[string]string{"gen_read": "read", "gen_edit": "edit", "gen_bash": "bash"}

func (m tuModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	var items []domain.FieldValue
	resolutions := make([]domain.ToolResolution, len(req.Generic))
	for i, g := range req.Generic {
		resolutions[i] = domain.ToolResolution{Generic: g, Outcome: domain.ToolMapped}
		if name, ok := tuMapping[g]; ok {
			items = append(items, domain.ScalarValue(name, domain.QuotePlain))
		} else if server := req.CustomNames[g]; server != "" {
			resolutions[i].Outcome = domain.ToolCustom
			items = append(items, domain.ScalarValue(server, domain.QuotePlain))
		}
	}
	field := domain.FrontmatterField{Key: "tools", Value: domain.ListValue(items, domain.ListFlow)}
	return domain.ToolResult{Fields: []domain.FrontmatterField{field}, Resolutions: resolutions}, nil
}

// tuPermModule is a harness module that renders the generic tools as the permission-shaped
// field "tools": every built-in tool is listed, allow when the agent's tools map onto it, deny
// otherwise.
type tuPermModule struct {
	*stubHarnessModule
}

func (m tuPermModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	granted := map[string]bool{}
	resolutions := make([]domain.ToolResolution, len(req.Generic))
	for i, g := range req.Generic {
		resolutions[i] = domain.ToolResolution{Generic: g, Outcome: domain.ToolMapped}
		if name, ok := tuMapping[g]; ok {
			granted[name] = true
		}
	}
	var pairs []domain.FieldPair
	for _, name := range []string{"read", "edit", "bash"} {
		disp := "deny"
		if granted[name] {
			disp = "allow"
		}
		pairs = append(pairs, domain.FieldPair{Key: name, Value: domain.ScalarValue(disp, domain.QuotePlain)})
	}
	field := domain.FrontmatterField{Key: "tools", Value: domain.MappingValue(pairs)}
	return domain.ToolResult{Fields: []domain.FrontmatterField{field}, Resolutions: resolutions}, nil
}

// tuRegistry returns a registry whose only harness is the list-shaped tools module.
func tuRegistry() *stubRegistry {
	base := tuBase(domain.ShapeList)
	return tuRegistryOf(tuModule{base})
}

// tuPermRegistry returns a registry whose only harness is the permission-shaped tools module.
func tuPermRegistry() *stubRegistry {
	base := tuBase(domain.ShapePermission)
	return tuRegistryOf(tuPermModule{base})
}

func tuBase(shape domain.ToolShape) *stubHarnessModule {
	base := newMinimalModule()
	base.descriptor.Frontmatter.ToolsKey = "tools"
	base.descriptor.Frontmatter.ModelKey = "model"
	base.descriptor.Tools = domain.ToolSpec{
		Shape:    shape,
		Universe: []domain.HarnessTool{{Name: "read"}, {Name: "edit"}, {Name: "bash"}},
	}
	return base
}

func tuRegistryOf(mod domain.HarnessModule) *stubRegistry {
	return &stubRegistry{
		list:    []domain.HarnessRef{mod.Ref()},
		modules: map[string]domain.HarnessModule{"stub-harness": mod},
	}
}

// tuSetup selects the harness registry and the catalog source of a run.
type tuSetup struct {
	registry *stubRegistry
	source   string
}

// tuDeployed returns a deployed agent file with the given tools field text and mapping stamp.
// An empty stamp leaves the stamp out.
func tuDeployed(version, toolsField, mappingsStamp string) string {
	stamp := ""
	if mappingsStamp != "" {
		stamp = "mosaic_tool_mappings_version: " + mappingsStamp + "\n"
	}
	return "---\nmosaic_id: 99\nversion: " + version + "\n" + stamp + "tools: " + toolsField +
		"\nmodel: model-a\n---\n\n<Identity type=\"core\">\nStatic prose.\n</Identity>\n"
}

// tuRun is the observable outcome of runToolsUpdate.
type tuRun struct {
	workspace string
	rec       *injectionRecorder
	summary   domain.RunSummary
	plans     []domain.Plan
	err       error
}

// tuReview keeps every plan shown for review and delegates the answer to the stub.
type tuReview struct {
	*interactiontest.Stub
	plans []domain.Plan
}

func (r *tuReview) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	r.plans = append(r.plans, p)
	return r.Stub.Review(ctx, p)
}

// item returns the plan item of the agent from the reviewed plan, false when absent.
func (r *tuRun) item() (domain.PlanItem, bool) {
	if len(r.plans) == 0 {
		return domain.PlanItem{}, false
	}
	for _, it := range r.plans[0].Items {
		if it.TargetPath == tuAgentPath {
			return it, true
		}
	}
	return domain.PlanItem{}, false
}

// written returns the agent file as it is on disk after the run.
func (r *tuRun) written(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.workspace, tuAgentPath))
	if err != nil {
		t.Fatalf("read %s: %v", tuAgentPath, err)
	}
	return string(b)
}

// toolsLine returns the "tools:" line of a written file, "" when absent.
func toolsLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "tools:") {
			return line
		}
	}
	return ""
}

// runToolsUpdate runs DeployAgents for the agent over a workspace that holds the deployed file
// recorded in the manifest. mutate, when non-nil, adjusts the request.
func runToolsUpdate(t *testing.T, deployed string, mutate func(*app.DeployAgentsRequest)) *tuRun {
	t.Helper()
	return runToolsUpdateWith(t, tuSetup{registry: tuRegistry(), source: tuCatalogSource}, deployed, mutate)
}

// toolsBlock returns the "tools:" line and its indented continuation lines, "" when absent.
func toolsBlock(content string) string {
	var out []string
	in := false
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "tools:"):
			in = true
			out = append(out, strings.TrimRight(line, " "))
		case in && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")):
			out = append(out, line)
		default:
			in = false
		}
	}
	return strings.Join(out, "\n")
}

// runToolsUpdateWith is runToolsUpdate with an explicit harness registry and catalog source.
func runToolsUpdateWith(t *testing.T, setup tuSetup, deployed string, mutate func(*app.DeployAgentsRequest)) *tuRun {
	t.Helper()
	review := &tuReview{Stub: interactiontest.NewBuilder().AnswerReview(true).Build()}
	deps, ws := newBaseDeps(t, review.Stub)
	deps.Interaction = review
	cat := injCatalog()
	cat.sources["src/"+tuAgentKey+".md"] = []byte(setup.source)
	deps.Catalog = cat
	deps.Planner = plan.New()
	deps.Registry = setup.registry
	if err := os.WriteFile(filepath.Join(ws, tuAgentPath), []byte(deployed), 0o644); err != nil {
		t.Fatalf("seed deployed agent: %v", err)
	}
	entry := domain.ManifestEntry{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: tuAgentKey},
		TargetPath: tuAgentPath, Version: "1.0.0", ContentHash: manifest.Hash([]byte(deployed)),
	}
	deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{State: manifest.StatePresent, Manifest: domain.Manifest{
		SchemaVersion: "1", Harnesses: []domain.HarnessSection{{HarnessID: "stub-harness"}},
		Entries: manifesttest.WithHarness("stub-harness", []domain.ManifestEntry{entry}),
	}}}
	rec := &injectionRecorder{inner: deploy.NewExecutor(deps.Manifest, deps.Logger, deps.Todo)}
	deps.Executor = rec
	req := app.DeployAgentsRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, Scope: domain.ScopeProject, AutoConfirmPlan: true,
		SubagentIDs: []string{tuAgentKey}, UtilityAgentIDs: []string{}, StandaloneAgentIDs: []string{},
		InfrastructureAgentIDs: []string{},
		SkipAll:                map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
	}
	if mutate != nil {
		mutate(&req)
	}
	summary, err := app.New(deps).DeployAgents(context.Background(), req)
	return &tuRun{workspace: ws, rec: rec, summary: summary, plans: review.plans, err: err}
}
