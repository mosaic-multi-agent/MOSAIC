package plan_test

// custom_tool_answers_test.go covers how the run's custom-tool answers (plan.Input.CustomToolNames
// and plan.Input.SkippedTools) shape the unmapped-tool gaps of a plan:
//   - a tool the user gave a server name for raises no GapUnmappedTool
//   - a skipped tool and an unasked unmapped tool still raise GapUnmappedTool
//   - the answers are forwarded into every tool request the planner issues

import (
	"context"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// answerAwareModule returns a fake module that resolves tools the way the real descriptor
// mapping does: a tool with a custom name is ToolCustom, a skipped tool is ToolSkipped, any
// other listed tool is ToolUnmapped and everything else is ToolMapped. Every request is
// appended to *seen.
func answerAwareModule(unmapped []string, seen *[]domain.ToolRequest) *fakeModule {
	module := newFakeModule()
	module.ToolsFn = func(req domain.ToolRequest) (domain.ToolResult, error) {
		*seen = append(*seen, req)
		resolutions := make([]domain.ToolResolution, len(req.Generic))
		for i, g := range req.Generic {
			outcome := domain.ToolMapped
			for _, u := range unmapped {
				if g != u {
					continue
				}
				switch {
				case req.CustomNames[g] != "":
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
	return module
}

// buildWithTools plans one new agent declaring the given tools, with the given answers.
func buildWithTools(t *testing.T, module domain.HarnessModule, tools []string,
	custom map[string]string, skipped map[string]bool) domain.Plan {
	t.Helper()
	agent := makeAgent("test-agent", "1.0")
	agent.Tools = tools
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
			agent.Key:      {ModelID: "test-model", Origin: domain.OriginHarnessList},
			"orchestrator": {ModelID: "test-model", Origin: domain.OriginHarnessList},
		},
		CustomToolNames: custom,
		SkippedTools:    skipped,
	})
	must(t, err)
	return p
}

// TestBuild_CustomNamedTool_RaisesNoUnmappedToolGap verifies that a tool the user gave a
// server name for is not reported as unmapped.
func TestBuild_CustomNamedTool_RaisesNoUnmappedToolGap(t *testing.T) {
	var seen []domain.ToolRequest
	module := answerAwareModule([]string{"bash-shell"}, &seen)

	p := buildWithTools(t, module, []string{"bash-shell"},
		map[string]string{"bash-shell": "my-shell-server"}, nil)

	if gap, ok := findGapForSubject(p.Gaps, domain.GapUnmappedTool, "bash-shell"); ok {
		t.Errorf("Plan.Gaps has an unmapped-tool gap %+v for a tool the user named; "+
			"answered tools must not be reported", gap)
	}
}

// TestBuild_AnsweredAndUnaskedTools_OnlyUnaskedReported verifies that with two unmapped tools
// on one agent and an answer for only one, the answered tool raises no gap and the other does.
func TestBuild_AnsweredAndUnaskedTools_OnlyUnaskedReported(t *testing.T) {
	var seen []domain.ToolRequest
	module := answerAwareModule([]string{"bash-shell", "web-search"}, &seen)

	p := buildWithTools(t, module, []string{"bash-shell", "web-search"},
		map[string]string{"bash-shell": "my-shell-server"}, nil)

	if _, ok := findGapForSubject(p.Gaps, domain.GapUnmappedTool, "bash-shell"); ok {
		t.Error("answered tool \"bash-shell\" is reported as unmapped")
	}
	gap, ok := findGapForSubject(p.Gaps, domain.GapUnmappedTool, "web-search")
	if !ok {
		t.Fatal("unasked unmapped tool \"web-search\" is not reported as unmapped")
	}
	if gap.Owner != "test-agent" {
		t.Errorf("gap Owner = %q, want the agent key %q", gap.Owner, "test-agent")
	}
}

// TestBuild_SkippedTool_RaisesUnmappedToolGap verifies that a tool the module resolves as
// skipped is still reported as unmapped, with the agent as owner.
func TestBuild_SkippedTool_RaisesUnmappedToolGap(t *testing.T) {
	module := newFakeModule()
	module.ToolsFn = func(req domain.ToolRequest) (domain.ToolResult, error) {
		res := make([]domain.ToolResolution, len(req.Generic))
		for i, g := range req.Generic {
			res[i] = domain.ToolResolution{Generic: g, Outcome: domain.ToolSkipped}
		}
		return domain.ToolResult{Resolutions: res}, nil
	}

	p := buildWithTools(t, module, []string{"bash-shell"}, nil, map[string]bool{"bash-shell": true})

	gap, ok := findGapForSubject(p.Gaps, domain.GapUnmappedTool, "bash-shell")
	if !ok {
		t.Fatal("a skipped tool is not reported as unmapped; skipped tools must still raise the gap")
	}
	if gap.Owner != "test-agent" {
		t.Errorf("gap Owner = %q, want the agent key %q", gap.Owner, "test-agent")
	}
}

// TestBuild_AnswersForwardedIntoEveryToolRequest verifies that each tool request the planner
// issues carries the run's custom names and skipped tools.
func TestBuild_AnswersForwardedIntoEveryToolRequest(t *testing.T) {
	var seen []domain.ToolRequest
	module := answerAwareModule([]string{"bash-shell", "web-search"}, &seen)
	custom := map[string]string{"bash-shell": "my-shell-server"}
	skipped := map[string]bool{"web-search": true}

	p := buildWithTools(t, module, []string{"bash-shell", "web-search"}, custom, skipped)

	if len(seen) == 0 {
		t.Fatal("the planner issued no tool request")
	}
	for _, req := range seen {
		if req.CustomNames["bash-shell"] != "my-shell-server" {
			t.Errorf("tool request for %q carries CustomNames %v, want bash-shell answered",
				req.AgentKey, req.CustomNames)
		}
		if !req.SkippedTools["web-search"] {
			t.Errorf("tool request for %q carries SkippedTools %v, want web-search skipped",
				req.AgentKey, req.SkippedTools)
		}
	}
	if _, ok := findGapForSubject(p.Gaps, domain.GapUnmappedTool, "web-search"); !ok {
		t.Error("skipped tool \"web-search\" is not reported as unmapped")
	}
}
