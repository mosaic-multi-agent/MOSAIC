package plan_test

// build_harness_scope_test.go verifies that classification sees only the manifest entries of
// the run's harness. The same path can be recorded once per harness; an entry of another
// harness says nothing about the file the run is about to classify.

import (
	"context"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const (
	scopeAgentTarget = "agents/test-agent.md"
	scopeOwnHash     = "sha256:own000"
	scopeOtherHash   = "sha256:other00"
)

// scopeEntry returns an entry of harness for the test agent whose versions all match a "1.0"
// source, so that an entry of the run's harness classifies the item as unchanged.
func scopeEntry(harness, hash string) domain.ManifestEntry {
	e := makeManifestEntry(agentRef("test-agent"), scopeAgentTarget, "1.0", hash)
	e.Harness = harness
	e.HarnessVersion = "1.0"
	e.InjectionsVersion = "1.0"
	return e
}

// scopeBuild plans the test agent for the fake module's harness ("test-harness") over a
// manifest holding the given entries, with the deployed file present under deployedHash.
func scopeBuild(t *testing.T, deployedHash string, entries ...domain.ManifestEntry) domain.PlanItem {
	t.Helper()
	sections := map[string]bool{}
	var secs []domain.HarnessSection
	for _, e := range entries {
		if !sections[e.Harness] {
			sections[e.Harness] = true
			secs = append(secs, domain.HarnessSection{HarnessID: e.Harness, UpdatedAt: time.Now()})
		}
	}
	snap := presentSnapshot(domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     secs,
		Entries:       entries,
	})
	ds := map[string]domain.DeployedArtifactState{
		scopeAgentTarget: deployedState(deployedHash, "1.0", "1.0", "1.0"),
	}
	input := buildInputFull(makeAgent("test-agent", "1.0"), makeSkill("a-skill", "1.0"), snap, ds)

	p, err := plan.New().Build(context.Background(), input)
	must(t, err)

	item, ok := findItem(p.Items, "test-agent")
	if !ok {
		t.Fatal("plan has no item for test-agent")
	}
	return item
}

func TestBuildHarnessScope_EntryOfAnotherHarnessAtTheSamePathDoesNotCountAsRecorded(t *testing.T) {
	// The file exists and the only record of it belongs to ghcp-cli: for test-harness the
	// file is unrecorded, so it is a conflict, never "unchanged".
	item := scopeBuild(t, scopeOtherHash, scopeEntry("ghcp-cli", scopeOtherHash))

	if item.Action != domain.ActionConflict {
		t.Fatalf("Action = %q, want %q (the entry of another harness must be ignored)", item.Action, domain.ActionConflict)
	}
	if item.Conflict == nil || !item.Conflict.ManifestMissing {
		t.Errorf("Conflict = %+v, want ManifestMissing (no entry for this harness at the path)", item.Conflict)
	}
}

// The two "OwnEntryIsUsed" tests below are regression coverage only: they guard that the
// run harness's own entry is still found when another harness's entry sits at the same path.
// They are not evidence for the scoping behaviour (the test above is).

func TestBuildHarnessScope_OwnEntryIsUsedWhenAnotherHarnessRecordsTheSamePathFirst(t *testing.T) {
	// The other harness's entry comes first; the run harness's own entry matches the
	// deployed file exactly.
	other := scopeEntry("ghcp-cli", scopeOtherHash)
	other.Version = "0.1"
	own := scopeEntry("test-harness", scopeOwnHash)

	item := scopeBuild(t, scopeOwnHash, other, own)

	if item.Action != domain.ActionUnchanged {
		t.Errorf("Action = %q, want %q (classified against the run harness's own entry)", item.Action, domain.ActionUnchanged)
	}
}

func TestBuildHarnessScope_OwnEntryIsUsedWhenAnotherHarnessRecordsTheSamePathLast(t *testing.T) {
	other := scopeEntry("opencode", scopeOtherHash)
	other.Version = "0.1"
	own := scopeEntry("test-harness", scopeOwnHash)

	item := scopeBuild(t, scopeOwnHash, own, other)

	if item.Action != domain.ActionUnchanged {
		t.Errorf("Action = %q, want %q", item.Action, domain.ActionUnchanged)
	}
}

// scopeSkillBuild plans the test skill for "test-harness" over a manifest holding the given
// entries, with the deployed skill file present.
func scopeSkillBuild(t *testing.T, entries ...domain.ManifestEntry) domain.PlanItem {
	t.Helper()
	const skillTarget = "skills/a-skill/SKILL.md"
	snap := presentSnapshot(domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: "test-harness", UpdatedAt: time.Now()}, {HarnessID: "ghcp-cli", UpdatedAt: time.Now()}},
		Entries:       entries,
	})
	ds := map[string]domain.DeployedArtifactState{skillTarget: deployedState(scopeOtherHash, "1.0", "1.0", "1.0")}
	agent := makeAgent("test-agent", "1.0")
	agent.RequiredSkills = []string{"a-skill"}
	p, err := plan.New().Build(context.Background(), buildInputFull(agent, makeSkill("a-skill", "1.0"), snap, ds))
	must(t, err)
	item, ok := findItem(p.Items, "a-skill")
	if !ok {
		t.Fatal("plan has no item for a-skill")
	}
	return item
}

// A skill recorded only for another harness at the same path classifies exactly like a skill
// that nothing records: the entry of the other harness must not reach the classifier.
func TestBuildHarnessScope_SkillRecordedOnlyForAnotherHarnessClassifiesAsUnrecorded(t *testing.T) {
	other := makeManifestEntry(skillRef("a-skill"), "skills/a-skill/SKILL.md", "1.0", scopeOtherHash)
	other.Harness = "ghcp-cli"
	other.HarnessVersion = "1.0"
	other.InjectionsVersion = "1.0"

	got := scopeSkillBuild(t, other)
	want := scopeSkillBuild(t)

	if got.Action != want.Action || (got.Conflict == nil) != (want.Conflict == nil) {
		t.Errorf("skill recorded only for ghcp-cli: Action %q conflict %v, want the unrecorded outcome: Action %q conflict %v",
			got.Action, got.Conflict, want.Action, want.Conflict)
	}
}
