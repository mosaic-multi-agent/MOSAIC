package app_test

// transform_opencode_pair_exec_test.go specifies the end state of a real transform between the
// two OpenCode targets, in both directions: the target's files replace the source's recorded
// agents, skills and logger files in place, source-variant files the target does not write are
// gone (so no V1 plugin file lingers in .opencode/plugins), user files are untouched, and every
// removed or replaced file can be recovered from .mosaic/backups. The manifest end state is
// specified in transform_opencode_pair_manifest_test.go.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestTransformOpenCodePair_RealRunReplacesTheRecordedAgentInPlaceWithTheTargetFormat(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		source := r.read(ocWorker)

		r.mustRun(r.request())

		got := trFilesIn(t, r.ws, ocAgentsDir)
		if want := []string{"notes.md", "worker.md"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("agents folder = %v, want %v: the agent is replaced in place, nothing added", got, want)
		}
		written := r.read(ocWorker)
		if written == source {
			t.Fatalf("%s still holds the source content", ocWorker)
		}
		if !trHasKey(written, "mosaic_harness_version") {
			t.Errorf("%s does not carry the harness stamp:\n%s", ocWorker, written)
		}
		if !strings.Contains(written, "You are the **CodeReviewer** agent.") {
			t.Errorf("%s lost the agent body:\n%s", ocWorker, written)
		}
		if !trHasKey(written, ocToolsKey[r.tgt]) {
			t.Errorf("%s lacks the target's %q key:\n%s", ocWorker, ocToolsKey[r.tgt], written)
		}
		if other := ocToolsKey[r.src]; trHasKey(written, other) {
			t.Errorf("%s still carries the source's %q key:\n%s", ocWorker, other, written)
		}
	})
}

func TestTransformOpenCodePair_RealRunLeavesOnlyTheTargetLoggerFilesAndRemovesTheSourceOnlyOnes(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		r.mustRun(r.request())

		entry := trReadTestdata(t, ocVariantFiles[r.tgt][0][0])
		if got := r.read(ocPluginsDir + "/mosaic-logger.ts"); got != entry {
			t.Errorf("mosaic-logger.ts holds %q, want the target variant file %q", got, entry)
		}
		helper := ocOnlyHelper[r.tgt]
		if got, want := r.read(ocPluginsDir+"/"+helper), trReadTestdata(t, ocVariantFiles[r.tgt][1][0]); got != want {
			t.Errorf("%s holds %q, want the target variant helper %q", helper, got, want)
		}
		if gone := ocOnlyHelper[r.src]; r.exists(ocPluginsDir + "/" + gone) {
			t.Errorf("source-only helper %s/%s is still there after the transform", ocPluginsDir, gone)
		}
		if got, want := trFilesIn(t, r.ws, ocPluginsDir), []string{"lib", "mosaic-logger.ts", "user-plugin.ts"}; !reflect.DeepEqual(got, want) {
			t.Errorf("plugins folder = %v, want %v", got, want)
		}
		if got, want := trFilesIn(t, r.ws, ocPluginsDir+"/lib"), []string{strings.TrimPrefix(ocOnlyHelper[r.tgt], "lib/")}; !reflect.DeepEqual(got, want) {
			t.Errorf("plugins/lib = %v, want only the target helper %v", got, want)
		}
	})
}

func TestTransformOpenCodePair_RealRunKeepsTheRecordedSkillBytesInPlaceWithoutABackup(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		skill := map[string]string{}
		for rel, content := range r.snapshot() {
			if strings.HasPrefix(rel, ".opencode/skills/") {
				skill[rel] = content
			}
		}
		if len(skill) != 2 {
			t.Fatalf("setup: %d skill files, want 2", len(skill))
		}

		res := r.mustRun(r.request())

		for rel, want := range skill {
			if got := r.read(rel); got != want {
				t.Errorf("%s changed: got %q, want %q", rel, got, want)
			}
			if n := len(trBackupsOf(t, r.ws, rel)); n != 0 {
				t.Errorf("%s has %d backups, want none for a carried skill file", rel, n)
			}
		}
		if f := trOutcome(t, res, domain.ArtifactSkill, trSkillKey); f.Status != app.StatusTransformed {
			t.Errorf("skill status %q, want %q", f.Status, app.StatusTransformed)
		}
		for _, ch := range append(append([]app.TransformFileChange{}, res.Removals...), res.Replacements...) {
			if strings.HasPrefix(ch.Path, ".opencode/skills/") {
				t.Errorf("skill file %s listed as removed or replaced", ch.Path)
			}
		}
	})
}

func TestTransformOpenCodePair_RealRunLeavesUnrecordedUserFilesUnchangedAndWithoutBackups(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		kept := map[string]string{
			ocUserPlugin:    r.read(ocUserPlugin),
			ocUserNote:      r.read(ocUserNote),
			ocUnrelatedPath: r.read(ocUnrelatedPath),
		}

		r.mustRun(r.request())

		for rel, want := range kept {
			if got := r.read(rel); got != want {
				t.Errorf("%s changed: got %q, want %q", rel, got, want)
			}
			if n := len(trBackupsOf(t, r.ws, rel)); n != 0 {
				t.Errorf("%s has %d backups, want none for a file that is neither removed nor replaced", rel, n)
			}
		}
	})
}

func TestTransformOpenCodePair_RealRunBacksUpEveryRemovedAndReplacedFileByteForByte(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		original := map[string]string{
			ocWorker:                                 r.read(ocWorker),
			ocPluginsDir + "/mosaic-logger.ts":       r.read(ocPluginsDir + "/mosaic-logger.ts"),
			ocPluginsDir + "/" + ocOnlyHelper[r.src]: r.read(ocPluginsDir + "/" + ocOnlyHelper[r.src]),
		}

		res := r.mustRun(r.request())

		for rel, want := range original {
			trAssertRecoverable(t, r.ws, rel, want)
		}
		if len(res.Removals) != 1 || len(res.Replacements) != 2 {
			t.Fatalf("Removals %+v / Replacements %+v, want the source-only helper removed and the agent and logger entry replaced",
				res.Removals, res.Replacements)
		}
		for _, ch := range append(append([]app.TransformFileChange{}, res.Removals...), res.Replacements...) {
			want, ok := original[ch.Path]
			if !ok {
				t.Errorf("result reports %s as removed or replaced, but it was not part of the source deployment", ch.Path)
				continue
			}
			trAssertBackedUp(t, ch, want)
		}
	})
}

func TestTransformOpenCodePair_RealRunWithoutATargetVariantRemovesTheSourceLoggerFilesAndWritesNone(t *testing.T) {
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := ocPairRun(t, pair[0], pair[1], []string{pair[0]})
			srcFiles := map[string]string{}
			for _, f := range ocVariantFiles[r.src] {
				srcFiles[ocPluginsDir+"/"+f[1]] = r.read(ocPluginsDir + "/" + f[1])
			}

			res := r.mustRun(r.request())

			for rel, content := range srcFiles {
				if r.exists(rel) {
					t.Errorf("%s still exists: V2 drops a V1-form plugin, so no source logger file may linger", rel)
				}
				trAssertRecoverable(t, r.ws, rel, content)
			}
			if got, want := trFilesIn(t, r.ws, ocPluginsDir), []string{"user-plugin.ts"}; !reflect.DeepEqual(got, want) {
				t.Errorf("plugins folder = %v, want only the user plugin %v", got, want)
			}
			if f := trOutcome(t, res, domain.ArtifactHook, ocHookKey); f.Status == app.StatusTransformed || f.Status == app.StatusFailed {
				t.Errorf("hook status %q, want the bundle reported as skipped (no target variant)", f.Status)
			}
			if len(res.Removals) != len(srcFiles) {
				t.Errorf("Removals = %+v, want the %d source logger files", res.Removals, len(srcFiles))
			}
		})
	}
}

func TestTransformOpenCodePair_RoundTripRestoresTheSourceDeployment(t *testing.T) {
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1]+"->"+pair[0], func(t *testing.T) {
			r := ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2})

			r.mustRun(ocRequest(r, pair[0], pair[1]))
			mid := trLoadManifest(t, r.ws)
			if !mid.Holds(pair[1]) || mid.Holds(pair[0]) {
				t.Fatalf("after the way there the manifest holds %v, want only %s", mid.HarnessIDs(), pair[1])
			}
			if got, want := r.read(ocPluginsDir+"/mosaic-logger.ts"), trReadTestdata(t, ocVariantFiles[pair[1]][0][0]); got != want {
				t.Fatalf("after the way there mosaic-logger.ts holds %q, want the %s variant %q", got, pair[1], want)
			}
			r.mustRun(ocRequest(r, pair[1], pair[0]))

			for _, f := range ocVariantFiles[pair[0]] {
				if got, want := r.read(ocPluginsDir+"/"+f[1]), trReadTestdata(t, f[0]); got != want {
					t.Errorf("%s holds %q after the round trip, want the %s variant %q", f[1], got, pair[0], want)
				}
			}
			if r.exists(ocPluginsDir + "/" + ocOnlyHelper[pair[1]]) {
				t.Errorf("%s of the intermediate target is still there after the way back", ocOnlyHelper[pair[1]])
			}
			full := trLoadManifest(t, r.ws)
			if _, ok := full.Section(pair[1]); ok {
				t.Errorf("manifest still has a section for %s after the way back", pair[1])
			}
			if len(ocEntriesOf(full, pair[1])) != 0 {
				t.Errorf("manifest still has entries of %s: %+v", pair[1], ocEntriesOf(full, pair[1]))
			}
			if len(ocEntriesOf(full, pair[0])) == 0 {
				t.Errorf("manifest has no entries of %s after the round trip", pair[0])
			}
		})
	}
}
