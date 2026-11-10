package app_test

// system_file_no_fallback_test.go verifies the no-fallback rule: when no layer of the resolved
// stack supplies the protocol or the bundle, the run stops before a plan is built and before
// anything is written, and the error names the missing file. The MOSAIC default catalog is
// present on disk in every case but is not a layer of the stack, so a loader that falls back
// to it would let the run through.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/systemfile"
)

// noFallbackOutcome is what a run left behind.
type noFallbackOutcome struct {
	err          error
	plannerCalls int
	executed     bool
	wroteOutput  bool
}

// noFallbackOp runs one app operation against a stack whose only layer is userRoot, and
// reports what happened. mosaicRoot holds a complete default catalog that is not in the stack.
type noFallbackOp struct {
	name string
	run  func(t *testing.T, mosaicRoot, userRoot string) noFallbackOutcome
}

func noFallbackOps() []noFallbackOp {
	return []noFallbackOp{
		{"DeployNew", runNoFallbackDeploy},
		{"Update", runNoFallbackUpdate},
		{"UpdateWorkflows", runNoFallbackUpdateWorkflows},
		{"RenderAgent", runNoFallbackRender},
	}
}

func runNoFallbackDeploy(t *testing.T, mosaicRoot, userRoot string) noFallbackOutcome {
	deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
	planner := &spyPlanner{response: newMinimalPlan(ws)}
	exec := &protocolTestExecutorSpy{result: newMinimalExecResult(ws)}
	deps.Planner, deps.Executor = planner, exec
	useResolvedStack(t, &deps, mosaicRoot, stack.Layer{Root: userRoot})
	_, err := app.New(deps).DeployNew(context.Background(), newDeployRequest(ws, "stub-harness", []string{"quick-fix"}))
	return noFallbackOutcome{err: err, plannerCalls: len(planner.capturedInputs), executed: exec.called}
}

func runNoFallbackUpdate(t *testing.T, mosaicRoot, userRoot string) noFallbackOutcome {
	deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
	planner := &spyPlanner{response: newMinimalPlan(ws)}
	exec := &protocolTestExecutorSpy{result: newMinimalExecResult(ws)}
	deps.Planner, deps.Executor = planner, exec
	useResolvedStack(t, &deps, mosaicRoot, stack.Layer{Root: userRoot})
	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, AutoConfirmPlan: true,
	})
	return noFallbackOutcome{err: err, plannerCalls: len(planner.capturedInputs), executed: exec.called}
}

func runNoFallbackUpdateWorkflows(t *testing.T, mosaicRoot, userRoot string) noFallbackOutcome {
	deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
	planner := &spyPlanner{response: newMinimalPlan(ws)}
	exec := &protocolTestExecutorSpy{result: newMinimalExecResult(ws)}
	deps.Planner, deps.Executor = planner, exec
	useResolvedStack(t, &deps, mosaicRoot, stack.Layer{Root: userRoot})
	_, err := app.New(deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: true,
	})
	return noFallbackOutcome{err: err, plannerCalls: len(planner.capturedInputs), executed: exec.called}
}

func runNoFallbackRender(t *testing.T, mosaicRoot, userRoot string) noFallbackOutcome {
	deps := newRenderDeps(t)
	useResolvedStack(t, &deps, mosaicRoot, stack.Layer{Root: userRoot})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-agent.md", minimalGenericSubagentBytes())
	dest := filepath.Join(dir, "out.md")
	_, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, dest))
	_, statErr := os.Stat(dest)
	return noFallbackOutcome{err: err, wroteOutput: statErr == nil}
}

// TestSystemFiles_NotSuppliedByStack_StopsRunBeforeWriting verifies, per operation and per
// missing file, that a stack with no supplier stops the run with an error that identifies
// the missing file, with no plan built and nothing written.
func TestSystemFiles_NotSuppliedByStack_StopsRunBeforeWriting(t *testing.T) {
	cases := []struct {
		missing string
		user    sysLayer
	}{
		{catalogpaths.RelProtocolFile, sysLayer{id: "user-a", bundleVersion: "3.0.0"}},
		{catalogpaths.RelBundleFile, sysLayer{id: "user-a", protocolVersion: "3.0"}},
	}
	for _, op := range noFallbackOps() {
		for _, c := range cases {
			t.Run(op.name+"/missing "+c.missing, func(t *testing.T) {
				mosaic := newSysMosaicRoot(t, sysLayer{id: "mosaic-default", protocolVersion: "1.0", bundleVersion: "1.0.0"})
				user := newSysUserLayer(t, c.user)

				out := op.run(t, mosaic, user)

				if out.err == nil {
					t.Fatalf("%s returned nil error for a stack that does not supply %s; "+
						"there is no fallback to the default catalog", op.name, c.missing)
				}
				var notSupplied *systemfile.NotSuppliedError
				if !errors.As(out.err, &notSupplied) {
					t.Errorf("error = %v; want one for which errors.As(*systemfile.NotSuppliedError) holds", out.err)
				}
				if !strings.Contains(out.err.Error(), c.missing) {
					t.Errorf("error %q does not name the missing file %s", out.err.Error(), c.missing)
				}
				if out.plannerCalls != 0 {
					t.Errorf("planner was called %d time(s); the run must stop before any plan is built", out.plannerCalls)
				}
				if out.executed || out.wroteOutput {
					t.Error("the run wrote output despite the missing system file; nothing may be written")
				}
			})
		}
	}
}

// TestSystemFiles_SuppliedOnlyByLowerLayer_RunProceeds verifies the other side of the rule:
// a file that a lower layer supplies is enough, so the run does not stop. It guards the
// no-fallback tests against passing merely because every run fails.
func TestSystemFiles_SuppliedOnlyByLowerLayer_RunProceeds(t *testing.T) {
	mosaic := newSysMosaicRoot(t, sysLayer{id: "mosaic-default", protocolVersion: "1.0", bundleVersion: "1.0.0"})
	high := newSysUserLayer(t, sysLayer{id: "user-high"})
	low := newSysUserLayer(t, sysLayer{id: "user-low", protocolVersion: "3.0", bundleVersion: "3.0.0"})
	deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
	planner := &spyPlanner{response: newMinimalPlan(ws)}
	deps.Planner = planner
	useResolvedStack(t, &deps, mosaic, stack.Layer{Root: high}, stack.Layer{Root: low})

	_, err := app.New(deps).DeployNew(context.Background(), newDeployRequest(ws, "stub-harness", []string{"quick-fix"}))

	var notSupplied *systemfile.NotSuppliedError
	if errors.As(err, &notSupplied) {
		t.Fatalf("DeployNew failed with %v although the lower layer supplies both files", err)
	}
	if len(planner.capturedInputs) == 0 {
		t.Fatalf("planner was never called (err = %v); the run must proceed on the lower layer's files", err)
	}
}
