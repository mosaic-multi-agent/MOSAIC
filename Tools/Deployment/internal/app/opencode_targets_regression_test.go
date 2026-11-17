package app_test

// opencode_targets_regression_test.go verifies that the OpenCode target check leaves permitted
// runs alone (the same target again, no OpenCode target, other harnesses including the GHCP
// pair) and that only manifest entries decide the held target: files at the shared .opencode
// paths, unrecorded user files and an unusable manifest never do.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// notOpenCodeRefusal fails the test when err is an OpenCode refusal.
func notOpenCodeRefusal(t *testing.T, what string, err error) {
	t.Helper()
	var ce *app.OpenCodeTargetConflictError
	if errors.As(err, &ce) || errors.Is(err, app.ErrOpenCodeOtherTarget) || errors.Is(err, app.ErrOpenCodeBothTargets) {
		t.Errorf("%s was refused as an OpenCode conflict: %v", what, err)
	}
}

// TestDeployFlows_PermittedOpenCodeStates_NotRefused covers runs the check must allow.
func TestDeployFlows_PermittedOpenCodeStates_NotRefused(t *testing.T) {
	cases := []struct {
		name      string
		kind      ocWorkspaceKind
		harnessID string
	}{
		{"opencode onto empty workspace", ocNone, hOpen},
		{"opencode-v2 onto empty workspace", ocNone, hOpenV2},
		{"opencode again onto opencode", ocOnlyV1, hOpen},
		{"opencode-v2 again onto opencode-v2", ocOnlyV2, hOpenV2},
		{"other harness onto opencode workspace", ocOnlyV1, nonGhcpHarness},
		{"other harness onto opencode-v2 workspace", ocOnlyV2, nonGhcpHarness},
		{"other harness onto workspace holding both", ocBothHeld, nonGhcpHarness},
		{"ghcp-cli onto workspace holding both", ocBothHeld, ghcpCLIID},
		{"vscode-ghcp onto opencode workspace", ocOnlyV1, ghcpVSCodeID},
	}
	for _, tc := range cases {
		for _, flow := range ghcpFlows() {
			t.Run(tc.name+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newOCFixture(t, tc.kind)

				// Act
				err := flow.run(f, tc.harnessID)

				// Assert
				notOpenCodeRefusal(t, flow.name+" for "+tc.harnessID, err)
			})
		}
	}
}

// TestDeployNew_PermittedOpenCodeStates_StillDeploys verifies the full deploy reaches the
// executor and succeeds where the check must not interfere.
func TestDeployNew_PermittedOpenCodeStates_StillDeploys(t *testing.T) {
	cases := []struct {
		name      string
		kind      ocWorkspaceKind
		harnessID string
	}{
		{"opencode onto empty workspace", ocNone, hOpen},
		{"opencode-v2 onto empty workspace", ocNone, hOpenV2},
		{"opencode again onto opencode", ocOnlyV1, hOpen},
		{"opencode-v2 again onto opencode-v2", ocOnlyV2, hOpenV2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			f := newOCFixture(t, tc.kind)

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

// TestDeployFlows_NoUsableManifest_OpenCodeNeverRefused verifies that a workspace with
// OpenCode-looking files on disk but no usable manifest holds no OpenCode target.
func TestDeployFlows_NoUsableManifest_OpenCodeNeverRefused(t *testing.T) {
	for _, state := range []manifest.State{manifest.StateAbsent, manifest.StateCorrupt} {
		for _, id := range []string{hOpen, hOpenV2} {
			for _, flow := range ghcpFlows() {
				t.Run(string(state)+"/"+id+"/"+flow.name, func(t *testing.T) {
					// Arrange: entries that a usable manifest would hold are served with the bad state.
					f := newOCFixtureWith(t, state, ocManifest(hOpen, hOpenV2), nil)

					// Act
					err := flow.run(f, id)

					// Assert
					notOpenCodeRefusal(t, flow.name+" for "+id+" with a "+string(state)+" manifest", err)
				})
			}
		}
	}
}

// TestDeployFlows_OpenCodeFilesOfAnotherHarnessEntry_DoNotMakeATarget verifies that a manifest
// naming another harness never makes the workspace hold an OpenCode target, whatever sits at the
// shared .opencode paths.
func TestDeployFlows_OpenCodeFilesOfAnotherHarnessEntry_DoNotMakeATarget(t *testing.T) {
	for _, id := range []string{hOpen, hOpenV2} {
		for _, flow := range ghcpFlows() {
			t.Run(id+"/"+flow.name, func(t *testing.T) {
				// Arrange: files at the OpenCode paths, recorded under an unrelated harness.
				f := newOCFixtureWith(t, manifest.StatePresent, ocManifest("claude-code"), nil)

				// Act
				err := flow.run(f, id)

				// Assert
				notOpenCodeRefusal(t, flow.name+" for "+id, err)
			})
		}
	}
}

// TestDeployFlows_RecordedEntryOfAnyKindMakesTheTarget verifies that the harness of a recorded
// entry decides, whatever its kind and path: a lone skill or hook entry of one OpenCode target
// refuses the other, and an entry path that looks like the other target's changes nothing.
func TestDeployFlows_RecordedEntryOfAnyKindMakesTheTarget(t *testing.T) {
	entries := []struct {
		name  string
		entry domain.ManifestEntry
	}{
		{"skill entry", domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "anchor"}, TargetPath: ocSkillRel}},
		{"hook entry", domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "mosaic-logger"}, TargetPath: ocPluginRel}},
		{"agent entry", domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "x"}, TargetPath: ocAgentRel}},
	}
	for _, e := range entries {
		for _, flow := range ghcpFlows() {
			t.Run(e.name+"/"+flow.name, func(t *testing.T) {
				// Arrange: only opencode-v2 records an entry; the run is for opencode.
				entry := e.entry
				entry.Harness = hOpenV2
				m := domain.Manifest{Harnesses: []domain.HarnessSection{{HarnessID: hOpenV2}}, Entries: []domain.ManifestEntry{entry}}
				f := newOCFixtureWith(t, manifest.StatePresent, m, nil)

				// Act
				err := assertGhcpRefusedUnchanged(t, f, flow, hOpen)

				// Assert
				if !errors.Is(err, app.ErrOpenCodeOtherTarget) {
					t.Fatalf("error %v does not match ErrOpenCodeOtherTarget", err)
				}
			})
		}
	}
}

// TestDeployFlows_GhcpCheckIsIndependentOfTheOpenCodeCheck verifies that holding both OpenCode
// targets never produces a GHCP refusal and holding both GHCP targets never an OpenCode one.
func TestDeployFlows_GhcpCheckIsIndependentOfTheOpenCodeCheck(t *testing.T) {
	t.Run("both opencode held, ghcp requested", func(t *testing.T) {
		f := newOCFixture(t, ocBothHeld)

		err := ghcpFlows()[0].run(f, ghcpCLIID)

		if errors.Is(err, app.ErrGhcpOtherTarget) || errors.Is(err, app.ErrGhcpBothTargets) {
			t.Errorf("GHCP refusal for an OpenCode workspace: %v", err)
		}
		notOpenCodeRefusal(t, "ghcp-cli deploy", err)
	})
	t.Run("both ghcp held, opencode requested", func(t *testing.T) {
		f := newGhcpFixture(t, ghcpBoth)

		err := ghcpFlows()[0].run(f, hOpen)

		notOpenCodeRefusal(t, "opencode deploy on a GHCP workspace", err)
	})
}
