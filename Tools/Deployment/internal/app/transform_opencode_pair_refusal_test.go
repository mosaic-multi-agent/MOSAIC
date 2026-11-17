package app_test

// transform_opencode_pair_refusal_test.go specifies that a transform between the two OpenCode
// targets is refused, before anything is asked or written, when the workspace holds both
// targets, and that no other transform state is refused: a workspace holding only the source or
// only the target, a transform from a third harness into an OpenCode target, and the GHCP pair.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// ocConflict reports whether err is an OpenCode target conflict of any kind.
func ocConflict(err error) bool {
	var ce *app.OpenCodeTargetConflictError
	return errors.As(err, &ce) || errors.Is(err, app.ErrOpenCodeBothTargets) || errors.Is(err, app.ErrOpenCodeOtherTarget)
}

func assertBothTargetsRefused(t *testing.T, r *trRun, src, tgt string, dry bool) {
	t.Helper()
	before := r.snapshot()
	req := ocRequest(r, src, tgt)
	req.DryRun = dry
	req.AutoConfirmPlan = false

	res, err := r.run(req)

	if !errors.Is(err, app.ErrOpenCodeBothTargets) {
		t.Fatalf("error = %v, want one matching ErrOpenCodeBothTargets", err)
	}
	var conflict *app.OpenCodeTargetConflictError
	if !errors.As(err, &conflict) || conflict.Requested.ID != tgt || len(conflict.Present) != 2 {
		t.Errorf("conflict = %+v, want Requested %s and both targets present", conflict, tgt)
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
}

func TestTransformOpenCodePair_HeldBothTargetsIsRefusedBeforeAnythingIsAskedOrWritten(t *testing.T) {
	for _, pair := range ocDirections {
		for _, dry := range []bool{false, true} {
			name := pair[0] + "->" + pair[1]
			if dry {
				name += " dry-run"
			}
			t.Run(name, func(t *testing.T) {
				r := ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2})
				r.svc = newHeldOpenCodeService(t, r.deps, true, true)

				assertBothTargetsRefused(t, r, pair[0], pair[1], dry)
			})
		}
	}
}

func TestTransformOpenCodePair_ManifestHoldingBothTargetsIsRefused(t *testing.T) {
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2})
			// The target also records an entry: both OpenCode harnesses now hold entries.
			e := ocTargetEntry(pair[1])
			trWrite(t, r.ws, e.TargetPath, "held skill\n")
			trAppendEntry(t, r.ws, e)

			assertBothTargetsRefused(t, r, pair[0], pair[1], false)
		})
	}
}

// ocTargetEntry is an entry of the given harness at the shared skills folder.
func ocTargetEntry(harnessID string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:    harnessID,
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "held-skill"},
		TargetPath: ".opencode/skills/held-skill/SKILL.md",
	}
}

func TestTransformOpenCodePair_WorkspaceHoldingOnlyTheSourceIsNotRefused(t *testing.T) {
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2})

			res, err := r.run(r.request())

			if err != nil {
				t.Fatalf("TransformHarness: %v", err)
			}
			if f := trFile(t, res, "worker"); f.Status != app.StatusTransformed {
				t.Errorf("agent status %q, want transformed", f.Status)
			}
		})
	}
}

func TestTransformOpenCodePair_HeldSetWithoutTheOtherTargetIsNotRefused(t *testing.T) {
	for _, held := range [][2]bool{{true, false}, {false, true}, {false, false}} {
		for _, pair := range ocDirections {
			t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
				r := ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2})
				r.svc = newHeldOpenCodeService(t, r.deps, held[0], held[1])

				_, err := r.run(r.request())

				if ocConflict(err) {
					t.Fatalf("held set %v was refused: %v", held, err)
				}
			})
		}
	}
}

func TestTransformOpenCodePair_AgentsAtSharedPathsNeverMakeTheWorkspaceHoldBothTargets(t *testing.T) {
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2})
			// Extra agent and plugin files with no entry of the target: only entries decide.
			trWrite(t, r.ws, ".opencode/agents/y.md", trFixture(t, "subagent"))
			trWrite(t, r.ws, ".opencode/plugins/extra.ts", "export default {}\n")

			_, err := r.run(r.request())

			if ocConflict(err) {
				t.Fatalf("workspace with unrecorded files at the shared paths was refused: %v", err)
			}
		})
	}
}

func TestTransformToAnOpenCodeTarget_FromAThirdHarnessIsNotRefusedWhenTheOtherTargetIsHeld(t *testing.T) {
	for _, tgt := range []string{hOpen, hOpenV2} {
		t.Run("claude-code->"+tgt, func(t *testing.T) {
			r := trNew(t, nil, hClaude, tgt, ocWithRegistry(t))
			rel := r.put(hClaude, "x", trFixture(t, "subagent"))
			trSaveManifest(t, r.ws, hClaude, rel)
			r.svc = newHeldOpenCodeService(t, r.deps, true, true)

			_, err := r.run(r.request())

			if ocConflict(err) {
				t.Fatalf("transform from claude-code into %s was refused as an OpenCode conflict: %v", tgt, err)
			}
		})
	}
}

func TestTransformGhcpPair_IsNotAffectedByHeldOpenCodeTargets(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := trCarryRun(t, pair[0], pair[1], nil)
			rel := r.put(pair[0], "x", trFixture(t, "subagent"))
			trSaveManifest(t, r.ws, pair[0], rel)
			r.svc = newHeldOpenCodeService(t, r.deps, true, true)

			res, err := r.run(r.request())

			if err != nil {
				t.Fatalf("GHCP pair transform with both OpenCode targets held: %v", err)
			}
			if f := trFile(t, res, "x"); f.Status != app.StatusTransformed {
				t.Errorf("agent status %q, want transformed", f.Status)
			}
		})
	}
}
