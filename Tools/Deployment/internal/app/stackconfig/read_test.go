package stackconfig_test

// read_test.go covers Encode followed by Read, and the manifest states Read reports as "none".
//
//   - An encoded stack that a manifest carries reads back with the same layers, order and
//     default-layer choice; a second encoded stack replaces the first and keeps none of its layers.
//   - A workspace without a manifest, with an empty manifest, with a manifest that has no stack,
//     with an empty stack, or with a manifest written before the feature, has no persisted stack.
//   - A corrupt manifest and a manifest of a future schema are errors that name the manifest.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/manifest"
)

func TestRead_EncodedStackReadsBackWithSameLayersAndDefaultChoice(t *testing.T) {
	one, two := newCatalogRoot(t, "cat-one"), newCatalogRoot(t, "cat-two")
	d := stackconfig.Description{UserLayers: []string{one, two}, IncludeDefault: true}
	ws := saveStack(t, mustEncode(t, d).Layers...)

	got, ok, err := stackconfig.Read(ws)

	if err != nil || !ok {
		t.Fatalf("Read = (_, %v, %v), want ok and no error", ok, err)
	}
	want := []string{"cat-one", "cat-two", ""}
	if len(got.Layers) != 3 {
		t.Fatalf("Layers = %+v, want three", got.Layers)
	}
	for i, id := range want {
		if got.Layers[i].CatalogID != id {
			t.Errorf("Layers[%d].CatalogID = %q, want %q", i, got.Layers[i].CatalogID, id)
		}
	}
	back := got.Description()
	if len(back.UserLayers) != 2 || back.UserLayers[0] != one || back.UserLayers[1] != two || !back.IncludeDefault {
		t.Errorf("Description() = %+v, want layers [%s %s] with the default layer", back, one, two)
	}
}

func TestRead_UserLayersWithoutDefault_ReadBackWithoutDefault(t *testing.T) {
	one := newCatalogRoot(t, "cat-one")
	ws := saveStack(t, mustEncode(t, stackconfig.Description{UserLayers: []string{one}}).Layers...)

	got, ok, err := stackconfig.Read(ws)

	if err != nil || !ok {
		t.Fatalf("Read = (_, %v, %v), want ok and no error", ok, err)
	}
	if d := got.Description(); d.IncludeDefault || len(d.UserLayers) != 1 || d.UserLayers[0] != one {
		t.Errorf("Description() = %+v, want only %s", d, one)
	}
}

func TestRead_SecondEncodedStackReplacesFirstWithoutMerging(t *testing.T) {
	one, two, three := newCatalogRoot(t, "cat-one"), newCatalogRoot(t, "cat-two"), newCatalogRoot(t, "cat-three")
	first := mustEncode(t, stackconfig.Description{UserLayers: []string{one, two}, IncludeDefault: true})
	second := mustEncode(t, stackconfig.Description{UserLayers: []string{three}})

	for _, l := range second.Layers {
		for _, old := range first.Layers {
			if l == old {
				t.Fatalf("the second encoded value carries the first value's layer %+v", l)
			}
		}
	}
	ws := saveStack(t, second.Layers...)
	got, ok, err := stackconfig.Read(ws)

	if err != nil || !ok {
		t.Fatalf("Read = (_, %v, %v), want ok and no error", ok, err)
	}
	if len(got.Layers) != 1 || got.Layers[0].CatalogID != "cat-three" || got.Layers[0].Path != three {
		t.Errorf("Layers = %+v, want only the second stack's layer", got.Layers)
	}
}

func TestRead_WorkspaceWithoutManifest_ReportsNone(t *testing.T) {
	requireNoStack(t, t.TempDir())
}

func TestRead_EmptyManifest_ReportsNone(t *testing.T) {
	requireNoStack(t, writeRawManifest(t, ""))
}

func TestRead_ManifestWithoutStack_ReportsNone(t *testing.T) {
	requireNoStack(t, saveStack(t))
}

func TestRead_PreFeatureManifest_ReportsNone(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "pre_feature_manifest.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	requireNoStack(t, writeRawManifest(t, string(data)))
}

func TestRead_EmptyStackList_ReportsNoneWithoutError(t *testing.T) {
	ws := writeRawManifest(t, "schema_version: \"1\"\nharness_id: claude-code\nupdated_at: \"2025-01-01T00:00:00Z\"\nentries: []\ncatalog_stack: []\n")

	requireNoStack(t, ws)
}

func TestRead_CorruptManifest_ErrorNamesManifestPath(t *testing.T) {
	ws := writeRawManifest(t, "schema_version: [unterminated\n  : :\n")

	_, ok, err := stackconfig.Read(ws)

	if err == nil || ok {
		t.Fatalf("Read = (_, %v, %v), want an error", ok, err)
	}
	if want := filepath.Join(ws, manifest.Dir, manifest.FileName); !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name the manifest %q", err, want)
	}
}

func TestRead_FutureSchemaManifest_ErrorNamesManifestPath(t *testing.T) {
	ws := writeRawManifest(t, "schema_version: \"99\"\nharness_id: claude-code\nupdated_at: \"2025-01-01T00:00:00Z\"\nentries: []\n")

	_, ok, err := stackconfig.Read(ws)

	if err == nil || ok {
		t.Fatalf("Read = (_, %v, %v), want an error", ok, err)
	}
	if want := filepath.Join(ws, manifest.Dir, manifest.FileName); !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name the manifest %q", err, want)
	}
}
