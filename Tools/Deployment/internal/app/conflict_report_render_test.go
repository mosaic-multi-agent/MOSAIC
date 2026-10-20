package app_test

// conflict_report_render_test.go verifies that render carries the catalog-stack conflict report
// on its result where it plans content, names the workflows it renders on the affected agent's
// line, keeps the report out of its JSON when nothing is shadowed, and records no gap.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

// crRenderStackDeps returns render deps whose catalog follows the fixture two-layer stack.
func crRenderStackDeps(t *testing.T, opts crStackOpts) (app.Deps, *spyTodo) {
	t.Helper()
	deps := newRenderDeps(t)
	spy := &spyTodo{}
	deps.Todo = spy
	cat := deps.Catalog.(*stubCatalog)
	cat.agents = append(cat.agents, crOtherAgent)
	cat.workflows = append(cat.workflows, opts.workflows...)
	mosaic, layers := crLayers(t, opts)
	useResolvedStack(t, &deps, mosaic, layers...)
	return deps, spy
}

// TestRenderAgent_ShadowedAgent_ResultCarriesReport verifies that a render over a two-layer
// stack in which the agent is shadowed carries the report with the winner and the shadowed
// layer, and adds no gap.
func TestRenderAgent_ShadowedAgent_ResultCarriesReport(t *testing.T) {
	deps, spy := crRenderStackDeps(t, crStackOpts{})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())
	req := newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md"))

	result, err := app.New(deps).RenderAgent(context.Background(), req)

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Fatal("ConflictReport = nil, want the report of the shadowed agent")
	}
	line := crLineFor(t, *result.ConflictReport, crAgentItem())
	if line.Severity != domain.ConflictWarning || line.Winner.ID != crUserID {
		t.Errorf("line = %+v, want a warning won by %q", line, crUserID)
	}
	if len(spy.gaps) != 0 {
		t.Errorf("render recorded gaps %+v, want none", spy.gaps)
	}
}

// TestRenderAgent_Orchestrator_NamesRenderedWorkflowsOnAgentLine verifies that the workflows
// an orchestrator render embeds count as the render's own selection: the one that routes to the
// shadowed agent is named, one that does not is not.
func TestRenderAgent_Orchestrator_NamesRenderedWorkflowsOnAgentLine(t *testing.T) {
	deps, _ := crRenderStackDeps(t, crStackOpts{
		workflows: []domain.Workflow{crWorkflow("unrelated-flow", crOtherAgentKey)},
	})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-orchestrator.md", minimalGenericOrchestratorBytes())
	req := newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md"))
	req.Workflows = []string{"quick-fix", "unrelated-flow"}

	result, err := app.New(deps).RenderAgent(context.Background(), req)

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Fatal("ConflictReport = nil, want the report of the shadowed agent")
	}
	got := crLineFor(t, *result.ConflictReport, crAgentItem()).AffectedWorkflows
	if !reflect.DeepEqual(got, []string{"quick-fix"}) {
		t.Errorf("AffectedWorkflows = %v, want [quick-fix]", got)
	}
}

// TestRenderAgent_NonOrchestrator_NamesNoWorkflow verifies that rendering an agent that embeds
// no workflow names none: render reads no workspace, so it has no deployed workflows.
func TestRenderAgent_NonOrchestrator_NamesNoWorkflow(t *testing.T) {
	deps, _ := crRenderStackDeps(t, crStackOpts{})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())
	req := newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md"))

	result, err := app.New(deps).RenderAgent(context.Background(), req)

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Fatal("ConflictReport = nil, want the report of the shadowed agent")
	}
	if got := crLineFor(t, *result.ConflictReport, crAgentItem()).AffectedWorkflows; len(got) != 0 {
		t.Errorf("AffectedWorkflows = %v, want none", got)
	}
}

// TestRenderAgent_DryRun_StillCarriesReportAndWritesNothing verifies that a dry-run render
// carries the report and creates no destination file.
func TestRenderAgent_DryRun_StillCarriesReportAndWritesNothing(t *testing.T) {
	deps, _ := crRenderStackDeps(t, crStackOpts{})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())
	dest := filepath.Join(dir, "out.md")
	req := newPreAnsweredRenderRequest(src, dest)
	req.DryRun = true

	result, err := app.New(deps).RenderAgent(context.Background(), req)

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Error("ConflictReport = nil on a dry run, want the report")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("destination exists after a dry run (stat err = %v)", statErr)
	}
}

// TestRenderAgent_ReportJSON_PresentWhenShadowedAbsentOtherwise verifies the additive JSON
// shape: the key conflictReport appears with its lines when the agent is shadowed, and is
// absent for a catalog without shadowing.
func TestRenderAgent_ReportJSON_PresentWhenShadowedAbsentOtherwise(t *testing.T) {
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	shadowedDeps, _ := crRenderStackDeps(t, crStackOpts{})
	shadowed, err := app.New(shadowedDeps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "a.md")))
	if err != nil {
		t.Fatalf("RenderAgent (shadowed): %v", err)
	}
	plain, err := app.New(newRenderDeps(t)).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "b.md")))
	if err != nil {
		t.Fatalf("RenderAgent (plain): %v", err)
	}

	shadowedKeys := crJSONKeys(t, shadowed)
	plainKeys := crJSONKeys(t, plain)

	raw, ok := shadowedKeys["conflictReport"]
	if !ok {
		t.Fatal("JSON of a shadowed render lacks the key conflictReport")
	}
	var doc struct {
		Lines []struct {
			Severity string `json:"severity"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Lines) == 0 || doc.Lines[0].Severity != "warning" {
		t.Errorf("conflictReport = %s (decode err %v), want at least one warning line", raw, err)
	}
	if _, present := plainKeys["conflictReport"]; present {
		t.Error("JSON of a render without shadowing carries conflictReport, want the key absent")
	}
}

// TestRenderAgent_PlainCatalog_NoReport verifies that a catalog that is not a stack leaves the
// result's report nil.
func TestRenderAgent_PlainCatalog_NoReport(t *testing.T) {
	deps := newRenderDeps(t)
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	result, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md")))

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport != nil {
		t.Errorf("ConflictReport = %+v, want nil without shadowing", result.ConflictReport)
	}
}

// TestRenderAgent_DefaultOnlyStack_ResultCarriesNoReportLines verifies that a stack of the
// default catalog alone has no shadow records, so the result carries no report line.
func TestRenderAgent_DefaultOnlyStack_ResultCarriesNoReportLines(t *testing.T) {
	deps := newRenderDeps(t)
	mosaic := newSysMosaicRoot(t, sysLayer{id: crDefaultID, protocolVersion: "1.0", bundleVersion: "1.0.0"})
	crWriteAgent(t, filepath.Join(mosaic, catalogpaths.CatalogDirName), crAgentKey, "1.0")
	useResolvedStack(t, &deps, mosaic, defaultStackLayer(mosaic))
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	result, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md")))

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport != nil && len(result.ConflictReport.Lines) != 0 {
		t.Errorf("ConflictReport = %+v, want none for a single layer", result.ConflictReport)
	}
}

// crJSONKeys marshals v and returns its top-level keys.
func crJSONKeys(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}
