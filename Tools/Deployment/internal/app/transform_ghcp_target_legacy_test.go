package app_test

// transform_ghcp_target_legacy_test.go specifies the GHCP agent-name migration in a transform
// from a harness outside the GHCP pair into a GHCP target: the manifest records X.md agents of
// the target (their files present or missing), the transform writes X.agent.md, and no X.md entry
// of the target is left behind. A present X.md is backed up, removed and shown in the reviewed
// plan; a transform inside the GHCP pair keeps migrating as before.
//
// Fixtures are the tool-local parity catalog; nothing here reads the repository Catalog.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// tgLegacyWorkspace deploys the parity sample for OpenCode and records an X.md agent of the
// target harness for every sample agent. The legacy files exist only when present is true. It
// returns the workspace and the legacy bytes by agent key.
func tgLegacyWorkspace(t *testing.T, target string, present bool) (string, map[string][]byte) {
	t.Helper()
	ws := parityDeploy(t, hOpen)
	m := gamManifest(t, ws)
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{
		HarnessID: target, UpdatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)})
	legacy := map[string][]byte{}
	for _, a := range parityAgents {
		content := parityAgentFile(t, ws, hOpen, a.key)
		legacy[a.key] = []byte(content)
		if present {
			gamWrite(t, ws, gamRel(a.key, ".md"), content)
		}
		m.Entries = append(m.Entries, domain.ManifestEntry{
			Harness:     target,
			Ref:         domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: a.key},
			TargetPath:  gamRel(a.key, ".md"),
			Version:     "0.0.1",
			ContentHash: manifest.Hash([]byte(content)),
			DeployedAt:  time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		})
	}
	gamSaveManifest(t, ws, m)
	return ws, legacy
}

// tgLegacyEntries returns the agent entries of the harness recorded under an X.md path.
func tgLegacyEntries(m domain.Manifest, harness string) []domain.ManifestEntry {
	var out []domain.ManifestEntry
	for _, e := range m.Entries {
		if e.Harness == harness && e.Ref.Kind == domain.ArtifactAgent && !strings.HasSuffix(e.TargetPath, ".agent.md") {
			out = append(out, e)
		}
	}
	return out
}

// tgAgentPaths returns the target paths recorded for an agent key under a harness.
func tgAgentPaths(m domain.Manifest, harness, key string) []string {
	var out []string
	for _, e := range m.Entries {
		if e.Harness == harness && e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key {
			out = append(out, e.TargetPath)
		}
	}
	return out
}

// TestTransformIntoGhcp_RecordedLegacyAgentsOfTheTarget_LeaveNoLegacyEntry verifies that after a
// transform from OpenCode the manifest holds no X.md entry of the target, whether the X.md files
// exist or are already gone, and each agent is recorded once as X.agent.md.
func TestTransformIntoGhcp_RecordedLegacyAgentsOfTheTarget_LeaveNoLegacyEntry(t *testing.T) {
	for _, target := range gamTargets {
		for _, present := range []bool{true, false} {
			name := target + "/legacy files missing"
			if present {
				name = target + "/legacy files present"
			}
			t.Run(name, func(t *testing.T) {
				// Arrange
				ws, _ := tgLegacyWorkspace(t, target, present)

				// Act
				parityTransform(t, ws, hOpen, target)

				// Assert
				m := gamManifest(t, ws)
				if left := tgLegacyEntries(m, target); len(left) != 0 {
					t.Errorf("manifest keeps X.md entries of %s: %+v", target, left)
				}
				for _, a := range parityAgents {
					want := gamRel(a.key, ".agent.md")
					if paths := tgAgentPaths(m, target, a.key); len(paths) != 1 || paths[0] != want {
						t.Errorf("%s: entries of %s = %v, want exactly [%s]", a.key, target, paths, want)
					}
					if !gamExists(ws, want) {
						t.Errorf("%s: %s was not written", a.key, want)
					}
				}
			})
		}
	}
}

// TestTransformIntoGhcp_PresentLegacyFiles_AreBackedUpAndRemoved verifies a present X.md of the
// target is removed after its bytes are saved to a backup, and that a missing one triggers no
// backup.
func TestTransformIntoGhcp_PresentLegacyFiles_AreBackedUpAndRemoved(t *testing.T) {
	for _, target := range gamTargets {
		for _, present := range []bool{true, false} {
			t.Run(target+"/present="+map[bool]string{true: "yes", false: "no"}[present], func(t *testing.T) {
				// Arrange
				ws, legacy := tgLegacyWorkspace(t, target, present)

				// Act
				parityTransform(t, ws, hOpen, target)

				// Assert
				for _, a := range parityAgents {
					oldRel := gamRel(a.key, ".md")
					if gamExists(ws, oldRel) {
						t.Errorf("%s: legacy %s still exists", a.key, oldRel)
					}
					backedUp := gamHasBackup(t, ws, oldRel, legacy[a.key])
					if present && !backedUp {
						t.Errorf("%s: no backup of %s holds the legacy bytes", a.key, oldRel)
					}
					if !present && len(gamBackups(t, ws, oldRel)) != 0 {
						t.Errorf("%s: %s was backed up although its file is missing", a.key, oldRel)
					}
				}
			})
		}
	}
}

// TestTransformIntoGhcp_ReviewedPlan_ListsTheLegacyRemovals verifies the plan the user reviews
// removes every present X.md with its manifest entry, and lists no removal for a missing one.
func TestTransformIntoGhcp_ReviewedPlan_ListsTheLegacyRemovals(t *testing.T) {
	for _, target := range gamTargets {
		for _, present := range []bool{true, false} {
			t.Run(target+"/present="+map[bool]string{true: "yes", false: "no"}[present], func(t *testing.T) {
				// Arrange
				ws, _ := tgLegacyWorkspace(t, target, present)
				c := gamNewReviewCapture(ws)

				// Act
				_, err := parityServiceWith(t, gamCaptureDeps(c)).TransformHarness(context.Background(),
					app.TransformHarnessRequest{SourceHarnessID: hOpen, TargetHarnessID: target,
						Workspace: ws, TargetModel: parityModel})

				// Assert
				if err != nil {
					t.Fatalf("transform %s -> %s: %v", hOpen, target, err)
				}
				p := gplReviewedPlan(t, c)
				for _, a := range parityAgents {
					r, ok := gmeRemoval(p, gamRel(a.key, ".md"))
					switch {
					case present && !ok:
						t.Errorf("%s: plan does not remove the present %s", a.key, gamRel(a.key, ".md"))
					case present && !r.DropEntry:
						t.Errorf("%s: removal of %s does not drop its entry", a.key, r.Path)
					case !present && ok:
						t.Errorf("%s: plan removes %s although its file is missing", a.key, r.Path)
					}
				}
			})
		}
	}
}

// TestTransformIntoGhcp_KeepsTheSourceDeployment verifies the migration of the target's legacy
// agents leaves the OpenCode deployment, its files and its manifest entries as they were.
func TestTransformIntoGhcp_KeepsTheSourceDeployment(t *testing.T) {
	for _, target := range gamTargets {
		t.Run(target, func(t *testing.T) {
			// Arrange
			ws, _ := tgLegacyWorkspace(t, target, true)
			source := map[string]string{}
			for _, a := range parityAgents {
				source[a.key] = parityAgentFile(t, ws, hOpen, a.key)
			}

			// Act
			parityTransform(t, ws, hOpen, target)

			// Assert
			m := gamManifest(t, ws)
			for _, a := range parityAgents {
				if got := parityAgentFile(t, ws, hOpen, a.key); got != source[a.key] {
					t.Errorf("%s: the OpenCode file changed", a.key)
				}
				if paths := tgAgentPaths(m, hOpen, a.key); len(paths) != 1 {
					t.Errorf("%s: OpenCode entries = %v, want exactly one", a.key, paths)
				}
			}
		})
	}
}

// TestTransformWithinTheGhcpPair_RecordedLegacyAgents_KeepMigrating verifies the pair transform
// still moves recorded X.md agents to X.agent.md of the other target.
func TestTransformWithinTheGhcpPair_RecordedLegacyAgents_KeepMigrating(t *testing.T) {
	for _, dir := range trPairDirections {
		src, tgt := dir[0], dir[1]
		t.Run(src+" to "+tgt, func(t *testing.T) {
			// Arrange
			ws, legacy := gamLegacyWorkspace(t, src)

			// Act
			parityTransform(t, ws, src, tgt)

			// Assert
			m := gamManifest(t, ws)
			if left := tgLegacyEntries(m, tgt); len(left) != 0 {
				t.Errorf("manifest keeps X.md entries of %s: %+v", tgt, left)
			}
			for _, a := range parityAgents {
				newRel, oldRel := gamRel(a.key, ".agent.md"), gamRel(a.key, ".md")
				if !gamExists(ws, newRel) || gamExists(ws, oldRel) {
					t.Errorf("%s: %s exists = %t, %s exists = %t, want the migrated file only", a.key,
						newRel, gamExists(ws, newRel), oldRel, gamExists(ws, oldRel))
				}
				if !gamHasBackup(t, ws, oldRel, legacy[a.key]) {
					t.Errorf("%s: no backup of %s holds the legacy bytes", a.key, oldRel)
				}
				if paths := tgAgentPaths(m, tgt, a.key); len(paths) != 1 || paths[0] != newRel {
					t.Errorf("%s: entries of %s = %v, want exactly [%s]", a.key, tgt, paths, newRel)
				}
			}
		})
	}
}
