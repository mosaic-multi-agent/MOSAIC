package app_test

// reconcile_flows_real_test.go drives the agent-writing flows (deploy, agents, update, update
// workflows) through the real planner, executor and manifest store over a workspace whose
// manifest records files that are not on disk, as it does after a deployed folder was copied
// between workspaces. A recorded file that does not exist is neither an update, a conflict nor
// a legacy-name migration; it is dropped, listed, and never written back.

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	rrOtherHarness = "opencode"
	rrOtherGhost   = ".opencode/agents/ghost.md"
	rrOtherKept    = ".claude/agents/kept.md"
)

// rrWorkspace deploys the parity sample for the harness, then records files that do not exist:
// the legacy X.md of an agent the harness has deployed under X.agent.md, and an agent of
// another harness. It also records, with its file present, an agent of a third harness.
func rrWorkspace(t *testing.T, harness string) (ws, legacyPath string) {
	t.Helper()
	ws = parityDeploy(t, harness)
	legacyPath = gamRel("parity-runner", ".md")
	gamWrite(t, ws, rrOtherKept, "kept")
	m := gamManifest(t, ws)
	m.Harnesses = append(m.Harnesses,
		domain.HarnessSection{HarnessID: rrOtherHarness}, domain.HarnessSection{HarnessID: "claude-code"})
	m.Entries = append(m.Entries,
		rfEntry(harness, "parity-runner", legacyPath),
		rfEntry(rrOtherHarness, "ghost", rrOtherGhost),
		rfEntry("claude-code", "kept", rrOtherKept))
	gamSaveManifest(t, ws, m)
	return ws, legacyPath
}

func rrManifestBytes(t *testing.T, ws string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, ".mosaic", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest file: %v", err)
	}
	return b
}

func rrDroppedPaths(p domain.Plan) []string {
	var out []string
	for _, d := range p.DroppedEntries {
		out = append(out, d.Entry.TargetPath)
	}
	return out
}

func rrHasPath(paths []string, want string) bool {
	for _, p := range paths {
		if strings.ReplaceAll(p, "\\", "/") == want {
			return true
		}
	}
	return false
}

// TestReconcileRealFlows_PreviewListsMissingEntriesAndPlansNothingForThem covers the update
// preview: the entry of a missing X.md and the entry of another harness's missing file are
// listed as dropped, and the agent X.md stood for is not a rename, a conflict or a removal.
func TestReconcileRealFlows_PreviewListsMissingEntriesAndPlansNothingForThem(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				ws, legacy := rrWorkspace(t, h)
				c := gamNewReviewCapture(ws)

				if err := flow.run(t, ws, h, c, true); err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}

				p := gplReviewedPlan(t, c)
				dropped := rrDroppedPaths(p)
				for _, want := range []string{legacy, rrOtherGhost} {
					if !rrHasPath(dropped, want) {
						t.Errorf("dropped entries %v lack %s", dropped, want)
					}
				}
				for _, d := range p.DroppedEntries {
					if d.Reason != domain.DropReasonMissing {
						t.Errorf("reason of %s = %q, want %q", d.Entry.TargetPath, d.Reason, domain.DropReasonMissing)
					}
				}
				if rrHasPath(dropped, rrOtherKept) {
					t.Errorf("the entry of %s, whose file exists, was dropped", rrOtherKept)
				}
				if gplRemovesPath(p, legacy) {
					t.Errorf("the plan removes %s, a file that does not exist", legacy)
				}
				for _, it := range p.Items {
					if it.RenamedFrom == legacy {
						t.Errorf("%s is planned as a rename from the missing %s", it.Ref.Key, legacy)
					}
					if it.Action == domain.ActionConflict {
						t.Errorf("%s is planned as a conflict: %v", it.Ref.Key, it.Reasons)
					}
				}
			})
		}
	}
}

// TestReconcileRealFlows_DryRunLeavesTheManifestFileByteIdentical verifies the reconciliation
// of a dry run is in memory only.
func TestReconcileRealFlows_DryRunLeavesTheManifestFileByteIdentical(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				ws, legacy := rrWorkspace(t, h)
				before := rrManifestBytes(t, ws)
				c := gamNewReviewCapture(ws)

				if err := flow.run(t, ws, h, c, true); err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}

				if !bytes.Equal(before, rrManifestBytes(t, ws)) {
					t.Error("a dry run changed the manifest file")
				}
				if !rrHasPath(rrDroppedPaths(gplReviewedPlan(t, c)), legacy) {
					t.Errorf("the dry run did not list %s as dropped", legacy)
				}
			})
		}
	}
}

// TestReconcileRealFlows_ExecutedRunSavesTheReconciledManifestPlusItsChanges verifies the
// manifest a real run writes: no entry of a missing file, whatever its harness, the entries of
// files that exist (of any harness) kept, and the run's own entries recorded.
func TestReconcileRealFlows_ExecutedRunSavesTheReconciledManifestPlusItsChanges(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				ws, legacy := rrWorkspace(t, h)
				c := gamNewReviewCapture(ws)

				if err := flow.run(t, ws, h, c, false); err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}

				m := gamManifest(t, ws)
				var paths []string
				for _, e := range m.Entries {
					paths = append(paths, e.TargetPath)
				}
				for _, gone := range []string{legacy, rrOtherGhost} {
					if rrHasPath(paths, gone) {
						t.Errorf("the saved manifest still records the missing file %s", gone)
					}
				}
				if m.Holds(rrOtherHarness) {
					t.Errorf("harness %s, whose entries were all dropped, is still held", rrOtherHarness)
				}
				if !rrHasPath(paths, rrOtherKept) {
					t.Errorf("the saved manifest lost the entry of %s, whose file exists", rrOtherKept)
				}
				for _, key := range flow.renamed {
					if !rrHasPath(paths, gamRel(key, ".agent.md")) {
						t.Errorf("the saved manifest has no entry for the run's agent %s", key)
					}
				}
			})
		}
	}
}

// TestReconcileRealFlows_MissingEntriesDoNotChangeThePlanOfAnOtherwiseUpToDateWorkspace
// verifies the plan of a workspace with recorded-but-missing files equals the plan of the same
// workspace without them: the ghost entries influence nothing.
func TestReconcileRealFlows_MissingEntriesDoNotChangeThePlanOfAnOtherwiseUpToDateWorkspace(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				clean := parityDeploy(t, h)
				cleanCapture := gamNewReviewCapture(clean)
				if err := flow.run(t, clean, h, cleanCapture, true); err != nil {
					t.Fatalf("clean %s: %v", flow.name, err)
				}
				ghosts, _ := rrWorkspace(t, h)
				ghostCapture := gamNewReviewCapture(ghosts)
				if err := flow.run(t, ghosts, h, ghostCapture, true); err != nil {
					t.Fatalf("with ghosts %s: %v", flow.name, err)
				}

				want, got := gplReviewedPlan(t, cleanCapture), gplReviewedPlan(t, ghostCapture)

				if !reflect.DeepEqual(rrActions(want), rrActions(got)) {
					t.Errorf("recorded-but-missing files changed the plan:\n clean  %v\n ghosts %v", rrActions(want), rrActions(got))
				}
				if len(want.Removals) != len(got.Removals) || len(want.Replacements) != len(got.Replacements) {
					t.Errorf("removals/replacements differ: clean %d/%d, ghosts %d/%d",
						len(want.Removals), len(want.Replacements), len(got.Removals), len(got.Replacements))
				}
			})
		}
	}
}

// rrActions lists "<target path> <action>" per item, in plan order.
func rrActions(p domain.Plan) []string {
	var out []string
	for _, it := range p.Items {
		out = append(out, filepath.ToSlash(it.TargetPath)+" "+string(it.Action))
	}
	return out
}

