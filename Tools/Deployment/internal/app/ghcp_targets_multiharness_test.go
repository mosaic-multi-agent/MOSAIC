package app_test

// ghcp_targets_multiharness_test.go verifies the GHCP target check over manifests written by
// real runs for several harnesses: a GHCP target stays recognised after a run for another
// harness, a second target is still refused, and holding both is still reported.

import (
	"context"
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/todo"
)

// workspaceRun is one executor run that records an agent for a harness.
type workspaceRun struct {
	harness string
	path    string
}

// runWorkspaceRuns executes the runs in order through the real executor and store, so the
// manifest on disk is what real runs for several harnesses leave behind.
func runWorkspaceRuns(t *testing.T, ws string, store manifest.Store, runs ...workspaceRun) {
	t.Helper()
	for _, r := range runs {
		plan := domain.Plan{
			Mode:          domain.ModeDeployWorkspace,
			Harness:       domain.HarnessRef{ID: r.harness},
			WorkspacePath: ws,
			Scope:         domain.ScopeProject,
			Items: []domain.PlanItem{{
				Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "x"},
				TargetPath: r.path,
				Action:     domain.ActionCreate,
			}},
		}
		req := deploy.ExecRequest{
			Plan:       plan,
			MosaicRoot: t.TempDir(),
			Content:    func(domain.PlanItem) ([]byte, error) { return []byte("# " + r.harness), nil },
		}
		if _, err := deploy.NewExecutor(store, &stubLogger{}, todo.NewCollector()).Execute(context.Background(), req); err != nil {
			t.Fatalf("run for %s: %v", r.harness, err)
		}
	}
}

// newFixtureOverRealManifest wires a service whose manifest store is the real one and whose
// registry knows both GHCP targets and one other harness, over the workspace ws.
func newFixtureOverRealManifest(t *testing.T, ws string, store manifest.Store) *ghcpFixture {
	t.Helper()
	stub := interactiontest.NewBuilder().Build()
	deps, _ := newBaseDeps(t, stub)
	cli := ghcpModule(ghcpCLIID, ghcpCLIName)
	vscode := ghcpModule(ghcpVSCodeID, ghcpVSCodeName)
	other := newMinimalModule()
	deps.Registry = &stubRegistry{
		list: []domain.HarnessRef{cli.ref, vscode.ref, other.ref},
		modules: map[string]domain.HarnessModule{
			ghcpCLIID: cli, ghcpVSCodeID: vscode, nonGhcpHarness: other},
	}
	deps.Manifest = store
	exec := &ghcpRecordingExecutor{ws: ws}
	deps.Executor = exec
	return &ghcpFixture{svc: app.New(deps), ws: ws, stub: stub, executor: exec}
}

func presentIDs(t *testing.T, err error) []string {
	t.Helper()
	var ce *app.GhcpTargetConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("error %v is not a *GhcpTargetConflictError", err)
	}
	var ids []string
	for _, p := range ce.Present {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestGhcpTargets_GhcpTargetStaysHeldAfterARunForAnotherHarness(t *testing.T) {
	cases := []struct {
		name      string
		held      workspaceRun
		requested string
	}{
		{"ghcp-cli held, vscode-ghcp requested", workspaceRun{ghcpCLIID, ".github/agents/x.agent.md"}, ghcpVSCodeID},
		{"vscode-ghcp held, ghcp-cli requested", workspaceRun{ghcpVSCodeID, ".github/agents/x.agent.md"}, ghcpCLIID},
	}
	for _, tc := range cases {
		for _, flow := range ghcpFlows() {
			t.Run(tc.name+"/"+flow.name, func(t *testing.T) {
				store := manifest.NewStore()
				f := newFixtureOverRealManifest(t, t.TempDir(), store)
				runWorkspaceRuns(t, f.ws, store, tc.held, workspaceRun{nonGhcpHarness, ".opencode/agents/x.md"})

				err := assertGhcpRefusedUnchanged(t, f, flow, tc.requested)

				if !errors.Is(err, app.ErrGhcpOtherTarget) {
					t.Fatalf("error %v does not match ErrGhcpOtherTarget", err)
				}
				if ids := presentIDs(t, err); len(ids) != 1 || ids[0] != tc.held.harness {
					t.Errorf("Present = %v, want only %s", ids, tc.held.harness)
				}
			})
		}
	}
}

func TestGhcpTargets_RunForTheHeldTargetOrAnotherHarnessIsNotRefusedAfterAMixedHistory(t *testing.T) {
	for _, requested := range []string{ghcpCLIID, nonGhcpHarness} {
		for _, flow := range ghcpFlows() {
			t.Run(requested+"/"+flow.name, func(t *testing.T) {
				store := manifest.NewStore()
				f := newFixtureOverRealManifest(t, t.TempDir(), store)
				runWorkspaceRuns(t, f.ws, store,
					workspaceRun{ghcpCLIID, ".github/agents/x.agent.md"},
					workspaceRun{nonGhcpHarness, ".opencode/agents/x.md"})

				err := flow.run(f, requested)

				var ce *app.GhcpTargetConflictError
				if errors.As(err, &ce) {
					t.Errorf("%s for %s was refused: %v", flow.name, requested, err)
				}
			})
		}
	}
}

func TestGhcpTargets_BothTargetsHeldIsStillReportedWhateverRanLast(t *testing.T) {
	for _, flow := range ghcpFlows() {
		t.Run(flow.name, func(t *testing.T) {
			store := manifest.NewStore()
			f := newFixtureOverRealManifest(t, t.TempDir(), store)
			runWorkspaceRuns(t, f.ws, store,
				workspaceRun{ghcpCLIID, ".github/agents/x.agent.md"},
				workspaceRun{ghcpVSCodeID, ".github/agents/x.agent.md"},
				workspaceRun{nonGhcpHarness, ".opencode/agents/x.md"})

			err := assertGhcpRefusedUnchanged(t, f, flow, ghcpCLIID)

			if !errors.Is(err, app.ErrGhcpBothTargets) {
				t.Fatalf("error %v does not match ErrGhcpBothTargets", err)
			}
			if ids := presentIDs(t, err); len(ids) != 2 || ids[0] != ghcpCLIID || ids[1] != ghcpVSCodeID {
				t.Errorf("Present = %v, want [ghcp-cli vscode-ghcp]", ids)
			}
		})
	}
}
