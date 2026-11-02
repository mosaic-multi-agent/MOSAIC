package app_test

// harness_region_stamps_test.go specifies that the deploy build callback and render apply the
// same per-region harness injection tagging: HarnessConstraints carries the HarnessInjections.md
// version and OrchestratorHarnessConstraints carries the HarnessInjectionsOrchestrator.md
// version, for both orchestrator roles, on every path. It also covers the missing-region
// warning reaching the deploy TODO output once per agent.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

const (
	stampSharedVersion = "1.4.0"
	stampOrchVersion   = "2.3.0"
	stampSharedText    = "STAMP_SHARED_TEXT"
	stampOrchText      = "STAMP_ORCH_TEXT"
)

const stampSourceHead = `---
role: orchestrator
version: "1.0"
name: Stamp Orchestrator
description: Orchestrator for per-region stamp tests.
tools: []
---

<Identity type="core">
You are the stamp test orchestrator.
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

`

const stampSourceBothMarkers = stampSourceHead + `<HarnessConstraints type="managed">
</HarnessConstraints>

<OrchestratorHarnessConstraints type="managed">
</OrchestratorHarnessConstraints>
</Constraints>
`

const stampSourceSharedMarkerOnly = stampSourceHead + `<HarnessConstraints type="managed">
</HarnessConstraints>
</Constraints>
`

// regionStubModule serves per-region content and reports distinct injection versions.
type regionStubModule struct {
	*stubHarnessModule
}

func newRegionStubModule() *regionStubModule {
	mod := newMinimalModule()
	mod.descriptor.InjectionsVersion = stampSharedVersion
	mod.descriptor.OrchestratorInjectionsVersion = stampOrchVersion
	return &regionStubModule{stubHarnessModule: mod}
}

func (m *regionStubModule) Injection(req domain.InjectionRequest) (string, bool) {
	switch req.Name {
	case docformat.HarnessRegionName:
		return stampSharedText, true
	case docformat.OrchestratorHarnessRegionName:
		if req.Role == domain.RoleOrchestrator {
			return stampOrchText, true
		}
	}
	return "", false
}

func assertRegionStamps(t *testing.T, label string, content []byte) {
	t.Helper()
	doc, err := docformat.Parse(content)
	if err != nil {
		t.Fatalf("%s: parse output: %v", label, err)
	}
	shared, ok := doc.Body().Deployed(docformat.HarnessRegionName)
	if !ok {
		t.Fatalf("%s: HarnessConstraints region absent:\n%s", label, content)
	}
	orch, ok := doc.Body().Deployed(docformat.OrchestratorHarnessRegionName)
	if !ok {
		t.Fatalf("%s: OrchestratorHarnessConstraints region absent:\n%s", label, content)
	}
	if got := shared.Version(); got != stampSharedVersion {
		t.Errorf("%s: HarnessConstraints version = %q, want %q", label, got, stampSharedVersion)
	}
	if got := orch.Version(); got != stampOrchVersion {
		t.Errorf("%s: OrchestratorHarnessConstraints version = %q, want %q", label, got, stampOrchVersion)
	}
	if !strings.Contains(string(shared.Content()), stampSharedText) || strings.Contains(string(shared.Content()), stampOrchText) {
		t.Errorf("%s: HarnessConstraints content = %q, want the shared text only", label, shared.Content())
	}
	if !strings.Contains(string(orch.Content()), stampOrchText) || strings.Contains(string(orch.Content()), stampSharedText) {
		t.Errorf("%s: OrchestratorHarnessConstraints content = %q, want the orchestrator text only", label, orch.Content())
	}
}

// repeatingContentExecutor calls the content callback invokes times per agent item and keeps
// the last result, the way a deploy probes and then writes the same item.
type repeatingContentExecutor struct {
	invokes  int
	contents map[string][]byte
	errors   map[string]error
}

func (e *repeatingContentExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.contents = make(map[string][]byte)
	e.errors = make(map[string]error)
	for _, item := range req.Plan.Items {
		if item.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		for i := 0; i < e.invokes; i++ {
			content, err := req.Content(item)
			if err != nil {
				e.errors[item.Ref.Key] = err
				continue
			}
			e.contents[item.Ref.Key] = content
		}
	}
	return deploy.ExecResult{DeploymentRoot: req.Plan.WorkspacePath}, nil
}

// deployCaptureDeps wires a deploy of both orchestrators from the given source text and returns
// the capturing executor and the TODO spy.
func deployCaptureDeps(t *testing.T, source string, invokes int) (*repeatingContentExecutor, *spyTodo, app.Deps, string) {
	t.Helper()
	mainPath, scriptPath := "/fake/orchestrator.md", "/fake/orchestrator-script.md"
	cat := scriptContentCatalog(scriptPath, mainPath)
	cat.sources[mainPath] = []byte(source)
	cat.sources[scriptPath] = []byte(source)

	exec := &repeatingContentExecutor{invokes: invokes}
	spy := &spyTodo{}
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newBaseDeps(t, stub)
	deps.Catalog = cat
	deps.Executor = exec
	deps.Todo = spy
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{minimalHarness},
		modules: map[string]domain.HarnessModule{minimalHarness.ID: newRegionStubModule()},
	}
	deps.Planner = &stubPlanner{plan: scriptPlan(workspace, "orchestrator.md", "orchestrator-script.md")}
	return exec, spy, deps, workspace
}

func runDeployCapture(t *testing.T, deps app.Deps, workspace string) {
	t.Helper()
	_, err := app.New(deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID:       minimalHarness.ID,
		WorkspacePath:   workspace,
		WorkflowIDs:     []string{minimalWorkflow.ID},
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
}

func TestDeployBuild_Orchestrators_TwoRegionsEachTaggedWithItsFileVersion(t *testing.T) {
	// The content callback runs twice per item (probe plus write); the output must stay the same.
	exec, _, deps, workspace := deployCaptureDeps(t, stampSourceBothMarkers, 2)

	runDeployCapture(t, deps, workspace)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if err := exec.errors[key]; err != nil {
			t.Fatalf("content callback for %s failed: %v", key, err)
		}
		assertRegionStamps(t, key, exec.contents[key])
	}
}

func TestDeployBuild_OrchestratorSourceWithoutSecondMarker_WarnsOncePerAgentAndContinues(t *testing.T) {
	// The content callback runs twice per item here (probe plus write); the TODO output must
	// still hold one warning per agent.
	exec, spy, deps, workspace := deployCaptureDeps(t, stampSourceSharedMarkerOnly, 2)

	runDeployCapture(t, deps, workspace)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if err := exec.errors[key]; err != nil {
			t.Fatalf("content callback for %s failed: %v", key, err)
		}
		if strings.Contains(string(exec.contents[key]), stampOrchText) {
			t.Errorf("%s: orchestrator text appears in output although the source has no region for it", key)
		}
		assertOneUndeliveredWarning(t, key, spy.gaps)
	}
}

// assertOneUndeliveredWarning requires exactly one undelivered-orchestrator-injection gap owned
// by key, and that its detail identifies the undelivered orchestrator content.
func assertOneUndeliveredWarning(t *testing.T, key string, gaps []domain.Gap) {
	t.Helper()
	n := 0
	for _, g := range gaps {
		if g.Kind != domain.GapUndeliveredOrchestratorInjection || g.Owner != key {
			continue
		}
		n++
		if !strings.Contains(g.Detail, stampOrchText) {
			t.Errorf("%s: warning detail %q does not identify the undelivered content %q", key, g.Detail, stampOrchText)
		}
	}
	if n != 1 {
		t.Errorf("%s: %d undelivered-orchestrator-injection warnings in the output, want exactly 1", key, n)
	}
}

func TestRender_Orchestrator_TwoRegionsEachTaggedWithItsFileVersion(t *testing.T) {
	deps := newRenderDeps(t)
	mod := newRegionStubModule()
	mod.ref = renderHarness
	mod.descriptor.ID = renderHarness.ID
	mod.descriptor.Paths = newRenderTargetModule().descriptor.Paths
	mod.descriptor.Extensions = newRenderTargetModule().descriptor.Extensions
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{renderHarness},
		modules: map[string]domain.HarnessModule{renderHarness.ID: mod},
	}
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-stamp-orchestrator.md", []byte(stampSourceBothMarkers))
	dest := filepath.Join(dir, "out.md")
	req := newPreAnsweredRenderRequest(src, dest)
	req.Workflows = []string{}

	if _, err := app.New(deps).RenderAgent(context.Background(), req); err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}

	out, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read rendered output: %v", err)
	}
	assertRegionStamps(t, "render", out)
}
