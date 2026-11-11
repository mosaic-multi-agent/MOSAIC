package app_test

// ghcp_targets_reconcile_test.go verifies the GHCP target check reads the reconciled manifest:
// a GHCP target whose recorded files are all gone (a manifest copied between workspaces, files
// deleted by hand) no longer counts as held, so it does not block the other GHCP target; a
// target that still has a file on disk keeps counting.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/manifest"
)

func removeWorkspaceFile(t *testing.T, ws, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(ws, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("remove %s: %v", rel, err)
	}
}

func TestGhcpTargets_TargetWhoseEveryRecordedFileIsGone_DoesNotBlockTheOtherTarget(t *testing.T) {
	cases := []struct {
		name      string
		held      string
		requested string
	}{
		{"ghcp-cli entries dropped, vscode-ghcp requested", ghcpCLIID, ghcpVSCodeID},
		{"vscode-ghcp entries dropped, ghcp-cli requested", ghcpVSCodeID, ghcpCLIID},
	}
	for _, tc := range cases {
		for _, flow := range ghcpFlows() {
			t.Run(tc.name+"/"+flow.name, func(t *testing.T) {
				store := manifest.NewStore()
				f := newFixtureOverRealManifest(t, t.TempDir(), store)
				runWorkspaceRuns(t, f.ws, store, workspaceRun{tc.held, ".github/agents/x.agent.md"})
				removeWorkspaceFile(t, f.ws, ".github/agents/x.agent.md")

				err := flow.run(f, tc.requested)

				var ce *app.GhcpTargetConflictError
				if errors.As(err, &ce) {
					t.Errorf("%s for %s was refused because of %v, although every recorded file of that target is gone: %v",
						flow.name, tc.requested, presentIDs(t, err), err)
				}
			})
		}
	}
}

func TestGhcpTargets_BothTargetsRecordedButOneIsGone_IsNotReportedAsBothHeld(t *testing.T) {
	for _, flow := range ghcpFlows() {
		t.Run(flow.name, func(t *testing.T) {
			store := manifest.NewStore()
			f := newFixtureOverRealManifest(t, t.TempDir(), store)
			runWorkspaceRuns(t, f.ws, store,
				workspaceRun{ghcpCLIID, ".github/agents/cli.agent.md"},
				workspaceRun{ghcpVSCodeID, ".github/agents/vscode.agent.md"})
			removeWorkspaceFile(t, f.ws, ".github/agents/cli.agent.md")

			err := flow.run(f, ghcpVSCodeID)

			if errors.Is(err, app.ErrGhcpBothTargets) {
				t.Errorf("%s reports both targets held although every file of ghcp-cli is gone: %v", flow.name, err)
			}
			var ce *app.GhcpTargetConflictError
			if errors.As(err, &ce) {
				t.Errorf("%s for the one target that still has files was refused: %v", flow.name, err)
			}
		})
	}
}

func TestGhcpTargets_TargetWithOneRecordedFileStillOnDisk_StillCountsAsHeld(t *testing.T) {
	for _, flow := range ghcpFlows() {
		t.Run(flow.name, func(t *testing.T) {
			store := manifest.NewStore()
			f := newFixtureOverRealManifest(t, t.TempDir(), store)
			runWorkspaceRuns(t, f.ws, store,
				workspaceRun{ghcpCLIID, ".github/agents/a.agent.md"},
				workspaceRun{ghcpCLIID, ".github/agents/b.agent.md"})
			removeWorkspaceFile(t, f.ws, ".github/agents/a.agent.md")

			err := assertGhcpRefusedUnchanged(t, f, flow, ghcpVSCodeID)

			if !errors.Is(err, app.ErrGhcpOtherTarget) {
				t.Fatalf("error %v does not match ErrGhcpOtherTarget", err)
			}
		})
	}
}
