package manifest_test

// catalog_stack_test.go covers the catalog stack the manifest persists.
//
//   - A stack survives a Save/Load round trip in order, including the default entry.
//   - The stack is stored under the top-level key catalog_stack as {path, catalog_id, default},
//     each omitted when empty or false, within schema "1".
//   - A manifest without a stack writes no catalog_stack key and loads with no stack, and a
//     manifest written before the feature loads normally with no stack.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func stackedManifest(layers ...domain.PersistedCatalogLayer) domain.Manifest {
	m := provenanceManifest()
	m.CatalogStack = layers
	return m
}

func readManifestText(t *testing.T, ws string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return string(data)
}

func TestCatalogStack_RoundTripsInOrder(t *testing.T) {
	want := []domain.PersistedCatalogLayer{
		{Path: "/abs/user-cat", CatalogID: "user-cat"},
		{Path: "/abs/other", CatalogID: "other-cat"},
		{Default: true},
	}

	snap, _ := saveAndLoad(t, stackedManifest(want...))

	if snap.State != manifest.StatePresent {
		t.Fatalf("State = %q (err %v), want present", snap.State, snap.Err)
	}
	if !reflect.DeepEqual(snap.Manifest.CatalogStack, want) {
		t.Errorf("CatalogStack = %+v, want %+v", snap.Manifest.CatalogStack, want)
	}
}

func TestCatalogStack_DefaultOnlyRoundTrips(t *testing.T) {
	want := []domain.PersistedCatalogLayer{{Default: true}}

	snap, _ := saveAndLoad(t, stackedManifest(want...))

	if !reflect.DeepEqual(snap.Manifest.CatalogStack, want) {
		t.Errorf("CatalogStack = %+v, want %+v", snap.Manifest.CatalogStack, want)
	}
}

func TestCatalogStack_WireKeysAreOmittedWhenEmpty(t *testing.T) {
	_, ws := saveAndLoad(t, stackedManifest(
		domain.PersistedCatalogLayer{Path: "/abs/user-cat", CatalogID: "user-cat"},
		domain.PersistedCatalogLayer{Default: true},
	))

	text := readManifestText(t, ws)

	for _, want := range []string{"catalog_stack:", "path:", "catalog_id: user-cat", "default: true", `schema_version: "1"`} {
		if !strings.Contains(text, want) {
			t.Errorf("manifest lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "default:") != 1 {
		t.Errorf("default must be written for the default entry only:\n%s", text)
	}
	if strings.Count(text, "path: /abs/user-cat") != 1 || strings.Count(text, "catalog_id:") != 1 {
		t.Errorf("empty path and catalog_id of the default entry must be omitted:\n%s", text)
	}
}

func TestCatalogStack_ManifestWithoutStackWritesNoKey(t *testing.T) {
	snap, ws := saveAndLoad(t, provenanceManifest())

	if strings.Contains(readManifestText(t, ws), "catalog_stack") {
		t.Error("a manifest without a stack must not write the catalog_stack key")
	}
	if len(snap.Manifest.CatalogStack) != 0 {
		t.Errorf("CatalogStack = %+v, want none", snap.Manifest.CatalogStack)
	}
}

func TestCatalogStack_PreFeatureManifestLoadsWithNoStack(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "pre_provenance_manifest.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, data)

	snap, err := manifest.NewStore().Load(ws)

	if err != nil || snap.State != manifest.StatePresent {
		t.Fatalf("Load: state %q, err %v / %v", snap.State, snap.Err, err)
	}
	if snap.Manifest.CatalogStack != nil {
		t.Errorf("CatalogStack = %+v, want nil for a manifest without the key", snap.Manifest.CatalogStack)
	}
}

func TestCatalogStack_HandWrittenKeyLoads(t *testing.T) {
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(`schema_version: "1"
harness_id: claude-code
updated_at: "2024-06-15T12:30:00Z"
entries: []
catalog_stack:
  - {path: /abs/user-cat, catalog_id: user-cat}
  - {default: true}
`))

	snap, err := manifest.NewStore().Load(ws)

	if err != nil || snap.State != manifest.StatePresent {
		t.Fatalf("Load: state %q, err %v / %v", snap.State, snap.Err, err)
	}
	want := []domain.PersistedCatalogLayer{{Path: "/abs/user-cat", CatalogID: "user-cat"}, {Default: true}}
	if !reflect.DeepEqual(snap.Manifest.CatalogStack, want) {
		t.Errorf("CatalogStack = %+v, want %+v", snap.Manifest.CatalogStack, want)
	}
}
