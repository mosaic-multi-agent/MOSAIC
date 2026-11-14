package screens_test

// review_reasons_test.go verifies that the plan review shows every reason of an item, also when
// version deltas are listed, and that removal and replacement entries state the backup action
// as a future action.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

const reviewDetailIndent = "               " // 15 spaces: the indent of an item's detail lines

func reasonedUpdatePlan(item domain.PlanItem) domain.Plan {
	item.Ref = agentRef("worker")
	item.TargetPath = "Agents/worker.agent.md"
	return planWithItems(domain.ModeUpdateWorkspace, []domain.PlanItem{item})
}

// viewLines returns the rendered review split into lines.
func viewLines(view string) []string { return strings.Split(view, "\n") }

// lineIndex returns the index of the first line that equals the indented text, or -1.
func lineIndex(lines []string, text string) int {
	for i, l := range lines {
		if strings.TrimRight(l, " ") == reviewDetailIndent+text {
			return i
		}
	}
	return -1
}

func TestReviewScreen_ItemWithSeveralReasons_ShowsEachOnItsOwnLineInOrder(t *testing.T) {
	// Arrange
	p := reasonedUpdatePlan(domain.PlanItem{
		Action:  domain.ActionUpdate,
		Reasons: []string{"first cause", "second cause", "third cause"},
	})

	// Act
	lines := viewLines(screens.NewReviewScreen(p, 120, 80, plainStyles()).View())

	// Assert
	prev := -1
	for _, r := range []string{"first cause", "second cause", "third cause"} {
		i := lineIndex(lines, r)
		if i < 0 {
			t.Fatalf("no detail line %q in view:\n%s", r, strings.Join(lines, "\n"))
		}
		if i <= prev {
			t.Errorf("reason %q is not shown after the previous reason", r)
		}
		prev = i
	}
}

func TestReviewScreen_ItemWithVersionDeltasAndSeveralReasons_ShowsEveryReason(t *testing.T) {
	// Arrange
	p := reasonedUpdatePlan(domain.PlanItem{
		Action: domain.ActionUpdate,
		Stale: []domain.VersionDelta{
			{Field: "version", Deployed: "1.0", Source: "2.0"},
		},
		Reasons: []string{`version changed from "1.0" to "2.0"`, "renamed from .github/agents/worker.md"},
	})

	// Act
	view := screens.NewReviewScreen(p, 120, 80, plainStyles()).View()

	// Assert
	for _, want := range []string{`version changed from "1.0" to "2.0"`, "renamed from .github/agents/worker.md"} {
		if lineIndex(viewLines(view), want) < 0 {
			t.Errorf("view lacks the reason %q although version deltas are listed:\n%s", want, view)
		}
	}
}

func TestReviewScreen_VersionDelta_IsShownOnceThroughItsReason(t *testing.T) {
	// Arrange: the deployed value is unique so occurrences can be counted
	p := reasonedUpdatePlan(domain.PlanItem{
		Action:  domain.ActionUpdate,
		Stale:   []domain.VersionDelta{{Field: "version", Deployed: "9.8.7", Source: "9.9.0"}},
		Reasons: []string{`version changed from "9.8.7" to "9.9.0"`},
	})

	// Act
	view := screens.NewReviewScreen(p, 120, 80, plainStyles()).View()

	// Assert
	if lineIndex(viewLines(view), `version changed from "9.8.7" to "9.9.0"`) < 0 {
		t.Errorf("the reason line is missing:\n%s", view)
	}
	if n := strings.Count(view, "9.8.7"); n != 1 {
		t.Errorf("deployed version appears %d times, want once (the reason; no separate delta line):\n%s", n, view)
	}
}

func TestReviewScreen_ConflictItemWithReasons_ShowsReasonsAndConflictLines(t *testing.T) {
	// Arrange
	p := reasonedUpdatePlan(domain.PlanItem{
		Action:   domain.ActionConflict,
		Reasons:  []string{"file exists but is not recorded", "migrates from the legacy file Agents/worker.md"},
		Conflict: &domain.LocalModification{ManifestMissing: true},
	})

	// Act
	view := screens.NewReviewScreen(p, 120, 80, plainStyles()).View()

	// Assert
	for _, want := range []string{"file exists but is not recorded", "migrates from the legacy file Agents/worker.md", "manifest missing"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestReviewScreen_ItemWarnings_AreShownAfterTheReasonsWithAWarningPrefix(t *testing.T) {
	// Arrange
	p := reasonedUpdatePlan(domain.PlanItem{
		Action:   domain.ActionUpdate,
		Reasons:  []string{"a cause"},
		Warnings: []string{"two catalogs may share this id", "another note"},
	})

	// Act
	lines := viewLines(screens.NewReviewScreen(p, 120, 80, plainStyles()).View())

	// Assert
	reason := lineIndex(lines, "a cause")
	w1 := lineIndex(lines, "warning: two catalogs may share this id")
	w2 := lineIndex(lines, "warning: another note")
	if reason < 0 || w1 < 0 || w2 < 0 {
		t.Fatalf("missing lines (reason=%d warning1=%d warning2=%d):\n%s", reason, w1, w2, strings.Join(lines, "\n"))
	}
	if !(reason < w1 && w1 < w2) {
		t.Errorf("order must be reason, then each warning in order (reason=%d w1=%d w2=%d)", reason, w1, w2)
	}
}

func TestReviewScreen_UnchangedItemWithoutReasons_ShowsNoDetailLine(t *testing.T) {
	// Arrange
	p := reasonedUpdatePlan(domain.PlanItem{Action: domain.ActionUnchanged})

	// Act
	view := screens.NewReviewScreen(p, 120, 80, plainStyles()).View()

	// Assert
	for _, l := range viewLines(view) {
		if strings.HasPrefix(l, reviewDetailIndent) && strings.TrimSpace(l) != "" {
			t.Errorf("unexpected detail line %q for an item without reasons", l)
		}
	}
}

func TestReviewScreen_Removal_StatesTheBackupActionAfterItsReason(t *testing.T) {
	// Arrange
	p := removalPlan()

	// Act
	lines := viewLines(screens.NewReviewScreen(p, 200, 60, plainStyles()).View())

	// Assert
	reason := lineIndex(lines, "legacy form migrated")
	action := lineIndex(lines, domain.BackupThenRemoveText)
	if reason < 0 || action < 0 {
		t.Fatalf("missing lines (reason=%d backup action=%d):\n%s", reason, action, strings.Join(lines, "\n"))
	}
	if action != reason+1 {
		t.Errorf("the backup action line (%d) must directly follow the removal reason (%d)", action, reason)
	}
}

func TestReviewScreen_RemovalWithoutReason_StillStatesTheBackupAction(t *testing.T) {
	// Arrange: the second removal of removalPlan has no reason
	p := removalPlan()

	// Act
	view := screens.NewReviewScreen(p, 200, 60, plainStyles()).View()

	// Assert
	if n := strings.Count(view, domain.BackupThenRemoveText); n != len(p.Removals) {
		t.Errorf("backup action shown %d times, want once per removal (%d):\n%s", n, len(p.Removals), view)
	}
}

func TestReviewScreen_Replacement_StatesTheBackupActionAfterItsReason(t *testing.T) {
	// Arrange
	p := removalPlan()

	// Act
	lines := viewLines(screens.NewReviewScreen(p, 200, 60, plainStyles()).View())

	// Assert
	reason := lineIndex(lines, "registration replaced")
	action := lineIndex(lines, domain.BackupThenReplaceText)
	if reason < 0 || action < 0 {
		t.Fatalf("missing lines (reason=%d backup action=%d):\n%s", reason, action, strings.Join(lines, "\n"))
	}
	if action != reason+1 {
		t.Errorf("the backup action line (%d) must directly follow the replacement reason (%d)", action, reason)
	}
}

func TestReviewScreen_NoPlanText_ReadsAsReferringToExistingBackupFiles(t *testing.T) {
	// Arrange
	p := removalPlan()

	// Act
	view := screens.NewReviewScreen(p, 200, 60, plainStyles()).View()

	// Assert
	for _, bad := range []string{"after backup", "replaced after", "(backed up)"} {
		if strings.Contains(view, bad) {
			t.Errorf("view contains %q, which reads as if backups already exist:\n%s", bad, view)
		}
	}
}
