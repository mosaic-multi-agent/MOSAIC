package app_test

// transform_catalog_identity_test.go specifies that every manifest entry a transform writes
// (agent, skill file, hook file) carries the catalog identity (id, name, version) of the source
// deployment's entry: agents and skill files are matched by the source file, hook files by the
// source hook bundle, within the source harness. A source file without an entry or without an
// identity yields an entry without identity.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestTransformCatalogIdentity_Agents_TargetEntryCopiesTheSourceEntryIdentity(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			target := trLoadManifest(t, r.ws).ForHarness(p.tgt)
			for _, key := range trRecordedAgents {
				e, ok := trEntryAt(target, r.dest(key))
				if !ok {
					t.Errorf("%s: no target entry at %s", key, r.dest(key))
					continue
				}
				if want := trCatalogOf("agent-" + key); e.Catalog != want {
					t.Errorf("%s: catalog = %+v, want the source entry's %+v", key, e.Catalog, want)
				}
			}
		})
	}
}

func TestTransformCatalogIdentity_Agents_SourceFileWithoutEntryYieldsNoIdentity(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			target := trLoadManifest(t, r.ws).ForHarness(p.tgt)
			for _, key := range []string{"standalone", "orchestrator"} {
				e, ok := trEntryAt(target, r.dest(key))
				if !ok {
					t.Errorf("%s: no target entry at %s", key, r.dest(key))
					continue
				}
				if e.Catalog != (domain.CatalogIdentity{}) {
					t.Errorf("%s: catalog = %+v, want none: the source file has no entry", key, e.Catalog)
				}
			}
		})
	}
}

func TestTransformCatalogIdentity_Agents_SourceEntryWithoutIdentityYieldsNoIdentity(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	entries := trSeedRecordedSource(t, r)
	m := trLoadManifest(t, r.ws)
	for i := range m.Entries {
		if m.Entries[i].Ref.Key == "utility" {
			m.Entries[i].Catalog = domain.CatalogIdentity{}
		}
	}
	trSaveManifestValue(t, r.ws, m)
	req := r.request()
	req.TargetModel = "pre/answered"

	r.mustRun(req)

	target := trLoadManifest(t, r.ws).ForHarness(hOpen)
	e, ok := trEntryAt(target, r.dest("utility"))
	if !ok {
		t.Fatalf("no target entry at %s (seeded %d source entries)", r.dest("utility"), len(entries))
	}
	if e.Catalog != (domain.CatalogIdentity{}) {
		t.Errorf("catalog = %+v, want none: the source entry has no identity", e.Catalog)
	}
	other, _ := trEntryAt(target, r.dest("subagent"))
	if other.Catalog != trCatalogOf("agent-subagent") {
		t.Errorf("subagent catalog = %+v, want the identity of its own source entry", other.Catalog)
	}
}

func TestTransformCatalogIdentity_Skills_EveryFileCopiesItsSourceFileIdentity(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			target := trLoadManifest(t, r.ws).ForHarness(p.tgt)
			for file, name := range map[string]string{"SKILL.md": "skill-SKILL.md", "refs/checklist.md": "skill-checklist.md"} {
				path := r.skillsDir(p.tgt) + "/" + trSkillKey + "/" + file
				e, ok := trEntryAt(target, path)
				if !ok {
					t.Errorf("no target entry at %s", path)
					continue
				}
				if want := trCatalogOf(name); e.Catalog != want {
					t.Errorf("%s: catalog = %+v, want the source file's %+v", path, e.Catalog, want)
				}
			}
		})
	}
}

func TestTransformCatalogIdentity_Hooks_EveryFileCopiesTheSourceBundleIdentity(t *testing.T) {
	for _, p := range trKeepPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trCarryRun(t, p.src, p.tgt, nil)
			trSeedRecordedSource(t, r)
			req := r.request()
			req.TargetModel = "pre/answered"

			r.mustRun(req)

			entries := trEntryByRef(trLoadManifest(t, r.ws).ForHarness(p.tgt), domain.ArtifactHook, trHookKey)
			if len(entries) == 0 {
				t.Fatalf("target harness records no hook file of %s", trHookKey)
			}
			for _, e := range entries {
				if want := trCatalogOf("hook"); e.Catalog != want {
					t.Errorf("%s: catalog = %+v, want the source bundle's %+v", e.TargetPath, e.Catalog, want)
				}
			}
		})
	}
}

func TestTransformCatalogIdentity_Hooks_BundleWithoutSourceEntryYieldsNoIdentity(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	trSeedRecordedSource(t, r)
	m := trLoadManifest(t, r.ws)
	kept := m.Entries[:0]
	for _, e := range m.Entries {
		if e.Ref.Kind != domain.ArtifactHook {
			kept = append(kept, e)
		}
	}
	m.Entries = kept
	trSaveManifestValue(t, r.ws, m)
	req := r.request()
	req.TargetModel = "pre/answered"

	r.mustRun(req)

	entries := trEntryByRef(trLoadManifest(t, r.ws).ForHarness(hOpen), domain.ArtifactHook, trHookKey)
	if len(entries) == 0 {
		t.Fatalf("target harness records no hook file of %s", trHookKey)
	}
	for _, e := range entries {
		if e.Catalog != (domain.CatalogIdentity{}) {
			t.Errorf("%s: catalog = %+v, want none: the source bundle has no entry", e.TargetPath, e.Catalog)
		}
	}
}

func TestTransformCatalogIdentity_IgnoresEntriesOfOtherHarnesses(t *testing.T) {
	r := trCarryRun(t, hClaude, hOpen, nil)
	trSeedRecordedSource(t, r)
	m := trLoadManifest(t, r.ws)
	// An entry of a third harness at the same path must not be taken for the source entry.
	m.Entries = append(m.Entries, domain.ManifestEntry{
		Harness: hVSCode, Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "subagent"},
		TargetPath: r.agentsDir(hClaude) + "/subagent.md", Version: "0.0.1", Catalog: trCatalogOf("foreign"),
	})
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{HarnessID: hVSCode})
	trSaveManifestValue(t, r.ws, m)
	req := r.request()
	req.TargetModel = "pre/answered"

	r.mustRun(req)

	e, ok := trEntryAt(trLoadManifest(t, r.ws).ForHarness(hOpen), r.dest("subagent"))
	if !ok {
		t.Fatalf("no target entry at %s", r.dest("subagent"))
	}
	if want := trCatalogOf("agent-subagent"); e.Catalog != want {
		t.Errorf("catalog = %+v, want %+v from the source harness's entry only", e.Catalog, want)
	}
}
