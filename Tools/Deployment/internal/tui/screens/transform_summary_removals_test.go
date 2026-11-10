package screens_test

// transform_summary_removals_test.go verifies that the transform summary screen shows the
// files a pair run removed or replaced, or would on a dry run.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func pairSummaryResult(dryRun bool) app.TransformHarnessResult {
	return app.TransformHarnessResult{
		SourceHarnessID: "ghcp-cli",
		TargetHarnessID: "vscode-ghcp",
		Workspace:       "/ws",
		DryRun:          dryRun,
		Files: []app.TransformFileOutcome{
			{Kind: domain.ArtifactAgent, Key: "writer", DestinationPath: ".github/agents/writer.agent.md",
				Action: app.PlanWriteOverwrite, Status: app.StatusTransformed},
		},
		Transformed: 1,
		TodoPath:    "/ws/mosaic-deploy-todo.md",
		Removals: []app.TransformFileChange{
			{Kind: domain.ArtifactAgent, Key: "reviewer", Path: ".github/agents/reviewer.md",
				Reason: "legacy form migrated", BackupPath: "/ws/.mosaic/backups/reviewer.bak"},
			{Kind: domain.ArtifactHook, Key: "logger", Path: ".github/hooks/old.json"},
		},
		Replacements: []app.TransformFileChange{
			{Kind: domain.ArtifactHook, Key: "logger", Path: ".github/hooks/mosaic-logger.json",
				Reason: "registration replaced"},
		},
	}
}

func summaryView(r app.TransformHarnessResult) string {
	return screens.NewTransformSummaryScreen(r, 200, 60, plainStyles()).View()
}

func TestTransformSummary_PairRun_ShowsRemovalsAndReplacements(t *testing.T) {
	view := summaryView(pairSummaryResult(false))

	for _, want := range []string{
		"remove agent:reviewer .github/agents/reviewer.md", "legacy form migrated",
		"remove hook:logger .github/hooks/old.json",
		"replace hook:logger .github/hooks/mosaic-logger.json", "registration replaced",
		"backup: /ws/.mosaic/backups/reviewer.bak",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("summary lacks %q:\n%s", want, view)
		}
	}
}

func TestTransformSummary_PairRun_ListsComeAfterFilesBeforeTrailer(t *testing.T) {
	view := summaryView(pairSummaryResult(false))

	file := strings.Index(view, "writer.agent.md")
	remove := strings.Index(view, "remove agent:reviewer")
	replace := strings.Index(view, "replace hook:logger")
	todo := strings.Index(view, "todo:")
	if file < 0 || remove < 0 || replace < 0 || todo < 0 {
		t.Fatalf("missing parts (file=%d remove=%d replace=%d todo=%d):\n%s", file, remove, replace, todo, view)
	}
	if !(file < remove && remove < replace && replace < todo) {
		t.Errorf("order must be files, removals, replacements, trailer:\n%s", view)
	}
}

func TestTransformSummary_DryRun_ShowsListsWithDryRunIndicator(t *testing.T) {
	view := summaryView(pairSummaryResult(true))

	if !strings.Contains(view, "nothing is written") {
		t.Errorf("dry run indicator missing:\n%s", view)
	}
	if !strings.Contains(view, "remove agent:reviewer") || !strings.Contains(view, "replace hook:logger") {
		t.Errorf("dry run must list removals and replacements:\n%s", view)
	}
}

func TestTransformSummary_NoBackupPath_ShowsNoBackupLine(t *testing.T) {
	r := pairSummaryResult(true)
	r.Removals[0].BackupPath = ""

	view := summaryView(r)

	if strings.Contains(view, "backup:") {
		t.Errorf("no backup line expected without a backup path:\n%s", view)
	}
	if !strings.Contains(view, "remove agent:reviewer") {
		t.Errorf("removal must still be listed:\n%s", view)
	}
}

func TestTransformSummary_NoRemovals_RendersWithoutRemovalLines(t *testing.T) {
	r := pairSummaryResult(false)
	r.Removals = nil
	r.Replacements = nil

	view := summaryView(r)

	if strings.Contains(view, "remove ") || strings.Contains(view, "replace ") {
		t.Errorf("result without removals must not show removal lines:\n%s", view)
	}
}
