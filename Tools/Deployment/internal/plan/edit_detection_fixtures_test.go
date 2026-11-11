package plan_test

// edit_detection_fixtures_test.go holds the builders for the edit-detection classification
// tests: a one-agent input and a one-skill input whose manifest entry and deployed state carry
// Deploy-owned content records.

import (
	"context"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const (
	editAgentKey  = "worker"
	editAgentPath = "agents/worker.md"
	editSkillKey  = "lean-tdd"
	editEntryPath = "skills/lean-tdd/SKILL.md"
	editOtherPath = "skills/lean-tdd/companion.md"

	// editReason is the fixed reason shown for an edit of Deploy-owned content.
	editReason = "Deploy-owned content was edited locally"
)

// ownedRecord returns a content record with the given hash and, optionally, region hashes
// given as name, hash pairs.
func ownedRecord(hash string, regionPairs ...string) *domain.OwnedContent {
	rec := &domain.OwnedContent{Hash: hash}
	for i := 0; i+1 < len(regionPairs); i += 2 {
		rec.Regions = append(rec.Regions, domain.RegionHash{Name: regionPairs[i], Hash: regionPairs[i+1]})
	}
	return rec
}

// editEntry returns the manifest entry of an up-to-date artifact carrying the given record.
// Its whole-file hash differs from every deployed hash used below, as it does once a project
// region is filled.
func editEntry(ref domain.ArtifactRef, path string, owned *domain.OwnedContent) domain.ManifestEntry {
	e := makeManifestEntry(ref, path, "1.0", "sha256:recorded-whole-file")
	e.Owned = owned
	return e
}

// editDeployed returns the deployed state of a file whose versions match the catalog's 1.0
// and whose recomputed record is owned.
func editDeployed(owned *domain.OwnedContent) domain.DeployedArtifactState {
	d := deployedState("sha256:current-whole-file", "1.0", "1.0", "1.0")
	d.Owned = owned
	return d
}

// buildEditAgentItem plans one worker agent at catalog version agentVersion.
func buildEditAgentItem(t *testing.T, entry *domain.ManifestEntry, deployed domain.DeployedArtifactState, agentVersion string) domain.PlanItem {
	t.Helper()
	agent := makeAgent(editAgentKey, agentVersion)
	in := plan.Input{
		Catalog: &fakeCatalog{
			orchestrator: makeOrchestrator(),
			workers:      []domain.Agent{agent},
			workflows:    []domain.Workflow{makeWorkflow("test-wf", agent.Key)},
		},
		Module:        newFakeModule(),
		Mode:          domain.ModeUpdateWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		WorkflowIDs:   []string{"test-wf"},
		Manifest:      absentSnapshot(),
		DeployedState: map[string]domain.DeployedArtifactState{editAgentPath: deployed},
		Models: map[string]domain.ModelSelection{
			agent.Key:      {ModelID: "test-model", Origin: domain.OriginHarnessList},
			"orchestrator": {ModelID: "test-model", Origin: domain.OriginHarnessList},
		},
	}
	if entry != nil {
		in.Manifest = makeManifestWithEntry(*entry)
	}
	p, err := plan.New().Build(context.Background(), in)
	must(t, err)
	item, ok := findItem(p.Items, editAgentKey)
	if !ok {
		t.Fatalf("plan has no item for agent %q", editAgentKey)
	}
	return item
}

// buildEditSkillItems plans one two-file skill; entries are the manifest entries (nil = no
// manifest) and deployed the deployed state per target path. It returns the item per path.
func buildEditSkillItems(t *testing.T, entries []domain.ManifestEntry, deployed map[string]domain.DeployedArtifactState) map[string]domain.PlanItem {
	t.Helper()
	skill := makeMultiFileSkill(editSkillKey, []string{"SKILL.md", "companion.md"})
	in := buildInputWithMultiFileSkill(skill, absentSnapshot(), deployed)
	if entries != nil {
		in.Manifest = presentSnapshot(domain.Manifest{
			SchemaVersion: manifest.SchemaVersion,
			Harnesses:     []domain.HarnessSection{{HarnessID: "test-harness", UpdatedAt: time.Now()}},
			Entries:       entries,
		})
	}
	p, err := plan.New().Build(context.Background(), in)
	must(t, err)
	out := map[string]domain.PlanItem{}
	for _, it := range allSkillItems(p.Items, editSkillKey) {
		out[it.TargetPath] = it
	}
	return out
}

// hasExactReason reports whether the item carries the reason text as one of its reasons.
func hasExactReason(item domain.PlanItem, text string) bool {
	for _, r := range item.Reasons {
		if r == text {
			return true
		}
	}
	return false
}
