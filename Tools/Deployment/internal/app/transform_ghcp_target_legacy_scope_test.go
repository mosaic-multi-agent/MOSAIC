package app_test

// transform_ghcp_target_legacy_scope_test.go bounds the legacy migration in a transform from a
// harness outside the GHCP pair into a GHCP target: it removes only the recorded X.md of an agent
// whose X.agent.md the transform writes, and it never changes the state an existing target
// X.agent.md has without the migration (conflict, backup marker, own reasons).
//
// Fixtures are the tool-local parity catalog; nothing here reads the repository Catalog.

import (
	"bytes"
	"context"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// tgsOtherKey is an agent key the transform does not write: the source deployment has no such agent.
const tgsOtherKey = "not-in-the-source-deployment"

// tgsTransformPlan runs the transform with a review capture and returns the reviewed plan.
func tgsTransformPlan(t *testing.T, ws, target string) domain.Plan {
	t.Helper()
	c := gamNewReviewCapture(ws)
	_, err := parityServiceWith(t, gamCaptureDeps(c)).TransformHarness(context.Background(),
		app.TransformHarnessRequest{SourceHarnessID: hOpen, TargetHarnessID: target,
			Workspace: ws, TargetModel: parityModel})
	if err != nil {
		t.Fatalf("transform %s -> %s: %v", hOpen, target, err)
	}
	return gplReviewedPlan(t, c)
}

// tgsEntryAt reports whether the manifest records an agent entry of the harness at rel.
func tgsEntryAt(m domain.Manifest, harness, rel string) bool {
	for _, e := range m.Entries {
		if e.Harness == harness && e.Ref.Kind == domain.ArtifactAgent && e.TargetPath == rel {
			return true
		}
	}
	return false
}

// TestTransformIntoGhcp_LegacyAgentTheTransformDoesNotWrite_IsLeftAlone verifies a recorded,
// present X.md of an agent the transform does not write keeps its file, its manifest entry and
// has no backup and no removal in the plan.
func TestTransformIntoGhcp_LegacyAgentTheTransformDoesNotWrite_IsLeftAlone(t *testing.T) {
	for _, target := range gamTargets {
		t.Run(target, func(t *testing.T) {
			// Arrange
			ws, _ := tgLegacyWorkspace(t, target, true)
			otherRel := gamRel(tgsOtherKey, ".md")
			other := []byte("an agent of the target the source does not have\n")
			gamWrite(t, ws, otherRel, string(other))
			gamRecordAgentAt(t, ws, tgsOtherKey, otherRel, string(other))
			m := gamManifest(t, ws)
			for i := range m.Entries {
				if m.Entries[i].TargetPath == otherRel {
					m.Entries[i].Harness = target
				}
			}
			gamSaveManifest(t, ws, m)

			// Act
			p := tgsTransformPlan(t, ws, target)

			// Assert
			if _, ok := gmeRemoval(p, otherRel); ok {
				t.Errorf("plan removes %s although the transform writes no %s", otherRel, gamRel(tgsOtherKey, ".agent.md"))
			}
			if got := gamRead(t, ws, otherRel); !bytes.Equal(got, other) {
				t.Errorf("%s changed or is gone: %q", otherRel, got)
			}
			if n := len(gamBackups(t, ws, otherRel)); n != 0 {
				t.Errorf("%s was backed up %d times although it was not removed", otherRel, n)
			}
			if !tgsEntryAt(gamManifest(t, ws), target, otherRel) {
				t.Errorf("manifest lost the entry of %s", otherRel)
			}
			for _, a := range parityAgents {
				gmeAssertLegacyRemoval(t, p, a.key)
			}
		})
	}
}

// tgsExisting is a state of a target X.agent.md that exists before the transform.
type tgsExisting struct {
	name    string
	prepare func(t *testing.T, ws, target, key string)
}

func tgsExistingStates() []tgsExisting {
	return []tgsExisting{
		{"unrecorded", func(t *testing.T, ws, _, key string) {
			gamWrite(t, ws, gamRel(key, ".agent.md"), "stale user copy\n")
		}},
		{"recorded then edited", func(t *testing.T, ws, target, key string) {
			rel := gamRel(key, ".agent.md")
			recorded := parityAgentFile(t, ws, hOpen, key)
			gamWrite(t, ws, rel, recorded+"\nA line the user added.\n")
			m := gamManifest(t, ws)
			m.Entries = append(m.Entries, domain.ManifestEntry{Harness: target,
				Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}, TargetPath: rel,
				Version: "0.0.1", ContentHash: manifest.Hash([]byte(recorded)),
				DeployedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)})
			gamSaveManifest(t, ws, m)
		}},
		{"recorded unreadable", func(t *testing.T, ws, target, key string) {
			rel := gamRel(key, ".agent.md")
			gamWrite(t, ws, rel, "recorded copy\n")
			m := gamManifest(t, ws)
			m.Entries = append(m.Entries, domain.ManifestEntry{Harness: target,
				Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}, TargetPath: rel,
				Version: "0.0.1", ContentHash: manifest.Hash([]byte("recorded copy\n")),
				DeployedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)})
			gamSaveManifest(t, ws, m)
		}},
	}
}

// tgsExistingWorkspace deploys OpenCode, puts the existing target X.agent.md of the first sample
// agent in the given state and, when withLegacy is true, records a present X.md of that agent.
func tgsExistingWorkspace(t *testing.T, target string, st tgsExisting, withLegacy bool) (string, string) {
	t.Helper()
	ws := parityDeploy(t, hOpen)
	key := parityAgents[0].key
	m := gamManifest(t, ws)
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{
		HarnessID: target, UpdatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)})
	gamSaveManifest(t, ws, m)
	st.prepare(t, ws, target, key)
	if withLegacy {
		content := parityAgentFile(t, ws, hOpen, key)
		gamWrite(t, ws, gamRel(key, ".md"), content)
		m = gamManifest(t, ws)
		m.Entries = append(m.Entries, domain.ManifestEntry{Harness: target,
			Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}, TargetPath: gamRel(key, ".md"),
			Version: "0.0.1", ContentHash: manifest.Hash([]byte(content)),
			DeployedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)})
		gamSaveManifest(t, ws, m)
	}
	return ws, key
}

// TestTransformIntoGhcp_ExistingTargetAgentFile_KeepsItsStateWithTheLegacyFilePresent verifies a
// present recorded X.md never turns an existing, kept target X.agent.md into a write or clears
// its state: action, conflict, backup marker, rename source and own reasons equal those of the
// same transform without the recorded X.md, no reason is added to the item, and the file
// on disk is not rewritten. The kept agent's recorded X.md is still backed up, removed and its
// entry dropped, so no X.md entry of the target remains.
func TestTransformIntoGhcp_ExistingTargetAgentFile_KeepsItsStateWithTheLegacyFilePresent(t *testing.T) {
	for _, target := range gamTargets {
		for _, st := range tgsExistingStates() {
			t.Run(target+"/"+st.name, func(t *testing.T) {
				// Arrange
				baseWs, key := tgsExistingWorkspace(t, target, st, false)
				legacyWs, _ := tgsExistingWorkspace(t, target, st, true)
				base, ok := gplAgentItem(tgsTransformPlan(t, baseWs, target), key)
				if !ok {
					t.Fatalf("%s: baseline plan has no item", key)
				}

				before := gamRead(t, legacyWs, gamRel(key, ".agent.md"))
				legacy := []byte(parityAgentFile(t, legacyWs, hOpen, key))

				// Act
				p := tgsTransformPlan(t, legacyWs, target)

				// Assert
				it, ok := gplAgentItem(p, key)
				if !ok {
					t.Fatalf("%s: plan has no item", key)
				}
				if it.Action != base.Action || (it.Conflict != nil) != (base.Conflict != nil) {
					t.Errorf("%s: action/conflict = %q/%v, want %q/%v as without the legacy file", key,
						it.Action, it.Conflict != nil, base.Action, base.Conflict != nil)
				}
				if it.BackupBeforeWrite != base.BackupBeforeWrite || it.RenamedFrom != base.RenamedFrom {
					t.Errorf("%s: backup/renamedFrom = %t/%q, want %t/%q as without the legacy file", key,
						it.BackupBeforeWrite, it.RenamedFrom, base.BackupBeforeWrite, base.RenamedFrom)
				}
				if len(it.Reasons) != len(base.Reasons) {
					t.Fatalf("%s: reasons = %q, want exactly the item's own reasons %q (the migration reason is the removal's)",
						key, it.Reasons, base.Reasons)
				}
				for i, r := range base.Reasons {
					if it.Reasons[i] != r {
						t.Errorf("%s: own reason %d = %q, want %q", key, i, it.Reasons[i], r)
					}
				}
				if got := gamRead(t, legacyWs, gamRel(key, ".agent.md")); !bytes.Equal(got, before) {
					t.Errorf("%s: the existing %s was rewritten", key, gamRel(key, ".agent.md"))
				}
				tgsAssertKeptAgentLegacyRemoved(t, p, legacyWs, target, key, legacy)
			})
		}
	}
}

// tgsAssertKeptAgentLegacyRemoved checks the transform removed the recorded X.md of a kept
// target X.agent.md: the plan lists the removal, the file is gone, a backup holds its bytes and
// the manifest records no X.md entry of the target for the agent.
func tgsAssertKeptAgentLegacyRemoved(t *testing.T, p domain.Plan, ws, target, key string, legacy []byte) {
	t.Helper()
	legacyRel := gamRel(key, ".md")
	gmeAssertLegacyRemoval(t, p, key)
	if gamExists(ws, legacyRel) {
		t.Errorf("%s: legacy %s still exists", key, legacyRel)
	}
	if !gamHasBackup(t, ws, legacyRel, legacy) {
		t.Errorf("%s: no backup of %s holds the legacy bytes", key, legacyRel)
	}
	if tgsEntryAt(gamManifest(t, ws), target, legacyRel) {
		t.Errorf("%s: manifest keeps the %s entry of %s", key, legacyRel, target)
	}
}
