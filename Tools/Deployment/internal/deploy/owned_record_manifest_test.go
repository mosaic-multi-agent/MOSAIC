package deploy_test

// owned_record_manifest_test.go specifies the Deploy-owned content record the executor writes
// with every agent and skill entry, and that entries recorded before it keep no record until
// their item is written.

import (
	"context"
	"testing"
	"time"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

const ownedAgentBody = `---
id: 7
version: 1.0.0
name: runner
---

Outside prose.

<Identity type="core">
Identity prose.
<IdentityExtension type="project">
</IdentityExtension>
</Identity>
`

const ownedSkillBody = "---\nname: lean-tdd\nversion: 1.0\n---\n\nSkill text.\n"

func runWrite(t *testing.T, item domain.PlanItem, content string) (string, deploy.ExecResult) {
	t.Helper()
	workspace := t.TempDir()
	res := runWriteIn(t, workspace, item, content)
	return workspace, res
}

func runWriteIn(t *testing.T, workspace string, item domain.PlanItem, content string) deploy.ExecResult {
	t.Helper()
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, item),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte(content)),
	}
	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

func TestExecute_WrittenAgentEntry_WithoutCallback_RecordsOwnedContentOfWholeWrittenBytes(t *testing.T) {
	item := newAgentItem("runner", "agents/runner.md", domain.ActionCreate)

	_, res := runWrite(t, item, ownedAgentBody)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok || entry.Owned == nil {
		t.Fatalf("entry = %+v, found = %v; want an entry with an Owned record", entry, ok)
	}
	want, err := transform.OwnedContentOf(domain.ArtifactAgent, []byte(ownedAgentBody))
	if err != nil {
		t.Fatalf("OwnedContentOf: %v", err)
	}
	if entry.Owned.Hash != want.Hash || len(entry.Owned.Regions) != len(want.Regions) || len(want.Regions) == 0 {
		t.Errorf("entry.Owned = %+v; want %+v with region hashes", *entry.Owned, want)
	}
	if entry.ContentHash != manifest.Hash([]byte(ownedAgentBody)) {
		t.Errorf("ContentHash = %q; want the whole-file hash of the written bytes", entry.ContentHash)
	}
}

func TestExecute_WrittenSkillEntry_RecordsWholeFileOwnedContent(t *testing.T) {
	item := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "lean-tdd"},
		SourcePath: "testdata/lean-tdd.md",
		TargetPath: "skills/lean-tdd/SKILL.md",
		Action:     domain.ActionCreate,
	}

	_, res := runWrite(t, item, ownedSkillBody)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok || entry.Owned == nil {
		t.Fatalf("entry = %+v, found = %v; want an entry with an Owned record", entry, ok)
	}
	want, err := transform.OwnedContentOf(domain.ArtifactSkill, []byte(ownedSkillBody))
	if err != nil {
		t.Fatalf("OwnedContentOf: %v", err)
	}
	if entry.Owned.Hash != want.Hash || entry.Owned.Regions != nil {
		t.Errorf("entry.Owned = %+v; want hash %s and no regions", *entry.Owned, want.Hash)
	}
}

func TestExecute_RecordIsSavedToManifestFile(t *testing.T) {
	item := newAgentItem("runner", "agents/runner.md", domain.ActionCreate)

	workspace, _ := runWrite(t, item, ownedAgentBody)

	snap, err := manifest.NewStore().Load(workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	entry, ok := snap.Manifest.Lookup(item.Ref)
	if !ok || entry.Owned == nil || entry.Owned.Hash == "" {
		t.Errorf("loaded entry = %+v, found = %v; want the Owned record persisted", entry, ok)
	}
}

// seedPreChangeEntry saves a manifest whose agent entry has no Owned record and writes the file.
func seedPreChangeEntry(t *testing.T, workspace string, item domain.PlanItem) {
	t.Helper()
	writeExisting(t, workspace, item.TargetPath, []byte(ownedAgentBody))
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: "test-harness", UpdatedAt: time.Now()}},
		Entries: []domain.ManifestEntry{{
			Harness: "test-harness", Ref: item.Ref, TargetPath: item.TargetPath,
			Version: "1.0", ContentHash: manifest.Hash([]byte(ownedAgentBody)),
			DeployedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
	if err := manifest.NewStore().Save(workspace, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestExecute_UnchangedPreChangeEntry_KeepsNoRecord(t *testing.T) {
	workspace := t.TempDir()
	item := newAgentItem("runner", "agents/runner.md", domain.ActionUnchanged)
	seedPreChangeEntry(t, workspace, item)

	res := runWriteIn(t, workspace, item, ownedAgentBody)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok || entry.Owned != nil {
		t.Errorf("entry = %+v, found = %v; an unchanged item keeps its prior entry without a record", entry, ok)
	}
}

func TestExecute_UpdatedPreChangeEntry_ReceivesRecordAtItsWrite(t *testing.T) {
	workspace := t.TempDir()
	item := newAgentItem("runner", "agents/runner.md", domain.ActionUpdate)
	seedPreChangeEntry(t, workspace, item)

	res := runWriteIn(t, workspace, item, ownedAgentBody)

	entry, ok := res.Manifest.Lookup(item.Ref)
	if !ok || entry.Owned == nil || entry.Owned.Hash == "" {
		t.Errorf("entry = %+v, found = %v; want the new record written with the item", entry, ok)
	}
}
