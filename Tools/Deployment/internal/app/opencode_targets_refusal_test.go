package app_test

// opencode_targets_refusal_test.go verifies that every deploy-family flow refuses to put a
// second OpenCode target into a workspace, and refuses to run on a workspace already holding
// both. A refused run asks nothing, invokes no executor and leaves the workspace untouched. The
// refusal names Transform and the exact command to run.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/manifest"
)

// TestDeployFlows_OtherOpenCodeTargetPresent_Refused covers the five flows in both directions.
// Both targets record the same paths, so only the manifest entries tell the targets apart.
func TestDeployFlows_OtherOpenCodeTargetPresent_Refused(t *testing.T) {
	directions := []struct {
		name          string
		kind          ocWorkspaceKind
		requested     string
		presentID     string
		presentName   string
		requestedName string
	}{
		{"opencode-v2 onto opencode", ocOnlyV1, hOpenV2, hOpen, ocV1Name, ocV2Name},
		{"opencode onto opencode-v2", ocOnlyV2, hOpen, hOpenV2, ocV2Name, ocV1Name},
	}
	for _, d := range directions {
		for _, flow := range ghcpFlows() {
			t.Run(d.name+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newOCFixture(t, d.kind)

				// Act
				err := assertGhcpRefusedUnchanged(t, f, flow, d.requested)

				// Assert
				if !errors.Is(err, app.ErrOpenCodeOtherTarget) {
					t.Fatalf("error %v does not match ErrOpenCodeOtherTarget", err)
				}
				if errors.Is(err, app.ErrOpenCodeBothTargets) {
					t.Errorf("error %v must not match ErrOpenCodeBothTargets", err)
				}
				if errors.Is(err, app.ErrGhcpOtherTarget) || errors.Is(err, app.ErrGhcpBothTargets) {
					t.Errorf("error %v must not match a GHCP sentinel", err)
				}
				var ce *app.OpenCodeTargetConflictError
				if !errors.As(err, &ce) {
					t.Fatalf("error %v is not a *OpenCodeTargetConflictError", err)
				}
				if ce.Requested.ID != d.requested {
					t.Errorf("Requested.ID = %q, want %q", ce.Requested.ID, d.requested)
				}
				if len(ce.Present) != 1 || ce.Present[0].ID != d.presentID {
					t.Errorf("Present = %+v, want exactly %q", ce.Present, d.presentID)
				}
				msg := err.Error()
				want := []string{d.presentID, d.presentName, d.requested, d.requestedName, f.ws,
					ocTransformCommand(d.presentID, d.requested, f.ws)}
				for _, w := range want {
					if !strings.Contains(msg, w) {
						t.Errorf("message %q lacks %q", msg, w)
					}
				}
			})
		}
	}
}

// TestDeployFlows_BothOpenCodeTargetsHeld_Refused covers the five flows for both target ids with
// the service forced to report both targets.
func TestDeployFlows_BothOpenCodeTargetsHeld_Refused(t *testing.T) {
	for _, requested := range []string{hOpen, hOpenV2} {
		for _, flow := range ghcpFlows() {
			t.Run(requested+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newOCFixture(t, ocBothHeld)

				// Act
				err := assertGhcpRefusedUnchanged(t, f, flow, requested)

				// Assert
				assertOpenCodeBothRefusal(t, err)
			})
		}
	}
}

// TestDeployFlows_ManifestHoldingBothOpenCodeTargets_Refused verifies the both-present refusal
// from real manifest entries of both harnesses, without the test-only constructor.
func TestDeployFlows_ManifestHoldingBothOpenCodeTargets_Refused(t *testing.T) {
	for _, requested := range []string{hOpen, hOpenV2} {
		for _, flow := range ghcpFlows() {
			t.Run(requested+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newOCFixtureWith(t, manifest.StatePresent, ocManifest(hOpen, hOpenV2), nil)

				// Act
				err := assertGhcpRefusedUnchanged(t, f, flow, requested)

				// Assert
				assertOpenCodeBothRefusal(t, err)
			})
		}
	}
}

// assertOpenCodeBothRefusal checks the both-present error: type, sentinel, order and message.
func assertOpenCodeBothRefusal(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, app.ErrOpenCodeBothTargets) {
		t.Fatalf("error %v does not match ErrOpenCodeBothTargets", err)
	}
	if errors.Is(err, app.ErrOpenCodeOtherTarget) {
		t.Errorf("error %v must not match ErrOpenCodeOtherTarget", err)
	}
	var ce *app.OpenCodeTargetConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("error %v is not a *OpenCodeTargetConflictError", err)
	}
	if len(ce.Present) != 2 || ce.Present[0].ID != hOpen || ce.Present[1].ID != hOpenV2 {
		t.Errorf("Present = %+v, want [opencode opencode-v2] in that order", ce.Present)
	}
	msg := err.Error()
	for _, w := range []string{hOpen, hOpenV2, ocV1Name, ocV2Name, "by hand"} {
		if !strings.Contains(msg, w) {
			t.Errorf("message %q lacks %q", msg, w)
		}
	}
}

// TestDeployFlows_OpenCodeHeldTargetsDecideRefusal covers each held set against each requested
// target. The workspace records nothing for either OpenCode target, so only the forced set can
// produce or withhold the refusal.
func TestDeployFlows_OpenCodeHeldTargetsDecideRefusal(t *testing.T) {
	cases := []struct {
		name      string
		held      [2]bool
		requested string
		wantErr   error // nil means not refused
	}{
		{"v1 held, v1 requested", [2]bool{true, false}, hOpen, nil},
		{"v1 held, v2 requested", [2]bool{true, false}, hOpenV2, app.ErrOpenCodeOtherTarget},
		{"v2 held, v2 requested", [2]bool{false, true}, hOpenV2, nil},
		{"v2 held, v1 requested", [2]bool{false, true}, hOpen, app.ErrOpenCodeOtherTarget},
		{"none held, v1 requested", [2]bool{false, false}, hOpen, nil},
		{"both held, v1 requested", [2]bool{true, true}, hOpen, app.ErrOpenCodeBothTargets},
		{"both held, v2 requested", [2]bool{true, true}, hOpenV2, app.ErrOpenCodeBothTargets},
	}
	for _, tc := range cases {
		for _, flow := range ghcpFlows() {
			t.Run(tc.name+"/"+flow.name, func(t *testing.T) {
				// Arrange
				f := newOCFixtureWith(t, manifest.StatePresent, ocManifest(nonGhcpHarness), &tc.held)

				// Act
				err := flow.run(f, tc.requested)

				// Assert
				var ce *app.OpenCodeTargetConflictError
				if tc.wantErr == nil {
					if errors.As(err, &ce) {
						t.Fatalf("%s for %s was refused: %v", flow.name, tc.requested, err)
					}
					return
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error %v does not match %v", err, tc.wantErr)
				}
			})
		}
	}
}
