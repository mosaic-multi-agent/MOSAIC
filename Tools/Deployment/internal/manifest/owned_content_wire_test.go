package manifest_test

// owned_content_wire_test.go specifies the wire form of the Deploy-owned content record: an
// entry's record is saved under owned_content, loads back equal, and an entry without a record
// (written before the record existed) loads with none and is saved without the key.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func ownedWireManifest(owned *domain.OwnedContent) domain.Manifest {
	e := entryFor("claude-code", ".claude/agents/reviewer.md", sharedAgent, "sha256:whole")
	e.Owned = owned
	return domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: "claude-code", UpdatedAt: t0}},
		Entries:       []domain.ManifestEntry{e},
	}
}

func saveAndRead(t *testing.T, m domain.Manifest) (string, string) {
	t.Helper()
	ws := t.TempDir()
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return ws, string(raw)
}

func TestSave_OwnedContent_WrittenUnderOwnedContentKey(t *testing.T) {
	owned := &domain.OwnedContent{Hash: "sha256:owned", Regions: []domain.RegionHash{{Name: "Identity", Hash: "sha256:r1"}}}

	_, raw := saveAndRead(t, ownedWireManifest(owned))

	for _, want := range []string{"owned_content:", "sha256:owned", "Identity", "sha256:r1"} {
		if !strings.Contains(raw, want) {
			t.Errorf("manifest file lacks %q:\n%s", want, raw)
		}
	}
	if !strings.Contains(raw, "content_hash: sha256:whole") {
		t.Errorf("content_hash must keep recording the whole written bytes:\n%s", raw)
	}
}

func TestLoad_OwnedContent_RoundTripsEqual(t *testing.T) {
	owned := &domain.OwnedContent{Hash: "sha256:owned", Regions: []domain.RegionHash{
		{Name: "Identity", Hash: "sha256:r1"}, {Name: domain.OutsideRegionsName, Hash: "sha256:r2"},
	}}
	ws, _ := saveAndRead(t, ownedWireManifest(owned))

	snap, err := manifest.NewStore().Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := snap.Manifest.Entries[0].Owned
	if got == nil || got.Hash != owned.Hash || len(got.Regions) != 2 || got.Regions[0] != owned.Regions[0] || got.Regions[1] != owned.Regions[1] {
		t.Errorf("loaded Owned = %+v; want %+v", got, owned)
	}
}

func TestSaveLoad_EntryWithoutRecord_HasNoRecordAndNoKey(t *testing.T) {
	ws, raw := saveAndRead(t, ownedWireManifest(nil))

	if strings.Contains(raw, "owned_content") {
		t.Errorf("an entry without a record must not write owned_content:\n%s", raw)
	}
	snap, err := manifest.NewStore().Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := snap.Manifest.Entries[0].Owned; got != nil {
		t.Errorf("loaded Owned = %+v; want nil", got)
	}
}
