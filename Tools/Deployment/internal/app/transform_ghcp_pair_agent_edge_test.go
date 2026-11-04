package app

// transform_ghcp_pair_agent_edge_test.go covers an edge of the GHCP pair agent planning: a
// manifest that cannot be read records no agent, so every recognised agent is an unrecorded skip
// and nothing is written, replaced or removed.

import (
	"testing"

	"mosaic-deploy/internal/manifest"
)

func TestPlanGhcpPair_UnreadableManifestPlansNoAgentChangeAndNoError(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "b.md", "subagent.md")
			tpWriteFile(t, ws, manifest.Dir+"/"+manifest.FileName, "{ this is not a manifest")

			p := tpPlan(t, ws, src, tgt, false)

			for _, f := range []string{"a.agent.md", "b.md"} {
				if a := tpAgent(t, p, f); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
					t.Errorf("%s: action %q skip %q, want skip %q when the manifest cannot say what MOSAIC deployed", f, a.Action, a.Skip, SkipNotRecorded)
				}
			}
			if len(p.Removals) != 0 || len(p.Replacements) != 0 {
				t.Errorf("Removals %v / Replacements %v, want none", tpRemovals(p), tpReplacements(p))
			}
		})
	}
}

func TestPlanGhcpPair_AbsentManifestRecordsNothing(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")

			p := tpPlan(t, ws, src, tgt, true)

			if a := tpAgent(t, p, "a.agent.md"); a.Skip != SkipNotRecorded {
				t.Errorf("skip %q, want %q", a.Skip, SkipNotRecorded)
			}
			if w := tpWrites(p); len(w) != 0 {
				t.Errorf("planned writes = %d, want none without a manifest", len(w))
			}
		})
	}
}
