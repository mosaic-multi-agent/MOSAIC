package app_test

// ghcp_targets_held_test.go verifies that the test-only held-targets constructor really
// decides the refusal: the workspaces below carry no recorded agent that any detection rule
// could read, so only the forced held set can produce or withhold a refusal.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
)

// TestDeployFlows_HeldTargetsDecideRefusal covers each held set against each requested target.
func TestDeployFlows_HeldTargetsDecideRefusal(t *testing.T) {
	cases := []struct {
		name      string
		held      [2]bool
		requested string
		wantErr   error // nil means not refused
		present   []string
	}{
		{"cli held, cli requested", [2]bool{true, false}, ghcpCLIID, nil, nil},
		{"cli held, vscode requested", [2]bool{true, false}, ghcpVSCodeID, app.ErrGhcpOtherTarget, []string{ghcpCLIID}},
		{"vscode held, vscode requested", [2]bool{false, true}, ghcpVSCodeID, nil, nil},
		{"vscode held, cli requested", [2]bool{false, true}, ghcpCLIID, app.ErrGhcpOtherTarget, []string{ghcpVSCodeID}},
		{"both held, cli requested", [2]bool{true, true}, ghcpCLIID, app.ErrGhcpBothTargets, []string{ghcpCLIID, ghcpVSCodeID}},
		{"both held, vscode requested", [2]bool{true, true}, ghcpVSCodeID, app.ErrGhcpBothTargets, []string{ghcpCLIID, ghcpVSCodeID}},
	}
	for _, tc := range cases {
		for _, flow := range ghcpFlows() {
			t.Run(tc.name+"/"+flow.name, func(t *testing.T) {
				// Arrange: a harness_id-only manifest with no recorded agents.
				f := newGhcpFixtureHeld(t, nonGhcpHarness, &tc.held)

				// Act
				err := flow.run(f, tc.requested)

				// Assert
				if tc.wantErr == nil {
					var ce *app.GhcpTargetConflictError
					if errors.As(err, &ce) {
						t.Fatalf("%s for %s was refused despite held %v: %v", flow.name, tc.requested, tc.held, err)
					}
					return
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want one matching %v", err, tc.wantErr)
				}
				var ce *app.GhcpTargetConflictError
				if !errors.As(err, &ce) {
					t.Fatalf("error %v is not a *GhcpTargetConflictError", err)
				}
				if len(ce.Present) != len(tc.present) {
					t.Fatalf("Present = %+v, want %v", ce.Present, tc.present)
				}
				for i, id := range tc.present {
					if ce.Present[i].ID != id {
						t.Errorf("Present[%d] = %q, want %q", i, ce.Present[i].ID, id)
					}
				}
			})
		}
	}
}

// TestDeployFlows_HarnessOnlyManifestWithoutAgents_HoldsItsOwnTarget verifies that a manifest
// naming a GHCP harness and recording no agents is a single-target workspace, so the other
// target is refused.
func TestDeployFlows_HarnessOnlyManifestWithoutAgents_HoldsItsOwnTarget(t *testing.T) {
	for _, flow := range ghcpFlows() {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			f := newGhcpFixtureHeld(t, ghcpCLIID, nil)

			// Act
			err := assertGhcpRefusedUnchanged(t, f, flow, ghcpVSCodeID)

			// Assert
			if !errors.Is(err, app.ErrGhcpOtherTarget) {
				t.Fatalf("error %v does not match ErrGhcpOtherTarget", err)
			}
		})
	}
}
