package app_test

// ghcp_targets_regression_test.go verifies that the GHCP target check leaves permitted runs
// alone: the same target again, workspaces without a GHCP target, and non-GHCP harnesses.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// TestDeployNew_PermittedGhcpStates_StillDeploys verifies the full deploy proceeds to the
// executor and succeeds in every state the check must not interfere with.
func TestDeployNew_PermittedGhcpStates_StillDeploys(t *testing.T) {
	cases := []struct {
		name      string
		kind      ghcpWorkspaceKind
		harnessID string
	}{
		{"ghcp-cli onto empty workspace", ghcpNone, ghcpCLIID},
		{"vscode-ghcp onto empty workspace", ghcpNone, ghcpVSCodeID},
		{"non-GHCP harness onto empty workspace", ghcpNone, nonGhcpHarness},
		{"ghcp-cli again onto ghcp-cli", ghcpOnlyCLI, ghcpCLIID},
		{"vscode-ghcp again onto vscode-ghcp", ghcpOnlyVSCode, ghcpVSCodeID},
		{"non-GHCP harness onto ghcp-cli workspace", ghcpOnlyCLI, nonGhcpHarness},
		{"non-GHCP harness onto vscode-ghcp workspace", ghcpOnlyVSCode, nonGhcpHarness},
		{"non-GHCP harness onto workspace holding both", ghcpBoth, nonGhcpHarness},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			f := newGhcpFixture(t, tc.kind)

			// Act
			err := ghcpFlows()[0].run(f, tc.harnessID)

			// Assert
			if err != nil {
				t.Fatalf("DeployNew for %s: unexpected error %v", tc.harnessID, err)
			}
			if !f.executor.called {
				t.Errorf("DeployNew for %s: executor was not invoked", tc.harnessID)
			}
		})
	}
}

// TestDeployFlows_PermittedGhcpStates_NotRefused verifies none of the five flows returns the
// GHCP refusal for runs the check must allow. The flows may fail for unrelated reasons in the
// stubbed environment; only the refusal is excluded.
func TestDeployFlows_PermittedGhcpStates_NotRefused(t *testing.T) {
	cases := []struct {
		name      string
		kind      ghcpWorkspaceKind
		harnessID string
	}{
		{"ghcp-cli onto empty workspace", ghcpNone, ghcpCLIID},
		{"ghcp-cli again onto ghcp-cli", ghcpOnlyCLI, ghcpCLIID},
		{"vscode-ghcp again onto vscode-ghcp", ghcpOnlyVSCode, ghcpVSCodeID},
		{"non-GHCP harness onto ghcp-cli workspace", ghcpOnlyCLI, nonGhcpHarness},
		{"non-GHCP harness onto workspace holding both", ghcpBoth, nonGhcpHarness},
	}
	for _, tc := range cases {
		for _, flow := range ghcpFlows() {
			t.Run(tc.name+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newGhcpFixture(t, tc.kind)

				// Act
				err := flow.run(f, tc.harnessID)

				// Assert
				var ce *app.GhcpTargetConflictError
				if errors.As(err, &ce) || errors.Is(err, app.ErrGhcpOtherTarget) || errors.Is(err, app.ErrGhcpBothTargets) {
					t.Errorf("%s for %s was refused as a GHCP conflict: %v", flow.name, tc.harnessID, err)
				}
			})
		}
	}
}

// TestDeployFlows_NoUsableManifest_NeverRefused verifies that a workspace with agent files on
// disk but no usable manifest holds no GHCP target, so no flow refuses either GHCP harness.
func TestDeployFlows_NoUsableManifest_NeverRefused(t *testing.T) {
	states := []manifest.State{manifest.StateAbsent, manifest.StateCorrupt}
	for _, state := range states {
		for _, id := range []string{ghcpCLIID, ghcpVSCodeID} {
			for _, flow := range ghcpFlows() {
				t.Run(string(state)+"/"+id+"/"+flow.name, func(t *testing.T) {
					// Arrange
					stub := interactiontest.NewBuilder().Build()
					deps, ws := newBaseDeps(t, stub)
					writeGhcpFile(t, ws, cliAgentRel)
					writeGhcpFile(t, ws, ".github/agents/z.agent.md")
					cli := ghcpModule(ghcpCLIID, ghcpCLIName)
					vscode := ghcpModule(ghcpVSCodeID, ghcpVSCodeName)
					deps.Registry = &stubRegistry{
						list:    []domain.HarnessRef{cli.ref, vscode.ref},
						modules: map[string]domain.HarnessModule{ghcpCLIID: cli, ghcpVSCodeID: vscode},
					}
					deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{State: state}}
					exec := &ghcpRecordingExecutor{ws: ws}
					deps.Executor = exec
					f := &ghcpFixture{svc: app.New(deps), ws: ws, stub: stub, executor: exec}

					// Act
					err := flow.run(f, id)

					// Assert
					var ce *app.GhcpTargetConflictError
					if errors.As(err, &ce) || errors.Is(err, app.ErrGhcpOtherTarget) || errors.Is(err, app.ErrGhcpBothTargets) {
						t.Errorf("%s for %s with %v manifest was refused as a GHCP conflict: %v", flow.name, id, state, err)
					}
				})
			}
		}
	}
}

// TestDeployNew_UnrecordedUserAgentFiles_DoNotBlockOtherTarget verifies that files MOSAIC
// did not deploy are not counted as a target: a workspace with only user-authored agent files
// accepts either GHCP target.
func TestDeployNew_UnrecordedUserAgentFiles_DoNotBlockOtherTarget(t *testing.T) {
	for _, id := range []string{ghcpCLIID, ghcpVSCodeID} {
		t.Run(id, func(t *testing.T) {
			// Arrange: ghcpNone fixture still contains an unrecorded .github/agents/mine.md.
			f := newGhcpFixture(t, ghcpNone)

			// Act
			err := ghcpFlows()[0].run(f, id)

			// Assert
			if err != nil {
				t.Fatalf("DeployNew for %s with user-authored agent files: %v", id, err)
			}
		})
	}
}
