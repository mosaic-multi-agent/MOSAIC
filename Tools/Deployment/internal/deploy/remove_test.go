package deploy_test

// remove_test.go covers the executor's removal of requested workspace files: the file is
// deleted after a backup, the manifest entry goes with it on request, other entries stay,
// an absent file is not a failure, and a dry run touches nothing.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func removalPlan(workspace string, removals []domain.Removal, items ...domain.PlanItem) domain.Plan {
	p := newPlan(workspace, items...)
	p.Removals = removals
	return p
}

func TestRemoval_DeletesFileAndDropsManifestEntry(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD", "agents/keep.md": "KEEP"})
	keep := newAgentItem("keep", "agents/keep.md", domain.ActionUnchanged)

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}, keep), false, false)

	if res.Partial != nil {
		t.Fatalf("Partial = %v; want nil", res.Partial)
	}
	if fileExistsAt(t, filepath.Join(ws, "agents", "old.md")) {
		t.Error("removed file still exists")
	}
	paths := manifestTargetPaths(t, ws)
	if hasPath(paths, "agents/old.md") {
		t.Errorf("saved manifest still has the removed entry: %v", paths)
	}
	if !hasPath(paths, "agents/keep.md") {
		t.Errorf("saved manifest lost an unrelated entry: %v", paths)
	}
	assertFileHolds(t, ws, "agents/keep.md", "KEEP")
}

func TestRemoval_ReportsRemovedRecordWithBackupPath(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), false, false)

	if len(res.Removals) != 1 {
		t.Fatalf("Removals = %d records; want 1", len(res.Removals))
	}
	rec := res.Removals[0]
	if rec.Taken != domain.RemovalRemoved {
		t.Errorf("Taken = %q; want %q", rec.Taken, domain.RemovalRemoved)
	}
	if rec.BackupPath == "" || !fileExistsAt(t, rec.BackupPath) {
		t.Fatalf("BackupPath %q is empty or missing on disk", rec.BackupPath)
	}
}

func TestRemoval_DropEntryFalse_DeletesFileButKeepsEntry(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	rm := agentRemoval("agents/old.md")
	rm.DropEntry = false

	execWithPlan(t, removalPlan(ws, []domain.Removal{rm}), false, false)

	if fileExistsAt(t, filepath.Join(ws, "agents", "old.md")) {
		t.Error("file still exists; a removal without DropEntry must still delete it")
	}
	if !hasPath(manifestTargetPaths(t, ws), "agents/old.md") {
		t.Error("manifest entry dropped although DropEntry is false")
	}
}

func TestRemoval_AbsentFile_NotAFailureAndEntryStillDropped(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/gone.md": "GONE"})
	if err := os.Remove(filepath.Join(ws, "agents", "gone.md")); err != nil {
		t.Fatal(err)
	}

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/gone.md")}), false, false)

	if res.Partial != nil {
		t.Fatalf("Partial = %v; an already-absent file must not fail the run", res.Partial)
	}
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalAbsent {
		t.Fatalf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalAbsent)
	}
	if res.Removals[0].BackupPath != "" {
		t.Errorf("BackupPath = %q for an absent file; want empty", res.Removals[0].BackupPath)
	}
	if hasPath(manifestTargetPaths(t, ws), "agents/gone.md") {
		t.Error("entry of the absent file was not dropped")
	}
}

func TestRemoval_PathWrittenByTheRunIsNeverDeleted(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/shared.md": "OLD"})
	item := newAgentItem("shared", "agents/shared.md", domain.ActionUpdate)

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/shared.md")}, item), false, false)

	assertFileHolds(t, ws, "agents/shared.md", "# new")
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalKept {
		t.Fatalf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalKept)
	}
	if !hasPath(manifestTargetPaths(t, ws), "agents/shared.md") {
		t.Error("manifest lost the entry the run wrote for the kept path")
	}
	if got := backupFilesFor(t, ws, "agents/shared.md"); len(got) != 0 {
		t.Errorf("backup created for a kept path: %v", got)
	}
}

func TestRemoval_RecordsFollowRequestOrder(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/a.md": "A", "agents/b.md": "B"})

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/b.md"), agentRemoval("agents/a.md")}), false, false)

	if len(res.Removals) != 2 || res.Removals[0].Removal.Path != "agents/b.md" || res.Removals[1].Removal.Path != "agents/a.md" {
		t.Fatalf("Removals = %+v; want records in request order b, a", res.Removals)
	}
}

func TestRemoval_DryRun_ReportsAndTouchesNothing(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	manifestPath := filepath.Join(ws, ".mosaic", "manifest.yaml")
	before := readFile(t, manifestPath)

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), false, true)

	assertFileHolds(t, ws, "agents/old.md", "OLD")
	if got := readFile(t, manifestPath); string(got) != string(before) {
		t.Error("dry run changed the manifest file")
	}
	if len(backupFilesFor(t, ws, "agents/old.md")) != 0 {
		t.Error("dry run created a backup")
	}
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalRemoved {
		t.Fatalf("Removals = %+v; want one record Taken=%q (would be removed)", res.Removals, domain.RemovalRemoved)
	}
	if res.Removals[0].BackupPath != "" {
		t.Errorf("BackupPath = %q on a dry run; want empty", res.Removals[0].BackupPath)
	}
	for _, e := range res.Manifest.Entries {
		if filepath.ToSlash(e.TargetPath) == "agents/old.md" {
			t.Error("ExecResult.Manifest still lists the entry a dry run would drop")
		}
	}
}

func TestRemoval_DryRun_AbsentFileReportedAbsent(t *testing.T) {
	ws := t.TempDir()

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/none.md")}), false, true)

	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalAbsent {
		t.Fatalf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalAbsent)
	}
}

func TestRemoval_InvalidPaths_RejectedBeforeAnyWrite(t *testing.T) {
	abs, _ := filepath.Abs(filepath.Join(t.TempDir(), "x.md"))
	cases := map[string]string{"empty": "", "parent escape": "../outside.md", "nested escape": "agents/../../outside.md", "absolute": abs}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			ws := t.TempDir()
			seedAgents(t, ws, map[string]string{"agents/ok.md": "OK"})
			plan := removalPlan(ws, []domain.Removal{agentRemoval("agents/ok.md"), {Ref: agentRefFor("agents/bad.md"), Path: bad}})

			ex := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector())
			_, err := ex.Execute(context.Background(), deploy.ExecRequest{Plan: plan, MosaicRoot: t.TempDir(), Content: fixedContent(nil)})

			if !errors.Is(err, deploy.ErrInvalidRemoval) {
				t.Fatalf("err = %v; want ErrInvalidRemoval", err)
			}
			assertFileHolds(t, ws, "agents/ok.md", "OK")
		})
	}
}

func TestReplacement_InvalidPath_RejectedBeforeAnyWrite(t *testing.T) {
	ws := t.TempDir()
	plan := newPlan(ws)
	plan.Replacements = []domain.Replacement{{Ref: agentRefFor("agents/x.md"), Path: "../outside.md"}}

	ex := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector())
	_, err := ex.Execute(context.Background(), deploy.ExecRequest{Plan: plan, MosaicRoot: t.TempDir(), Content: fixedContent(nil)})

	if !errors.Is(err, deploy.ErrInvalidRemoval) {
		t.Fatalf("err = %v; want ErrInvalidRemoval", err)
	}
}
