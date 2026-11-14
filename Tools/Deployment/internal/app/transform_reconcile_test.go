package app_test

// transform_reconcile_test.go verifies the whole-deployment transform reconciles the manifest
// before planning: entries of any harness whose files are missing are listed in the result and
// on the reviewed plan, are never carried into the saved manifest, and a dry run leaves the
// manifest file as it was. The GHCP pair has its own planning path and is covered as well.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	trcSourceGhost = "ghost-source"
	trcOtherGhost  = "ghost-other"
)

// trcDirections are one pair outside the GHCP pair and both GHCP directions.
var trcDirections = [][2]string{{hClaude, hOpen}, {hCLI, hVSCode}, {hVSCode, hCLI}}

// trcWorkspace deploys the parity sample for src, then records two agents whose files do not
// exist: one of the source harness and one of a harness that is not part of the transform.
func trcWorkspace(t *testing.T, src string) (ws, sourceGhost, otherGhost string) {
	t.Helper()
	ws = parityDeploy(t, src)
	dir := rbModule(t, src).Descriptor().Paths.Agents.Project
	sourceGhost = dir + "/" + trcSourceGhost + ".md"
	otherHarness, otherGhost := hClaude, ".claude/agents/"+trcOtherGhost+".md"
	if src == hClaude {
		otherHarness, otherGhost = hOpen, ".opencode/agents/"+trcOtherGhost+".md"
	}
	m := gamManifest(t, ws)
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{HarnessID: otherHarness})
	m.Entries = append(m.Entries,
		rfEntry(src, trcSourceGhost, sourceGhost),
		rfEntry(otherHarness, trcOtherGhost, otherGhost))
	gamSaveManifest(t, ws, m)
	return ws, sourceGhost, otherGhost
}

func trcRequest(ws, src, tgt string, dry bool) app.TransformHarnessRequest {
	return app.TransformHarnessRequest{SourceHarnessID: src, TargetHarnessID: tgt, Workspace: ws,
		TargetModel: parityModel, AutoConfirmPlan: true, DryRun: dry}
}

func trcDroppedPaths(res app.TransformHarnessResult) []string {
	var out []string
	for _, d := range res.DroppedEntries {
		out = append(out, d.Path)
	}
	return out
}

func TestTransformReconcile_ResultListsEveryMissingEntryWithHarnessKindKeyPathAndReason(t *testing.T) {
	for _, dir := range trcDirections {
		for _, dry := range []bool{true, false} {
			name := dir[0] + "-to-" + dir[1]
			if dry {
				name += "/dry-run"
			}
			t.Run(name, func(t *testing.T) {
				ws, sourceGhost, otherGhost := trcWorkspace(t, dir[0])

				res, err := parityServiceWith(t).TransformHarness(context.Background(), trcRequest(ws, dir[0], dir[1], dry))

				if err != nil {
					t.Fatalf("transform: %v", err)
				}
				got := map[string]app.TransformDroppedEntry{}
				for _, d := range res.DroppedEntries {
					got[d.Path] = d
				}
				if len(got) != 2 {
					t.Fatalf("dropped entries = %+v, want the two missing files", res.DroppedEntries)
				}
				want := app.TransformDroppedEntry{Harness: dir[0], Kind: domain.ArtifactAgent, Key: trcSourceGhost,
					Path: sourceGhost, Reason: domain.DropReasonMissing}
				if got[sourceGhost] != want {
					t.Errorf("dropped source entry = %+v, want %+v", got[sourceGhost], want)
				}
				if d := got[otherGhost]; d.Key != trcOtherGhost || d.Reason != domain.DropReasonMissing || d.Harness == "" {
					t.Errorf("dropped entry of the other harness = %+v, want its key, harness and the missing reason", d)
				}
			})
		}
	}
}

func TestTransformReconcile_ReviewedPlanCarriesTheDroppedEntries(t *testing.T) {
	for _, dir := range trcDirections {
		t.Run(dir[0]+"-to-"+dir[1], func(t *testing.T) {
			ws, sourceGhost, otherGhost := trcWorkspace(t, dir[0])
			c := gamNewReviewCapture(ws)
			// The plan is only reviewed when it is not auto-confirmed.
			req := trcRequest(ws, dir[0], dir[1], false)
			req.AutoConfirmPlan = false

			if _, err := parityServiceWith(t, gamCaptureDeps(c)).TransformHarness(
				context.Background(), req); err != nil {
				t.Fatalf("transform: %v", err)
			}

			p := gplReviewedPlan(t, c)
			if got := rrDroppedPaths(p); !reflect.DeepEqual(sorted(got), sorted([]string{sourceGhost, otherGhost})) {
				t.Errorf("dropped entries on the reviewed plan = %v, want %v and %v", got, sourceGhost, otherGhost)
			}
			if gplRemovesPath(p, sourceGhost) {
				t.Errorf("the plan removes %s, a file that does not exist", sourceGhost)
			}
		})
	}
}

func TestTransformReconcile_DryRunLeavesTheManifestFileByteIdentical(t *testing.T) {
	for _, dir := range trcDirections {
		t.Run(dir[0]+"-to-"+dir[1], func(t *testing.T) {
			ws, _, _ := trcWorkspace(t, dir[0])
			before := rrManifestBytes(t, ws)

			if _, err := parityServiceWith(t).TransformHarness(context.Background(), trcRequest(ws, dir[0], dir[1], true)); err != nil {
				t.Fatalf("transform: %v", err)
			}

			if !bytes.Equal(before, rrManifestBytes(t, ws)) {
				t.Error("a dry run changed the manifest file")
			}
		})
	}
}

func TestTransformReconcile_ExecutedRunSavesNoEntryOfAMissingFile(t *testing.T) {
	for _, dir := range trcDirections {
		t.Run(dir[0]+"-to-"+dir[1], func(t *testing.T) {
			ws, sourceGhost, otherGhost := trcWorkspace(t, dir[0])

			if _, err := parityServiceWith(t).TransformHarness(context.Background(), trcRequest(ws, dir[0], dir[1], false)); err != nil {
				t.Fatalf("transform: %v", err)
			}

			m := gamManifest(t, ws)
			for _, e := range m.Entries {
				if e.TargetPath == sourceGhost || e.TargetPath == otherGhost {
					t.Errorf("the saved manifest still records the missing file %s", e.TargetPath)
				}
			}
			if !m.Holds(dir[1]) {
				t.Errorf("the saved manifest holds no entry of the target %s", dir[1])
			}
		})
	}
}

// TestTransformReconcile_OldFormatMigrationDropsAreListedWithTheirOwnReason verifies an entry the
// old-format upgrade could not attribute to a harness is reported by the transform with the
// upgrade's reason.
func TestTransformReconcile_OldFormatMigrationDropsAreListedWithTheirOwnReason(t *testing.T) {
	ws := t.TempDir()
	old := "schema_version: \"1\"\nharness_id: claude-code\nupdated_at: \"2026-11-02T09:15:00Z\"\nentries:\n" +
		"  - ref:\n      kind: agent\n      key: notes\n    target_path: docs/notes.md\n" +
		"    version: \"1.0.0\"\n    harness_version: \"\"\n    injections_version: \"\"\n" +
		"    content_hash: sha256:00\n    deployed_at: \"2026-11-01T10:00:00Z\"\n"
	if err := os.MkdirAll(filepath.Join(ws, ".mosaic"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".mosaic", "manifest.yaml"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := parityServiceWith(t).TransformHarness(context.Background(), trcRequest(ws, hClaude, hOpen, true))

	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	var found *app.TransformDroppedEntry
	for i, d := range res.DroppedEntries {
		if d.Path == "docs/notes.md" {
			found = &res.DroppedEntries[i]
		}
	}
	if found == nil {
		t.Fatalf("dropped entries %+v lack the entry the upgrade dropped", res.DroppedEntries)
	}
	if found.Reason != domain.DropReasonUnattributed {
		t.Errorf("reason = %q, want the upgrade's reason %q", found.Reason, domain.DropReasonUnattributed)
	}
}

func TestTransformReconcile_NothingMissing_ListsNothing(t *testing.T) {
	ws := parityDeploy(t, hClaude)
	before := rrManifestBytes(t, ws)

	res, err := parityServiceWith(t).TransformHarness(context.Background(), trcRequest(ws, hClaude, hOpen, true))

	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if len(res.DroppedEntries) != 0 {
		t.Errorf("dropped entries = %+v, want none", res.DroppedEntries)
	}
	if !bytes.Equal(before, rrManifestBytes(t, ws)) {
		t.Error("the manifest file changed")
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
