package app_test

// ghcp_agent_migration_rollback_test.go covers the safety of a migration that keeps the file
// content: a dry run and a declined review write nothing, even though the legacy bytes are
// read ahead of the run, and a run that fails part way through a migration restores the
// workspace in the flows that run atomically. Workspaces carry planted user state.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// TestGhcpMigration_DryRunAndDeclinedReview_WriteNothing verifies neither a dry run nor a
// review the user declines renames, removes, backs up or writes anything.
func TestGhcpMigration_DryRunAndDeclinedReview_WriteNothing(t *testing.T) {
	modes := []struct {
		name string
		opts gsOpts
	}{
		{"dry run", gsOpts{dry: true}},
		{"declined review", gsOpts{decline: true}},
	}
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			for _, m := range modes {
				t.Run(h+"/"+r.name+"/"+m.name, func(t *testing.T) {
					// Arrange
					ws, planted := gsPlantedWorkspace(t, h, false)
					before := captureFS(t, ws)

					// Act
					err := r.run(t, ws, h, m.opts)

					// Assert
					if m.opts.decline && !errors.Is(err, app.ErrPlanNotConfirmed) {
						t.Errorf("declined review: error = %v, want ErrPlanNotConfirmed", err)
					}
					if m.opts.dry && err != nil {
						t.Fatalf("dry run: %v", err)
					}
					if changed, what := fsChanged(before, captureFS(t, ws)); changed {
						t.Errorf("%s changed the workspace: %s", m.name, what)
					}
					for _, a := range parityAgents {
						gamAssertKept(t, ws, a.key, planted[a.key])
					}
				})
			}
		}
	}
}

// TestGhcpMigration_AtomicRunFailingMidMigration_RestoresTheWorkspace verifies that when the
// new file of an agent cannot be written, the flows that run atomically put every legacy
// X.md back with its planted bytes and record, and leave no X.agent.md behind.
func TestGhcpMigration_AtomicRunFailingMidMigration_RestoresTheWorkspace(t *testing.T) {
	atomic := map[string]string{"update/full": "parity-helper", "workflows/full": "orchestrator"}
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			blocked, ok := atomic[r.name]
			if !ok {
				continue
			}
			t.Run(h+"/"+r.name, func(t *testing.T) {
				// Arrange
				ws, planted := gsPlantedWorkspace(t, h, false)
				blockedRel := gamRel(blocked, ".agent.md")
				gamWrite(t, ws, blockedRel+"/keep.txt", "blocker")

				// Act
				var sum domain.RunSummary
				err := r.run(t, ws, h, gsOpts{out: &sum})

				// Assert
				if err == nil && sum.Outcome != domain.OutcomeFailed {
					t.Fatalf("%s for %s: the failure was not surfaced", r.name, h)
				}
				for _, a := range parityAgents {
					legacyRel := gamRel(a.key, ".md")
					if !gamExists(ws, legacyRel) || string(gamRead(t, ws, legacyRel)) != string(planted[a.key]) {
						t.Errorf("%s: %s was not restored with its bytes", a.key, legacyRel)
					}
					if paths := gamEntryPaths(gamManifest(t, ws), a.key); len(paths) != 1 || paths[0] != legacyRel {
						t.Errorf("%s: manifest entries = %v, want exactly [%s]", a.key, paths, legacyRel)
					}
					if a.key != blocked && gamExists(ws, gamRel(a.key, ".agent.md")) {
						t.Errorf("%s: %s was left behind", a.key, gamRel(a.key, ".agent.md"))
					}
				}
			})
		}
	}
}
