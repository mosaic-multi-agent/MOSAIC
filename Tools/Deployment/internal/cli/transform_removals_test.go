package cli_test

// transform_removals_test.go covers how the human transform output lists the files a
// pair run removes or replaces, and that these never change the documented exit codes.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

const (
	rmLegacyPath  = ".github/agents/reviewer.md"
	rmHookPath    = ".github/hooks/mosaic-logger.json"
	rmAgentPath   = ".github/agents/writer.agent.md"
	rmBackupPath  = "/ws/.mosaic/backups/.github/agents/reviewer.md.20260101.bak"
	rmBackupPath2 = "/ws/.mosaic/backups/.github/hooks/mosaic-logger.json.20260101.bak"
)

// pairRemovalResult builds a clean pair result carrying two removals and two replacements.
func pairRemovalResult(dryRun bool, withBackups bool) app.TransformHarnessResult {
	removals := []app.TransformFileChange{
		{Kind: domain.ArtifactAgent, Key: "reviewer", Path: rmLegacyPath, Reason: "legacy agent form migrated"},
		{Kind: domain.ArtifactHook, Key: "logger", Path: ".github/hooks/old-logger.json"},
	}
	replacements := []app.TransformFileChange{
		{Kind: domain.ArtifactAgent, Key: "writer", Path: rmAgentPath, Reason: "rewritten in place"},
		{Kind: domain.ArtifactHook, Key: "logger", Path: rmHookPath, Reason: "registration replaced"},
	}
	if withBackups {
		removals[0].BackupPath = rmBackupPath
		replacements[1].BackupPath = rmBackupPath2
	}
	return app.TransformHarnessResult{
		SourceHarnessID: "ghcp-cli",
		TargetHarnessID: "vscode-ghcp",
		Workspace:       tfWorkspace,
		DryRun:          dryRun,
		Files: []app.TransformFileOutcome{
			{Kind: domain.ArtifactAgent, Key: "writer", SourcePath: ".github/agents/writer.agent.md",
				DestinationPath: rmAgentPath, Action: app.PlanWriteOverwrite, Status: app.StatusTransformed},
		},
		Transformed:  1,
		Removals:     removals,
		Replacements: replacements,
	}
}

func lineIndex(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			return i
		}
	}
	return -1
}

func assertRemovalListing(t *testing.T, out string, withBackups bool) {
	t.Helper()
	lines := strings.Split(out, "\n")
	removeHdr := lineIndex(lines, "removals")
	replaceHdr := lineIndex(lines, "replacements")
	if removeHdr < 0 || replaceHdr < 0 {
		t.Fatalf("output lacks the removals/replacements sections:\n%s", out)
	}
	fileLine := lineIndex(lines, "transformed [")
	counts := lineIndex(lines, "transformed:")
	if fileLine >= 0 && removeHdr < fileLine {
		t.Errorf("removals section must come after the per-file lines:\n%s", out)
	}
	if counts < replaceHdr {
		t.Errorf("sections must come before the counts line:\n%s", out)
	}
	want := []string{
		"remove agent:reviewer " + rmLegacyPath + " - legacy agent form migrated",
		"remove hook:logger .github/hooks/old-logger.json",
		"replace agent:writer " + rmAgentPath + " - rewritten in place",
		"replace hook:logger " + rmHookPath + " - registration replaced",
	}
	for _, w := range want {
		if lineIndex(lines, w) < 0 {
			t.Errorf("output lacks line %q:\n%s", w, out)
		}
	}
	if i := lineIndex(lines, "remove hook:logger .github/hooks/old-logger.json"); i >= 0 &&
		strings.Contains(lines[i], " - ") {
		t.Errorf("an entry without a reason must have no reason suffix: %q", lines[i])
	}
	hasBackup := strings.Contains(out, "backup: "+rmBackupPath) && strings.Contains(out, "backup: "+rmBackupPath2)
	if withBackups != hasBackup {
		t.Errorf("backup lines present = %v, want %v:\n%s", hasBackup, withBackups, out)
	}
	if !withBackups && strings.Contains(out, "backup:") {
		t.Errorf("no backup line expected when no backup path is set:\n%s", out)
	}
	if !strings.Contains(lines[counts], "removed: 2") || !strings.Contains(lines[counts], "replaced: 2") {
		t.Errorf("counts line must carry removed and replaced counts: %q", lines[counts])
	}
}

func TestTransformCmd_RealRun_ListsEveryRemovalAndReplacement(t *testing.T) {
	svc := &spyService{transformResp: pairRemovalResult(false, true)}

	_, out, _ := runTransform(svc, transformArgs("--yes")...)

	assertRemovalListing(t, out, true)
}

func TestTransformCmd_DryRun_ListsEveryRemovalAndReplacement(t *testing.T) {
	svc := &spyService{transformResp: pairRemovalResult(true, false)}

	_, out, _ := runTransform(svc, transformArgs("--dry-run")...)

	assertRemovalListing(t, out, false)
}

func TestTransformCmd_OnlyReplacements_ShowsNoRemovalsSection(t *testing.T) {
	res := pairRemovalResult(false, false)
	res.Removals = nil

	_, out, _ := runTransform(&spyService{transformResp: res}, transformArgs("--yes")...)

	if !strings.Contains(out, "replace agent:writer") || !strings.Contains(out, "replaced: 2") {
		t.Errorf("replacements must be listed:\n%s", out)
	}
	if strings.Contains(out, "removals") || strings.Contains(out, "removed:") {
		t.Errorf("no removals section or count expected:\n%s", out)
	}
}

func TestTransformCmd_NoRemovals_OutputHasNoRemovalText(t *testing.T) {
	_, out, _ := runTransform(&spyService{transformResp: planResult()}, transformArgs("--yes")...)

	for _, s := range []string{"removals", "replacements", "removed:", "replaced:"} {
		if strings.Contains(out, s) {
			t.Errorf("output without removals must not contain %q:\n%s", s, out)
		}
	}
}

func TestTransformCmd_RemovalsAndReplacements_KeepExitCodeMeanings(t *testing.T) {
	clean := pairRemovalResult(false, true)
	followUp := pairRemovalResult(false, true)
	followUp.UnresolvedModels = []string{"writer"}
	failed := pairRemovalResult(false, true)
	failed.Failed = 1
	failed.Files[0].Status = app.StatusFailed
	cases := []struct {
		name string
		res  app.TransformHarnessResult
		want int
	}{
		{"removals and replacements alone", clean, cli.ExitSuccess},
		{"with follow-up", followUp, cli.ExitWithSkips},
		{"with a failed file", failed, cli.ExitFailure},
	}
	for _, tc := range cases {
		code, out, _ := runTransform(&spyService{transformResp: tc.res}, transformArgs("--yes")...)
		if code != tc.want {
			t.Errorf("%s: exit code = %d, want %d", tc.name, code, tc.want)
		}
		if !strings.Contains(out, "remove agent:reviewer") {
			t.Errorf("%s: removals must be listed:\n%s", tc.name, out)
		}
	}
}
