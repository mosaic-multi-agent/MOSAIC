package app

// transform_ghcp_pair_harness_scope_test.go specifies that the GHCP-pair transform plans only
// the agents the manifest records for the source harness. An agent recorded for another harness
// is not the source's deployed file: it is skipped as not recorded, never replaced or removed.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestPlanGhcpPair_AgentRecordedOnlyForAnotherHarnessIsNotTheSourcesDeployedFile(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "b.agent.md", "subagent.md")
			entry := func(harnessID, key string) domain.ManifestEntry {
				e := tpAgentEntry(key, tpAgentsDir+"/"+key+".agent.md")
				e.Harness = harnessID
				return e
			}
			m := domain.Manifest{
				SchemaVersion: manifest.SchemaVersion,
				Harnesses:     []domain.HarnessSection{{HarnessID: "opencode"}, {HarnessID: c.srcID}},
				Entries:       []domain.ManifestEntry{entry(c.srcID, "a"), entry("opencode", "b")},
			}
			if err := manifest.NewStore().Save(ws, m); err != nil {
				t.Fatalf("save manifest: %v", err)
			}

			p := tpPlan(t, ws, src, tgt, false)

			if a := tpAgent(t, p, "a.agent.md"); a.Action != PlanWriteOverwrite {
				t.Errorf("a: action %q, want overwrite: the source harness records it", a.Action)
			}
			if b := tpAgent(t, p, "b.agent.md"); b.Action != PlanSkip || b.Skip != SkipNotRecorded {
				t.Errorf("b: action %q skip %q, want skip %q: only another harness records it", b.Action, b.Skip, SkipNotRecorded)
			}
			want := []string{"agent:a " + tpAgentsDir + "/a.agent.md"}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want %v", got, want)
			}
		})
	}
}
