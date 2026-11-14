package cli_test

// dryrun_edit_backup_test.go specifies the dry-run report of an update caused by a local edit
// of Deploy-owned content: its reason, then the backup that precedes the overwrite.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func editBackupPlan(ws string, mode domain.RunMode, backup bool) *domain.Plan {
	return &domain.Plan{Mode: mode, WorkspacePath: ws, Items: []domain.PlanItem{{
		Ref:               domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "alpha"},
		TargetPath:        ".claude/agents/alpha.md",
		Action:            domain.ActionUpdate,
		Reasons:           []string{"Deploy-owned content was edited locally", `version changed from "1.0" to "2.0"`},
		BackupBeforeWrite: backup,
	}}}
}

func TestDryRunCommands_EditCausedUpdate_PrintsReasonsThenBackupLine(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, editBackupPlan(ws, c.mode, true)))

			want := "  update .claude/agents/alpha.md\n" +
				"    - Deploy-owned content was edited locally\n" +
				"    - version changed from \"1.0\" to \"2.0\"\n" +
				"    - " + domain.BackupThenReplaceText + "\n"
			if !strings.Contains(out, want) {
				t.Errorf("report lacks the edit reason followed by the backup line.\nwant:\n%s\ngot:\n%s", want, out)
			}
		})
	}
}

func TestDryRunCommands_UpdateWithoutBackupMarker_PrintsNoBackupLine(t *testing.T) {
	ws := "/ws/dry-run"
	c := dryRunCommands(ws)[1]

	out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, editBackupPlan(ws, c.mode, false)))

	if strings.Contains(out, domain.BackupThenReplaceText) {
		t.Errorf("backup line printed for an update without a backup:\n%s", out)
	}
}
