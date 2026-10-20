package app_test

// promote_layer_resolved_test.go covers promote of a file whose agent key the catalog stack
// already resolved: the write goes to the catalog the item was resolved from and to no other
// layer, and no layer question is asked.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/stack"
)

const plAgentKey = "my-harness-agent" // key derived from the eligible source file name

// TestPromote_ResolvedFromLowerLayer_WritesOnlyThatLayer verifies that an agent key resolved
// from a lower layer is overwritten in that layer, leaving every other layer untouched.
func TestPromote_ResolvedFromLowerLayer_WritesOnlyThatLayer(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "mid", "low")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	ix := newPlInteraction("")
	svc := app.New(ls.deps(t, ix))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	result, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "Audit", HarnessID: "stub-harness", Overwrite: true,
	})

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"low"}) {
		t.Errorf("changed layers = %v, want only [low]", got)
	}
	want := filepath.Join(ls.roots["low"], "Subagents", "Audit", plAgentKey+".md")
	if result.DestinationPath != want {
		t.Errorf("DestinationPath = %q, want %q", result.DestinationPath, want)
	}
	if result.TargetCatalogID != "low" {
		t.Errorf("TargetCatalogID = %q, want low", result.TargetCatalogID)
	}
	if ix.layerAsked != 0 {
		t.Errorf("layer question asked %d times for an item with a resolved source, want 0", ix.layerAsked)
	}
}

// TestPromote_ResolvedFromTopLayer_WritesOnlyTopLayer verifies that when the top layer wins
// the key (a lower layer holds a shadowed copy), only the top layer changes. The write
// isolation is regression coverage (the highest layer is also the default); the
// TargetCatalogID echo is the assertion that requires the new behavior.
func TestPromote_ResolvedFromTopLayer_WritesOnlyTopLayer(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "top", "Audit", plAgentKey, "301")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	result, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "Audit", HarnessID: "stub-harness", Overwrite: true,
	})

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"top"}) {
		t.Errorf("changed layers = %v, want only [top]", got)
	}
	if result.TargetCatalogID != "top" {
		t.Errorf("TargetCatalogID = %q, want top", result.TargetCatalogID)
	}
}

// TestPromote_ResolvedItem_IgnoresValidTargetCatalogID verifies that a valid layer id given
// for an item with a resolved source does not redirect the write.
func TestPromote_ResolvedItem_IgnoresValidTargetCatalogID(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	_, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "Audit", HarnessID: "stub-harness", Overwrite: true,
		TargetCatalogID: "top",
	})

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"low"}) {
		t.Errorf("changed layers = %v, want only [low] (resolved source wins over the flag)", got)
	}
}

// TestPromote_ResolvedItem_UnknownTargetCatalogID_RejectedWritesNothing verifies that a
// layer id naming no active layer is an error even for an item with a resolved source.
func TestPromote_ResolvedItem_UnknownTargetCatalogID_RejectedWritesNothing(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	_, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "Audit", HarnessID: "stub-harness", Overwrite: true,
		TargetCatalogID: "nope",
	})

	// Assert
	if err == nil {
		t.Fatal("Promote succeeded with a layer id naming no active layer")
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the error: %v", got)
	}
}

// TestPromote_ResolvedItem_ExistingDestinationStillRefusedWithoutOverwrite verifies the
// collision policy is unchanged: the resolved layer's existing file is not replaced.
func TestPromote_ResolvedItem_ExistingDestinationStillRefusedWithoutOverwrite(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	_, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "Audit", HarnessID: "stub-harness",
	})

	// Assert
	if err == nil {
		t.Fatal("Promote replaced an existing destination without Overwrite")
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the refusal: %v", got)
	}
}

// TestPromote_DefaultLayerIsValidTargetForNewItem verifies that the default catalog layer
// takes part in the stack like any other: it can be chosen as the target of a new item.
func TestPromote_DefaultLayerIsValidTargetForNewItem(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top")
	deps := ls.deps(t, newPlInteraction(""))
	defRoot := filepath.Join(ls.mosaicRoot, "Catalog")
	st, stErr := stack.New(ls.mosaicRoot, []stack.Layer{{Root: ls.roots["top"]}, defaultStackLayer(ls.mosaicRoot)})
	if stErr != nil {
		t.Fatalf("stack.New: %v", stErr)
	}
	deps.Catalog = st
	svc := app.New(deps)
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")

	// Act
	result, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "Audit", HarnessID: "stub-harness",
		TargetCatalogID: "plmosaicdefault",
	})

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	want := filepath.Join(defRoot, "Subagents", "Audit", plAgentKey+".md")
	if _, statErr := os.Stat(want); statErr != nil {
		t.Errorf("expected the file at %q: %v", want, statErr)
	}
	if result.TargetCatalogID != "plmosaicdefault" {
		t.Errorf("TargetCatalogID = %q, want plmosaicdefault", result.TargetCatalogID)
	}
	if strings.Contains(filepath.ToSlash(result.DestinationPath), ls.roots["top"]) {
		t.Errorf("DestinationPath %q points into the top layer", result.DestinationPath)
	}
}
