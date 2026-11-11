package plan_test

// edit_detection_skill_test.go specifies the same edit rule for skill files: the Deploy-owned
// content of a skill file is the whole file, for the entry file and every other file.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func skillEntries(entryOwned, otherOwned *domain.OwnedContent) []domain.ManifestEntry {
	return []domain.ManifestEntry{
		editEntry(skillRef(editSkillKey), editEntryPath, entryOwned),
		editEntry(skillRef(editSkillKey), editOtherPath, otherOwned),
	}
}

// skillDeployed returns deployed states for both files; the companion file carries no version.
func skillDeployed(entryOwned, otherOwned *domain.OwnedContent) map[string]domain.DeployedArtifactState {
	other := deployedState("sha256:current-other", "", "", "")
	other.Owned = otherOwned
	return map[string]domain.DeployedArtifactState{
		editEntryPath: editDeployed(entryOwned),
		editOtherPath: other,
	}
}

func TestClassifySkill_EditedEntryFile_UpdateWithReasonAndBackup(t *testing.T) {
	items := buildEditSkillItems(t,
		skillEntries(ownedRecord("sha256:r1"), ownedRecord("sha256:r2")),
		skillDeployed(ownedRecord("sha256:edited"), ownedRecord("sha256:r2")))

	entry := items[editEntryPath]
	if entry.Action != domain.ActionUpdate {
		t.Fatalf("entry file Action = %v; want ActionUpdate although versions match", entry.Action)
	}
	if !hasExactReason(entry, editReason) {
		t.Errorf("entry file Reasons = %q; want %q", entry.Reasons, editReason)
	}
	if !entry.BackupBeforeWrite {
		t.Error("entry file BackupBeforeWrite = false")
	}
	if other := items[editOtherPath]; other.Action != domain.ActionUnchanged || other.BackupBeforeWrite {
		t.Errorf("untouched companion file: Action = %v, backup = %v; want unchanged without backup", other.Action, other.BackupBeforeWrite)
	}
}

func TestClassifySkill_EditedCompanionFile_UpdateWithReasonAndBackup(t *testing.T) {
	items := buildEditSkillItems(t,
		skillEntries(ownedRecord("sha256:r1"), ownedRecord("sha256:r2")),
		skillDeployed(ownedRecord("sha256:r1"), ownedRecord("sha256:edited")))

	other := items[editOtherPath]
	if other.Action != domain.ActionUpdate || !hasExactReason(other, editReason) || !other.BackupBeforeWrite {
		t.Errorf("companion file: Action = %v, Reasons = %q, backup = %v; want update with edit reason and backup",
			other.Action, other.Reasons, other.BackupBeforeWrite)
	}
}

func TestClassifySkill_EditReasonHasNoRegionSuffix(t *testing.T) {
	items := buildEditSkillItems(t,
		skillEntries(ownedRecord("sha256:r1"), ownedRecord("sha256:r2")),
		skillDeployed(ownedRecord("sha256:edited"), ownedRecord("sha256:r2")))

	if !hasExactReason(items[editEntryPath], editReason) {
		t.Errorf("Reasons = %q; want exactly %q for a skill", items[editEntryPath].Reasons, editReason)
	}
}

func TestClassifySkill_RecordsMatch_Unchanged(t *testing.T) {
	items := buildEditSkillItems(t,
		skillEntries(ownedRecord("sha256:r1"), ownedRecord("sha256:r2")),
		skillDeployed(ownedRecord("sha256:r1"), ownedRecord("sha256:r2")))

	for path, it := range items {
		if it.Action != domain.ActionUnchanged || it.BackupBeforeWrite {
			t.Errorf("%s: Action = %v, backup = %v; want unchanged", path, it.Action, it.BackupBeforeWrite)
		}
	}
}

func TestClassifySkill_EntryRecordedBeforeContentRecord_NoEditCheck(t *testing.T) {
	items := buildEditSkillItems(t,
		skillEntries(nil, nil),
		skillDeployed(ownedRecord("sha256:anything"), ownedRecord("sha256:anything")))

	for path, it := range items {
		if it.Action != domain.ActionUnchanged || it.BackupBeforeWrite {
			t.Errorf("%s: Action = %v, backup = %v; want unchanged without backup for a pre-change entry", path, it.Action, it.BackupBeforeWrite)
		}
	}
}

func TestClassifySkill_UnrecordedFile_ConflictNotEditUpdate(t *testing.T) {
	items := buildEditSkillItems(t, nil,
		skillDeployed(ownedRecord("sha256:edited"), ownedRecord("sha256:edited")))

	for path, it := range items {
		if it.Action != domain.ActionConflict {
			t.Errorf("%s: Action = %v; want ActionConflict without a manifest record", path, it.Action)
		}
		if hasExactReason(it, editReason) {
			t.Errorf("%s: unrecorded file given the edit reason", path)
		}
	}
}

func TestClassifySkill_EditAndStaleVersion_BothReasonsAndBackup(t *testing.T) {
	entries := skillEntries(ownedRecord("sha256:r1"), ownedRecord("sha256:r2"))
	deployed := skillDeployed(ownedRecord("sha256:edited"), ownedRecord("sha256:r2"))
	d := deployed[editEntryPath]
	d.Version = "0.5"
	deployed[editEntryPath] = d

	it := buildEditSkillItems(t, entries, deployed)[editEntryPath]

	if it.Action != domain.ActionUpdate || !hasExactReason(it, editReason) || len(it.Reasons) < 2 || !it.BackupBeforeWrite {
		t.Errorf("Action = %v, Reasons = %q, backup = %v; want update with the edit reason plus a version reason and backup",
			it.Action, it.Reasons, it.BackupBeforeWrite)
	}
}
