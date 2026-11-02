package app_test

// todo_dedup_flow_test.go runs a real fresh deploy (real planner, transform, executor and
// collector over the tool-local sample catalog) with no model for any agent and with two
// generic tools the harness cannot map, then reads the written TODO file. Each missing model
// and each unmapped tool of an agent must be listed once, however many components report it.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// repeatContentExecutor runs the real executor but makes every content request produce each
// item's content three times, as a repeated content callback would, returning the last result.
type repeatContentExecutor struct{ inner deploy.Executor }

func (e repeatContentExecutor) Execute(ctx context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	orig := req.Content
	req.Content = func(it domain.PlanItem) ([]byte, error) {
		_, _ = orig(it)
		_, _ = orig(it)
		return orig(it)
	}
	return e.inner.Execute(ctx, req)
}

// toolFlippingModule reports the listed generic tools as unmapped and defers everything else to
// the wrapped real harness module.
// When settled is set, the flip stops once the run has re-resolved tools with the user's
// custom-name and skip maps (the first request carrying a non-nil SkippedTools): later
// requests, including the planner's, see the real mapping, while the transform's requests carry
// the skip map and so resolve to ToolSkipped.
type toolFlippingModule struct {
	domain.HarnessModule
	unmapped map[string]bool
	settled  *atomic.Bool
}

func (m toolFlippingModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	res, err := m.HarnessModule.Tools(req)
	if err != nil {
		return res, err
	}
	if m.settled != nil {
		if req.SkippedTools != nil {
			m.settled.Store(true)
		}
		if m.settled.Load() {
			return res, nil
		}
	}
	for i, r := range res.Resolutions {
		if m.unmapped[r.Generic] && r.Outcome != domain.ToolCustom && r.Outcome != domain.ToolSkipped {
			res.Resolutions[i] = domain.ToolResolution{Generic: r.Generic, Outcome: domain.ToolUnmapped}
		}
	}
	return res, nil
}

// deployWithoutModels deploys the sample catalog with no model for any agent and the given
// tools unmapped, and returns the TODO file lines.
func deployWithoutModels(t *testing.T, unmapped ...string) []string {
	t.Helper()
	return deployWithoutModelsReq(t, nil, unmapped...)
}

// deployWithoutModelsReq is deployWithoutModels with the deploy request adjusted by tweak
// (which may be nil) before the deploy runs.
func deployWithoutModelsReq(t *testing.T, tweak func(*app.DeployRequest), unmapped ...string) []string {
	t.Helper()
	return deployFlow(t, tweak, nil, nil, unmapped...)
}

// flippedService wires the parity service over a Claude module that reports the given generic
// tools unmapped (see toolFlippingModule); depsTweak (may be nil) adjusts the dependencies last.
func flippedService(t *testing.T, settled *atomic.Bool, depsTweak func(*app.Deps), unmapped ...string) app.Service {
	t.Helper()
	set := map[string]bool{}
	for _, u := range unmapped {
		set[u] = true
	}
	return parityServiceWith(t, func(d *app.Deps) {
		real := rbModule(t, hClaude)
		d.Registry = &stubRegistry{
			list:    []domain.HarnessRef{real.Ref()},
			modules: map[string]domain.HarnessModule{hClaude: toolFlippingModule{real, set, settled}},
		}
		if depsTweak != nil {
			depsTweak(d)
		}
	})
}

// deployFlow is the full builder: settled (may be nil) switches the tool flip off after the
// custom-tool resolution, and depsTweak (may be nil) adjusts the dependencies last.
func deployFlow(t *testing.T, tweak func(*app.DeployRequest), settled *atomic.Bool,
	depsTweak func(*app.Deps), unmapped ...string) []string {
	t.Helper()
	svc := flippedService(t, settled, depsTweak, unmapped...)
	req := app.DeployRequest{
		HarnessID:       hClaude,
		WorkspacePath:   t.TempDir(),
		Scope:           domain.ScopeProject,
		WorkflowIDs:     []string{"parity-flow"},
		UtilityAgentIDs: []string{"parity-helper"},
		AutoConfirmPlan: true,
	}
	if tweak != nil {
		tweak(&req)
	}
	res, err := svc.DeployNew(context.Background(), req)
	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
	b, err := os.ReadFile(res.TodoFilePath)
	if err != nil {
		t.Fatalf("read TODO file %q: %v", res.TodoFilePath, err)
	}
	return strings.Split(string(b), "\n")
}

// sectionLines returns the checklist lines under the "## <heading>" section.
func sectionLines(lines []string, heading string) []string {
	var out []string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			in = strings.TrimPrefix(l, "## ") == heading
			continue
		}
		if in && strings.HasPrefix(l, "- [ ]") {
			out = append(out, l)
		}
	}
	return out
}

// TestFreshDeploy_TodoListsEachMissingModelOnce checks every agent, including the one
// rendered first, has exactly one model line.
func TestFreshDeploy_TodoListsEachMissingModelOnce(t *testing.T) {
	lines := deployWithoutModels(t)
	models := sectionLines(lines, string(domain.TodoModels))

	for _, a := range parityAgents {
		n := 0
		for _, l := range models {
			if strings.Contains(l, "**"+a.key+"**") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("model lines for agent %q = %d, want exactly 1; section:\n%s",
				a.key, n, strings.Join(models, "\n"))
		}
	}
}

// TestFreshDeploy_TodoListsEachUnmappedToolOncePerAgent checks one line per unmapped tool per
// agent. parity-runner declares both tools and parity-helper and orchestrator-script only file_search; the placeholder
// orchestrators declare none.
func TestFreshDeploy_TodoListsEachUnmappedToolOncePerAgent(t *testing.T) {
	lines := deployWithoutModels(t, "terminal", "file_search")
	assertOneToolLinePerAgent(t, sectionLines(lines, string(domain.TodoToolMappings)))
}

// TestFreshDeploy_SkippedCustomToolsStillListedOncePerAgent skips the custom tool question for
// every unmapped tool (skip-all). The harness reports the tools unmapped only while the run
// asks for custom names; the planner then sees them mapped and adds no gap of its own, so the
// only report of each tool is the transform's user-skipped one. Each tool must still have
// exactly one line per agent that declares it, and none may come from the plan.
func TestFreshDeploy_SkippedCustomToolsStillListedOncePerAgent(t *testing.T) {
	lines := deployFlow(t, func(r *app.DeployRequest) {
		r.SkipAll = map[domain.QuestionID]bool{domain.QCustomTool: true}
	}, &atomic.Bool{}, nil, "terminal", "file_search")
	tools := sectionLines(lines, string(domain.TodoToolMappings))

	for _, l := range tools {
		if strings.Contains(l, "no harness mapping") {
			t.Errorf("plan-reported tool line present; the scenario must leave only the skipped-tool report: %s", l)
		}
	}
	assertOneToolLinePerAgent(t, tools)
}

// TestFreshDeploy_RepeatedContentCallbacksAddNoDuplicateLines makes the executor produce every
// item's content three times (as a repeated content callback does) and checks the TODO still
// has one model line per agent and one line per unmapped tool per agent.
func TestFreshDeploy_RepeatedContentCallbacksAddNoDuplicateLines(t *testing.T) {
	lines := deployFlow(t, nil, nil, func(d *app.Deps) {
		d.Executor = repeatContentExecutor{inner: d.Executor}
	}, "terminal", "file_search")

	models := sectionLines(lines, string(domain.TodoModels))
	for _, a := range parityAgents {
		n := 0
		for _, l := range models {
			if strings.Contains(l, "**"+a.key+"**") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("model lines for agent %q = %d, want exactly 1; section:\n%s",
				a.key, n, strings.Join(models, "\n"))
		}
	}
	assertOneToolLinePerAgent(t, sectionLines(lines, string(domain.TodoToolMappings)))
}

// TestFreshDeploy_RepeatedContentCallbacksWithSkippedToolsAddNoDuplicateLines isolates the
// repeated-callback behavior: with skip-all and a settled tool flip the plan reports no tool
// gap, so the transform's user-skipped report is the only source of each tool line. Producing
// every item's content three times must still leave exactly one line per tool per agent; only
// a repeated callback re-adding the transform-only gap can make it more.
func TestFreshDeploy_RepeatedContentCallbacksWithSkippedToolsAddNoDuplicateLines(t *testing.T) {
	lines := deployFlow(t, func(r *app.DeployRequest) {
		r.SkipAll = map[domain.QuestionID]bool{domain.QCustomTool: true}
	}, &atomic.Bool{}, func(d *app.Deps) {
		d.Executor = repeatContentExecutor{inner: d.Executor}
	}, "terminal", "file_search")
	tools := sectionLines(lines, string(domain.TodoToolMappings))

	for _, l := range tools {
		if strings.Contains(l, "no harness mapping") {
			t.Errorf("plan-reported tool line present; the scenario must leave only the skipped-tool report: %s", l)
		}
	}
	assertOneToolLinePerAgent(t, tools)
}

// TestUpdate_AgentsWithoutDeployedModel_TodoListsModelAndToolsOnce runs the update flow over a
// workspace deployed without models whose manifest is gone, so every agent goes through the
// conflict path and is rewritten with no model anywhere. The missing model of each agent and
// each unmapped tool of each agent must be listed exactly once, however many components
// report them.
func TestUpdate_AgentsWithoutDeployedModel_TodoListsModelAndToolsOnce(t *testing.T) {
	ws := t.TempDir()
	deployWithoutModelsReq(t, func(r *app.DeployRequest) { r.WorkspacePath = ws }, "terminal", "file_search")
	if err := os.Remove(filepath.Join(ws, manifest.Dir, manifest.FileName)); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}

	res, err := flippedService(t, nil, nil, "terminal", "file_search").Update(context.Background(), app.UpdateRequest{
		HarnessID:       hClaude,
		WorkspacePath:   ws,
		ConflictDefault: domain.DecisionOverwrite,
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	b, err := os.ReadFile(res.TodoFilePath)
	if err != nil {
		t.Fatalf("read TODO file %q: %v", res.TodoFilePath, err)
	}
	lines := strings.Split(string(b), "\n")

	models := sectionLines(lines, string(domain.TodoModels))
	for _, a := range parityAgents {
		n := 0
		for _, l := range models {
			if strings.Contains(l, "**"+a.key+"**") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("model lines for agent %q = %d, want exactly 1; TODO:\n%s",
				a.key, n, strings.Join(lines, "\n"))
		}
	}
	assertOneToolLinePerAgent(t, sectionLines(lines, string(domain.TodoToolMappings)))
}

// assertOneToolLinePerAgent checks the tool section has exactly one line for each of the four
// tool/agent pairs of the sample catalog and nothing else.
func assertOneToolLinePerAgent(t *testing.T, tools []string) {
	t.Helper()

	want := map[string]int{
		"terminal/parity-runner":          0,
		"file_search/parity-runner":       0,
		"file_search/parity-helper":       0,
		"file_search/orchestrator-script": 0,
	}
	for _, l := range tools {
		matched := false
		for k := range want {
			tool, agent, _ := strings.Cut(k, "/")
			if strings.Contains(l, tool) && strings.Contains(l, agent) {
				want[k]++
				matched = true
			}
		}
		if !matched {
			t.Errorf("unexpected tool line (matches no tool/agent pair): %s", l)
		}
	}
	for k, n := range want {
		if n != 1 {
			t.Errorf("tool lines for %s = %d, want exactly 1; section:\n%s", k, n, strings.Join(tools, "\n"))
		}
	}
	if len(tools) != len(want) {
		t.Errorf("tool lines = %d, want %d; section:\n%s", len(tools), len(want), strings.Join(tools, "\n"))
	}
}
