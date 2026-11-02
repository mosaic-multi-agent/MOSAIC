package deploy_test

// remove_regression_test.go pins that a request with no removals and no replacements behaves
// exactly as before: same files, manifest and gaps, and hook problems stay gaps.

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestNoRemovals_PlainRunWritesFilesAndManifestAsBefore(t *testing.T) {
	ws := t.TempDir()
	item := newAgentItem("a", "agents/a.md", domain.ActionCreate)

	res := execWithPlan(t, newPlan(ws, item), false, false)

	assertFileHolds(t, ws, "agents/a.md", "# new")
	if !hasPath(manifestTargetPaths(t, ws), "agents/a.md") {
		t.Error("manifest lacks the written entry")
	}
	if res.Partial != nil || res.Reverted {
		t.Errorf("Partial=%v Reverted=%v; want a clean run", res.Partial, res.Reverted)
	}
	if len(res.Removals) != 0 || len(res.Replacements) != 0 {
		t.Errorf("Removals=%v Replacements=%v; want none", res.Removals, res.Replacements)
	}
	if len(backupFilesFor(t, ws, "agents/a.md")) != 0 {
		t.Error("backup created by a run without removals or replacements")
	}
}

func TestNoRemovals_ExistingRegistrationFileStillKeptWithGap(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	_, spy := execReplacing(t, newPlan(ws), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, regTarget, `{"user":"edited"}`)
	if !spy.hasGapKind(domain.GapHookRegistration) {
		t.Error("GapHookRegistration not emitted")
	}
}

func TestNoRemovals_UnreadableHookSource_StaysAGapNotAFailure(t *testing.T) {
	ws := t.TempDir()
	hp := domain.HookPlan{
		Supported: true,
		TargetDir: ".github/hooks",
		Files:     []domain.HookFile{{SourcePath: filepath.Join(t.TempDir(), "missing.sh"), TargetName: "h.sh"}},
	}

	for _, atomic := range []bool{false, true} {
		res, spy := execReplacing(t, newPlan(ws), []domain.HookPlan{hp}, atomic, false)

		if res.Partial != nil || res.Reverted {
			t.Errorf("atomic=%v: Partial=%v Reverted=%v; a hook problem without removals must stay a gap", atomic, res.Partial, res.Reverted)
		}
		if !spy.hasGapKind(domain.GapManualStep) {
			t.Errorf("atomic=%v: GapManualStep not emitted for the unreadable hook source", atomic)
		}
	}
}

func TestNoRemovals_HookFileStillOverwrittenWithoutBackupRequest(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "NEW HOOK")

	execReplacing(t, newPlan(ws), []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, ".github/hooks/h.sh", "NEW HOOK")
}
