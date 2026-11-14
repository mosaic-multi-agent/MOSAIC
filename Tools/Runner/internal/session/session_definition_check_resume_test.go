package session_test

// A resumed run with a misdeclared agent is refused before anything touches the
// run: the artifact stays as it was (an intact one byte for byte, a damaged one
// unrepaired) and no agent is invoked, including the orchestrator's repair
// consultation. The bad definition here is on a declared infrastructure agent,
// which only the run-start check (not the later settings and selection steps)
// can see before the repair.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// runFolderListing returns the entry names directly inside dir.
func runFolderListing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestSession_Resume_CLIHarness_DefinitionWithoutModel_LeavesIntactArtifactUnchanged
// resumes a run whose artifact is intact.
func TestSession_Resume_CLIHarness_DefinitionWithoutModel_LeavesIntactArtifactUnchanged(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
			rig.mutate(t, unselectedCheckpoint, withoutModel)
			runFolder := filepath.Join(rig.workDir, domain.RunScopedFolder(testRunID))
			if err := os.MkdirAll(runFolder, 0o700); err != nil {
				t.Fatalf("test setup: %v", err)
			}
			artifactPath := filepath.Join(runFolder, "Orchestration.md")
			store := artifact.NewFileStore(artifactPath)
			if _, err := store.Create(context.Background(), domain.WorkflowInfo{ID: "linear", Version: "1.0"}, "task",
				domain.RunSettings{Mode: domain.ExecutionModeAuto}, epoch, testRunID); err != nil {
				t.Fatalf("test setup: create artifact: %v", err)
			}
			before, listingBefore := readText(t, artifactPath), runFolderListing(t, runFolder)

			cfg := rig.config()
			cfg.IsNewRun = false
			cfg.RunID = testRunID
			cfg.RunFolder = runFolder
			f := harness.NewMockAdapter()
			ses := session.New(session.Deps{Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

			out, err := ses.Start(context.Background(), cfg)

			requireRefusedBy(t, out, err, unselectedCheckpoint, rig.path(unselectedCheckpoint), agentresolve.ErrDefinitionModelMissing)
			if len(f.Invocations()) != 0 {
				t.Errorf("want no agent invoked, got %d invocation(s)", len(f.Invocations()))
			}
			if after := readText(t, artifactPath); after != before {
				t.Error("want the artifact byte-identical after the refused resume")
			}
			if listingAfter := runFolderListing(t, runFolder); !reflect.DeepEqual(listingAfter, listingBefore) {
				t.Errorf("want the run folder unchanged, got %v, was %v", listingAfter, listingBefore)
			}
		})
	}
}

// TestSession_Resume_CLIHarness_DefinitionWithoutModel_SkipsRepairOfDamagedArtifact
// resumes a run whose artifact is damaged, so that without the check the
// orchestrator would be consulted to repair it first.
func TestSession_Resume_CLIHarness_DefinitionWithoutModel_SkipsRepairOfDamagedArtifact(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			var dr *defRig
			rig := newRepairRig(t, []repairStep{
				{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
			}, func(r *repairRig) {
				dr = newDefRig(t, l, multiInfraFixture, multiInfraAgents)
				dr.mutate(t, unselectedCommit, withoutModel)
				runFolder := filepath.Join(dr.workDir, domain.RunScopedFolder(testRunID))
				r.run = seededRun{orchPath: dr.orchPath, artifactPath: filepath.Join(runFolder, "Orchestration.md"), runFolder: runFolder}
				evidDir, ok := artifact.EvidenceDir(r.run.artifactPath)
				if !ok {
					t.Fatal("test setup: the run's artifact path has no evidence location")
				}
				r.evidDir = evidDir
				r.cons.path, r.cons.evidenceDir = r.run.artifactPath, evidDir
				r.deps.Store = artifact.NewFileStore(r.run.artifactPath)
				r.cfg = dr.config()
				r.cfg.RunID = testRunID
				r.cfg.RunFolder = runFolder
				r.cfg.RunSettings.Mode = domain.ExecutionModeOrchestrated
				withDamagedRun(t, startOpts{})(r)
			})

			got := rig.start(t)

			requireRefusedBy(t, got, nil, unselectedCommit, dr.path(unselectedCommit), agentresolve.ErrDefinitionModelMissing)
			if rig.cons.calls() != 0 {
				t.Errorf("want no repair consultation, got %d consultation(s)", rig.cons.calls())
			}
			if len(rig.f.Invocations()) != 0 {
				t.Errorf("want no agent invoked, got %v", invokedInstances(rig.f))
			}
			requireNoWriteOnDamaged(t, rig)
		})
	}
}
