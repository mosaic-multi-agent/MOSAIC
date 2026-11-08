package app_test

// update_kept_fixtures_test.go holds the shared fixtures and helpers of the Update workspace
// tests for workflows that are deployed in an orchestrator-role file but not supplied by the
// stack. The tests run Update against a fixture workspace and read back the bytes the content
// step produces for each orchestrator-role file.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
	"mosaic-deploy/internal/transform"
)

const (
	keptOrchPath   = "orchestrator.md"
	keptScriptPath = "orchestrator-script.md"
	keptWorkerPath = "test-runner.md"
)

const keptDeployedHead = "---\nversion: \"1.0\"\n---\n\n<AvailableWorkflows type=\"managed\">\n"

// Deployed blocks. Custom ids (custom-x, custom-y, custom-z) are in no catalog of the tests.
const (
	keptQuickFixDeployed = "<Workflow type=\"managed\" name=\"quick-fix\" version=\"0.9\">\n## Quick Fix\nOLD-QUICKFIX-BODY\n</Workflow>\n"
	keptOtherDeployed    = "<Workflow type=\"managed\" name=\"other-flow\" version=\"0.9\">\n## Other Flow\nOLD-OTHER-BODY\n</Workflow>\n"

	keptCustomXOrch = "<Workflow type=\"managed\" name=\"custom-x\" version=\"1.0\">\n## Custom X\n\n" +
		"| Phase | Subagent |\n|-------|----------|\n| ONE | ghost-agent |\n\nCUSTOM-X-ORCH-BODY\n</Workflow>\n"
	keptCustomXScript = "<Workflow type=\"managed\" name=\"custom-x\" version=\"2.0\">\n## Custom X\n\n" +
		"| Phase | Subagent |\n|-------|----------|\n| TWO | ghost-agent |\n\nCUSTOM-X-SCRIPT-BODY, longer than the orchestrator copy\n</Workflow>\n"
	keptCustomYScript = "<Workflow type=\"managed\" name=\"custom-y\" version=\"1.0\">\n## Custom Y\nCUSTOM-Y-SCRIPT-BODY\n</Workflow>\n"
	keptCustomZOrch   = "<Workflow type=\"managed\" name=\"custom-z\" version=\"1.0\">\n## Custom Z\nCUSTOM-Z-ORCH-BODY\n</Workflow>\n"

	// keptRaggedX has a ragged routing table, a nested custom region, an indented opening tag
	// and trailing spaces after the closing tag.
	keptRaggedX = "  <Workflow type=\"managed\" name=\"custom-x\" version=\"1.0\">\n## Ragged\n\n" +
		"| A | B | C |\n|---|---|---|\n| 1 | 2 |\n| 1 | 2 | 3 | 4 |\n\n" +
		"<RaggedNotes type=\"custom\">\nnested custom region\n</RaggedNotes>\n  </Workflow>   \n"
)

const keptWorkerSource = `---
role: subagent
version: "2.0"
name: Test Runner
description: Subagent for kept workflow tests.
tools: []
---

<Identity type="core">
NEW-WORKER-MARKER
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

<HarnessConstraints type="managed">
</HarnessConstraints>
</Constraints>
`

const keptWorkerDeployed = "---\nversion: \"0.5\"\n---\n\n<Identity type=\"core\">\nOLD-WORKER-MARKER\n</Identity>\n"

// keptDeployedFile returns a deployed orchestrator-role file holding the blocks in order.
func keptDeployedFile(blocks ...string) string {
	return keptDeployedHead + strings.Join(blocks, "") + "</AvailableWorkflows>\n"
}

// keptCatalog returns a stub catalog with both orchestrators, the worker test-runner, and the
// catalog workflows quick-fix (routes to the worker) and other-flow. Section bodies carry
// markers that tell a refreshed catalog block from a deployed one.
func keptCatalog() *stubCatalog {
	cat := scriptContentCatalog("/fake/orchestrator-script.md", "/fake/orchestrator.md")
	worker := minimalAgent
	worker.SourcePath = "/fake/test-runner.md"
	cat.agents = []domain.Agent{worker}
	cat.sources[worker.SourcePath] = []byte(keptWorkerSource)
	cat.workflows = append(cat.workflows, domain.Workflow{ID: "other-flow", Name: "Other Flow", Version: "1.0"})
	cat.sections = map[string][]byte{
		"quick-fix":  []byte("<Workflow type=\"core\" name=\"quick-fix\">\n## Quick Fix\nCATALOG-QUICKFIX-BODY\n</Workflow>\n"),
		"other-flow": []byte("<Workflow type=\"core\" name=\"other-flow\">\n## Other Flow\nCATALOG-OTHER-BODY\n</Workflow>\n"),
		"custom-x":   []byte("<Workflow type=\"core\" name=\"custom-x\">\n## Custom X\nCATALOG-X-BODY\n</Workflow>\n"),
		"broken":     []byte("<Workflow type=\"core\" name=\"broken\">\n## Broken\nCATALOG-BROKEN-BODY\n</Workflow>\n"),
	}
	return cat
}

// domainWorkflowRoutingTo returns a catalog workflow whose routing names the given agent.
func domainWorkflowRoutingTo(id, agentKey string) domain.Workflow {
	return domain.Workflow{ID: id, Name: id, Version: "1.0", ReferencedAgents: []string{agentKey}}
}

// keptPlan returns an update plan with one item per deployed file of the fixture workspace.
func keptPlan(ws string) domain.Plan {
	item := func(key string) domain.PlanItem {
		return domain.PlanItem{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
			TargetPath: key + ".md",
			SourcePath: "/fake/" + key + ".md",
			Action:     domain.ActionUpdate,
			Model:      domain.ModelSelection{ModelID: "model-a", Origin: domain.OriginTierMapping},
		}
	}
	return domain.Plan{
		Mode: domain.ModeUpdateWorkspace, Harness: minimalHarness, WorkspacePath: ws, Scope: domain.ScopeProject,
		Items: []domain.PlanItem{item("orchestrator"), item("orchestrator-script"), item("test-runner")},
	}
}

// writeKeptWorkspace writes the two orchestrator-role files and the deployed worker.
func writeKeptWorkspace(t *testing.T, ws, orch, script string) {
	t.Helper()
	for path, content := range map[string]string{keptOrchPath: orch, keptScriptPath: script, keptWorkerPath: keptWorkerDeployed} {
		if err := os.WriteFile(filepath.Join(ws, path), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// keptRunResult is what one Update run produced: the content of each agent item as the
// executor would write it, the content errors, and the input the planner received.
type keptRunResult struct {
	outputs map[string][]byte
	errs    map[string]error
	called  bool
	input   *plan.Input
}

// keptExecutor asks for the content of every agent item, the way the real executor does.
type keptExecutor struct{ res *keptRunResult }

func (e *keptExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.res.called = true
	for _, item := range req.Plan.Items {
		if item.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		out, err := req.Content(item)
		if err != nil {
			e.res.errs[item.TargetPath] = err
			continue
		}
		e.res.outputs[item.TargetPath] = out
	}
	return deploy.ExecResult{DeploymentRoot: req.Plan.WorkspacePath}, nil
}

// runKeptUpdate runs Update workspace over ws with the catalog and extra workflow ids.
func runKeptUpdate(t *testing.T, ws string, cat *stubCatalog, addIDs []string) (*keptRunResult, error) {
	t.Helper()
	res := &keptRunResult{outputs: map[string][]byte{}, errs: map[string]error{}}
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	deps.Catalog = cat
	deps.Executor = &keptExecutor{res: res}
	planner := &capturingPlanner{result: keptPlan(ws)}
	deps.Planner = planner

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: minimalHarness.ID, WorkspacePath: ws, AutoConfirmPlan: true, AddWorkflowIDs: addIDs,
	})
	res.input = planner.capturedInput
	return res, err
}

// mustRunKeptUpdate runs the update and fails the test on any run or content error.
func mustRunKeptUpdate(t *testing.T, ws string, cat *stubCatalog, addIDs []string) *keptRunResult {
	t.Helper()
	res, err := runKeptUpdate(t, ws, cat, addIDs)
	if err != nil {
		t.Fatalf("Update must succeed: %v", err)
	}
	for path, cerr := range res.errs {
		t.Fatalf("content for %s failed: %v", path, cerr)
	}
	return res
}

// keptBlocksIn returns the direct workflow blocks of an output file in document order.
func keptBlocksIn(t *testing.T, label string, out []byte) []transform.WorkflowBlock {
	t.Helper()
	if len(out) == 0 {
		t.Fatalf("%s: the run produced no content", label)
	}
	blocks, err := transform.KeptWorkflowBlocks(out)
	if err != nil {
		t.Fatalf("%s: read workflow blocks of the output: %v\n%s", label, err, out)
	}
	return blocks
}

// keptIDsOf returns the ids of blocks in order.
func keptIDsOf(blocks []transform.WorkflowBlock) []string {
	ids := make([]string, 0, len(blocks))
	for _, b := range blocks {
		ids = append(ids, b.ID)
	}
	return ids
}

// requireKeptIDs fails unless got equals want, in order.
func requireKeptIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s: workflow ids in file order = %v, want %v", label, got, want)
	}
}

// blockBytes returns the bytes of the block with id, or nil when absent.
func blockBytes(blocks []transform.WorkflowBlock, id string) []byte {
	for _, b := range blocks {
		if b.ID == id {
			return b.Block
		}
	}
	return nil
}

// requireVerbatim fails unless the block with id holds exactly the deployed bytes.
func requireVerbatim(t *testing.T, label string, blocks []transform.WorkflowBlock, id, deployed string) {
	t.Helper()
	got := blockBytes(blocks, id)
	if !bytes.Equal(got, []byte(deployed)) {
		t.Errorf("%s: block %q is not byte-identical to the deployed block\ngot:  %q\nwant: %q", label, id, got, deployed)
	}
}

// requireRefreshed fails unless the block with id holds the catalog marker and not the old one.
func requireRefreshed(t *testing.T, label string, blocks []transform.WorkflowBlock, id, newMarker, oldMarker string) {
	t.Helper()
	got := blockBytes(blocks, id)
	if !bytes.Contains(got, []byte(newMarker)) || bytes.Contains(got, []byte(oldMarker)) {
		t.Errorf("%s: block %q = %q, want the refreshed catalog block holding %q and not %q", label, id, got, newMarker, oldMarker)
	}
}

// requireKeptInput fails unless the planner received every id as a kept id and none of them as
// a catalog workflow id.
func requireKeptInput(t *testing.T, in *plan.Input, ids ...string) {
	t.Helper()
	if in == nil {
		t.Fatal("the planner was never called")
	}
	for _, id := range ids {
		if !containsID(in.KeptWorkflowIDs, id) {
			t.Errorf("plan.Input.KeptWorkflowIDs = %v, want it to hold %q", in.KeptWorkflowIDs, id)
		}
		if containsID(in.WorkflowIDs, id) {
			t.Errorf("plan.Input.WorkflowIDs = %v, must not hold the kept id %q", in.WorkflowIDs, id)
		}
	}
}

func containsID(ids []string, id string) bool {
	for _, have := range ids {
		if have == id {
			return true
		}
	}
	return false
}
