package app_test

// transform_ghcp_target_legacy_skip_test.go verifies that in a transform from a harness outside
// the GHCP pair into a GHCP target, a recorded, present X.md of a source file the transform skips
// (it is not a Deploy agent) is never removed: only a written agent, or a kept existing target
// X.agent.md, replaces its legacy file.
//
// Fixtures are written inline into temp workspaces; nothing here reads the repository Catalog.

import (
	"bytes"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// tgkSkippedSources are OpenCode agent-folder files the transform skips as non-agents.
var tgkSkippedSources = []struct{ name, key, content string }{
	{"no frontmatter", "tgk-plain-notes", "Just some notes, no frontmatter.\n"},
	{"unterminated frontmatter", "tgk-broken", "---\nname: tgk-broken\nno closing delimiter\n"},
}

// TestTransformIntoGhcp_SkippedSourceFile_KeepsItsRecordedLegacyFile verifies the recorded,
// present X.md of a skipped source file keeps its file and manifest entry, gets no backup and no
// removal in the plan, while the written sample agents still replace their legacy files.
func TestTransformIntoGhcp_SkippedSourceFile_KeepsItsRecordedLegacyFile(t *testing.T) {
	for _, target := range gamTargets {
		for _, sk := range tgkSkippedSources {
			t.Run(target+"/"+sk.name, func(t *testing.T) {
				// Arrange
				ws, _ := tgLegacyWorkspace(t, target, true)
				srcRel := rbModule(t, hOpen).Descriptor().Paths.Agents.Project + "/" + sk.key + ".md"
				gamWrite(t, ws, srcRel, sk.content)
				legacyRel := gamRel(sk.key, ".md")
				legacy := "legacy copy of a skipped file\n"
				gamWrite(t, ws, legacyRel, legacy)
				m := gamManifest(t, ws)
				m.Entries = append(m.Entries, domain.ManifestEntry{Harness: target,
					Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: sk.key},
					TargetPath: legacyRel, Version: "0.0.1", ContentHash: manifest.Hash([]byte(legacy)),
					DeployedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)})
				gamSaveManifest(t, ws, m)

				// Act
				p := tgsTransformPlan(t, ws, target)

				// Assert
				if _, ok := gmeRemoval(p, legacyRel); ok {
					t.Errorf("plan removes %s although its source file is skipped", legacyRel)
				}
				if got := gamRead(t, ws, legacyRel); !bytes.Equal(got, []byte(legacy)) {
					t.Errorf("%s changed or is gone: %q", legacyRel, got)
				}
				if n := len(gamBackups(t, ws, legacyRel)); n != 0 {
					t.Errorf("%s was backed up %d times although it was not removed", legacyRel, n)
				}
				if !tgsEntryAt(gamManifest(t, ws), target, legacyRel) {
					t.Errorf("manifest lost the entry of %s", legacyRel)
				}
				for _, a := range parityAgents {
					gmeAssertLegacyRemoval(t, p, a.key)
				}
			})
		}
	}
}
