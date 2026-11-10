package app_test

// system_file_resolved_layer_test.go verifies that the protocol and the bundle a run parses
// are the whole files of the layer the stack resolved, and that the versions the planner
// compares deployed files against are the resolved files' versions. The planner's own
// comparison is unchanged; what is specified here is the input it receives.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/stack"
)

// plannerInputOps run an operation that builds a plan and return the planner's input.
func plannerInputOps() map[string]func(t *testing.T, mosaicRoot string, layers ...stack.Layer) captured {
	return map[string]func(t *testing.T, mosaicRoot string, layers ...stack.Layer) captured{
		"DeployNew": func(t *testing.T, mosaicRoot string, layers ...stack.Layer) captured {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
			spy := &spyPlanner{response: newMinimalPlan(ws)}
			deps.Planner = spy
			useResolvedStack(t, &deps, mosaicRoot, layers...)
			_, _ = app.New(deps).DeployNew(context.Background(), newDeployRequest(ws, "stub-harness", []string{"quick-fix"}))
			return captured{spy.lastCaptured(t).ProtocolVersion, spy.lastCaptured(t).BundleVersion}
		},
		"Update": func(t *testing.T, mosaicRoot string, layers ...stack.Layer) captured {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
			spy := &spyPlanner{response: newMinimalPlan(ws)}
			deps.Planner = spy
			useResolvedStack(t, &deps, mosaicRoot, layers...)
			_, _ = app.New(deps).Update(context.Background(), app.UpdateRequest{
				HarnessID: "stub-harness", WorkspacePath: ws, AutoConfirmPlan: true,
			})
			return captured{spy.lastCaptured(t).ProtocolVersion, spy.lastCaptured(t).BundleVersion}
		},
		"UpdateWorkflows": func(t *testing.T, mosaicRoot string, layers ...stack.Layer) captured {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
			spy := &spyPlanner{response: newMinimalPlan(ws)}
			deps.Planner = spy
			useResolvedStack(t, &deps, mosaicRoot, layers...)
			_, _ = app.New(deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
				HarnessID: "stub-harness", WorkspacePath: ws, WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: true,
			})
			return captured{spy.lastCaptured(t).ProtocolVersion, spy.lastCaptured(t).BundleVersion}
		},
	}
}

// captured is the pair of versions a planner received.
type captured struct{ protocol, bundle string }

// TestSystemFiles_UserLayerAboveDefault_PlannerComparesAgainstUserFiles verifies that with a
// user layer and the default layer both supplying the protocol and the bundle, the planner is
// given the user layer's versions: deployed files are judged against the replacement files.
func TestSystemFiles_UserLayerAboveDefault_PlannerComparesAgainstUserFiles(t *testing.T) {
	for name, run := range plannerInputOps() {
		t.Run(name, func(t *testing.T) {
			mosaic := newSysMosaicRoot(t, sysLayer{id: "mosaic-default", protocolVersion: "1.0", bundleVersion: "1.0.0"})
			user := newSysUserLayer(t, sysLayer{id: "user-a", protocolVersion: "3.0", bundleVersion: "3.0.0"})

			got := run(t, mosaic, stack.Layer{Root: user}, defaultStackLayer(mosaic))

			if got.protocol != "3.0" {
				t.Errorf("planner ProtocolVersion = %q, want %q (the user layer's protocol)", got.protocol, "3.0")
			}
			if got.bundle != "3.0.0" {
				t.Errorf("planner BundleVersion = %q, want %q (the user layer's bundle)", got.bundle, "3.0.0")
			}
		})
	}
}

// TestSystemFiles_EachFileResolvesIndependently verifies that the protocol and the bundle are
// each taken whole from their own highest supplier: a user layer that replaces only the
// protocol leaves the bundle to the default layer.
func TestSystemFiles_EachFileResolvesIndependently(t *testing.T) {
	for name, run := range plannerInputOps() {
		t.Run(name, func(t *testing.T) {
			mosaic := newSysMosaicRoot(t, sysLayer{id: "mosaic-default", protocolVersion: "1.0", bundleVersion: "1.0.0"})
			user := newSysUserLayer(t, sysLayer{id: "user-a", protocolVersion: "3.0"})

			got := run(t, mosaic, stack.Layer{Root: user}, defaultStackLayer(mosaic))

			if got.protocol != "3.0" || got.bundle != "1.0.0" {
				t.Errorf("planner versions = %+v, want protocol 3.0 from the user layer and bundle 1.0.0 from the default layer", got)
			}
		})
	}
}

// TestSystemFiles_DefaultOnlyStack_PlannerComparesAgainstDefaultFiles verifies the case with
// no user catalog: the default layer supplies both files, as it did before stacks.
func TestSystemFiles_DefaultOnlyStack_PlannerComparesAgainstDefaultFiles(t *testing.T) {
	for name, run := range plannerInputOps() {
		t.Run(name, func(t *testing.T) {
			mosaic := newSysMosaicRoot(t, sysLayer{id: "mosaic-default", protocolVersion: "1.0", bundleVersion: "1.0.0"})

			got := run(t, mosaic, defaultStackLayer(mosaic))

			if got.protocol != "1.0" || got.bundle != "1.0.0" {
				t.Errorf("planner versions = %+v, want the default layer's 1.0 and 1.0.0", got)
			}
		})
	}
}

// TestSystemFiles_RenderUsesTheResolvedProtocolWhole verifies that the protocol block written
// into a rendered agent comes from the resolved layer's file: the user layer's version marker
// and block text appear, and nothing of the default layer's protocol does.
func TestSystemFiles_RenderUsesTheResolvedProtocolWhole(t *testing.T) {
	mosaic := newSysMosaicRoot(t, sysLayer{id: "mosaic-default", protocolVersion: "1.0", bundleVersion: "1.0.0"})
	user := newSysUserLayer(t, sysLayer{id: "user-a", protocolVersion: "3.0", bundleVersion: "3.0.0"})
	deps := newRenderDeps(t)
	useResolvedStack(t, &deps, mosaic, stack.Layer{Root: user}, defaultStackLayer(mosaic))
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-agent.md", minimalGenericSubagentBytes())
	dest := filepath.Join(dir, "out.md")

	_, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, dest))

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	out, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("read rendered output: %v", readErr)
	}
	text := string(out)
	if !strings.Contains(text, `version="3.0"`) || !strings.Contains(text, "Subagent protocol text from user-a") {
		t.Errorf("rendered output lacks the user layer's protocol (version 3.0 and its block text):\n%s", text)
	}
	if strings.Contains(text, "mosaic-default") || strings.Contains(text, `version="1.0"`) {
		t.Errorf("rendered output carries content of the default layer's protocol:\n%s", text)
	}
}
