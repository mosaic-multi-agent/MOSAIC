package plan_test

// edit_detection_agent_test.go specifies how the agent classifier treats edits of
// Deploy-owned content: an edit makes an up-to-date agent an update with a backup, and edits
// elsewhere, entries recorded before the content record, and unrecorded files do not.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func agentEntry(owned *domain.OwnedContent) *domain.ManifestEntry {
	e := editEntry(agentRef(editAgentKey), editAgentPath, owned)
	return &e
}

func TestClassifyAgent_EditedDeployOwnedContent_UpdateWithReasonAndBackup(t *testing.T) {
	item := buildEditAgentItem(t, agentEntry(ownedRecord("sha256:recorded")), editDeployed(ownedRecord("sha256:edited")), "1.0")

	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %v; want ActionUpdate although every version matches", item.Action)
	}
	if !hasExactReason(item, editReason) {
		t.Errorf("Reasons = %q; want %q", item.Reasons, editReason)
	}
	if !item.BackupBeforeWrite {
		t.Error("BackupBeforeWrite = false; an edit-caused update backs the file up first")
	}
	if item.Conflict != nil {
		t.Errorf("Conflict = %+v; an edit-caused update is not a conflict", item.Conflict)
	}
}

func TestClassifyAgent_EditedRegions_ReasonNamesRegionsInDiffOrder(t *testing.T) {
	recorded := ownedRecord("sha256:recorded", "Identity", "h1", "Constraints", "h2", "Gone", "h3")
	current := ownedRecord("sha256:edited", "Identity", "h1", "Constraints", "changed", "New", "h4")

	item := buildEditAgentItem(t, agentEntry(recorded), editDeployed(current), "1.0")

	want := editReason + " (regions: Constraints, Gone, New)"
	if !hasExactReason(item, want) {
		t.Errorf("Reasons = %q; want %q", item.Reasons, want)
	}
}

func TestClassifyAgent_EditWithoutRegionInformation_ReasonHasNoRegionSuffix(t *testing.T) {
	item := buildEditAgentItem(t, agentEntry(ownedRecord("sha256:recorded")), editDeployed(ownedRecord("sha256:edited")), "1.0")

	for _, r := range item.Reasons {
		if strings.Contains(r, "regions:") {
			t.Errorf("reason %q names regions although no region hashes were recorded", r)
		}
	}
}

func TestClassifyAgent_EditAndStaleVersion_BothReasonsAndBackup(t *testing.T) {
	item := buildEditAgentItem(t, agentEntry(ownedRecord("sha256:recorded")), editDeployed(ownedRecord("sha256:edited")), "2.0")

	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %v; want ActionUpdate", item.Action)
	}
	if !hasExactReason(item, editReason) {
		t.Errorf("Reasons = %q; missing %q", item.Reasons, editReason)
	}
	if !strings.Contains(reasonText(item), `changed from "1.0" to "2.0"`) {
		t.Errorf("Reasons = %q; missing the version reason next to the edit reason", item.Reasons)
	}
	if !item.BackupBeforeWrite {
		t.Error("BackupBeforeWrite = false; the edit still needs its backup when versions are stale too")
	}
}

func TestClassifyAgent_StaleVersionWithoutEdit_NoBackupAndNoEditReason(t *testing.T) {
	owned := ownedRecord("sha256:same")
	item := buildEditAgentItem(t, agentEntry(owned), editDeployed(ownedRecord("sha256:same")), "2.0")

	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %v; want ActionUpdate", item.Action)
	}
	if item.BackupBeforeWrite {
		t.Error("BackupBeforeWrite = true for a version-only update")
	}
	if hasExactReason(item, editReason) {
		t.Errorf("Reasons = %q; edit reason given without an edit", item.Reasons)
	}
}

func TestClassifyAgent_RecordMatches_UnchangedEvenWhenWholeFileHashDiffers(t *testing.T) {
	// Edits in project regions, custom regions or frontmatter change the whole-file hash but
	// leave the Deploy-owned record equal.
	item := buildEditAgentItem(t, agentEntry(ownedRecord("sha256:same", "Identity", "h1")), editDeployed(ownedRecord("sha256:same", "Identity", "h1")), "1.0")

	if item.Action != domain.ActionUnchanged {
		t.Errorf("Action = %v; want ActionUnchanged (Reasons %q)", item.Action, item.Reasons)
	}
	if item.BackupBeforeWrite {
		t.Error("BackupBeforeWrite = true for an unchanged item")
	}
}

func TestClassifyAgent_EntryRecordedBeforeContentRecord_NoEditCheck(t *testing.T) {
	item := buildEditAgentItem(t, agentEntry(nil), editDeployed(ownedRecord("sha256:anything")), "1.0")

	if item.Action != domain.ActionUnchanged {
		t.Errorf("Action = %v; want ActionUnchanged for an entry without a content record (Reasons %q)", item.Action, item.Reasons)
	}
	if item.BackupBeforeWrite || hasExactReason(item, editReason) {
		t.Errorf("edit handling applied to a pre-change entry: backup=%v reasons=%q", item.BackupBeforeWrite, item.Reasons)
	}
}

func TestClassifyAgent_EntryRecordedBeforeContentRecord_StaleVersionStillUpdatesWithoutBackup(t *testing.T) {
	item := buildEditAgentItem(t, agentEntry(nil), editDeployed(ownedRecord("sha256:anything")), "2.0")

	if item.Action != domain.ActionUpdate || item.BackupBeforeWrite {
		t.Errorf("Action = %v, backup = %v; want a plain version update without backup", item.Action, item.BackupBeforeWrite)
	}
}

func TestClassifyAgent_RecordNotComputableFromDisk_NoEditCheck(t *testing.T) {
	item := buildEditAgentItem(t, agentEntry(ownedRecord("sha256:recorded")), editDeployed(nil), "1.0")

	if item.Action != domain.ActionUnchanged {
		t.Errorf("Action = %v; want ActionUnchanged when the disk record is unavailable", item.Action)
	}
}

func TestClassifyAgent_UnrecordedFile_ConflictNotEditUpdate(t *testing.T) {
	item := buildEditAgentItem(t, nil, editDeployed(ownedRecord("sha256:edited")), "1.0")

	if item.Action != domain.ActionConflict {
		t.Fatalf("Action = %v; want ActionConflict for a file with no manifest record", item.Action)
	}
	if item.Conflict == nil || !item.Conflict.ManifestMissing {
		t.Errorf("Conflict = %+v; want ManifestMissing", item.Conflict)
	}
	if hasExactReason(item, editReason) {
		t.Errorf("Reasons = %q; an unrecorded file is not an edit update", item.Reasons)
	}
}

func TestClassifyAgent_ParseFailedFile_StaysConflictDespiteRecordDifference(t *testing.T) {
	deployed := editDeployed(nil)
	deployed.ParseFailed = true

	item := buildEditAgentItem(t, agentEntry(ownedRecord("sha256:recorded")), deployed, "1.0")

	if item.Action != domain.ActionConflict {
		t.Errorf("Action = %v; want ActionConflict for an unparseable file", item.Action)
	}
}
