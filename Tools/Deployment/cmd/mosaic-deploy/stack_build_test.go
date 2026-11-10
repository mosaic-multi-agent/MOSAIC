package main

// stack_build_test.go verifies buildCatalogStack: which layers a pre-scanned stack input
// yields, in which order, and that a user catalog never receives default-catalog content.

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
)

// stackFor scans args and builds the stack against mosaicRoot, failing the test on error.
func stackFor(t *testing.T, mosaicRoot string, args ...string) stack.Catalog {
	t.Helper()
	cat, err := buildCatalogStack(mosaicRoot, scanStackInput(args))
	if err != nil {
		t.Fatalf("buildCatalogStack for %v: %v", args, err)
	}
	return cat
}

// layerRoots returns the roots of the stack's layers, highest first.
func layerRoots(cat stack.Catalog) []string {
	var roots []string
	for _, l := range cat.Layers() {
		roots = append(roots, l.Root)
	}
	return roots
}

func TestBuildCatalogStack_NoStackInput_IsTheDefaultCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)

	cat := stackFor(t, mosaicRoot, "deploy", "--harness", "h")

	layers := cat.Layers()
	if len(layers) != 1 || !layers[0].Default {
		t.Fatalf("Layers() = %+v, want exactly the default layer", layers)
	}
	if want := filepath.Join(mosaicRoot, "Catalog"); filepath.Clean(layers[0].Root) != filepath.Clean(want) {
		t.Errorf("default layer root = %q, want %q", layers[0].Root, want)
	}
	if _, ok := cat.Agent("default-only-agent"); !ok {
		t.Error("default catalog agent missing from the default-only stack")
	}
}

func TestBuildCatalogStack_DefaultFlagAlone_EqualsTheNoStackInputStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)

	none := stackFor(t, mosaicRoot, "deploy")
	flagOnly := stackFor(t, mosaicRoot, "deploy", "--include-default-catalog")

	if !reflect.DeepEqual(none.Layers(), flagOnly.Layers()) {
		t.Errorf("--include-default-catalog alone gave layers %+v, want %+v", flagOnly.Layers(), none.Layers())
	}
}

func TestBuildCatalogStack_UserCatalogWithoutDefaultFlag_DeploysNothingFromTheDefaultCatalog(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	writeStackSkill(t, user, "user-skill")

	cat := stackFor(t, mosaicRoot, "--catalog-folder", user, "deploy")

	if layers := cat.Layers(); len(layers) != 1 || layers[0].Default {
		t.Fatalf("Layers() = %+v, want the user layer alone", layers)
	}
	if _, ok := cat.Agent("user-a-agent"); !ok {
		t.Error("user catalog agent missing")
	}
	if _, ok := cat.Agent("default-only-agent"); ok {
		t.Error("default-only-agent resolved although the default layer was left out")
	}
	if _, ok := cat.Workflow("default-wf"); ok {
		t.Error("default-wf resolved although the default layer was left out")
	}
	if _, ok := cat.Skill("default-skill"); ok {
		t.Error("default-skill merged into a user-only stack")
	}
	if _, ok := cat.Hook("default-hook"); ok {
		t.Error("default-hook merged into a user-only stack")
	}
	if _, ok := cat.Skill("user-skill"); !ok {
		t.Error("the user catalog's own skill is missing")
	}
	if got := cat.Orchestrator().Version; got != "2.0.0" {
		t.Errorf("orchestrator version = %q, want 2.0.0 (the user catalog's pair)", got)
	}
}

func TestBuildCatalogStack_UserCatalogWithDefaultFlag_PutsTheDefaultLayerAtTheBottom(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")

	cat := stackFor(t, mosaicRoot, "--include-default-catalog", "--catalog-folder", user, "deploy")

	layers := cat.Layers()
	if len(layers) != 2 {
		t.Fatalf("Layers() has %d entries, want 2: %+v", len(layers), layers)
	}
	if layers[0].Default || !layers[1].Default {
		t.Errorf("layers = %+v, want the user layer first and the default layer last", layers)
	}
	if _, ok := cat.Agent("default-only-agent"); !ok {
		t.Error("default-only-agent missing although the default layer was included")
	}
	if _, ok := cat.Hook("default-hook"); !ok {
		t.Error("default-hook missing although the default layer was included")
	}
	if shared, _ := cat.Agent("shared-agent"); shared.Version != "2.0.0" {
		t.Errorf("shared-agent version = %q, want 2.0.0 (user layer shadows the default layer)", shared.Version)
	}
}

func TestBuildCatalogStack_DefaultFlagPositionDoesNotMoveTheDefaultLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")

	before := stackFor(t, mosaicRoot, "--include-default-catalog", "--catalog-folder", user)
	after := stackFor(t, mosaicRoot, "--catalog-folder", user, "--include-default-catalog")

	if !reflect.DeepEqual(layerRoots(before), layerRoots(after)) {
		t.Errorf("layer order depends on flag position: %v vs %v", layerRoots(before), layerRoots(after))
	}
	if layers := after.Layers(); !layers[len(layers)-1].Default {
		t.Errorf("default layer is not the bottom layer: %+v", layers)
	}
}

func TestBuildCatalogStack_SeveralUserLayers_FirstIsHighest(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	high := newStackUserCatalog(t, "high", "3.0.0")
	low := newStackUserCatalog(t, "low", "2.0.0")

	cat := stackFor(t, mosaicRoot, "--catalog-folder", high, "--catalog-folder="+low, "--include-default-catalog")

	wantRoots := []string{absOf(t, high), absOf(t, low), filepath.Join(mosaicRoot, "Catalog")}
	var gotRoots []string
	for _, r := range layerRoots(cat) {
		gotRoots = append(gotRoots, filepath.Clean(r))
	}
	for i := range wantRoots {
		wantRoots[i] = filepath.Clean(wantRoots[i])
	}
	if !reflect.DeepEqual(gotRoots, wantRoots) {
		t.Fatalf("layer roots = %v, want %v", gotRoots, wantRoots)
	}
	if shared, _ := cat.Agent("shared-agent"); shared.Version != "3.0.0" {
		t.Errorf("shared-agent version = %q, want 3.0.0 (first --catalog-folder is the highest layer)", shared.Version)
	}
	if got := cat.Orchestrator().Version; got != "3.0.0" {
		t.Errorf("orchestrator version = %q, want 3.0.0 from the highest layer", got)
	}
	if _, ok := cat.Agent("low-agent"); !ok {
		t.Error("the lower user layer's own agent is missing")
	}
}

func TestBuildCatalogStack_SwappingTheUserLayerOrder_SwapsTheWinner(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	a := newStackUserCatalog(t, "a", "3.0.0")
	b := newStackUserCatalog(t, "b", "2.0.0")

	cat := stackFor(t, mosaicRoot, "--catalog-folder", b, "--catalog-folder", a)

	if shared, _ := cat.Agent("shared-agent"); shared.Version != "2.0.0" {
		t.Errorf("shared-agent version = %q, want 2.0.0 from the catalog named first", shared.Version)
	}
}
