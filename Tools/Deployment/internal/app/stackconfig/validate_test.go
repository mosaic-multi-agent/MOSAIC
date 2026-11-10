package stackconfig_test

// validate_test.go covers Validate and the unresolvable-layer error.
//
//   - A stack whose layers still resolve (same path, same catalog id) is accepted, as are an
//     empty stack and a default-only stack.
//   - A layer whose folder is missing, whose identity is missing or invalid, or where a
//     different catalog id is now at the path, is an UnresolvableLayerError that names the
//     layer's path and expected id and wraps the cause.
//   - The first failing layer, highest first, is the one reported.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/domain"
)

// requireUnresolvable asserts err is an UnresolvableLayerError for the layer and returns it.
func requireUnresolvable(t *testing.T, err error, path, id string) *stackconfig.UnresolvableLayerError {
	t.Helper()
	var ule *stackconfig.UnresolvableLayerError
	if !errors.As(err, &ule) {
		t.Fatalf("error = %v (%T), want *UnresolvableLayerError", err, err)
	}
	if !errors.Is(err, stackconfig.ErrLayerUnresolvable) {
		t.Errorf("error %v does not match ErrLayerUnresolvable", err)
	}
	if ule.Path != path || ule.ExpectedID != id {
		t.Errorf("error names (%q, %q), want (%q, %q)", ule.Path, ule.ExpectedID, path, id)
	}
	if ule.Err == nil {
		t.Error("UnresolvableLayerError.Err is nil; the cause must always be set")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), id) {
		t.Errorf("error text %q must name the layer path %q and expected id %q", err, path, id)
	}
	return ule
}

func TestValidate_ResolvableLayers_AreAccepted(t *testing.T) {
	one, two := newCatalogRoot(t, "cat-one"), newCatalogRoot(t, "cat-two")
	s := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{userLayer(one, "cat-one"), userLayer(two, "cat-two"), defaultLayer}}

	if err := stackconfig.Validate(s, t.TempDir()); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidate_NoLayersOrDefaultOnly_AreAccepted(t *testing.T) {
	for name, s := range map[string]stackconfig.Stored{
		"nil layers":   {},
		"empty layers": {Layers: []domain.PersistedCatalogLayer{}},
		"default only": {Layers: []domain.PersistedCatalogLayer{defaultLayer}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := stackconfig.Validate(s, t.TempDir()); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

func TestValidate_MissingFolder_NamesLayer(t *testing.T) {
	root := newCatalogRoot(t, "cat-one")
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove folder: %v", err)
	}
	s := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{userLayer(root, "cat-one")}}

	err := stackconfig.Validate(s, t.TempDir())

	ule := requireUnresolvable(t, err, root, "cat-one")
	if !errors.Is(ule.Err, os.ErrNotExist) {
		t.Errorf("cause = %v, want the folder to be reported as missing", ule.Err)
	}
}

func TestValidate_MissingIdentity_NamesLayer(t *testing.T) {
	root := newCatalogRoot(t, "cat-one")
	if err := os.Remove(filepath.Join(root, "catalog-identity.yaml")); err != nil {
		t.Fatalf("remove identity: %v", err)
	}
	s := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{userLayer(root, "cat-one")}}

	err := stackconfig.Validate(s, t.TempDir())

	ule := requireUnresolvable(t, err, root, "cat-one")
	if !errors.Is(ule.Err, identity.ErrIdentityMissing) {
		t.Errorf("cause = %v, want identity.ErrIdentityMissing", ule.Err)
	}
}

func TestValidate_InvalidIdentity_NamesLayer(t *testing.T) {
	root := newCatalogRoot(t, "cat-one")
	if err := os.WriteFile(filepath.Join(root, "catalog-identity.yaml"), []byte("id: \"Not A Valid Id\"\nname: x\nversion: \"1\"\n"), 0o644); err != nil {
		t.Fatalf("write identity: %v", err)
	}
	s := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{userLayer(root, "cat-one")}}

	err := stackconfig.Validate(s, t.TempDir())

	ule := requireUnresolvable(t, err, root, "cat-one")
	if !errors.Is(ule.Err, identity.ErrIdentityInvalid) {
		t.Errorf("cause = %v, want identity.ErrIdentityInvalid", ule.Err)
	}
}

func TestValidate_DifferentCatalogIdAtPath_NamesLayerAndNewId(t *testing.T) {
	root := newCatalogRoot(t, "replacement-cat")
	s := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{userLayer(root, "cat-one")}}

	err := stackconfig.Validate(s, t.TempDir())

	ule := requireUnresolvable(t, err, root, "cat-one")
	if !strings.Contains(ule.Err.Error(), "replacement-cat") {
		t.Errorf("cause %q does not name the catalog id now at the path", ule.Err)
	}
}

func TestValidate_ReportsFirstFailingLayerHighestFirst(t *testing.T) {
	good := newCatalogRoot(t, "cat-good")
	brokenHigh := newCatalogRoot(t, "other-id")
	brokenLow := filepath.Join(t.TempDir(), "gone")
	s := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{
		userLayer(good, "cat-good"), userLayer(brokenHigh, "cat-high"), userLayer(brokenLow, "cat-low"),
	}}

	err := stackconfig.Validate(s, t.TempDir())

	requireUnresolvable(t, err, brokenHigh, "cat-high")
}

func TestUnresolvableLayerError_UnwrapsSentinelAndCause(t *testing.T) {
	cause := errors.New("folder is gone")
	err := &stackconfig.UnresolvableLayerError{Path: "/p", ExpectedID: "cat", Err: cause}

	if !errors.Is(err, stackconfig.ErrLayerUnresolvable) || !errors.Is(err, cause) {
		t.Errorf("errors.Is must match both ErrLayerUnresolvable and the cause: %v", err)
	}
	if u := err.Unwrap(); len(u) != 2 {
		t.Errorf("Unwrap() = %v, want exactly the sentinel and the cause", u)
	}
}
