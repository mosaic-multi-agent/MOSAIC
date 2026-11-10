package stack_test

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// writeTestIdentityVersion writes catalog-identity.yaml with the given id and version.
func writeTestIdentityVersion(t *testing.T, root, id, version string) {
	t.Helper()
	mustWrite(t, root, "catalog-identity.yaml", "id: "+id+"\nname: Catalog "+id+"\nversion: \""+version+"\"\n")
}

func composeRoots(mosaic string, roots ...string) error {
	layers := make([]stack.Layer, len(roots))
	for i, r := range roots {
		layers[i] = stack.Layer{Root: r}
	}
	_, err := stack.New(mosaic, layers)
	return err
}

func TestNew_SameFolderTwice_FailsNamingIdAndBothLayers(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "shared-id")

	err := composeRoots(mosaic, root, root)

	var dup *stackerr.DuplicateCatalogIDError
	if !errors.As(err, &dup) {
		t.Fatalf("New error = %v, want *stackerr.DuplicateCatalogIDError", err)
	}
	if !errors.Is(err, stackerr.ErrDuplicateCatalogID) {
		t.Error("errors.Is(err, ErrDuplicateCatalogID) = false")
	}
	if dup.CatalogID != "shared-id" {
		t.Errorf("CatalogID = %q, want shared-id", dup.CatalogID)
	}
	if want := []string{root, root}; !reflect.DeepEqual(dup.Roots, want) {
		t.Errorf("Roots = %v, want %v", dup.Roots, want)
	}
	assertContains(t, err.Error(), "shared-id")
	assertContains(t, err.Error(), root)
}

func TestNew_TwoFoldersSameIdDifferentVersions_FailsNamingBothRoots(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := t.TempDir(), t.TempDir()
	writeTestIdentityVersion(t, high, "twin", "2.0.0")
	writeTestIdentityVersion(t, low, "twin", "1.0.0")

	err := composeRoots(mosaic, high, low)

	var dup *stackerr.DuplicateCatalogIDError
	if !errors.As(err, &dup) {
		t.Fatalf("New error = %v, want *stackerr.DuplicateCatalogIDError", err)
	}
	if want := []string{high, low}; !reflect.DeepEqual(dup.Roots, want) {
		t.Errorf("Roots = %v, want %v (highest first)", dup.Roots, want)
	}
	assertContains(t, err.Error(), high)
	assertContains(t, err.Error(), low)
}

func TestNew_DuplicateNotAdjacent_StillRejected(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	a, mid, c := t.TempDir(), newTestCatalogRoot(t, "mid"), t.TempDir()
	writeTestIdentityVersion(t, a, "twin", "1.0.0")
	writeTestIdentityVersion(t, c, "twin", "2.0.0")

	err := composeRoots(mosaic, a, mid, c)

	if !errors.Is(err, stackerr.ErrDuplicateCatalogID) {
		t.Fatalf("New error = %v, want ErrDuplicateCatalogID", err)
	}
}

func TestNew_InterleavedDuplicates_ReportsIdWithHighestFirstOccurrence(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	x1, y1, y2, x2 := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	writeTestIdentityVersion(t, x1, "x", "1.0.0")
	writeTestIdentityVersion(t, y1, "y", "1.0.0")
	writeTestIdentityVersion(t, y2, "y", "2.0.0")
	writeTestIdentityVersion(t, x2, "x", "2.0.0")

	err := composeRoots(mosaic, x1, y1, y2, x2)

	var dup *stackerr.DuplicateCatalogIDError
	if !errors.As(err, &dup) {
		t.Fatalf("New error = %v, want *stackerr.DuplicateCatalogIDError", err)
	}
	if dup.CatalogID != "x" {
		t.Errorf("CatalogID = %q, want x (first occurrence is highest)", dup.CatalogID)
	}
	if want := []string{x1, x2}; !reflect.DeepEqual(dup.Roots, want) {
		t.Errorf("Roots = %v, want %v", dup.Roots, want)
	}
}

func TestNew_ThreeLayersSameId_ListsEveryRootHighestFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
	writeTestIdentityVersion(t, a, "tri", "3.0.0")
	writeTestIdentityVersion(t, b, "tri", "2.0.0")
	writeTestIdentityVersion(t, c, "tri", "1.0.0")

	err := composeRoots(mosaic, a, b, c)

	var dup *stackerr.DuplicateCatalogIDError
	if !errors.As(err, &dup) {
		t.Fatalf("New error = %v, want *stackerr.DuplicateCatalogIDError", err)
	}
	if want := []string{a, b, c}; !reflect.DeepEqual(dup.Roots, want) {
		t.Errorf("Roots = %v, want %v", dup.Roots, want)
	}
}

func TestNew_DistinctIds_Accepted(t *testing.T) {
	mosaic := newTestMosaicRoot(t)

	err := composeRoots(mosaic, newTestCatalogRoot(t, "one"), newTestCatalogRoot(t, "two"))

	if err != nil {
		t.Fatalf("New with distinct ids: %v", err)
	}
}

func TestNew_DuplicateIdAndIncompletePair_ReportsDuplicateFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := t.TempDir(), t.TempDir()
	writeTestIdentityVersion(t, high, "twin", "2.0.0")
	writeTestIdentityVersion(t, low, "twin", "1.0.0")
	addTestOrchestrator(t, high, "1.0.0")

	err := composeRoots(mosaic, high, low)

	if !errors.Is(err, stackerr.ErrDuplicateCatalogID) {
		t.Fatalf("New error = %v, want ErrDuplicateCatalogID before the pair check", err)
	}
}
