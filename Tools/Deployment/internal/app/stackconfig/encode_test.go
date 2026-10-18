package stackconfig_test

// encode_test.go covers Encode.
//
//   - User layers are encoded in the description's order with the catalog id read from each
//     layer's identity, followed by one default entry when the description includes it.
//   - The default layer alone encodes to exactly one default entry.
//   - A layer with no folder or no valid identity is an UnresolvableLayerError naming the path.
//   - An empty description is an empty-stack error, not an unresolvable-layer error.
//   - Whatever Encode returns passes the structural check of Validate.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/stackerr"
	"mosaic-deploy/internal/domain"
)

func requireLayers(t *testing.T, got stackconfig.Stored, want ...domain.PersistedCatalogLayer) {
	t.Helper()
	if len(got.Layers) != len(want) {
		t.Fatalf("Layers = %+v, want %+v", got.Layers, want)
	}
	for i := range want {
		if got.Layers[i] != want[i] {
			t.Errorf("Layers[%d] = %+v, want %+v", i, got.Layers[i], want[i])
		}
	}
}

func TestEncode_UserLayersInOrderThenDefault(t *testing.T) {
	one, two := newCatalogRoot(t, "cat-one"), newCatalogRoot(t, "cat-two")

	got := mustEncode(t, stackconfig.Description{UserLayers: []string{two, one}, IncludeDefault: true})

	requireLayers(t, got, userLayer(two, "cat-two"), userLayer(one, "cat-one"), defaultLayer)
}

func TestEncode_UserLayersWithoutDefault_HaveNoDefaultEntry(t *testing.T) {
	one := newCatalogRoot(t, "cat-one")

	got := mustEncode(t, stackconfig.Description{UserLayers: []string{one}})

	requireLayers(t, got, userLayer(one, "cat-one"))
}

func TestEncode_DefaultLayerAlone_IsExactlyOneDefaultEntry(t *testing.T) {
	got := mustEncode(t, stackconfig.Description{IncludeDefault: true})

	requireLayers(t, got, defaultLayer)
}

func TestEncode_MissingFolder_IsUnresolvableLayerNamingPath(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")

	_, err := stackconfig.Encode(stackconfig.Description{UserLayers: []string{gone}})

	var ule *stackconfig.UnresolvableLayerError
	if !errors.As(err, &ule) || ule.Path != gone {
		t.Fatalf("error = %v, want an UnresolvableLayerError for %q", err, gone)
	}
}

func TestEncode_LayerWithoutIdentity_IsUnresolvableLayerNamingPath(t *testing.T) {
	bare := t.TempDir()

	got, err := stackconfig.Encode(stackconfig.Description{UserLayers: []string{bare}, IncludeDefault: true})

	var ule *stackconfig.UnresolvableLayerError
	if !errors.As(err, &ule) || ule.Path != bare {
		t.Fatalf("error = %v, want an UnresolvableLayerError for %q", err, bare)
	}
	if !errors.Is(err, identity.ErrIdentityMissing) {
		t.Errorf("error %v does not wrap identity.ErrIdentityMissing", err)
	}
	if len(got.Layers) != 0 {
		t.Errorf("Layers = %+v on error, want a zero Stored", got.Layers)
	}
}

func TestEncode_SecondLayerInvalid_ErrorNamesThatLayer(t *testing.T) {
	good := newCatalogRoot(t, "cat-one")
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "catalog-identity.yaml"), []byte("id: [unterminated\n"), 0o644); err != nil {
		t.Fatalf("write identity: %v", err)
	}

	_, err := stackconfig.Encode(stackconfig.Description{UserLayers: []string{good, bad}})

	var ule *stackconfig.UnresolvableLayerError
	if !errors.As(err, &ule) || ule.Path != bad {
		t.Fatalf("error = %v, want an UnresolvableLayerError for %q", err, bad)
	}
}

func TestEncode_EmptyDescription_IsEmptyStackError(t *testing.T) {
	got, err := stackconfig.Encode(stackconfig.Description{})

	if !errors.Is(err, stackerr.ErrEmptyStack) {
		t.Fatalf("error = %v, want stackerr.ErrEmptyStack", err)
	}
	var ule *stackconfig.UnresolvableLayerError
	if errors.As(err, &ule) {
		t.Errorf("an empty description must not be reported as an unresolvable layer: %v", err)
	}
	if len(got.Layers) != 0 {
		t.Errorf("Layers = %+v on error, want a zero Stored", got.Layers)
	}
}

func TestEncode_Result_PassesValidate(t *testing.T) {
	one := newCatalogRoot(t, "cat-one")
	for name, d := range map[string]stackconfig.Description{
		"user layer and default": {UserLayers: []string{one}, IncludeDefault: true},
		"user layer only":        {UserLayers: []string{one}},
		"default only":           {IncludeDefault: true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := stackconfig.Validate(mustEncode(t, d), t.TempDir()); err != nil {
				t.Errorf("Validate(Encode(%+v)): %v", d, err)
			}
		})
	}
}
