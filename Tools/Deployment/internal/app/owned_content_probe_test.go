package app_test

// owned_content_probe_test.go verifies that the flows hand the planner the Deploy-owned
// content record recomputed from the files on disk (never taken from the manifest), for agents
// and skill files, with the same result the executor records at write time.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const probedAgentText = `---
id: 1
version: "1.0"
name: Test Runner
---

Outside prose.

<Identity type="core">
Identity prose.
<IdentityExtension type="project">
</IdentityExtension>
</Identity>
`

const probedSkillText = "---\nmosaic_id: lean-tdd\nversion: \"1.0\"\nname: Lean TDD\n---\n\nSkill body.\n"

// capturedDeployedState runs DeployNew over a workspace holding the given files (name -> bytes)
// and returns the DeployedState the planner received.
func capturedDeployedState(t *testing.T, files map[string][]byte) map[string]domain.DeployedArtifactState {
	t.Helper()
	ws := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ws, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	spy := &spyPlanner{response: newMinimalPlan(ws)}
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	deps.Planner = spy
	cat := newMinimalCatalog()
	agent := minimalAgent
	agent.RequiredSkills = []string{"lean-tdd"}
	cat.agents = []domain.Agent{agent}
	cat.skills = []domain.Skill{{Key: "lean-tdd", Name: "Lean TDD", Version: "1.0", EntryFile: "SKILL.md"}}
	deps.Catalog = cat
	if _, err := app.New(deps).DeployNew(context.Background(), newDeployRequest(ws, "stub-harness", []string{"quick-fix"})); err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
	return spy.lastCaptured(t).DeployedState
}

// requireProbedOwned fails the test unless the probe filled Owned for name. The probe result is
// checked before any expected record is computed, so a missing probe fails here and not in the
// expected-value computation.
func requireProbedOwned(t *testing.T, state map[string]domain.DeployedArtifactState, name string) *domain.OwnedContent {
	t.Helper()
	got := state[name].Owned
	if got == nil {
		t.Fatalf("DeployedState[%s].Owned is nil; the probe must fill it from the file on disk", name)
	}
	return got
}

func wantOwned(t *testing.T, kind domain.ArtifactKind, content []byte) domain.OwnedContent {
	t.Helper()
	want, err := transform.OwnedContentOf(kind, content)
	if err != nil {
		t.Fatalf("OwnedContentOf: %v", err)
	}
	return want
}

func TestDeployFlow_ProbeFillsOwnedContentOfDeployedAgentFromDisk(t *testing.T) {
	state := capturedDeployedState(t, map[string][]byte{"test-runner.md": []byte(probedAgentText)})

	got := requireProbedOwned(t, state, "test-runner.md")

	want := wantOwned(t, domain.ArtifactAgent, []byte(probedAgentText))
	if got.Hash != want.Hash || len(got.Regions) != len(want.Regions) || len(want.Regions) == 0 {
		t.Errorf("DeployedState[test-runner.md].Owned = %+v; want %+v", got, want)
	}
}

func TestDeployFlow_ProbeOwnedContentIgnoresProjectRegionFrontmatterBOMAndCRLF(t *testing.T) {
	filled := []byte("\xEF\xBB\xBF" + strings.ReplaceAll(
		"---\nid: 1\nversion: \"1.0\"\nname: Test Runner\nextra: user key\n---\n\nOutside prose.\n\n"+
			"<Identity type=\"core\">\nIdentity prose.\n<IdentityExtension type=\"project\">\nTeam notes.\n</IdentityExtension>\n</Identity>\n",
		"\n", "\r\n"))
	state := capturedDeployedState(t, map[string][]byte{"test-runner.md": filled})

	got := requireProbedOwned(t, state, "test-runner.md")

	want := wantOwned(t, domain.ArtifactAgent, []byte(probedAgentText))
	if got.Hash != want.Hash {
		t.Errorf("Owned.Hash = %s; want the record of the unedited form %s", got.Hash, want.Hash)
	}
}

func TestDeployFlow_ProbeOwnedContentChangesWithDeployOwnedEdit(t *testing.T) {
	edited := []byte(strings.Replace(probedAgentText, "Identity prose.", "Identity prose, edited.", 1))
	state := capturedDeployedState(t, map[string][]byte{"test-runner.md": edited})

	got := requireProbedOwned(t, state, "test-runner.md")

	unedited := wantOwned(t, domain.ArtifactAgent, []byte(probedAgentText))
	if got.Hash == unedited.Hash {
		t.Errorf("Owned.Hash = %s; want a record that differs from the unedited one", got.Hash)
	}
}

func TestDeployFlow_ProbeFillsWholeFileOwnedContentOfDeployedSkill(t *testing.T) {
	state := capturedDeployedState(t, map[string][]byte{"lean-tdd.md": []byte(probedSkillText)})

	got := requireProbedOwned(t, state, "lean-tdd.md")

	want := wantOwned(t, domain.ArtifactSkill, []byte(probedSkillText))
	if got.Hash != want.Hash {
		t.Errorf("DeployedState[lean-tdd.md].Owned = %+v; want %+v", got, want)
	}
}

func TestDeployFlow_ProbeLeavesOwnedNilForAbsentFile(t *testing.T) {
	state := capturedDeployedState(t, map[string][]byte{})

	if got, ok := state["test-runner.md"]; !ok || got.Owned != nil {
		t.Errorf("absent file state = %+v, present in map = %v; want Owned nil", got, ok)
	}
}

func TestDeployFlow_ProbeLeavesOwnedNilAndMarksParseFailedForMalformedRegionAgent(t *testing.T) {
	broken := strings.Replace(probedAgentText, "</Identity>\n", "", 1)
	state := capturedDeployedState(t, map[string][]byte{"test-runner.md": []byte(broken)})

	got := state["test-runner.md"]

	if got.Owned != nil {
		t.Errorf("Owned = %+v; want nil when the record cannot be computed", got.Owned)
	}
	if !got.ParseFailed {
		t.Error("ParseFailed = false; a malformed-region agent must stay a conflict, not be silently never edit-checked")
	}
}
