package screens_test

// review_edit_backup_test.go verifies that the plan review shows, for an update caused by a
// local edit of Deploy-owned content, the reason and the backup that happens before the file
// is overwritten, and shows no backup line for other updates.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

const editedLocallyReason = "Deploy-owned content was edited locally (regions: Identity)"

func TestReviewScreen_EditCausedUpdate_ShowsReasonThenBackupLine(t *testing.T) {
	p := reasonedUpdatePlan(domain.PlanItem{
		Action:            domain.ActionUpdate,
		Reasons:           []string{editedLocallyReason},
		BackupBeforeWrite: true,
	})

	lines := viewLines(screens.NewReviewScreen(p, 120, 80, plainStyles()).View())

	reason := lineIndex(lines, editedLocallyReason)
	backup := lineIndex(lines, domain.BackupThenReplaceText)
	if reason < 0 || backup < 0 {
		t.Fatalf("reason line index %d, backup line index %d; want both shown:\n%s", reason, backup, strings.Join(lines, "\n"))
	}
	if backup <= reason {
		t.Errorf("backup line (%d) must follow the reasons (%d)", backup, reason)
	}
}

func TestReviewScreen_UpdateWithoutBackupMarker_ShowsNoBackupLine(t *testing.T) {
	p := reasonedUpdatePlan(domain.PlanItem{
		Action:  domain.ActionUpdate,
		Reasons: []string{`version changed from "1.0" to "2.0"`},
	})

	view := screens.NewReviewScreen(p, 120, 80, plainStyles()).View()

	if strings.Contains(view, domain.BackupThenReplaceText) {
		t.Errorf("backup line shown for an update without a backup:\n%s", view)
	}
}
