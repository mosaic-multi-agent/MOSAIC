package deploy_test

// registration_replace_test.go covers replacements: existing files that a run overwrites
// after a backup because the caller asked for it. That includes a hook registration file,
// which is otherwise never modified once it exists.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const regTarget = ".github/hooks/mosaic-logger.json"

func hookRef() domain.ArtifactRef {
	return domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "logger"}
}

func replacementOf(path string) domain.Replacement {
	return domain.Replacement{Ref: hookRef(), Path: path, Reason: "moved"}
}

// execReplacing runs the executor with the given plan and hooks and returns the result and
// the collector so gaps can be inspected.
func execReplacing(t *testing.T, plan domain.Plan, hooks []domain.HookPlan, atomic, dryRun bool) (deploy.ExecResult, *spyCollector) {
	t.Helper()
	spy := newSpyCollector()
	req := deploy.ExecRequest{
		Plan:       plan,
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte("# new")),
		Hooks:      hooks,
		Atomic:     atomic,
		DryRun:     dryRun,
	}
	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), spy).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res, spy
}

func planReplacing(ws string, reps ...domain.Replacement) domain.Plan {
	p := newPlan(ws)
	p.Replacements = reps
	return p
}

func TestRegistrationReplace_ExistingFileReplacedByFragmentAfterBackup(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	res, spy := execReplacing(t, planReplacing(ws, replacementOf(regTarget)), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, regTarget, `{"fragment":1}`)
	path := assertBackupHolds(t, ws, regTarget, `{"user":"edited"}`)
	if spy.hasGapKind(domain.GapHookRegistration) {
		t.Error("GapHookRegistration emitted for a replaced registration file")
	}
	if len(res.Replacements) != 1 || res.Replacements[0].BackupPath != path {
		t.Errorf("Replacements = %+v; want one record with BackupPath %q", res.Replacements, path)
	}
}

func TestRegistrationReplace_NotRequested_ExistingFileKeptAndGapEmitted(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	res, spy := execReplacing(t, newPlan(ws), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, regTarget, `{"user":"edited"}`)
	if !spy.hasGapKind(domain.GapHookRegistration) {
		t.Error("GapHookRegistration not emitted when replacement was not requested")
	}
	if len(backupFilesFor(t, ws, regTarget)) != 0 {
		t.Error("backup created although no replacement was requested")
	}
	if len(res.Replacements) != 0 {
		t.Errorf("Replacements = %+v; want none", res.Replacements)
	}
}

func TestRegistrationReplace_OnlyTheRequestedPathIsReplaced(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	_, spy := execReplacing(t, planReplacing(ws, replacementOf(".github/hooks/other.json")), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, regTarget, `{"user":"edited"}`)
	if !spy.hasGapKind(domain.GapHookRegistration) {
		t.Error("GapHookRegistration not emitted for a registration file that was not requested")
	}
}

func TestRegistrationReplace_AbsentTarget_CreatedWithoutBackup(t *testing.T) {
	ws := t.TempDir()
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	res, _ := execReplacing(t, planReplacing(ws, replacementOf(regTarget)), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, regTarget, `{"fragment":1}`)
	if len(backupFilesFor(t, ws, regTarget)) != 0 {
		t.Error("backup created for a file that did not exist")
	}
	if len(res.Replacements) != 1 || res.Replacements[0].BackupPath != "" {
		t.Errorf("Replacements = %+v; want one record with empty BackupPath", res.Replacements)
	}
}

func TestReplacement_HookFileOverwrittenAfterBackup(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "NEW HOOK")

	res, _ := execReplacing(t, planReplacing(ws, replacementOf(".github/hooks/h.sh")), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, ".github/hooks/h.sh", "NEW HOOK")
	path := assertBackupHolds(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	if len(res.Replacements) != 1 || res.Replacements[0].BackupPath != path {
		t.Errorf("Replacements = %+v; want BackupPath %q", res.Replacements, path)
	}
}

func TestReplacement_PlanItemOverwrittenAfterBackup(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/x.md": "OLD AGENT"})
	item := newAgentItem("x", "agents/x.md", domain.ActionUpdate)
	rep := domain.Replacement{Ref: item.Ref, Path: "agents/x.md"}

	execReplacing(t, planWithItems(ws, item, rep), nil, false, false)

	assertFileHolds(t, ws, "agents/x.md", "# new")
	assertBackupHolds(t, ws, "agents/x.md", "OLD AGENT")
}

func planWithItems(ws string, item domain.PlanItem, reps ...domain.Replacement) domain.Plan {
	p := newPlan(ws, item)
	p.Replacements = reps
	return p
}

func TestReplacement_PathTheRunDoesNotWrite_LeftAloneWithoutBackup(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, "notes/keep.txt", "KEEP")

	res, _ := execReplacing(t, planReplacing(ws, replacementOf("notes/keep.txt")), nil, false, false)

	assertFileHolds(t, ws, "notes/keep.txt", "KEEP")
	if len(backupFilesFor(t, ws, "notes/keep.txt")) != 0 {
		t.Error("backup created for a path the run does not write")
	}
	if len(res.Replacements) != 1 || res.Replacements[0].BackupPath != "" {
		t.Errorf("Replacements = %+v; want one record with empty BackupPath", res.Replacements)
	}
}

func TestReplacement_DryRun_ReportsAndTouchesNothing(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	res, _ := execReplacing(t, planReplacing(ws, replacementOf(regTarget)), []domain.HookPlan{hp}, false, true)

	assertFileHolds(t, ws, regTarget, `{"user":"edited"}`)
	if _, err := os.Stat(filepath.Join(ws, ".mosaic", "backups")); err == nil {
		t.Error("dry run created the backups directory")
	}
	if len(res.Replacements) != 1 || res.Replacements[0].BackupPath != "" {
		t.Errorf("Replacements = %+v; want one record with empty BackupPath", res.Replacements)
	}
}

func TestMovingRequest_UnreadableHookSource_IsAFailureNotAGap(t *testing.T) {
	ws := t.TempDir()
	hp := domain.HookPlan{
		Supported: true,
		TargetDir: ".github/hooks",
		Files:     []domain.HookFile{{SourcePath: filepath.Join(t.TempDir(), "missing.sh"), TargetName: "h.sh"}},
	}

	res, _ := execReplacing(t, planReplacing(ws, replacementOf(".github/hooks/h.sh")), []domain.HookPlan{hp}, false, false)

	if res.Partial == nil {
		t.Error("Partial is nil; an unreadable hook source in a request with replacements must fail the run")
	}
}
