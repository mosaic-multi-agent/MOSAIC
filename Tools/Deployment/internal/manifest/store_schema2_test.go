package manifest_test

// Tests for the multi-harness manifest schema: several harnesses in one file, one entry per
// harness for a shared path, per-harness sections, and the unchanged top-level entries list.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-yaml"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

var (
	t0           = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	catProtocol  = domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC Default Catalog", Version: "0.7.0"}
	catBundle    = domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"}
	sharedAgent  = domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "reviewer"}
	sharedTarget = ".github/agents/reviewer.agent.md"
)

// entryFor returns a populated entry of harness at targetPath.
func entryFor(harness, targetPath string, ref domain.ArtifactRef, hash string) domain.ManifestEntry {
	e := sampleEntry(targetPath, ref)
	e.Harness = harness
	e.ContentHash = hash
	return e
}

// multiHarnessManifest holds opencode and ghcp-cli, with the agent at the shared .github path
// recorded once per harness (each harness has its own view of the file).
func multiHarnessManifest() domain.Manifest {
	return domain.Manifest{
		SchemaVersion: "2",
		ToolVersion:   "1.7.0",
		Harnesses: []domain.HarnessSection{
			{HarnessID: "ghcp-cli", UpdatedAt: t0.Add(time.Hour), ProtocolSource: catProtocol, BundleSource: catBundle},
			{HarnessID: "opencode", UpdatedAt: t0},
		},
		Entries: []domain.ManifestEntry{
			entryFor("opencode", ".opencode/agents/reviewer.md", sharedAgent, "sha256:aa"),
			entryFor("ghcp-cli", sharedTarget, sharedAgent, "sha256:bb"),
			entryFor("opencode", sharedTarget, sharedAgent, "sha256:cc"),
		},
	}
}

// yamlDoc reads the saved manifest file of ws as a generic document.
func yamlDoc(t *testing.T, ws string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("manifest is not valid YAML: %v", err)
	}
	return doc
}

func saveMulti(t *testing.T) (manifest.Store, string) {
	t.Helper()
	store := manifest.NewStore()
	ws := makeWorkspace(t)
	must(t, store.Save(ws, multiHarnessManifest()))
	return store, ws
}

func TestStoreSchema2_SaveWritesSchemaVersion2(t *testing.T) {
	_, ws := saveMulti(t)

	doc := yamlDoc(t, ws)

	if doc["schema_version"] != "2" {
		t.Errorf("schema_version = %v, want 2", doc["schema_version"])
	}
}

func TestStoreSchema2_SavedFileHasNoTopLevelSingleHarnessKeys(t *testing.T) {
	_, ws := saveMulti(t)

	doc := yamlDoc(t, ws)

	for _, key := range []string{"harness_id", "updated_at", "protocol_source", "bundle_source"} {
		if _, found := doc[key]; found {
			t.Errorf("saved manifest has top-level key %q; the values live in the harness sections", key)
		}
	}
}

func TestStoreSchema2_SavedFileKeepsToolVersionAtTopLevel(t *testing.T) {
	_, ws := saveMulti(t)

	doc := yamlDoc(t, ws)

	if doc["tool_version"] != "1.7.0" {
		t.Errorf("tool_version = %v, want 1.7.0 at the top level", doc["tool_version"])
	}
}

func TestStoreSchema2_SavedFileKeepsTopLevelEntriesWithTargetPath(t *testing.T) {
	_, ws := saveMulti(t)

	doc := yamlDoc(t, ws)

	entries, ok := doc["entries"].([]any)
	if !ok || len(entries) != 3 {
		t.Fatalf("entries = %v, want a top-level list of 3 entries", doc["entries"])
	}
	for i, raw := range entries {
		e, _ := raw.(map[string]any)
		if p, _ := e["target_path"].(string); p == "" {
			t.Errorf("entry %d has no target_path: %v", i, e)
		}
		if h, _ := e["harness"].(string); h == "" {
			t.Errorf("entry %d records no harness: %v", i, e)
		}
	}
}

func TestStoreSchema2_SavedFileHoldsOneSectionPerHarnessWithItsValues(t *testing.T) {
	_, ws := saveMulti(t)

	doc := yamlDoc(t, ws)

	sections, ok := doc["harnesses"].([]any)
	if !ok || len(sections) != 2 {
		t.Fatalf("harnesses = %v, want 2 sections", doc["harnesses"])
	}
	first, _ := sections[0].(map[string]any)
	second, _ := sections[1].(map[string]any)
	if first["harness_id"] != "ghcp-cli" || second["harness_id"] != "opencode" {
		t.Errorf("section order = %v, %v, want ghcp-cli then opencode (sorted by harness id)",
			first["harness_id"], second["harness_id"])
	}
	for _, key := range []string{"updated_at", "protocol_source", "bundle_source"} {
		if _, found := first[key]; !found {
			t.Errorf("ghcp-cli section lacks %q", key)
		}
	}
	for _, key := range []string{"protocol_source", "bundle_source"} {
		if _, found := second[key]; found {
			t.Errorf("opencode section has %q although its sources are unknown", key)
		}
	}
}

func TestStoreSchema2_SectionIsWrittenOnlyForHarnessesWithEntries(t *testing.T) {
	store := manifest.NewStore()
	ws := makeWorkspace(t)
	m := multiHarnessManifest()
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{HarnessID: "claude-code", UpdatedAt: t0})

	must(t, store.Save(ws, m))

	sections, _ := yamlDoc(t, ws)["harnesses"].([]any)
	if len(sections) != 2 {
		t.Errorf("harnesses = %v, want the two harnesses that have entries (no claude-code)", sections)
	}
}

func TestStoreSchema2_RoundTripKeepsEveryHarnessSection(t *testing.T) {
	store, ws := saveMulti(t)

	snap, err := store.Load(ws)

	must(t, err)
	if snap.State != manifest.StatePresent {
		t.Fatalf("State = %q (err %v), want present", snap.State, snap.Err)
	}
	ghcp, okG := snap.Manifest.Section("ghcp-cli")
	open, okO := snap.Manifest.Section("opencode")
	if !okG || !okO {
		t.Fatalf("sections found: ghcp-cli %v, opencode %v; want both", okG, okO)
	}
	if !ghcp.UpdatedAt.Equal(t0.Add(time.Hour)) || ghcp.ProtocolSource != catProtocol || ghcp.BundleSource != catBundle {
		t.Errorf("ghcp-cli section = %+v, want the saved values", ghcp)
	}
	if !open.UpdatedAt.Equal(t0) || open.ProtocolSource != (domain.CatalogIdentity{}) {
		t.Errorf("opencode section = %+v, want its own UpdatedAt and unknown sources", open)
	}
}

func TestStoreSchema2_RoundTripKeepsTheSamePathOncePerHarness(t *testing.T) {
	store, ws := saveMulti(t)

	snap, err := store.Load(ws)

	must(t, err)
	var atShared []domain.ManifestEntry
	for _, e := range snap.Manifest.Entries {
		if e.TargetPath == sharedTarget {
			atShared = append(atShared, e)
		}
	}
	if len(atShared) != 2 {
		t.Fatalf("entries at %s = %+v, want one per harness", sharedTarget, atShared)
	}
	byHarness := map[string]string{}
	for _, e := range atShared {
		byHarness[e.Harness] = e.ContentHash
	}
	if byHarness["ghcp-cli"] != "sha256:bb" || byHarness["opencode"] != "sha256:cc" {
		t.Errorf("content hashes by harness = %v, want each harness to keep its own entry", byHarness)
	}
}

func TestStoreSchema2_RoundTripKeepsTheHarnessOfEveryEntry(t *testing.T) {
	store, ws := saveMulti(t)

	snap, err := store.Load(ws)

	must(t, err)
	got, ok := snap.Manifest.LookupAt(sharedAgent, ".opencode/agents/reviewer.md")
	if !ok || got.Harness != "opencode" {
		t.Errorf("entry at the opencode path = %+v (found %v), want harness opencode", got, ok)
	}
}

func TestStoreSchema2_EntriesAreSortedByTargetPathThenHarness(t *testing.T) {
	store, ws := saveMulti(t)

	snap, err := store.Load(ws)

	must(t, err)
	want := [][2]string{
		{".github/agents/reviewer.agent.md", "ghcp-cli"},
		{".github/agents/reviewer.agent.md", "opencode"},
		{".opencode/agents/reviewer.md", "opencode"},
	}
	if len(snap.Manifest.Entries) != len(want) {
		t.Fatalf("entries = %d, want %d", len(snap.Manifest.Entries), len(want))
	}
	for i, w := range want {
		e := snap.Manifest.Entries[i]
		if e.TargetPath != w[0] || e.Harness != w[1] {
			t.Errorf("entry %d = (%s, %s), want (%s, %s)", i, e.TargetPath, e.Harness, w[0], w[1])
		}
	}
}

func TestStoreSchema2_RoundTripKeepsToolVersionAndSchemaVersion(t *testing.T) {
	store, ws := saveMulti(t)

	snap, err := store.Load(ws)

	must(t, err)
	if snap.Manifest.ToolVersion != "1.7.0" || snap.Manifest.SchemaVersion != "2" {
		t.Errorf("ToolVersion = %q, SchemaVersion = %q, want 1.7.0 and 2",
			snap.Manifest.ToolVersion, snap.Manifest.SchemaVersion)
	}
}

func TestStoreSchema2_NewSchemaFileLoadsWithoutMigration(t *testing.T) {
	store, ws := saveMulti(t)

	snap, err := store.Load(ws)

	must(t, err)
	if snap.MigratedFrom != "" || snap.MigrationDropped != nil {
		t.Errorf("MigratedFrom = %q, MigrationDropped = %+v, want neither for a new-schema file",
			snap.MigratedFrom, snap.MigrationDropped)
	}
}

func TestStoreSchema2_EntryWithoutHarnessIsCorrupt(t *testing.T) {
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte("schema_version: \"2\"\n"+
		"harnesses:\n  - harness_id: opencode\n    updated_at: \"2026-11-05T10:00:00Z\"\n"+
		"entries:\n  - ref:\n      kind: agent\n      key: a\n    target_path: .opencode/agents/a.md\n"+
		"    version: \"1\"\n    content_hash: sha256:00\n    deployed_at: \"2026-11-05T10:00:00Z\"\n"))

	snap, err := manifest.NewStore().Load(ws)

	must(t, err)
	if snap.State != manifest.StateCorrupt || snap.Err == nil {
		t.Errorf("State = %q, Err = %v, want corrupt with an error", snap.State, snap.Err)
	}
}

func TestStoreSchema2_SecondSaveReplacesTheHarnessSections(t *testing.T) {
	store, ws := saveMulti(t)
	m := multiHarnessManifest()
	m.Harnesses = m.Harnesses[1:] // opencode only
	m.Entries = []domain.ManifestEntry{m.Entries[0]}

	must(t, store.Save(ws, m))
	snap, err := store.Load(ws)

	must(t, err)
	if _, ok := snap.Manifest.Section("ghcp-cli"); ok {
		t.Errorf("ghcp-cli section survived a save that no longer has the harness")
	}
	if len(snap.Manifest.Entries) != 1 {
		t.Errorf("entries = %d, want 1", len(snap.Manifest.Entries))
	}
}

func TestStoreSchema2_SaveWritesTheNewSchemaWhateverTheManifestFieldSays(t *testing.T) {
	for _, stale := range []string{"", "1"} {
		store := manifest.NewStore()
		ws := makeWorkspace(t)
		m := multiHarnessManifest()
		m.SchemaVersion = stale

		must(t, store.Save(ws, m))

		if got := yamlDoc(t, ws)["schema_version"]; got != "2" {
			t.Errorf("SchemaVersion field %q: saved schema_version = %v, want 2", stale, got)
		}
		snap, err := store.Load(ws)
		must(t, err)
		if snap.State != manifest.StatePresent || len(snap.Manifest.Entries) != 3 {
			t.Errorf("SchemaVersion field %q: reload gives State %q with %d entries, want present with 3",
				stale, snap.State, len(snap.Manifest.Entries))
		}
	}
}

func TestStoreSchema2_SchemaVersionIsRaisedAndTheOldOneIsKnownAsLegacy(t *testing.T) {
	if manifest.SchemaVersion != "2" || manifest.LegacySchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, LegacySchemaVersion = %q, want 2 and 1",
			manifest.SchemaVersion, manifest.LegacySchemaVersion)
	}
}
