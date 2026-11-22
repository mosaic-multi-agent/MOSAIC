package app_test

// thirdformat_edit_detection_parity_test.go specifies that Update treats the five record-present
// hand-edit scenarios of a deployed agent identically for the test-only agent format and for
// Markdown: a frontmatter edit and a project or custom region edit are not edits of Deploy-owned
// content, while an edit of the body outside regions or of a Deploy-owned region is reported,
// naming the region where known, and backed up before the update. It also specifies that the
// user-owned keys of a test-format agent survive an update.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// twinEditRun deploys to both sides, applies edit to the deployed runner of both and runs Update
// (conflicts skipped).
func twinEditRun(t *testing.T, edit func([]byte) []byte) (p twinPair, test, md twinObservation) {
	t.Helper()
	p = twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, edit)
	}
	test, md = runTwinStep(t, p, twinUpdateStep)
	return p, test, md
}

// requireTwinNoEdit asserts the observation holds no edit of Deploy-owned content for the runner.
func requireTwinNoEdit(t *testing.T, side twinSide, obs twinObservation) {
	t.Helper()
	it := twinItem(t, obs, owRunner)
	for _, r := range it.Reasons {
		if strings.HasPrefix(r, owEditReason) {
			t.Errorf("%s twin: reasons = %q; want no %q reason", side.name(), it.Reasons, owEditReason)
		}
	}
	if it.Action == domain.ActionConflict {
		t.Errorf("%s twin: action = %s; a file the manifest record vouches for must not become a conflict", side.name(), it.Action)
	}
	if backups := gamBackups(t, side.Workspace, twinAgentRel(t, side, owRunner)); it.BackupBeforeWrite || len(backups) != 0 {
		t.Errorf("%s twin: BackupBeforeWrite = %v, backups taken = %d; want no backup", side.name(), it.BackupBeforeWrite, len(backups))
	}
	if len(obs.OwnedKeyDifferences) != 0 {
		t.Errorf("%s twin: OwnedKeyDifferences = %v; want none", side.name(), obs.OwnedKeyDifferences)
	}
}

// requireTwinEdit asserts the observation reports exactly the given edit reason for the runner,
// an update, and a backup of the edited file.
func requireTwinEdit(t *testing.T, side twinSide, obs twinObservation, wantReason string) {
	t.Helper()
	it := twinItem(t, obs, owRunner)
	found := false
	for _, r := range it.Reasons {
		found = found || r == wantReason
	}
	if !found {
		t.Errorf("%s twin: reasons = %q; want %q", side.name(), it.Reasons, wantReason)
	}
	if it.Action != domain.ActionUpdate {
		t.Errorf("%s twin: action = %s; want %s", side.name(), it.Action, domain.ActionUpdate)
	}
	if !it.BackupBeforeWrite {
		t.Errorf("%s twin: BackupBeforeWrite = false; want the edited file backed up before the update", side.name())
	}
	if backups := gamBackups(t, side.Workspace, twinAgentRel(t, side, owRunner)); len(backups) != 1 {
		t.Errorf("%s twin: backups = %d; want one backup of the edited file", side.name(), len(backups))
	}
	if len(obs.OwnedKeyDifferences) != 0 {
		t.Errorf("%s twin: OwnedKeyDifferences = %v; want none", side.name(), obs.OwnedKeyDifferences)
	}
}

func TestTwinParity_EditDetection_FrontmatterKeyEdit_IsNotAnEditOfOwnedContent(t *testing.T) {
	for _, field := range []string{"model", twinUserKey} {
		t.Run(field, func(t *testing.T) {
			// Arrange / Act
			p, test, md := twinEditRun(t, twinSetField(field, "edited-value"))

			// Assert
			assertTwinParity(t, "frontmatter key edit", test, md)
			requireTwinNoEdit(t, p.Test, test)
			requireTwinNoEdit(t, p.Markdown, md)
		})
	}
}

func TestTwinParity_EditDetection_BodyEditOutsideRegions_IsReportedAndBackedUp(t *testing.T) {
	// Arrange / Act
	p, test, md := twinEditRun(t, twinBodyEdit(owOutsideEdit(t)))

	// Assert
	assertTwinParity(t, "body edit outside regions", test, md)
	want := owEditReason + " (regions: " + domain.OutsideRegionsName + ")"
	requireTwinEdit(t, p.Test, test, want)
	requireTwinEdit(t, p.Markdown, md, want)
}

func TestTwinParity_EditDetection_DeployOwnedRegionEdit_NamesTheRegionAndIsBackedUp(t *testing.T) {
	// Arrange / Act
	p, test, md := twinEditRun(t, twinBodyEdit(owIdentityEdit(t)))

	// Assert
	assertTwinParity(t, "deploy-owned region edit", test, md)
	want := owEditReason + " (regions: Identity)"
	requireTwinEdit(t, p.Test, test, want)
	requireTwinEdit(t, p.Markdown, md, want)
}

func TestTwinParity_EditDetection_ProjectRegionEdit_IsNotAnEditOfOwnedContent(t *testing.T) {
	// Arrange / Act
	p, test, md := twinEditRun(t, twinBodyEdit(owProjectEdit(t)))

	// Assert
	assertTwinParity(t, "project region edit", test, md)
	requireTwinNoEdit(t, p.Test, test)
	requireTwinNoEdit(t, p.Markdown, md)
}

func TestTwinParity_EditDetection_CustomRegionEdit_IsNotAnEditOfOwnedContent(t *testing.T) {
	// Arrange / Act
	p, test, md := twinEditRun(t, twinBodyEdit(owCustomEdit(t)))

	// Assert
	assertTwinParity(t, "custom region edit", test, md)
	requireTwinNoEdit(t, p.Test, test)
	requireTwinNoEdit(t, p.Markdown, md)
}

func TestTwinParity_Update_UserOwnedKeysSurvive(t *testing.T) {
	// Arrange: a stale agent holding a user-owned key
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinChain(twinSetField(twinUserKey, twinUserValue), twinAgeVersion()))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update over a user-owned key", test, md)
	if got := twinItem(t, test, owRunner).Action; got != domain.ActionUpdate {
		t.Errorf("plan action of %s = %s; want %s, so the key must survive a rewrite", owRunner, got, domain.ActionUpdate)
	}
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		if got := obs.Agents[owRunner].Frontmatter[twinUserKey]; got != twinUserValue {
			t.Errorf("%s: user-owned key %s = %q after the update; want %q", label, twinUserKey, got, twinUserValue)
		}
	}
}

// twinNativeMember is a format-native member of the test format that never reaches the
// canonical form; it can only survive an update through the prior bytes.
const twinNativeMember = `"native": {"kept": "nativevalue"}`

func TestTwinParity_Update_TestFormatNativeMemberSurvives(t *testing.T) {
	// Arrange: a stale test-format agent holding a native member next to its user-owned key
	p := twinDeployed(t)
	editTwinAgent(t, p.Test, owRunner, twinChain(twinSetField(twinUserKey, twinUserValue), twinAgeVersion()))
	rel := twinAgentRel(t, p.Test, owRunner)
	raw := strings.TrimRight(string(gamRead(t, p.Test.Workspace, rel)), "\n")
	if !strings.HasSuffix(raw, "}") {
		t.Fatalf("test-format agent does not end with a JSON object: %q", raw)
	}
	gamWrite(t, p.Test.Workspace, rel, strings.TrimSuffix(raw, "}")+",\n  "+twinNativeMember+"\n}\n")

	// Act
	test, _ := runTwinStep(t, twinPair{Test: p.Test, Markdown: p.Markdown}, twinUpdateStep)

	// Assert
	if got := twinItem(t, test, owRunner).Action; got != domain.ActionUpdate {
		t.Fatalf("plan action of %s = %s; want %s", owRunner, got, domain.ActionUpdate)
	}
	after := string(gamRead(t, p.Test.Workspace, rel))
	if !strings.Contains(after, "nativevalue") {
		t.Errorf("the native member of the test format file did not survive the update:\n%s", after)
	}
	if got := test.Agents[owRunner].Frontmatter[twinUserKey]; got != twinUserValue {
		t.Errorf("user-owned key %s = %q after the update; want %q", twinUserKey, got, twinUserValue)
	}
}
