package app_test

// transform_opencode_pair_manifest_test.go specifies the manifest end state of a real transform
// between the two OpenCode targets. Both targets write identical paths, so the entries are what
// tells them apart: the source harness is retired with every entry, the target section records
// the written agent, the carried skill files and the written hook files, and entries of every
// other harness are untouched.

import (
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestTransformOpenCodePair_ManifestRetiresTheSourceHarnessWithAllItsEntries(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		r.mustRun(r.request())

		full := trLoadManifest(t, r.ws)
		if _, ok := full.Section(r.src); ok {
			t.Errorf("manifest still has a section for the source %q", r.src)
		}
		if full.Holds(r.src) {
			t.Errorf("manifest still holds %q", r.src)
		}
		if got := ocEntriesOf(full, r.src); len(got) != 0 {
			t.Errorf("manifest still has %d entries of the source %q: %+v", len(got), r.src, got)
		}
		if _, ok := full.Section(r.tgt); !ok || !full.Holds(r.tgt) {
			t.Errorf("manifest has no section or entries for the target %q", r.tgt)
		}
	})
}

func TestTransformOpenCodePair_ManifestRecordsTheWrittenAgentForTheTarget(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		r.mustRun(r.request())

		m := trLoadManifest(t, r.ws).ForHarness(r.tgt)
		agents := trEntryByRef(m, domain.ArtifactAgent, "worker")
		if len(agents) != 1 || agents[0].TargetPath != ocWorker {
			t.Fatalf("agent entries = %+v, want exactly one at %s", agents, ocWorker)
		}
		if want := manifest.Hash([]byte(r.read(ocWorker))); agents[0].ContentHash != want {
			t.Errorf("agent content hash = %q, want the hash of the written bytes %q", agents[0].ContentHash, want)
		}
	})
}

func TestTransformOpenCodePair_ManifestRecordsTheCarriedSkillFilesForTheTarget(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		before := trEntryByRef(trLoadManifest(t, r.ws).ForHarness(r.src), domain.ArtifactSkill, trSkillKey)
		if len(before) != 2 {
			t.Fatalf("setup: %d source skill entries, want 2", len(before))
		}

		r.mustRun(r.request())

		got := trEntryByRef(trLoadManifest(t, r.ws).ForHarness(r.tgt), domain.ArtifactSkill, trSkillKey)
		if len(got) != len(before) {
			t.Fatalf("target skill entries = %+v, want one per carried file (%d)", got, len(before))
		}
		byPath := map[string]domain.ManifestEntry{}
		for _, e := range got {
			byPath[e.TargetPath] = e
		}
		for _, src := range before {
			e, ok := byPath[src.TargetPath]
			if !ok {
				t.Errorf("no target entry at %s", src.TargetPath)
				continue
			}
			if e.Version != src.Version || e.ContentHash != src.ContentHash {
				t.Errorf("%s recorded as version %q hash %q, want the source's %q / %q",
					src.TargetPath, e.Version, e.ContentHash, src.Version, src.ContentHash)
			}
		}
	})
}

func TestTransformOpenCodePair_ManifestRecordsTheWrittenHookFilesForTheTarget(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		r.mustRun(r.request())

		m := trLoadManifest(t, r.ws).ForHarness(r.tgt)
		got := ocHookPaths(m)
		sort.Strings(got)
		want := []string{ocPluginsDir + "/" + ocOnlyHelper[r.tgt], ocPluginsDir + "/mosaic-logger.ts"}
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("hook entry paths = %v, want the files the target variant wrote %v", got, want)
		}
		for _, e := range trEntryByRef(m, domain.ArtifactHook, ocHookKey) {
			if want := manifest.Hash([]byte(r.read(e.TargetPath))); e.ContentHash != want {
				t.Errorf("%s recorded with hash %q, want the hash of the written bytes %q", e.TargetPath, e.ContentHash, want)
			}
		}
		if _, ok := trEntryAt(trLoadManifest(t, r.ws), ocPluginsDir+"/"+ocOnlyHelper[r.src]); ok {
			t.Errorf("manifest still has an entry at the removed %s", ocOnlyHelper[r.src])
		}
	})
}

func TestTransformOpenCodePair_ManifestKeepsEntriesOfOtherHarnessesAndNeverRecordsUserFiles(t *testing.T) {
	ocEach(t, func(t *testing.T, r *trRun) {
		r.mustRun(r.request())

		full := trLoadManifest(t, r.ws)
		got, ok := trEntryAt(full, ocUnrelatedPath)
		want := ocUnrelatedEntry()
		if !ok || got.Harness != want.Harness || got.Ref != want.Ref || got.Version != want.Version || got.ContentHash != want.ContentHash {
			t.Errorf("unrelated entry = %+v (found %v), want it kept as %+v", got, ok, want)
		}
		for _, rel := range []string{ocUserPlugin, ocUserNote} {
			if e, found := trEntryAt(full, rel); found {
				t.Errorf("user file %s was recorded: %+v", rel, e)
			}
		}
	})
}

func TestTransformOpenCodePair_BundleWithoutATargetVariantLeavesNoSourceHookEntries(t *testing.T) {
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r := ocPairRun(t, pair[0], pair[1], []string{pair[0]})

			r.mustRun(r.request())

			full := trLoadManifest(t, r.ws)
			if got := ocEntriesOfKind(full, domain.ArtifactHook); len(got) != 0 {
				t.Errorf("hook entries = %+v, want none: the source files are gone and the target wrote none", got)
			}
			if full.Holds(pair[0]) {
				t.Errorf("manifest still holds the source %q", pair[0])
			}
		})
	}
}
