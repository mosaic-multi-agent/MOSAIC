package app_test

// ghcp_targets_refusal_test.go verifies that every deploy flow refuses to put a second GHCP
// target into a workspace, and refuses to run on a workspace already holding both. A refused
// run asks nothing, invokes no executor and leaves the workspace untouched.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

// assertGhcpRefusedUnchanged runs one flow and checks the refusal invariants shared by all
// refusal cases: error returned, executor idle, no question asked, workspace identical.
func assertGhcpRefusedUnchanged(t *testing.T, f *ghcpFixture, flow ghcpFlow, harnessID string) error {
	t.Helper()
	before := captureFS(t, f.ws)

	err := flow.run(f, harnessID)

	if err == nil {
		t.Fatalf("%s for %s: expected a GHCP refusal error, got nil", flow.name, harnessID)
	}
	if f.executor.called {
		t.Errorf("%s: executor was invoked for a refused run", flow.name)
	}
	if order := f.stub.QuestionOrder(); len(order) != 0 {
		t.Errorf("%s: refused run asked questions %v; the refusal must come before any question", flow.name, order)
	}
	if changed, desc := fsChanged(before, captureFS(t, f.ws)); changed {
		t.Errorf("%s: workspace changed after refusal (%s)", flow.name, desc)
	}
	return err
}

// TestDeployFlows_OtherGhcpTargetPresent_Refused covers the five flows for each direction.
func TestDeployFlows_OtherGhcpTargetPresent_Refused(t *testing.T) {
	directions := []struct {
		name          string
		kind          ghcpWorkspaceKind
		requested     string
		presentID     string
		presentName   string
		requestedName string
	}{
		{"vscode-ghcp onto ghcp-cli", ghcpOnlyCLI, ghcpVSCodeID, ghcpCLIID, ghcpCLIName, ghcpVSCodeName},
		{"ghcp-cli onto vscode-ghcp", ghcpOnlyVSCode, ghcpCLIID, ghcpVSCodeID, ghcpVSCodeName, ghcpCLIName},
	}
	for _, d := range directions {
		for _, flow := range ghcpFlows() {
			t.Run(d.name+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newGhcpFixture(t, d.kind)

				// Act
				err := assertGhcpRefusedUnchanged(t, f, flow, d.requested)

				// Assert
				if !errors.Is(err, app.ErrGhcpOtherTarget) {
					t.Fatalf("error %v does not match ErrGhcpOtherTarget", err)
				}
				if errors.Is(err, app.ErrGhcpBothTargets) {
					t.Errorf("error %v must not match ErrGhcpBothTargets", err)
				}
				var ce *app.GhcpTargetConflictError
				if !errors.As(err, &ce) {
					t.Fatalf("error %v is not a *GhcpTargetConflictError", err)
				}
				if ce.Requested.ID != d.requested {
					t.Errorf("Requested.ID = %q, want %q", ce.Requested.ID, d.requested)
				}
				if len(ce.Present) != 1 || ce.Present[0].ID != d.presentID {
					t.Errorf("Present = %+v, want exactly %q", ce.Present, d.presentID)
				}
				msg := err.Error()
				for _, want := range []string{d.presentID, d.presentName, d.requested, d.requestedName, "transform", f.ws} {
					if !strings.Contains(msg, want) {
						t.Errorf("message %q lacks %q", msg, want)
					}
				}
			})
		}
	}
}

// TestDeployFlows_BothGhcpTargetsPresent_Refused covers the five flows for both target ids.
// No manifest can express "both present", so the service is built with the test-only
// constructor that forces both targets.
func TestDeployFlows_BothGhcpTargetsPresent_Refused(t *testing.T) {
	for _, requested := range []string{ghcpCLIID, ghcpVSCodeID} {
		for _, flow := range ghcpFlows() {
			t.Run(requested+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newGhcpFixture(t, ghcpBoth)

				// Act
				err := assertGhcpRefusedUnchanged(t, f, flow, requested)

				// Assert
				if !errors.Is(err, app.ErrGhcpBothTargets) {
					t.Fatalf("error %v does not match ErrGhcpBothTargets", err)
				}
				if errors.Is(err, app.ErrGhcpOtherTarget) {
					t.Errorf("error %v must not match ErrGhcpOtherTarget", err)
				}
				var ce *app.GhcpTargetConflictError
				if !errors.As(err, &ce) {
					t.Fatalf("error %v is not a *GhcpTargetConflictError", err)
				}
				if len(ce.Present) != 2 || ce.Present[0].ID != ghcpCLIID || ce.Present[1].ID != ghcpVSCodeID {
					t.Errorf("Present = %+v, want [ghcp-cli vscode-ghcp] in that order", ce.Present)
				}
				msg := err.Error()
				for _, want := range []string{ghcpCLIID, ghcpVSCodeID, ghcpCLIName, ghcpVSCodeName, "by hand"} {
					if !strings.Contains(msg, want) {
						t.Errorf("message %q lacks %q", msg, want)
					}
				}
			})
		}
	}
}
