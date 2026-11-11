package app_test

// transform_ghcp_pair_refusal_test.go specifies that a transform between the two GHCP targets
// is refused, before anything is asked or written, when the workspace holds both targets. The
// message tells the user to remove one target by hand.

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// trSaveManifest records the harness and agent files (workspace-relative, key = file stem) in the
// workspace manifest.
func trSaveManifest(t *testing.T, ws, harnessID string, agentFiles ...string) {
	t.Helper()
	m := domain.Manifest{SchemaVersion: manifest.SchemaVersion, Harnesses: []domain.HarnessSection{{HarnessID: harnessID}}}
	for _, rel := range agentFiles {
		key := rel[strings.LastIndex(rel, "/")+1:]
		key = strings.TrimSuffix(strings.TrimSuffix(key, ".agent.md"), ".md")
		m.Entries = append(m.Entries, domain.ManifestEntry{
			Harness:    harnessID,
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
			TargetPath: rel,
		})
	}
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
}

// trBothGhcpRun is a pair run over a workspace holding a recorded source agent whose service
// is built with the test-only constructor that reports both GHCP targets for every workspace.
// No manifest can express "both present".
func trBothGhcpRun(t *testing.T, src, tgt string) *trRun {
	t.Helper()
	r := trCarryRun(t, src, tgt, nil)
	rel := r.put(src, "x", trFixture(t, "subagent"))
	trSaveManifest(t, r.ws, src, rel)
	r.svc = newHeldGhcpService(t, r.deps, true, true)
	return r
}

func TestTransformGhcpPair_WorkspaceHoldingBothTargetsIsRefusedBeforeAnythingIsAskedOrWritten(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		for _, dry := range []bool{false, true} {
			name := pair[0] + "->" + pair[1]
			if dry {
				name += " dry-run"
			}
			t.Run(name, func(t *testing.T) {
				r := trBothGhcpRun(t, pair[0], pair[1])
				before := r.snapshot()
				req := r.request()
				req.DryRun = dry
				req.AutoConfirmPlan = false

				res, err := r.run(req)

				if !errors.Is(err, app.ErrGhcpBothTargets) {
					t.Fatalf("error = %v, want one matching ErrGhcpBothTargets", err)
				}
				var conflict *app.GhcpTargetConflictError
				if !errors.As(err, &conflict) || conflict.Requested.ID != pair[1] || len(conflict.Present) != 2 {
					t.Errorf("conflict = %+v, want Requested %s and both targets present", conflict, pair[1])
				}
				if len(r.spy.choices) != 0 || len(r.spy.texts) != 0 || len(r.spy.reviews) != 0 {
					t.Errorf("refused run asked %d choices, %d texts, %d reviews; want none",
						len(r.spy.choices), len(r.spy.texts), len(r.spy.reviews))
				}
				if diff := trSameTree(before, r.snapshot()); diff != "" {
					t.Errorf("refused run changed the workspace: %s", diff)
				}
				if !reflect.DeepEqual(res, app.TransformHarnessResult{}) {
					t.Errorf("result = %+v, want the zero result", res)
				}
			})
		}
	}
}

func TestTransformGhcpPair_WorkspaceHoldingOnlyTheSourceTargetIsNotRefused(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := trCarryRun(t, pair[0], pair[1], nil)
			rel := r.put(pair[0], "x", trFixture(t, "subagent"))
			trSaveManifest(t, r.ws, pair[0], rel)

			res, err := r.run(r.request())

			if err != nil {
				t.Fatalf("TransformHarness: %v", err)
			}
			if f := trFile(t, res, "x"); f.Status != app.StatusTransformed {
				t.Errorf("agent status %q, want transformed", f.Status)
			}
		})
	}
}

func TestTransformGhcpPair_WorkspaceAlreadyOnTheTargetIsNotRefused(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := trCarryRun(t, pair[0], pair[1], nil)
			rel := r.put(pair[0], "x", trFixture(t, "subagent"))
			trSaveManifest(t, r.ws, pair[1], rel)

			_, err := r.run(r.request())

			var conflict *app.GhcpTargetConflictError
			if errors.As(err, &conflict) || errors.Is(err, app.ErrGhcpBothTargets) {
				t.Fatalf("workspace whose manifest already names %s was refused: %v", pair[1], err)
			}
		})
	}
}

func TestTransformGhcpPair_MixedAgentFileNamesDoNotMakeTheWorkspaceHoldBothTargets(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := trCarryRun(t, pair[0], pair[1], nil)
			trWrite(t, r.ws, ".github/agents/x.md", trFixture(t, "subagent"))
			trWrite(t, r.ws, ".github/agents/y.agent.md", trFixture(t, "subagent"))
			trSaveManifest(t, r.ws, pair[0], ".github/agents/x.md", ".github/agents/y.agent.md")

			_, err := r.run(r.request())

			var conflict *app.GhcpTargetConflictError
			if errors.As(err, &conflict) || errors.Is(err, app.ErrGhcpBothTargets) {
				t.Fatalf("workspace of %s with X.md and X.agent.md agents was refused: %v", pair[0], err)
			}
		})
	}
}
