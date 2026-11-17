package app

// transform_opencode_pair_plan_test.go specifies how transform plans the move between the two
// OpenCode targets (opencode and opencode-v2, either direction). Both targets name agents
// .opencode/agents/X.md and keep skills in .opencode/skills, so the target replaces the source in
// place: a recorded agent is overwritten, a recorded skill folder is carried in place, and
// everything the manifest does not record is left alone. Hook planning is specified in
// transform_opencode_pair_hooks_plan_test.go.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	tpOpenV2 = "opencode-v2.yaml"

	tpOCAgentsDir = ".opencode/agents"
	tpOCSkillsDir = ".opencode/skills"
)

// tpOCCase is one direction of the OpenCode pair.
type tpOCCase struct {
	name, src, tgt, srcID string
}

var tpOCCases = []tpOCCase{
	{"opencode to opencode-v2", tpOpen, tpOpenV2, "opencode"},
	{"opencode-v2 to opencode", tpOpenV2, tpOpen, "opencode-v2"},
}

// tpSkillFileEntry is a manifest entry of one deployed skill file.
func tpSkillFileEntry(key, rel string) domain.ManifestEntry {
	return domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: key}, TargetPath: rel}
}

// tpOCRemovals renders each planned removal as "kind:key path", in plan order.
func tpOCRemovals(p TransformPlan) []string {
	var out []string
	for _, r := range p.Removals {
		out = append(out, fmt.Sprintf("%s:%s %s", r.Ref.Kind, r.Ref.Key, strings.ReplaceAll(r.Path, "\\", "/")))
	}
	return out
}

func TestPlanOpenCodePair_RecordedAgentIsOverwrittenInPlaceAndUnrecordedOnesAreSkipped(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpOCAgentsDir, "a.md", "subagent.md")
			tpPlaceAgent(t, ws, tpOCAgentsDir, "b.md", "subagent.md")
			// "c" is recorded but its file is gone; "a" is the only recorded file that exists.
			tpRecord(t, ws, c.srcID,
				tpAgentEntry("a", tpOCAgentsDir+"/a.md"),
				tpAgentEntry("c", tpOCAgentsDir+"/c.md"))

			p := tpPlan(t, ws, src, tgt, false)

			a := tpAgent(t, p, "a.md")
			if a.Action != PlanWriteOverwrite || tpDest(a) != tpOCAgentsDir+"/a.md" {
				t.Errorf("a: action %q destination %q, want overwrite in place at %s/a.md", a.Action, tpDest(a), tpOCAgentsDir)
			}
			if b := tpAgent(t, p, "b.md"); b.Action != PlanSkip || b.Skip != SkipNotRecorded {
				t.Errorf("b: action %q skip %q, want skip %q", b.Action, b.Skip, SkipNotRecorded)
			}
			if got := tpOCRemovals(p); len(got) != 0 {
				t.Errorf("Removals = %v, want none: nothing is removed for an in-place agent", got)
			}
			want := []string{"agent:a " + tpOCAgentsDir + "/a.md"}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want %v", got, want)
			}
		})
	}
}

func TestPlanOpenCodePair_OverwriteFlagDoesNotChangeAgentPlanning(t *testing.T) {
	for _, c := range tpOCCases {
		for _, overwrite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s overwrite=%t", c.name, overwrite), func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
				tpPlaceAgent(t, ws, tpOCAgentsDir, "a.md", "subagent.md")
				tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpOCAgentsDir+"/a.md"))

				p := tpPlan(t, ws, src, tgt, overwrite)

				if a := tpAgent(t, p, "a.md"); a.Action != PlanWriteOverwrite {
					t.Errorf("action %q, want overwrite whatever the overwrite flag says", a.Action)
				}
				if len(p.Replacements) != 1 {
					t.Errorf("Replacements = %v, want the one recorded agent", tpReplacements(p))
				}
			})
		}
	}
}

func TestPlanOpenCodePair_AgentTheOtherTargetRecordsIsNotRecordedForTheSource(t *testing.T) {
	// The agent file sits at the shared path, but only the target's manifest section names it.
	// "Recorded" means named by an entry of the source harness.
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpOCAgentsDir, "a.md", "subagent.md")
			otherID := "opencode"
			if c.srcID == "opencode" {
				otherID = "opencode-v2"
			}
			tpRecord(t, ws, otherID, tpAgentEntry("a", tpOCAgentsDir+"/a.md"))

			p := tpPlan(t, ws, src, tgt, true)

			if a := tpAgent(t, p, "a.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
				t.Errorf("action %q skip %q, want skip %q: the source does not record the file", a.Action, a.Skip, SkipNotRecorded)
			}
			if len(p.Replacements) != 0 {
				t.Errorf("Replacements = %v, want none", tpReplacements(p))
			}
		})
	}
}

func TestPlanOpenCodePair_UnrecordedAgentsAreNeverWrittenReplacedOrRemoved(t *testing.T) {
	for _, c := range tpOCCases {
		for _, withManifest := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s manifest=%t", c.name, withManifest), func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
				tpPlaceAgent(t, ws, tpOCAgentsDir, "mine.md", "subagent.md")
				if withManifest {
					tpRecord(t, ws, c.srcID, tpAgentEntry("other", tpOCAgentsDir+"/other.md"))
				}

				p := tpPlan(t, ws, src, tgt, true)

				if a := tpAgent(t, p, "mine.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
					t.Errorf("action %q skip %q, want skip %q", a.Action, a.Skip, SkipNotRecorded)
				}
				if len(p.Removals) != 0 || len(p.Replacements) != 0 {
					t.Errorf("Removals %v / Replacements %v, want none for an unrecorded file", tpOCRemovals(p), tpReplacements(p))
				}
			})
		}
	}
}

func TestPlanOpenCodePair_NonAgentFileIsSkippedAsNotAnAgent(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpOCAgentsDir, "notes.md", "plain-notes.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("notes", tpOCAgentsDir+"/notes.md"))

			p := tpPlan(t, ws, src, tgt, true)

			if a := tpAgent(t, p, "notes.md"); a.Action != PlanSkip || a.Skip != SkipNotAgent {
				t.Errorf("action %q skip %q, want skip %q", a.Action, a.Skip, SkipNotAgent)
			}
			if len(p.Replacements) != 0 {
				t.Errorf("Replacements = %v, want none for a file that is not an agent", tpReplacements(p))
			}
		})
	}
}

func TestPlanOpenCodePair_ReplacementsFollowPlanOrder(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			var entries []domain.ManifestEntry
			for _, k := range []string{"c", "a", "b"} {
				tpPlaceAgent(t, ws, tpOCAgentsDir, k+".md", "subagent.md")
				entries = append(entries, tpAgentEntry(k, tpOCAgentsDir+"/"+k+".md"))
			}
			tpRecord(t, ws, c.srcID, entries...)

			p := tpPlan(t, ws, src, tgt, false)

			var want []string
			for _, a := range tpWrites(p) {
				k := strings.TrimSuffix(a.FileName, ".md")
				want = append(want, "agent:"+k+" "+tpOCAgentsDir+"/"+a.FileName)
			}
			if len(want) != 3 {
				t.Fatalf("setup: %d planned writes, want 3", len(want))
			}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want plan order %v", got, want)
			}
		})
	}
}

func TestPlanOpenCodePair_RecordedSkillFolderIsCarriedInPlaceWithoutReplacement(t *testing.T) {
	for _, c := range tpOCCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceSkill(t, ws, src)
			tpRecord(t, ws, c.srcID, tpSkillFileEntry("review-skill", tpOCSkillsDir+"/review-skill/SKILL.md"))

			p := tpPlan(t, ws, src, tgt, false)

			sk := tpSkill(t, p)
			if sk.Action != PlanWriteOverwrite || sk.Skip != "" {
				t.Errorf("skill action %q skip %q, want overwrite with no skip", sk.Action, sk.Skip)
			}
			if sk.Key != "review-skill" || strings.ReplaceAll(sk.DestinationDir, "\\", "/") != tpOCSkillsDir+"/review-skill" {
				t.Errorf("skill key %q destination %q, want review-skill carried in place", sk.Key, sk.DestinationDir)
			}
			for _, s := range append(tpOCRemovals(p), tpReplacements(p)...) {
				if strings.Contains(s, tpOCSkillsDir) {
					t.Errorf("skill path planned for removal or replacement: %s", s)
				}
			}
		})
	}
}

func TestPlanOpenCodePair_SkillFolderWithoutASourceEntryIsSkippedAsNotRecorded(t *testing.T) {
	for _, c := range tpOCCases {
		for _, withManifest := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s manifest=%t", c.name, withManifest), func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
				tpPlaceSkill(t, ws, src)
				if withManifest {
					tpRecord(t, ws, c.srcID, tpSkillFileEntry("another-skill", tpOCSkillsDir+"/another-skill/SKILL.md"))
				}

				sk := tpSkill(t, tpPlan(t, ws, src, tgt, true))

				if sk.Action != PlanSkip || sk.Skip != SkipNotRecorded {
					t.Errorf("skill action %q skip %q, want skip %q", sk.Action, sk.Skip, SkipNotRecorded)
				}
			})
		}
	}
}

func TestPlanOpenCodePair_SkillIsCarriedWhateverTheOverwriteFlagSays(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		t.Run(fmt.Sprintf("overwrite=%t", overwrite), func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, tpOpen), tpHarness(t, tpOpenV2)
			tpPlaceSkill(t, ws, src)
			tpRecord(t, ws, "opencode", tpSkillFileEntry("review-skill", tpOCSkillsDir+"/review-skill/SKILL.md"))

			sk := tpSkill(t, tpPlan(t, ws, src, tgt, overwrite))

			if sk.Action != PlanWriteOverwrite {
				t.Errorf("skill action %q, want overwrite", sk.Action)
			}
		})
	}
}

func TestPlanTransform_OpenCodeWithAnotherHarnessPlansNoRemovalsOrReplacements(t *testing.T) {
	// A transform between an OpenCode target and a harness outside the pair is not the pair.
	for _, c := range []struct {
		name, src, tgt, srcID, dir, file string
	}{
		{"claude-code to opencode-v2", tpClaude, tpOpenV2, "claude-code", ".claude/agents", "x.md"},
		{"opencode-v2 to claude-code", tpOpenV2, tpClaude, "opencode-v2", tpOCAgentsDir, "x.md"},
		{"opencode to ghcp-cli", tpOpen, tpCLI, "opencode", tpOCAgentsDir, "x.md"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, c.dir, c.file, "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("x", c.dir+"/"+c.file))

			p := tpPlan(t, ws, src, tgt, true)

			if p.Removals != nil || p.Replacements != nil {
				t.Errorf("Removals %v / Replacements %v, want none outside the OpenCode pair", tpOCRemovals(p), tpReplacements(p))
			}
		})
	}
}
