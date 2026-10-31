package app_test

// transform_parity_test.go specifies the equality guarantee of the whole-deployment transform:
// for every ordered pair of the four built-in harnesses, each sample agent (a subagent, the
// orchestrator, the script orchestrator and a utility agent) that a transform writes is
// byte-identical to the file a fresh deploy of the same catalog versions writes to the target
// harness, given the same model.
//
// Allowed differences, and only these:
//   - file naming (GHCP CLI X.md from deploy, X.agent.md from transform);
//   - skill access on a utility agent whose catalog entry does not list the skill tool;
//   - for the GHCP CLI / VS Code GHCP pair, a source the transform cannot discover because
//     the pair's naming makes the destination the source itself: skipped with a stated reason.
//
// Carried user content needs no allowance: the sample deployment holds none.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestDeployTransformParity_EveryOrderedPairMatchesAFreshDeploy(t *testing.T) {
	fresh := map[string]string{}
	for _, id := range parityHarnesses {
		fresh[id] = parityDeploy(t, id)
	}

	for _, src := range parityHarnesses {
		for _, tgt := range parityHarnesses {
			if src == tgt {
				continue
			}
			src, tgt := src, tgt
			t.Run(src+"_to_"+tgt, func(t *testing.T) {
				ws := parityDeploy(t, src)
				res := parityTransform(t, ws, src, tgt)

				for _, a := range parityAgents {
					a := a
					t.Run(a.key, func(t *testing.T) {
						assertParityAgent(t, res, ws, fresh[tgt], src, tgt, a)
					})
				}
			})
		}
	}
}

// assertParityAgent checks one agent of one pair against the fresh deploy of the target.
func assertParityAgent(t *testing.T, res app.TransformHarnessResult, ws, freshWs, src, tgt string, a parityAgent) {
	t.Helper()
	out := parityOutcome(t, res, a.key)
	if out.Status != app.StatusTransformed {
		namingUnknown := out.Skip == app.SkipDestinationIsSource || out.Skip == app.SkipTargetExtension
		if namingUnknown && paritySharedAgentsFolder(t, src, tgt) {
			t.Skipf("source %s files of %s cannot be told apart from the %s destination "+
				"(shared agents folder and naming; accepted unknown of this pair)", src, a.key, tgt)
		}
		t.Fatalf("agent %s was not transformed: status %q skip %q reason %q", a.key, out.Status, out.Skip, out.Reason)
	}

	want := parityAgentFile(t, freshWs, tgt, a.key)
	got := parityReadRel(t, ws, out.DestinationPath)
	if got == want {
		return
	}
	if a.utility && paritySkillEntryRemoved(t, tgt, got) == want {
		return // allowed: explicit skill access on a utility agent that does not list the skill tool
	}
	t.Errorf("transformed %s differs from a fresh deploy to %s\n%s", a.key, tgt, parityFirstDiff(want, got))
}

// TestDeployTransformParity_SkillAccessAllowanceHidesNothingElse pins that the allowance for
// a utility agent is exactly the skill entry: a transformed utility agent that differs in
// anything else is a divergence, so the helper must not hide other differences.
func TestDeployTransformParity_SkillAccessAllowanceHidesNothingElse(t *testing.T) {
	for _, id := range parityHarnesses {
		doc := "---\nname: x\ntools: [read]\nmodel: m\n---\nbody mentions skill\n"
		if got := paritySkillEntryRemoved(t, id, doc); got != doc {
			t.Errorf("%s: a document without a skill entry was changed:\n%s", id, got)
		}
	}
}

// parityOutcome returns the agent outcome of a transform result for a key.
func parityOutcome(t *testing.T, res app.TransformHarnessResult, key string) app.TransformFileOutcome {
	t.Helper()
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactAgent && f.Key == key {
			return f
		}
	}
	t.Fatalf("transform reported no outcome for agent %q; agent keys: %v", key, trAgentKeys(res))
	return app.TransformFileOutcome{}
}

// paritySharedAgentsFolder reports whether both harnesses use the same agents folder.
func paritySharedAgentsFolder(t *testing.T, a, b string) bool {
	t.Helper()
	return rbModule(t, a).Descriptor().Paths.Agents.Project == rbModule(t, b).Descriptor().Paths.Agents.Project
}

// parityFirstDiff describes the first line where two documents differ.
func parityFirstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return "first difference at line " + strconv.Itoa(i+1) + ":\n  fresh deploy: " + wl + "\n  transform:    " + gl
		}
	}
	return "documents differ only in length"
}
