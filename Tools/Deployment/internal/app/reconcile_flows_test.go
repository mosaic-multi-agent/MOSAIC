package app_test

// reconcile_flows_test.go verifies that every write flow (deploy, update, update workflows,
// agents, hooks) reconciles the whole manifest with the disk before planning: entries of any
// harness whose files are missing never reach the planner, are listed on the plan shown for
// review, and are never written back by a preview or dry run. Old-format migration drops are
// listed the same way with their own reason.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	rfGhostPath      = "ghost.md"
	rfOtherGhostPath = ".github/agents/other-ghost.agent.md"
	rfKeptPath       = "kept.md"
	rfOtherKeptPath  = ".claude/agents/kept.md"
)

// droppedView is the part of a dropped entry a user sees.
type droppedView struct{ Harness, Path, Reason string }

func droppedViews(entries []domain.DroppedEntry) []droppedView {
	var out []droppedView
	for _, d := range entries {
		out = append(out, droppedView{d.Entry.Harness, d.Entry.TargetPath, d.Reason})
	}
	return out
}

func rfEntry(harness, key, path string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:     harness,
		Ref:         domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
		TargetPath:  path,
		Version:     "1.0",
		ContentHash: manifest.Hash([]byte(path)),
	}
}

// rfManifest records, for the run harness, a ghost (file missing) and a kept entry (file
// present), and the same pair for two other harnesses.
func rfManifest() domain.Manifest {
	return domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses: []domain.HarnessSection{
			{HarnessID: "claude-code"}, {HarnessID: "ghcp-cli"}, {HarnessID: "stub-harness"}},
		Entries: []domain.ManifestEntry{
			rfEntry("claude-code", "kept", rfOtherKeptPath),
			rfEntry("ghcp-cli", "other-ghost", rfOtherGhostPath),
			rfEntry("stub-harness", "ghost", rfGhostPath),
			rfEntry("stub-harness", "kept", rfKeptPath),
		},
	}
}

// rfWriteKept puts the files of the entries that are meant to be present on disk.
func rfWriteKept(t *testing.T, ws string) {
	t.Helper()
	for _, rel := range []string{rfKeptPath, rfOtherKeptPath} {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("deployed"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// rfRun runs one operation of the table over a manifest with a ghost entry of the run harness
// and one of another harness. It returns the plan shown for review and the planner input.
func rfRun(t *testing.T, op crOperation, snap manifest.Snapshot, dryRun bool) (domain.Plan, *capturingPlanner, domain.RunSummary) {
	t.Helper()
	r := newDrReview(domain.Answered)
	r.answer = domain.ConfirmAnswer{Status: domain.Answered, Confirm: true}
	deps, ws, _ := drDeps(t, r)
	rfWriteKept(t, ws)
	cp := &capturingPlanner{result: drPlan(ws)}
	deps.Planner = cp
	deps.Manifest = &stubManifestStore{snap: snap}

	sum, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true, dryRun: dryRun})

	if err != nil {
		t.Fatalf("%s: %v", op.name, err)
	}
	if len(r.plans) != 1 {
		t.Fatalf("%s: plan was reviewed %d times, want once", op.name, len(r.plans))
	}
	return r.plans[0], cp, sum
}

func rfPresentSnapshot(m domain.Manifest) manifest.Snapshot {
	return manifest.Snapshot{State: manifest.StatePresent, Manifest: m}
}

func TestReconcileFlows_MissingEntriesOfEveryHarnessAreListedOnTheReviewedPlan(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			reviewed, _, _ := rfRun(t, op, rfPresentSnapshot(rfManifest()), false)

			got := droppedViews(reviewed.DroppedEntries)

			want := []droppedView{
				{"ghcp-cli", rfOtherGhostPath, domain.DropReasonMissing},
				{"stub-harness", rfGhostPath, domain.DropReasonMissing},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("dropped entries on the reviewed plan = %+v, want %+v", got, want)
			}
		})
	}
}

func TestReconcileFlows_PlannerNeverSeesAnEntryWhoseFileIsMissing(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			_, cp, _ := rfRun(t, op, rfPresentSnapshot(rfManifest()), false)

			if cp.capturedInput == nil {
				t.Fatal("planner.Build was never called")
			}
			var paths []string
			for _, e := range cp.capturedInput.Manifest.Manifest.Entries {
				paths = append(paths, e.TargetPath)
			}
			if !reflect.DeepEqual(paths, []string{rfKeptPath}) {
				t.Errorf("planner manifest entries = %v, want only the present entry of the run harness", paths)
			}
		})
	}
}

func TestReconcileFlows_DryRunPlanCarriesTheDroppedEntries(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			reviewed, _, sum := rfRun(t, op, rfPresentSnapshot(rfManifest()), true)

			if sum.DryRunPlan == nil {
				t.Fatal("summary carries no dry-run plan")
			}
			got := droppedViews(sum.DryRunPlan.DroppedEntries)
			if want := droppedViews(reviewed.DroppedEntries); len(want) != 2 || !reflect.DeepEqual(got, want) {
				t.Errorf("dry-run plan dropped entries = %+v, want the two reviewed ones %+v", got, want)
			}
		})
	}
}

func TestReconcileFlows_OldFormatMigrationDropsAreListedWithTheirOwnReasonBeforeTheMissingOnes(t *testing.T) {
	migrated := rfEntry("", "docs", "docs/notes.md")
	snap := rfPresentSnapshot(rfManifest())
	snap.MigratedFrom = manifest.LegacySchemaVersion
	snap.MigrationDropped = []domain.DroppedEntry{{Entry: migrated, Reason: domain.DropReasonUnattributed}}
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			reviewed, _, _ := rfRun(t, op, snap, false)

			got := droppedViews(reviewed.DroppedEntries)

			want := []droppedView{
				{"", "docs/notes.md", domain.DropReasonUnattributed},
				{"ghcp-cli", rfOtherGhostPath, domain.DropReasonMissing},
				{"stub-harness", rfGhostPath, domain.DropReasonMissing},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("dropped entries = %+v, want the migration drop first, then the missing files: %+v", got, want)
			}
		})
	}
}

func TestReconcileFlows_NothingMissingMeansNothingDropped(t *testing.T) {
	m := rfManifest()
	m.Entries = []domain.ManifestEntry{rfEntry("stub-harness", "kept", rfKeptPath)}
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			reviewed, _, _ := rfRun(t, op, rfPresentSnapshot(m), false)

			if len(reviewed.DroppedEntries) != 0 {
				t.Errorf("dropped entries = %+v, want none", droppedViews(reviewed.DroppedEntries))
			}
		})
	}
}

// TestReconcileFlows_DryRunLeavesTheManifestFileByteIdentical verifies a dry run reconciles in
// memory only: the manifest file on disk, which still records the missing files, is untouched.
func TestReconcileFlows_DryRunLeavesTheManifestFileByteIdentical(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			r := newDrReview(domain.SkippedOne)
			deps, ws, _ := drDeps(t, r)
			rfWriteKept(t, ws)
			store := manifest.NewStore()
			deps.Manifest = store
			if err := store.Save(ws, rfManifest()); err != nil {
				t.Fatalf("seed manifest: %v", err)
			}
			manifestFile := filepath.Join(ws, ".mosaic", "manifest.yaml")
			before, err := os.ReadFile(manifestFile)
			if err != nil {
				t.Fatalf("read seeded manifest: %v", err)
			}
			tree := crSnapshotDir(t, ws)

			sum, err := op.run(context.Background(), app.New(deps), ws, crFlags{dryRun: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			after, err := os.ReadFile(manifestFile)
			if err != nil {
				t.Fatalf("read manifest after dry run: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("a dry run changed the manifest file:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			if !reflect.DeepEqual(crSnapshotDir(t, ws), tree) {
				t.Error("a dry run changed the workspace")
			}
			if sum.DryRunPlan == nil || len(sum.DryRunPlan.DroppedEntries) != 2 {
				t.Errorf("dry-run plan lists %v dropped entries, want the two missing ones",
					droppedViews(dryDropped(sum)))
			}
		})
	}
}

func dryDropped(sum domain.RunSummary) []domain.DroppedEntry {
	if sum.DryRunPlan == nil {
		return nil
	}
	return sum.DryRunPlan.DroppedEntries
}
