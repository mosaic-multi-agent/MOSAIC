package app_test

// ghcp_targets_agentnames_test.go verifies that the deploy flows decide which GHCP target a
// workspace holds from the manifest harness_id alone. Whatever the recorded agents are called
// (X.md, X.agent.md or a mix), the workspace holds only the target its manifest names.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
)

type ghcpAgentNaming struct {
	name   string
	agents []string
}

func ghcpAgentNamings() []ghcpAgentNaming {
	return []ghcpAgentNaming{
		{"X.md agents", []string{".github/agents/a.md", ".github/agents/b.md"}},
		{"X.agent.md agents", []string{".github/agents/a.agent.md", ".github/agents/b.agent.md"}},
		{"mixed agents", []string{".github/agents/a.md", ".github/agents/b.agent.md"}},
	}
}

// ghcpHeldCases pairs each target a workspace can be deployed for with the other target.
func ghcpHeldCases() []struct{ held, other string } {
	return []struct{ held, other string }{
		{ghcpCLIID, ghcpVSCodeID},
		{ghcpVSCodeID, ghcpCLIID},
	}
}

// TestDeployNew_SameTargetWorkspace_ProceedsWhateverTheAgentNames verifies a full deploy for
// the target the workspace holds reaches the executor, for both targets and every naming.
func TestDeployNew_SameTargetWorkspace_ProceedsWhateverTheAgentNames(t *testing.T) {
	for _, c := range ghcpHeldCases() {
		for _, n := range ghcpAgentNamings() {
			t.Run(c.held+"/"+n.name, func(t *testing.T) {
				// Arrange
				f := newGhcpFixtureFor(t, c.held, false, n.agents...)

				// Act
				err := ghcpFlows()[0].run(f, c.held)

				// Assert
				if err != nil {
					t.Fatalf("DeployNew for %s on a %s workspace with %s: %v", c.held, c.held, n.name, err)
				}
				if !f.executor.called {
					t.Errorf("DeployNew for %s: executor was not invoked", c.held)
				}
			})
		}
	}
}

// TestDeployFlows_SameTargetWorkspace_NotRefusedWhateverTheAgentNames covers all five flows.
func TestDeployFlows_SameTargetWorkspace_NotRefusedWhateverTheAgentNames(t *testing.T) {
	for _, c := range ghcpHeldCases() {
		for _, n := range ghcpAgentNamings() {
			for _, flow := range ghcpFlows() {
				t.Run(c.held+"/"+n.name+"/"+flow.name, func(t *testing.T) {
					// Arrange
					f := newGhcpFixtureFor(t, c.held, false, n.agents...)

					// Act
					err := flow.run(f, c.held)

					// Assert
					var ce *app.GhcpTargetConflictError
					if errors.As(err, &ce) || errors.Is(err, app.ErrGhcpOtherTarget) || errors.Is(err, app.ErrGhcpBothTargets) {
						t.Errorf("%s for %s was refused as a GHCP conflict: %v", flow.name, c.held, err)
					}
				})
			}
		}
	}
}

// TestDeployFlows_OtherTargetOnWorkspace_RefusedWhateverTheAgentNames verifies the run for the
// other target is refused as "other target" (never "both"), asks nothing and changes nothing.
func TestDeployFlows_OtherTargetOnWorkspace_RefusedWhateverTheAgentNames(t *testing.T) {
	for _, c := range ghcpHeldCases() {
		for _, n := range ghcpAgentNamings() {
			for _, flow := range ghcpFlows() {
				t.Run(c.held+"/"+n.name+"/"+flow.name, func(t *testing.T) {
					// Arrange
					f := newGhcpFixtureFor(t, c.held, false, n.agents...)

					// Act
					err := assertGhcpRefusedUnchanged(t, f, flow, c.other)

					// Assert
					if !errors.Is(err, app.ErrGhcpOtherTarget) {
						t.Fatalf("error %v does not match ErrGhcpOtherTarget", err)
					}
					if errors.Is(err, app.ErrGhcpBothTargets) {
						t.Errorf("error %v must not match ErrGhcpBothTargets", err)
					}
					var ce *app.GhcpTargetConflictError
					if !errors.As(err, &ce) || len(ce.Present) != 1 || ce.Present[0].ID != c.held {
						t.Errorf("conflict = %+v, want exactly %q present", ce, c.held)
					}
				})
			}
		}
	}
}
