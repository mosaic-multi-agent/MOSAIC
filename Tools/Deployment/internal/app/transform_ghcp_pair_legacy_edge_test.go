package app

// transform_ghcp_pair_legacy_edge_test.go specifies the edges of legacy X.md detection in the
// GHCP pair: the file-name match ignores case, only direct children of the agents folder are
// legacy entries, and the both-forms note names only the pair that is left side by side.

import (
	"strings"
	"testing"
)

func TestPlanGhcpPair_LegacyFileNameMatchIgnoresCase(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "Foo.MD", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("Foo", tpAgentsDir+"/Foo.MD"))

			p := tpPlan(t, ws, src, tgt, false)

			if want := []string{"agent:Foo " + tpAgentsDir + "/Foo.MD drop=true"}; !tpEqualStrings(tpRemovals(p), want) {
				t.Errorf("Removals = %v, want %v", tpRemovals(p), want)
			}
			if w := tpWrites(p); len(w) != 1 || tpDest(w[0]) != tpAgentsDir+"/Foo.agent.md" {
				t.Errorf("planned writes = %+v, want one write to %s/Foo.agent.md", w, tpAgentsDir)
			}
		})
	}
}

func TestPlanGhcpPair_RecordedFileBelowTheAgentsFolderIsNotALegacyEntry(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir+"/nested", "a.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/nested/a.md"))

			p := tpPlan(t, ws, src, tgt, false)

			if len(p.Removals) != 0 || len(p.Replacements) != 0 {
				t.Errorf("Removals %v / Replacements %v, want none: the entry is not a direct child of %s",
					tpRemovals(p), tpReplacements(p), tpAgentsDir)
			}
			if a := tpAgent(t, p, "a.agent.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
				t.Errorf("a.agent.md: action %q skip %q, want skip %q", a.Action, a.Skip, SkipNotRecorded)
			}
		})
	}
}

func TestPlanGhcpPair_BothFormsNoteLeavesOutTheMigratedLegacyFile(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "b.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "b.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"), tpAgentEntry("b", tpAgentsDir+"/b.agent.md"))

			p := tpPlan(t, ws, src, tgt, false)

			notes := tpNotes(p, NoteBothForms)
			if len(notes) != 1 || !tpNoteMentions(notes[0], tpAgentsDir+"/b.md", tpAgentsDir+"/b.agent.md") {
				t.Fatalf("both-forms notes = %+v, want one naming b.md and b.agent.md", notes)
			}
			if strings.Contains(strings.Join(notes[0].Paths, " ")+notes[0].Message, "a.md") {
				t.Errorf("note %+v names the migrated a.md, want only the key left side by side", notes[0])
			}
		})
	}
}
