package main

// stack_error_test.go verifies that stack errors surface from buildCatalogStack with the
// offending layer named, so the composition root can stop before any service work.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// buildErr builds the stack for args and returns the error, failing the test when none occurs.
func buildErr(t *testing.T, mosaicRoot string, args ...string) error {
	t.Helper()
	cat, err := buildCatalogStack(mosaicRoot, scanStackInput(args))
	if err == nil {
		t.Fatalf("buildCatalogStack for %v succeeded (layers %+v), want a stack error", args, cat.Layers())
	}
	return err
}

func assertNamesLayer(t *testing.T, err error, roots ...string) {
	t.Helper()
	for _, r := range roots {
		if !strings.Contains(err.Error(), r) {
			t.Errorf("error %q does not name the layer %q", err.Error(), r)
		}
	}
}

func TestBuildCatalogStack_LayerWithoutIdentity_NamesTheLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	bare := t.TempDir() // no catalog-identity.yaml

	err := buildErr(t, mosaicRoot, "--catalog-folder", bare, "deploy")

	if !errors.Is(err, stackerr.ErrLayerIdentity) {
		t.Errorf("error = %v, want stackerr.ErrLayerIdentity", err)
	}
	assertNamesLayer(t, err, absOf(t, bare))
}

func TestBuildCatalogStack_MissingLayerFolder_NamesTheLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	missing := filepath.Join(t.TempDir(), "no-such-catalog")

	err := buildErr(t, mosaicRoot, "--catalog-folder", missing, "deploy")

	if !errors.Is(err, stackerr.ErrLayerLoad) {
		t.Errorf("error = %v, want stackerr.ErrLayerLoad", err)
	}
	assertNamesLayer(t, err, absOf(t, missing))
}

func TestBuildCatalogStack_DuplicateCatalogID_NamesBothLayers(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	a := newStackUserCatalog(t, "same-id", "2.0.0")
	b := newStackUserCatalog(t, "same-id", "3.0.0")

	err := buildErr(t, mosaicRoot, "--catalog-folder", a, "--catalog-folder", b, "deploy")

	if !errors.Is(err, stackerr.ErrDuplicateCatalogID) {
		t.Errorf("error = %v, want stackerr.ErrDuplicateCatalogID", err)
	}
	assertNamesLayer(t, err, "same-id", absOf(t, a), absOf(t, b))
}

func TestBuildCatalogStack_IncompleteOrchestratorPair_NamesTheLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	half := newIdentifiedCatalogRoot(t, "half-pair")
	writeStackOrchestratorFile(t, half, "orchestrator", "2.0.0")

	err := buildErr(t, mosaicRoot, "--catalog-folder", half, "deploy")

	if !errors.Is(err, stackerr.ErrOrchestratorPairIncomplete) {
		t.Errorf("error = %v, want stackerr.ErrOrchestratorPairIncomplete", err)
	}
	assertNamesLayer(t, err, absOf(t, half))
}

func TestBuildCatalogStack_NumericIDCollision_NamesBothCatalogs(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	a := newIdentifiedCatalogRoot(t, "owner-a")
	b := newIdentifiedCatalogRoot(t, "owner-b")
	writeStackAgent(t, a, "agent-one", "clash-001", "1.0.0")
	writeStackAgent(t, b, "agent-two", "clash-001", "1.0.0")

	err := buildErr(t, mosaicRoot, "--catalog-folder", a, "--catalog-folder", b, "deploy")

	if !errors.Is(err, stackerr.ErrNumericIDCollision) {
		t.Errorf("error = %v, want stackerr.ErrNumericIDCollision", err)
	}
	assertNamesLayer(t, err, "owner-a", "owner-b")
}

func TestBuildCatalogStack_FailingUserLayerWithDefaultFlag_StillFails(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	bare := t.TempDir()

	err := buildErr(t, mosaicRoot, "--include-default-catalog", "--catalog-folder", bare)

	assertNamesLayer(t, err, absOf(t, bare))
}

func TestBuildCatalogStack_DefaultFlagFalseWithoutUserCatalog_IsAnEmptyStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)

	err := buildErr(t, mosaicRoot, "deploy", "--include-default-catalog=false")

	if !errors.Is(err, stackerr.ErrEmptyStack) {
		t.Errorf("error = %v, want stackerr.ErrEmptyStack", err)
	}
}

func TestBuildCatalogStack_UnreadableDefaultCatalog_NamesTheDefaultLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	if err := os.Remove(filepath.Join(mosaicRoot, "Catalog", "catalog-identity.yaml")); err != nil {
		t.Fatalf("remove default identity: %v", err)
	}

	err := buildErr(t, mosaicRoot, "deploy")

	if !errors.Is(err, stackerr.ErrLayerIdentity) {
		t.Errorf("error = %v, want stackerr.ErrLayerIdentity for the default layer", err)
	}
	assertNamesLayer(t, err, filepath.Join(mosaicRoot, "Catalog"))
}
