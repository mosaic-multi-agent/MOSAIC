package manifest_test

// provenance_test.go covers the catalog provenance the manifest records: the source catalog of
// each deployed file and the catalogs that supplied the protocol and the bundle.
//
//   - Entry provenance and the two top-level suppliers survive a Save/Load round trip.
//   - Provenance is additive within the current schema: a manifest carrying it still loads in
//     the normal present state.
//   - A manifest written before provenance existed loads normally, with the catalog fields of
//     its entries and the top-level suppliers absent (zero).
//   - A zero identity writes no catalog keys at all.
//   - Workflow provenance is not stored per entry: the manifest has no workflow entries.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func provenanceManifest() domain.Manifest {
	agent := sampleEntry(".claude/agents/code-reviewer.md", domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "code-reviewer"})
	agent.Catalog = domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"}
	skill := sampleEntry(".claude/skills/s/SKILL.md", domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "s"})
	skill.Catalog = domain.CatalogIdentity{ID: "mosaic-default", Version: "0.7.0"}
	hook := sampleEntry(".claude/hooks/h", domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "h"})
	hook.Catalog = domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"}
	return domain.Manifest{
		SchemaVersion:  manifest.SchemaVersion,
		HarnessID:      "claude-code",
		UpdatedAt:      time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC),
		Entries:        []domain.ManifestEntry{agent, skill, hook},
		ProtocolSource: domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC Default Catalog", Version: "0.7.0"},
		BundleSource:   domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"},
	}
}

func saveAndLoad(t *testing.T, m domain.Manifest) (manifest.Snapshot, string) {
	t.Helper()
	ws := makeWorkspace(t)
	store := manifest.NewStore()
	if err := store.Save(ws, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	snap, err := store.Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return snap, ws
}

// TestProvenance_EntryCatalogRoundTrips verifies each entry keeps the id, name and version of
// its source catalog, and that a version such as 1.10 is not reformatted.
func TestProvenance_EntryCatalogRoundTrips(t *testing.T) {
	want := provenanceManifest()

	snap, _ := saveAndLoad(t, want)

	if snap.State != manifest.StatePresent {
		t.Fatalf("State = %q, want present", snap.State)
	}
	for _, w := range want.Entries {
		got, ok := snap.Manifest.Lookup(w.Ref)
		if !ok {
			t.Fatalf("entry %v missing after round trip", w.Ref)
		}
		if got.Catalog != w.Catalog {
			t.Errorf("entry %v Catalog = %+v, want %+v", w.Ref, got.Catalog, w.Catalog)
		}
	}
}

// TestProvenance_SuppliersRoundTrip verifies the protocol and bundle suppliers survive a round trip.
func TestProvenance_SuppliersRoundTrip(t *testing.T) {
	want := provenanceManifest()

	snap, _ := saveAndLoad(t, want)

	if snap.Manifest.ProtocolSource != want.ProtocolSource {
		t.Errorf("ProtocolSource = %+v, want %+v", snap.Manifest.ProtocolSource, want.ProtocolSource)
	}
	if snap.Manifest.BundleSource != want.BundleSource {
		t.Errorf("BundleSource = %+v, want %+v", snap.Manifest.BundleSource, want.BundleSource)
	}
}

// TestProvenance_ZeroIdentityWritesNoCatalogKeys verifies unknown provenance leaves the file free
// of catalog keys, and the manifest stays on schema "1".
func TestProvenance_ZeroIdentityWritesNoCatalogKeys(t *testing.T) {
	m := sampleManifest()
	// Provenance recorded for one entry, so a store that writes nothing at all cannot pass the
	// positive half below.
	m.Entries[0].Catalog = domain.CatalogIdentity{ID: "user-cat", Version: "1.0"}
	m.Entries[1].Catalog = domain.CatalogIdentity{}

	_, ws := saveAndLoad(t, m)

	data, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "user-cat") {
		t.Fatalf("manifest does not record the entry's catalog id:\n%s", text)
	}
	if strings.Count(text, "catalog:") != 1 {
		t.Errorf("want exactly one catalog key (one entry has provenance), got:\n%s", text)
	}
	for _, key := range []string{"protocol_source", "bundle_source"} {
		if strings.Contains(text, key) {
			t.Errorf("manifest contains %q although no supplier is recorded:\n%s", key, text)
		}
	}
	if !strings.Contains(text, `schema_version: "1"`) {
		t.Errorf("provenance must stay within schema \"1\":\n%s", text)
	}
}

// TestProvenance_PreFeatureManifestLoadsWithUnknownProvenance verifies a manifest written before
// provenance existed loads in the normal state, keeps all its data, and has zero catalog fields.
func TestProvenance_PreFeatureManifestLoadsWithUnknownProvenance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "pre_provenance_manifest.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, data)

	snap, err := manifest.NewStore().Load(ws)

	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.State != manifest.StatePresent {
		t.Fatalf("State = %q (err %v), want present", snap.State, snap.Err)
	}
	if len(snap.Manifest.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(snap.Manifest.Entries))
	}
	for _, e := range snap.Manifest.Entries {
		if e.Catalog != (domain.CatalogIdentity{}) {
			t.Errorf("entry %v Catalog = %+v, want zero (unknown)", e.Ref, e.Catalog)
		}
	}
	if snap.Manifest.ProtocolSource != (domain.CatalogIdentity{}) || snap.Manifest.BundleSource != (domain.CatalogIdentity{}) {
		t.Errorf("suppliers = %+v / %+v, want zero (unknown)", snap.Manifest.ProtocolSource, snap.Manifest.BundleSource)
	}
	cr, ok := snap.Manifest.Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "code-reviewer"})
	if !ok || cr.Version != "1.2.3" || cr.InjectionsVersion != "3.1.0" {
		t.Errorf("pre-feature entry data lost: %+v (found %v)", cr, ok)
	}
}

// TestProvenance_PreFeatureManifestGainsProvenanceOnResave verifies that saving a pre-feature
// manifest back with provenance filled in makes the provenance known on the next load.
func TestProvenance_PreFeatureManifestGainsProvenanceOnResave(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "pre_provenance_manifest.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, data)
	store := manifest.NewStore()
	snap, err := store.Load(ws)
	if err != nil || snap.State != manifest.StatePresent {
		t.Fatalf("Load: state %q err %v / %v", snap.State, snap.Err, err)
	}
	m := snap.Manifest
	m.Entries[0].Catalog = domain.CatalogIdentity{ID: "user-cat", Version: "2.0"}
	m.ProtocolSource = domain.CatalogIdentity{ID: "mosaic-default", Version: "0.7.0"}

	if err := store.Save(ws, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	again, err := store.Load(ws)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}

	if again.Manifest.Entries[0].Catalog.ID != "user-cat" {
		t.Errorf("entry Catalog = %+v after redeploy, want id user-cat", again.Manifest.Entries[0].Catalog)
	}
	if again.Manifest.ProtocolSource.ID != "mosaic-default" {
		t.Errorf("ProtocolSource = %+v after redeploy, want id mosaic-default", again.Manifest.ProtocolSource)
	}
	if again.Manifest.BundleSource != (domain.CatalogIdentity{}) {
		t.Errorf("BundleSource = %+v, want it still unknown", again.Manifest.BundleSource)
	}
}
