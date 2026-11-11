package app_test

// transform_source_keep_test.go specifies what a whole-deployment transform leaves in the
// workspace manifest. For every harness pair outside GHCP CLI / VS Code GHCP the source
// deployment stays: its files, its entries and its section remain, and the manifest holds both
// harnesses. The GHCP pair keeps replace behaviour: afterwards only the target GHCP harness is
// held, while the entries of any other harness are untouched.

import (
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/domain"
)

// trKeepPairs are the pairs outside GHCP CLI / VS Code GHCP the source-keep tests run over.
var trKeepPairs = []struct{ name, src, tgt string }{
	{"claude-to-opencode", hClaude, hOpen},
	{"opencode-to-ghcp-cli", hOpen, hCLI},
}

// trKeepFields is the part of a source entry a transform must not change.
type trKeepFields struct {
	Harness     string
	Ref         domain.ArtifactRef
	Path        string
	Version     string
	ContentHash string
	Catalog     domain.CatalogIdentity
}

func trKeepOf(e domain.ManifestEntry) trKeepFields {
	return trKeepFields{e.Harness, e.Ref, e.TargetPath, e.Version, e.ContentHash, e.Catalog}
}

func TestTransformSourceKeep_OutsideTheGhcpPair_ManifestHoldsBothHarnesses(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			m := trLoadManifest(t, r.ws)
			if got, want := m.HarnessIDs(), trSortedIDs(p.src, p.tgt); !reflect.DeepEqual(got, want) {
				t.Errorf("held harnesses = %v, want both %v", got, want)
			}
			if _, ok := m.Section(p.src); !ok {
				t.Errorf("manifest has no section for the source %q", p.src)
			}
			if _, ok := m.Section(p.tgt); !ok {
				t.Errorf("manifest has no section for the target %q", p.tgt)
			}
		})
	}
}

func TestTransformSourceKeep_OutsideTheGhcpPair_SourceEntriesAndSectionAreUnchanged(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			seeded := trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			m := trLoadManifest(t, r.ws)
			var want, got []trKeepFields
			for _, e := range seeded {
				want = append(want, trKeepOf(e))
			}
			for _, e := range trEntriesOfHarness(m, p.src) {
				got = append(got, trKeepOf(e))
			}
			byPath := func(s []trKeepFields) {
				sort.Slice(s, func(i, j int) bool { return s[i].Path < s[j].Path })
			}
			byPath(got)
			byPath(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("source entries changed:\n got %+v\nwant %+v", got, want)
			}
			if s, ok := m.Section(p.src); !ok || !s.UpdatedAt.Equal(trSourceTime) {
				t.Errorf("source section = %+v (found %v), want it untouched at %v", s, ok, trSourceTime)
			}
		})
	}
}

func TestTransformSourceKeep_OutsideTheGhcpPair_SourceFilesAreUnchanged(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			seeded := trSeedRecordedSource(t, r)
			before := map[string]string{}
			for _, e := range seeded {
				before[e.TargetPath] = r.read(e.TargetPath)
			}
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			for rel, want := range before {
				if !r.exists(rel) {
					t.Errorf("source file %s was removed", rel)
				} else if got := r.read(rel); got != want {
					t.Errorf("source file %s was modified", rel)
				}
			}
		})
	}
}

func TestTransformSourceKeep_OutsideTheGhcpPair_TargetEntriesBelongToTheTargetHarness(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			target := trLoadManifest(t, r.ws).ForHarness(p.tgt)
			for _, key := range trAgentFixtures {
				if _, ok := trEntryAt(target, r.dest(key)); !ok {
					t.Errorf("target harness records no agent entry at %s", r.dest(key))
				}
			}
			if len(trEntryByRef(target, domain.ArtifactSkill, trSkillKey)) == 0 {
				t.Errorf("target harness records no entry for skill %s", trSkillKey)
			}
			if len(trEntryByRef(target, domain.ArtifactHook, trHookKey)) == 0 {
				t.Errorf("target harness records no entry for hook %s", trHookKey)
			}
		})
	}
}

func TestTransformSourceKeep_GhcpPair_OnlyTheTargetHarnessIsHeldAmongTheGhcpHarnesses(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		r.mustRun(r.request())

		m := trLoadManifest(t, r.ws)
		if m.Holds(r.src) {
			t.Errorf("manifest still holds the source harness %q: %+v", r.src, trEntriesOfHarness(m, r.src))
		}
		if _, ok := m.Section(r.src); ok {
			t.Errorf("manifest still has a section for the source harness %q", r.src)
		}
		if !m.Holds(r.tgt) {
			t.Errorf("manifest does not hold the target harness %q", r.tgt)
		}
		for _, e := range m.Entries {
			if e.Harness == r.src {
				t.Errorf("entry %s %s still belongs to the source harness", e.Ref.Kind, e.TargetPath)
			}
		}
	})
}

func TestTransformSourceKeep_GhcpPair_EntriesOfAnotherHarnessStayUntouched(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		r.mustRun(r.request())

		m := trLoadManifest(t, r.ws)
		if got, want := m.HarnessIDs(), trSortedIDs(hOpen, r.tgt); !reflect.DeepEqual(got, want) {
			t.Errorf("held harnesses = %v, want %v: the retired source is gone, the other harness stays", got, want)
		}
		got := trEntriesOfHarness(m, hOpen)
		want := trPairUnrelatedEntry()
		if len(got) != 1 || trKeepOf(got[0]) != trKeepOf(want) {
			t.Errorf("entries of %s = %+v, want exactly %+v", hOpen, got, want)
		}
	})
}
