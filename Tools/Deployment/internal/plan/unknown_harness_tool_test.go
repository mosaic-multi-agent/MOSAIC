package plan_test

// unknown_harness_tool_test.go covers the gap reported when a tool name written into an agent
// for a harness that ignores unknown names (the GHCP targets) is neither a known tool of that
// harness nor a valid MCP name.
//
// Rules under test:
//   - An unknown name written for an agent that is created or updated is reported once per
//     agent as GapUnknownHarnessTool (Subject = the name, Owner = the agent key).
//   - Names in the harness universe, names a mapping writes, and valid MCP forms (including
//     hierarchical names such as edit/editFiles) are known.
//   - Nothing is reported when the descriptor does not ask for it, or when the agent file is
//     not rewritten (ActionUnchanged).

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/manifest/manifesttest"
	"mosaic-deploy/internal/plan"
)

// reportingModule returns a fake module whose descriptor has GHCP-style tool names and whose
// Tools call writes exactly the given names into the agent's tools field.
func reportingModule(report bool, written ...string) *fakeModule {
	m := newFakeModule()
	m.desc.Frontmatter.ToolsKey = "tools"
	m.desc.Tools = domain.ToolSpec{
		Shape:              domain.ShapeList,
		ReportUnknownNames: report,
		Universe:           []domain.HarnessTool{{Name: "read"}, {Name: "edit"}},
		Mappings: []domain.ToolMapping{{
			Generic:      "content_search",
			Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"search/textSearch"}}},
		}},
		MCP: domain.MCPNameForms{ServerForm: "{server}/*", ToolForm: "{server}/{tool}"},
	}
	m.ToolsFn = func(req domain.ToolRequest) (domain.ToolResult, error) {
		items := make([]domain.FieldValue, len(written))
		for i, n := range written {
			items[i] = domain.ScalarValue(n, domain.QuotePlain)
		}
		res := domain.ToolResolution{Generic: "bash", Outcome: domain.ToolMapped, HarnessTools: written}
		return domain.ToolResult{
			Fields:      []domain.FrontmatterField{{Key: "tools", Value: domain.ListValue(items, domain.ListFlow)}},
			Resolutions: []domain.ToolResolution{res},
		}, nil
	}
	return m
}

// buildCreatePlan builds a deploy plan with one worker agent and no deployed state.
func buildCreatePlan(t *testing.T, module *fakeModule) domain.Plan {
	t.Helper()
	agent := makeAgent("test-agent", "1.0")
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workers:      []domain.Agent{agent},
		workflows:    []domain.Workflow{makeWorkflow("test-wf", agent.Key)},
	}
	p, err := plan.New().Build(context.Background(), plan.Input{
		Catalog:       cat,
		Module:        module,
		Mode:          domain.ModeDeployWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		Manifest:      absentSnapshot(),
		WorkflowIDs:   []string{"test-wf"},
		Models: map[string]domain.ModelSelection{
			agent.Key:      {ModelID: "m", Origin: domain.OriginHarnessList},
			"orchestrator": {ModelID: "m", Origin: domain.OriginHarnessList},
		},
	})
	must(t, err)
	return p
}

func unknownToolGaps(p domain.Plan) []domain.Gap {
	var out []domain.Gap
	for _, g := range p.Gaps {
		if g.Kind == domain.GapUnknownHarnessTool {
			out = append(out, g)
		}
	}
	return out
}

func TestBuild_UnknownToolNameForReportingHarness_ProducesGapOwnedByAgent(t *testing.T) {
	p := buildCreatePlan(t, reportingModule(true, "read", "bogus-tool"))

	gap, ok := findGapForSubject(p.Gaps, domain.GapUnknownHarnessTool, "bogus-tool")
	if !ok {
		t.Fatalf("Plan.Gaps missing GapUnknownHarnessTool for bogus-tool; gaps: %+v", p.Gaps)
	}
	if gap.Owner != "test-agent" {
		t.Errorf("gap Owner = %q, want the agent key %q", gap.Owner, "test-agent")
	}
	for _, want := range []string{`"bogus-tool"`, "{server}/*", "{server}/{tool}"} {
		if !strings.Contains(gap.Detail, want) {
			t.Errorf("gap Detail %q must contain %q", gap.Detail, want)
		}
	}
}

func TestBuild_KnownToolsAndValidMcpForms_ProduceNoUnknownToolGap(t *testing.T) {
	// universe names, a mapping destination name, a whole-server form, a single-tool form,
	// and a hierarchical name that reads as an MCP form are all known.
	p := buildCreatePlan(t, reportingModule(true, "read", "edit", "search/textSearch", "my-server/*", "my-server/send", "edit/editFiles"))

	if gaps := unknownToolGaps(p); len(gaps) != 0 {
		t.Errorf("want no unknown-tool gaps for known names, got %+v", gaps)
	}
}

func TestBuild_UnknownToolNameWithoutReportFlag_ProducesNoGap(t *testing.T) {
	p := buildCreatePlan(t, reportingModule(false, "read", "bogus-tool"))

	if gaps := unknownToolGaps(p); len(gaps) != 0 {
		t.Errorf("a descriptor without the report flag must not produce unknown-tool gaps, got %+v", gaps)
	}
}

func TestBuild_UnknownToolNameWrittenTwice_ProducesOneGap(t *testing.T) {
	p := buildCreatePlan(t, reportingModule(true, "bogus-tool", "read", "bogus-tool"))

	if gaps := unknownToolGaps(p); len(gaps) != 1 {
		t.Errorf("want exactly one gap for a name written twice, got %d: %+v", len(gaps), gaps)
	}
}

func TestBuild_SeveralUnknownNames_ProduceOneGapEachInWrittenOrder(t *testing.T) {
	p := buildCreatePlan(t, reportingModule(true, "zeta-tool", "read", "alpha-tool"))

	gaps := unknownToolGaps(p)
	if len(gaps) != 2 || gaps[0].Subject != "zeta-tool" || gaps[1].Subject != "alpha-tool" {
		t.Errorf("want gaps for zeta-tool then alpha-tool, got %+v", gaps)
	}
}

// buildExistingAgentPlan builds an update plan for an agent whose deployed file carries
// deployedVersion while the catalog carries "1.0".
func buildExistingAgentPlan(t *testing.T, module *fakeModule, deployedVersion string) (domain.Plan, domain.PlanItem) {
	t.Helper()
	agent := makeAgent("test-agent", "1.0")
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workers:      []domain.Agent{agent},
		workflows:    []domain.Workflow{makeWorkflow("test-wf", agent.Key)},
	}
	const target = "agents/test-agent.md"
	const hash = "sha256:aaaaaa"
	entry := makeManifestEntry(agentRef("test-agent"), target, deployedVersion, hash)
	snap := presentSnapshot(domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: "test-harness", UpdatedAt: time.Now()}},
		Entries:       manifesttest.WithHarness("test-harness", []domain.ManifestEntry{entry}),
	})
	p, err := plan.New().Build(context.Background(), plan.Input{
		Catalog:       cat,
		Module:        module,
		Mode:          domain.ModeUpdateWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		Manifest:      snap,
		WorkflowIDs:   []string{"test-wf"},
		Models: map[string]domain.ModelSelection{
			agent.Key:      {ModelID: "m", Origin: domain.OriginHarnessList},
			"orchestrator": {ModelID: "m", Origin: domain.OriginHarnessList},
		},
		DeployedState: map[string]domain.DeployedArtifactState{
			target: deployedState(hash, deployedVersion, "1.0", "1.0"),
		},
	})
	must(t, err)
	item, ok := findItem(p.Items, "test-agent")
	if !ok {
		t.Fatal("plan has no item for test-agent")
	}
	return p, item
}

func TestBuild_UnknownToolNameOnUpdatedAgent_ProducesGap(t *testing.T) {
	p, item := buildExistingAgentPlan(t, reportingModule(true, "bogus-tool"), "0.9")
	if item.Action != domain.ActionUpdate {
		t.Fatalf("test-agent action = %q, want ActionUpdate; check test setup", item.Action)
	}

	if _, ok := findGapForSubject(p.Gaps, domain.GapUnknownHarnessTool, "bogus-tool"); !ok {
		t.Errorf("an agent that is rewritten must report its unknown tool name; gaps: %+v", p.Gaps)
	}
}

func TestBuild_UnknownToolNameOnUnchangedAgent_ProducesNoGap(t *testing.T) {
	p, item := buildExistingAgentPlan(t, reportingModule(true, "bogus-tool"), "1.0")
	if item.Action != domain.ActionUnchanged {
		t.Fatalf("test-agent action = %q, want ActionUnchanged; check test setup", item.Action)
	}

	if gaps := unknownToolGaps(p); len(gaps) != 0 {
		t.Errorf("an agent that is not rewritten must not report unknown tool names, got %+v", gaps)
	}
}
