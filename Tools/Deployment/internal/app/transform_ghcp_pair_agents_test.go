package app

// transform_ghcp_pair_agents_test.go specifies how transform plans the agents of the GHCP pair
// (ghcp-cli and vscode-ghcp, either direction) when the manifest records legacy X.md files: the
// recorded legacy agent is migrated to X.agent.md (the old file is removed with its manifest
// entry, an existing X.agent.md is replaced), unrecorded files are left alone, and a migrated
// agent produces no both-forms note. Recorded X.agent.md files replaced in place are specified in
// transform_ghcp_pair_plan_test.go.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanGhcpPair_RecordedLegacyAgentIsMigratedToTheAgentMdName(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"))

			p := tpPlan(t, ws, src, tgt, false)

			a := tpAgent(t, p, "a.md")
			if a.Action != PlanWriteCreate || tpDest(a) != tpAgentsDir+"/a.agent.md" {
				t.Errorf("action %q destination %q, want create at %s/a.agent.md", a.Action, tpDest(a), tpAgentsDir)
			}
			if want := []string{"agent:a " + tpAgentsDir + "/a.md drop=true"}; !tpEqualStrings(tpRemovals(p), want) {
				t.Errorf("Removals = %v, want %v", tpRemovals(p), want)
			}
			if got := tpReplacements(p); len(got) != 0 {
				t.Errorf("Replacements = %v, want none: no X.agent.md exists yet", got)
			}
			if notes := tpNotes(p, NoteBothForms); len(notes) != 0 {
				t.Errorf("both-forms notes = %+v, want none for a migrated agent", notes)
			}
		})
	}
}

func TestPlanGhcpPair_ExistingAgentMdBesideRecordedLegacyFileIsReplacedWhetherRecordedOrNot(t *testing.T) {
	for _, c := range tpPairCases {
		for _, recorded := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s agent.md recorded=%t", c.name, recorded), func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
				tpPlaceAgent(t, ws, tpAgentsDir, "a.md", "subagent.md")
				tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
				if recorded {
					tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"), tpAgentEntry("a", tpAgentsDir+"/a.agent.md"))
				} else {
					tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"))
				}

				p := tpPlan(t, ws, src, tgt, false)

				if len(p.Agents) != 1 {
					t.Fatalf("agent rows = %v, want a single row for the migrated agent", tpAgentNames(p))
				}
				a := p.Agents[0]
				if a.Action != PlanWriteOverwrite || tpDest(a) != tpAgentsDir+"/a.agent.md" {
					t.Errorf("action %q destination %q, want overwrite at %s/a.agent.md", a.Action, tpDest(a), tpAgentsDir)
				}
				if want := []string{"agent:a " + tpAgentsDir + "/a.md drop=true"}; !tpEqualStrings(tpRemovals(p), want) {
					t.Errorf("Removals = %v, want %v", tpRemovals(p), want)
				}
				if want := []string{"agent:a " + tpAgentsDir + "/a.agent.md"}; !tpEqualStrings(tpReplacements(p), want) {
					t.Errorf("Replacements = %v, want %v", tpReplacements(p), want)
				}
				if notes := tpNotes(p, NoteBothForms); len(notes) != 0 {
					t.Errorf("both-forms notes = %+v, want none for a migrated agent", notes)
				}
			})
		}
	}
}

func TestPlanGhcpPair_MigratedAgentTakesItsSourceFromTheRecordedAgentMdFileFirst(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			// The two files carry different models so the row's source is identifiable.
			base := tpFixture(t, "agents/subagent.md")
			tpWriteFile(t, ws, tpAgentsDir+"/a.md", strings.Replace(base, "model: claude-sonnet-4-5", "model: model-of-legacy-file", 1))
			tpWriteFile(t, ws, tpAgentsDir+"/a.agent.md", strings.Replace(base, "model: claude-sonnet-4-5", "model: model-of-agent-md-file", 1))
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"), tpAgentEntry("a", tpAgentsDir+"/a.agent.md"))

			p := tpPlan(t, ws, src, tgt, false)

			if len(p.Agents) != 1 {
				t.Fatalf("agent rows = %v, want a single merged row", tpAgentNames(p))
			}
			a := p.Agents[0]
			if got := filepath.Base(a.SourcePath); got != "a.agent.md" {
				t.Errorf("source file = %q, want the recorded a.agent.md", got)
			}
			if a.SourceModel != "model-of-agent-md-file" {
				t.Errorf("source model = %q, want the model of the recorded a.agent.md", a.SourceModel)
			}
			if a.Action != PlanWriteOverwrite || tpDest(a) != tpAgentsDir+"/a.agent.md" {
				t.Errorf("action %q destination %q, want overwrite at %s/a.agent.md", a.Action, tpDest(a), tpAgentsDir)
			}
		})
	}
}

func TestPlanGhcpPair_MigratedAgentTakesItsSourceFromTheLegacyFileWhenAgentMdIsNotRecorded(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.md", "subagent.md")
			tpWriteFile(t, ws, tpAgentsDir+"/a.agent.md", "user edited, not deployed by MOSAIC\n")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"))

			a := tpPlan(t, ws, src, tgt, false).Agents[0]

			if got := filepath.Base(a.SourcePath); got != "a.md" {
				t.Errorf("source file = %q, want the recorded legacy a.md", got)
			}
		})
	}
}

func TestPlanGhcpPair_UnrecordedLegacyFilesAreLeftAlone(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "mine.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("other", tpAgentsDir+"/other.md"))

			p := tpPlan(t, ws, src, tgt, true)

			if a := tpAgent(t, p, "mine.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
				t.Errorf("action %q skip %q, want skip %q", a.Action, a.Skip, SkipNotRecorded)
			}
			if len(p.Removals) != 0 || len(p.Replacements) != 0 {
				t.Errorf("Removals %v / Replacements %v, want none", tpRemovals(p), tpReplacements(p))
			}
		})
	}
}

func TestPlanGhcpPair_UnrecordedLegacyFileBesideAnInPlaceAgentStaysAndIsNoted(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "Foo.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "Foo.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("Foo", tpAgentsDir+"/Foo.agent.md"))

			p := tpPlan(t, ws, src, tgt, false)

			if a := tpAgent(t, p, "Foo.agent.md"); a.Action != PlanWriteOverwrite || tpDest(a) != tpAgentsDir+"/Foo.agent.md" {
				t.Errorf("Foo.agent.md: action %q destination %q, want overwrite in place", a.Action, tpDest(a))
			}
			if a := tpAgent(t, p, "Foo.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
				t.Errorf("Foo.md: action %q skip %q, want skip %q", a.Action, a.Skip, SkipNotRecorded)
			}
			if got := tpRemovals(p); len(got) != 0 {
				t.Errorf("Removals = %v, want none", got)
			}
			notes := tpNotes(p, NoteBothForms)
			if len(notes) != 1 || !tpNoteMentions(notes[0], tpAgentsDir+"/Foo.md", tpAgentsDir+"/Foo.agent.md") {
				t.Errorf("both-forms notes = %+v, want one naming both files", notes)
			}
		})
	}
}

func TestPlanGhcpPair_RecordedLegacyFileThatIsNotAMosaicAgentIsNeitherWrittenNorRemoved(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.md", "plain-notes.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"))

			p := tpPlan(t, ws, src, tgt, true)

			for _, name := range []string{"a.md", "a.agent.md"} {
				if a := tpAgent(t, p, name); a.Action != PlanSkip || a.Skip != SkipNotAgent {
					t.Errorf("%s: action %q skip %q, want skip %q", name, a.Action, a.Skip, SkipNotAgent)
				}
			}
			if len(p.Removals) != 0 || len(p.Replacements) != 0 {
				t.Errorf("Removals %v / Replacements %v, want none for an agent without a recognised source",
					tpRemovals(p), tpReplacements(p))
			}
			if w := tpWrites(p); len(w) != 0 {
				t.Errorf("planned writes = %d, want none", len(w))
			}
		})
	}
}

func TestPlanGhcpPair_RecordedLegacyEntryWhoseFileIsGoneIsNotMigrated(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.md"), tpAgentEntry("a", tpAgentsDir+"/a.agent.md"))

			p := tpPlan(t, ws, src, tgt, false)

			if got := tpRemovals(p); len(got) != 0 {
				t.Errorf("Removals = %v, want none: the legacy file does not exist", got)
			}
			if want := []string{"agent:a " + tpAgentsDir + "/a.agent.md"}; !tpEqualStrings(tpReplacements(p), want) {
				t.Errorf("Replacements = %v, want the in-place agent %v", tpReplacements(p), want)
			}
		})
	}
}

func TestPlanGhcpPair_MixedAgentsAreDecidedPerKeyAndListedInRowOrder(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "b.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "c.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "c.agent.md", "subagent.md")
			tpRecord(t, ws, c.srcID,
				tpAgentEntry("c", tpAgentsDir+"/c.md"),
				tpAgentEntry("b", tpAgentsDir+"/b.agent.md"),
				tpAgentEntry("a", tpAgentsDir+"/a.md"))

			p := tpPlan(t, ws, src, tgt, false)

			wantRm := []string{
				"agent:a " + tpAgentsDir + "/a.md drop=true",
				"agent:c " + tpAgentsDir + "/c.md drop=true",
			}
			if got := tpRemovals(p); !tpEqualStrings(got, wantRm) {
				t.Errorf("Removals = %v, want %v", got, wantRm)
			}
			wantRp := []string{
				"agent:b " + tpAgentsDir + "/b.agent.md",
				"agent:c " + tpAgentsDir + "/c.agent.md",
			}
			if got := tpReplacements(p); !tpEqualStrings(got, wantRp) {
				t.Errorf("Replacements = %v, want %v", got, wantRp)
			}
		})
	}
}
