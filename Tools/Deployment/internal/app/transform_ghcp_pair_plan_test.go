package app

// transform_ghcp_pair_plan_test.go specifies how transform plans the move between the two GHCP
// targets (ghcp-cli and vscode-ghcp, either direction). Both targets name agents
// .github/agents/X.agent.md, so a recorded X.agent.md is replaced in place, unrecorded files stay,
// and skills are left alone. Legacy X.md migration is specified in
// transform_ghcp_pair_agents_test.go. Every other pair plans no removal.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// tpAgentsDir is the agents folder both GHCP targets share.
const tpAgentsDir = ".github/agents"

// tpPairCase is one direction of the GHCP pair. Both targets name agents X.agent.md.
type tpPairCase struct {
	name, src, tgt, srcID, srcExt, tgtExt string
}

var tpPairCases = []tpPairCase{
	{"cli to vscode", tpCLI, tpVSCode, "ghcp-cli", ".agent.md", ".agent.md"},
	{"vscode to cli", tpVSCode, tpCLI, "vscode-ghcp", ".agent.md", ".agent.md"},
}

// tpRecord saves a manifest holding the given entries for the workspace.
func tpRecord(t *testing.T, ws, harnessID string, entries ...domain.ManifestEntry) {
	t.Helper()
	m := domain.Manifest{SchemaVersion: manifest.SchemaVersion, HarnessID: harnessID, Entries: entries}
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
}

// tpAgentEntry is a manifest entry of a deployed agent file.
func tpAgentEntry(key, rel string) domain.ManifestEntry {
	return domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}, TargetPath: rel}
}

// tpRemovals renders each planned removal as "kind:key path drop=<bool>", in plan order.
func tpRemovals(p TransformPlan) []string {
	var out []string
	for _, r := range p.Removals {
		out = append(out, fmt.Sprintf("%s:%s %s drop=%t", r.Ref.Kind, r.Ref.Key, strings.ReplaceAll(r.Path, "\\", "/"), r.DropEntry))
	}
	return out
}

// tpReplacements renders each planned replacement as "kind:key path", in plan order.
func tpReplacements(p TransformPlan) []string {
	var out []string
	for _, r := range p.Replacements {
		out = append(out, fmt.Sprintf("%s:%s %s", r.Ref.Kind, r.Ref.Key, strings.ReplaceAll(r.Path, "\\", "/")))
	}
	return out
}

func tpEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPlanGhcpPair_RecordedAgentIsReplacedInPlaceAndUnrecordedOnesAreSkipped(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpPlaceAgent(t, ws, tpAgentsDir, "b.agent.md", "subagent.md")
			// "c" is recorded but its file is gone; "a" is the only recorded file that exists.
			tpRecord(t, ws, c.srcID,
				tpAgentEntry("a", tpAgentsDir+"/a.agent.md"),
				tpAgentEntry("c", tpAgentsDir+"/c.agent.md"))

			p := tpPlan(t, ws, src, tgt, false)

			a := tpAgent(t, p, "a.agent.md")
			if a.Action != PlanWriteOverwrite || tpDest(a) != tpAgentsDir+"/a.agent.md" {
				t.Errorf("a: action %q destination %q, want overwrite in place at %s/a.agent.md", a.Action, tpDest(a), tpAgentsDir)
			}
			if b := tpAgent(t, p, "b.agent.md"); b.Action != PlanSkip || b.Skip != SkipNotRecorded {
				t.Errorf("b: action %q skip %q, want skip %q", b.Action, b.Skip, SkipNotRecorded)
			}
			if got := tpRemovals(p); len(got) != 0 {
				t.Errorf("Removals = %v, want none: nothing is removed for an in-place agent", got)
			}
			want := []string{"agent:a " + tpAgentsDir + "/a.agent.md"}
			if got := tpReplacements(p); !tpEqualStrings(got, want) {
				t.Errorf("Replacements = %v, want %v", got, want)
			}
		})
	}
}

func TestPlanGhcpPair_AgentsTheManifestDoesNotRecordAreNeverWrittenReplacedOrRemoved(t *testing.T) {
	for _, c := range tpPairCases {
		for _, withManifest := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s manifest=%t", c.name, withManifest), func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
				tpPlaceAgent(t, ws, tpAgentsDir, "mine.agent.md", "subagent.md")
				if withManifest {
					tpRecord(t, ws, c.srcID, tpAgentEntry("other", tpAgentsDir+"/other.agent.md"))
				}

				p := tpPlan(t, ws, src, tgt, true)

				if a := tpAgent(t, p, "mine.agent.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
					t.Errorf("action %q skip %q, want skip %q", a.Action, a.Skip, SkipNotRecorded)
				}
				if len(p.Removals) != 0 || len(p.Replacements) != 0 {
					t.Errorf("Removals %v / Replacements %v, want none for an unrecorded file", tpRemovals(p), tpReplacements(p))
				}
			})
		}
	}
}

func TestPlanGhcpPair_OnlyRecordedEntriesOfKindAgentCount(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpCLI), tpHarness(t, tpVSCode)
	tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
	skill := domain.ManifestEntry{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "a"},
		TargetPath: tpAgentsDir + "/a.agent.md",
	}
	tpRecord(t, ws, "ghcp-cli", skill)

	p := tpPlan(t, ws, src, tgt, false)

	if a := tpAgent(t, p, "a.agent.md"); a.Action != PlanSkip || a.Skip != SkipNotRecorded {
		t.Errorf("action %q skip %q, want skip %q: only agent entries record a file", a.Action, a.Skip, SkipNotRecorded)
	}
	if len(p.Removals) != 0 || len(p.Replacements) != 0 {
		t.Errorf("Removals %v / Replacements %v, want none: the entry is not an agent entry", tpRemovals(p), tpReplacements(p))
	}
}

func TestPlanGhcpPair_ManifestPathWithBackslashesStillMatchesTheAgentFile(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpCLI), tpHarness(t, tpVSCode)
	tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
	tpRecord(t, ws, "ghcp-cli", tpAgentEntry("a", `.github\agents\a.agent.md`))

	p := tpPlan(t, ws, src, tgt, false)

	want := []string{"agent:a " + tpAgentsDir + "/a.agent.md"}
	if got := tpReplacements(p); !tpEqualStrings(got, want) {
		t.Errorf("Replacements = %v, want %v", got, want)
	}
}

func TestPlanGhcpPair_OverwriteFlagDoesNotChangeAgentPlanning(t *testing.T) {
	for _, c := range tpPairCases {
		for _, overwrite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s overwrite=%t", c.name, overwrite), func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
				tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
				tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.agent.md"))

				p := tpPlan(t, ws, src, tgt, overwrite)

				if a := tpAgent(t, p, "a.agent.md"); a.Action != PlanWriteOverwrite {
					t.Errorf("action %q, want overwrite whatever the overwrite flag says", a.Action)
				}
				if len(p.Replacements) != 1 {
					t.Errorf("Replacements = %v, want the one recorded agent", tpReplacements(p))
				}
			})
		}
	}
}

func TestPlanGhcpPair_ReplacementsFollowPlanOrder(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			var entries []domain.ManifestEntry
			for _, k := range []string{"c", "a", "b"} {
				tpPlaceAgent(t, ws, tpAgentsDir, k+".agent.md", "subagent.md")
				entries = append(entries, tpAgentEntry(k, tpAgentsDir+"/"+k+".agent.md"))
			}
			tpRecord(t, ws, c.srcID, entries...)

			p := tpPlan(t, ws, src, tgt, false)

			var want []string
			for _, a := range tpWrites(p) {
				k := strings.TrimSuffix(a.FileName, ".agent.md")
				want = append(want, "agent:"+k+" "+tpAgentsDir+"/"+a.FileName)
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

func TestPlanGhcpPair_SkillsAreNeitherRemovedNorReplaced(t *testing.T) {
	for _, c := range tpPairCases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.src), tpHarness(t, c.tgt)
			tpPlaceSkill(t, ws, src)
			tpPlaceAgent(t, ws, tpAgentsDir, "a.agent.md", "subagent.md")
			tpRecord(t, ws, c.srcID, tpAgentEntry("a", tpAgentsDir+"/a.agent.md"))

			p := tpPlan(t, ws, src, tgt, true)

			sk := tpSkill(t, p)
			if sk.Action != PlanSkip || sk.Skip != SkipDestinationIsSource {
				t.Errorf("skill action %q skip %q, want the shared skills folder left as it is", sk.Action, sk.Skip)
			}
			if len(p.Replacements) == 0 {
				t.Fatal("setup: the recorded agent was not planned for replacement")
			}
			for _, s := range append(tpRemovals(p), tpReplacements(p)...) {
				if strings.Contains(s, ".github/skills") {
					t.Errorf("skill path planned for removal or replacement: %s", s)
				}
			}
		})
	}
}

func TestPlanTransform_PairWithOnlyOneGhcpSidePlansNoRemovals(t *testing.T) {
	// A transform between a GHCP target and another harness plans no removal, even when the
	// manifest records the source agent.
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpCLI)
	tpPlaceAgent(t, ws, ".claude/agents", "x.md", "subagent.md")
	tpRecord(t, ws, "claude-code", tpAgentEntry("x", ".claude/agents/x.md"))

	p := tpPlan(t, ws, src, tgt, true)

	if p.Removals != nil || p.Replacements != nil {
		t.Errorf("Removals %v / Replacements %v, want none outside the GHCP pair", tpRemovals(p), tpReplacements(p))
	}
}
