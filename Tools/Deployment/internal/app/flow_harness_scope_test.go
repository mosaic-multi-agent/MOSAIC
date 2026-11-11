package app_test

// flow_harness_scope_test.go verifies that a flow hands the planner only the manifest entries
// of the run's harness. A path used by two harnesses carries one entry per harness; the entry
// of the other harness must not reach the run's classification.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	scopeRunHarness   = "stub-harness"
	scopeOtherHarness = "ghcp-cli"
	scopeSharedPath   = "test-runner.md"
)

// twoHarnessSnapshot is a present manifest in which scopeSharedPath is recorded once for the
// run harness and once for another harness, plus one path only the other harness holds.
func twoHarnessSnapshot() manifest.Snapshot {
	ref := domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "test-runner"}
	entry := func(harness, path, hash string) domain.ManifestEntry {
		return domain.ManifestEntry{Ref: ref, TargetPath: path, Harness: harness, Version: "1.0", ContentHash: hash}
	}
	return manifest.Snapshot{
		State: manifest.StatePresent,
		Manifest: domain.Manifest{
			SchemaVersion: "2",
			Harnesses: []domain.HarnessSection{
				{HarnessID: scopeOtherHarness}, {HarnessID: scopeRunHarness},
			},
			Entries: []domain.ManifestEntry{
				entry(scopeOtherHarness, scopeSharedPath, "sha256:other"),
				entry(scopeRunHarness, scopeSharedPath, "sha256:own"),
				entry(scopeOtherHarness, ".github/agents/only-other.agent.md", "sha256:other2"),
			},
		},
	}
}

// assertOnlyRunHarnessEntries fails unless the planner input carries the run harness's entry
// at the shared path and nothing recorded for another harness.
func assertOnlyRunHarnessEntries(t *testing.T, cp *capturingPlanner) {
	t.Helper()
	if cp.capturedInput == nil {
		t.Fatal("planner.Build was never called")
	}
	var shared []domain.ManifestEntry
	for _, e := range cp.capturedInput.Manifest.Manifest.Entries {
		if e.Harness != scopeRunHarness {
			t.Errorf("planner received an entry of harness %q at %q; only %q entries may reach a run",
				e.Harness, e.TargetPath, scopeRunHarness)
		}
		if e.TargetPath == scopeSharedPath {
			shared = append(shared, e)
		}
	}
	if len(shared) != 1 || shared[0].ContentHash != "sha256:own" {
		t.Errorf("entries at %q handed to the planner = %+v, want exactly the run harness's own entry",
			scopeSharedPath, shared)
	}
}

// scopeFlow runs one flow that loads the manifest against twoHarnessSnapshot, for the run
// harness, and returns the planner that captured the input the flow built.
type scopeFlow struct {
	name string
	run  func(t *testing.T) *capturingPlanner
}

// seedSnapshotFiles creates, under workspace, every file twoHarnessSnapshot records, so that
// reconciliation keeps all of its entries and the run sees them as present on disk.
func seedSnapshotFiles(t *testing.T, workspace string) {
	t.Helper()
	for _, e := range twoHarnessSnapshot().Manifest.Entries {
		full := filepath.Join(workspace, filepath.FromSlash(e.TargetPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create directory for %q: %v", e.TargetPath, err)
		}
		if err := os.WriteFile(full, []byte("deployed"), 0o644); err != nil {
			t.Fatalf("write %q: %v", e.TargetPath, err)
		}
	}
}

// newScopedFlowDeps returns base deps over the two-harness manifest with a capturing planner.
func newScopedFlowDeps(t *testing.T, planResult func(string) domain.Plan) (app.Deps, string, *capturingPlanner) {
	t.Helper()
	deps, workspace := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	seedSnapshotFiles(t, workspace)
	cp := &capturingPlanner{result: planResult(workspace)}
	deps.Planner = cp
	deps.Manifest = &stubManifestStore{snap: twoHarnessSnapshot()}
	return deps, workspace, cp
}

func scopeFlows() []scopeFlow {
	ctx := context.Background()
	return []scopeFlow{
		{"DeployNew", func(t *testing.T) *capturingPlanner {
			deps, ws, cp := newScopedFlowDeps(t, newMinimalPlan)
			if _, err := app.New(deps).DeployNew(ctx, newDeployRequest(ws, scopeRunHarness, []string{"quick-fix"})); err != nil {
				t.Fatalf("DeployNew: %v", err)
			}
			return cp
		}},
		{"DeployAgents", func(t *testing.T) *capturingPlanner {
			deps, ws, cp := newScopedFlowDeps(t, newMinimalPlan)
			_, err := app.New(deps).DeployAgents(ctx, app.DeployAgentsRequest{
				HarnessID: scopeRunHarness, WorkspacePath: ws, Scope: domain.ScopeProject,
				SubagentIDs: []string{}, UtilityAgentIDs: []string{},
				InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
				AutoConfirmPlan: true})
			if err != nil {
				t.Fatalf("DeployAgents: %v", err)
			}
			return cp
		}},
		{"DeployHooks", func(t *testing.T) *capturingPlanner {
			deps, ws, cp := newScopedFlowDeps(t, hooksPlan)
			deps.Catalog = catalogWithHookBundles()
			if _, err := app.New(deps).DeployHooks(ctx, newDeployHooksRequest(ws, scopeRunHarness)); err != nil {
				t.Fatalf("DeployHooks: %v", err)
			}
			return cp
		}},
		{"UpdateWorkflows", func(t *testing.T) *capturingPlanner {
			deps, ws, cp := newScopedFlowDeps(t, newMinimalPlan)
			_, err := app.New(deps).UpdateWorkflows(ctx, app.WorkflowUpdateRequest{
				HarnessID: scopeRunHarness, WorkspacePath: ws, WorkflowIDs: []string{"quick-fix"},
				AutoConfirmPlan: true})
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			return cp
		}},
		{"Update", func(t *testing.T) *capturingPlanner {
			deps, ws, cp := newUpdateDepsWithScanMembership(t, interactiontest.NewBuilder().Build())
			deps.Manifest = &stubManifestStore{snap: twoHarnessSnapshot()}
			seedSnapshotFiles(t, ws)
			_, err := app.New(deps).Update(ctx, app.UpdateRequest{
				HarnessID: scopeRunHarness, WorkspacePath: ws, AutoConfirmPlan: true})
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			return cp
		}},
	}
}

func TestFlowHarnessScope_EveryFlowHandsThePlannerOnlyTheRunHarnessEntries(t *testing.T) {
	for _, f := range scopeFlows() {
		t.Run(f.name, func(t *testing.T) {
			assertOnlyRunHarnessEntries(t, f.run(t))
		})
	}
}
