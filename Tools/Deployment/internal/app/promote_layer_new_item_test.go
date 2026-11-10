package app_test

// promote_layer_new_item_test.go covers promote of a NEW item (no resolved source): the
// user chooses the receiving layer every time, interactively through QPromoteTargetCatalog
// or through the request field; a missing or unknown choice is an error that writes nothing.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

func plNewItemRequest(src string) app.PromoteRequest {
	return app.PromoteRequest{FilePath: src, Category: "Audit", HarnessID: "stub-harness"}
}

// TestPromote_NewItem_Interactive_AsksAndWritesOnlyChosenLayer verifies the question is
// asked with every active layer as an option and that only the chosen layer changes.
func TestPromote_NewItem_Interactive_AsksAndWritesOnlyChosenLayer(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "mid", "low")
	ix := newPlInteraction("mid")
	svc := app.New(ls.deps(t, ix))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	result, err := svc.Promote(context.Background(), plNewItemRequest(src))

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if ix.layerAsked != 1 {
		t.Fatalf("layer question asked %d times, want 1", ix.layerAsked)
	}
	var ids []string
	for _, o := range ix.layerOptions {
		ids = append(ids, o.ID)
	}
	if !reflect.DeepEqual(ids, []string{"top", "mid", "low"}) {
		t.Errorf("layer options = %v, want [top mid low] (highest first)", ids)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"mid"}) {
		t.Errorf("changed layers = %v, want only [mid]", got)
	}
	want := filepath.Join(ls.roots["mid"], "Subagents", "Audit", plAgentKey+".md")
	if result.DestinationPath != want || result.TargetCatalogID != "mid" {
		t.Errorf("result = (%q, %q), want (%q, mid)", result.DestinationPath, result.TargetCatalogID, want)
	}
}

// TestPromote_NewItem_LayerOptionsCarryIdentityLabelAndRoot verifies the option shape.
func TestPromote_NewItem_LayerOptionsCarryIdentityLabelAndRoot(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ix := newPlInteraction("top")
	svc := app.New(ls.deps(t, ix))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")

	// Act
	if _, err := svc.Promote(context.Background(), plNewItemRequest(src)); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	// Assert
	if len(ix.layerOptions) != 2 {
		t.Fatalf("got %d layer options, want 2", len(ix.layerOptions))
	}
	o := ix.layerOptions[1]
	if o.Label != "Fixture low (low 1.0.0)" {
		t.Errorf("Label = %q, want %q", o.Label, "Fixture low (low 1.0.0)")
	}
	if o.Description != ls.roots["low"] {
		t.Errorf("Description = %q, want the layer root %q", o.Description, ls.roots["low"])
	}
}

// TestPromote_NewItem_TargetCatalogID_SelectsLayerWithoutAsking verifies the request field
// chooses the layer non-interactively and the question is not asked.
func TestPromote_NewItem_TargetCatalogID_SelectsLayerWithoutAsking(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ix := newPlInteraction("")
	svc := app.New(ls.deps(t, ix))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)
	req := plNewItemRequest(src)
	req.TargetCatalogID = "low"

	// Act
	result, err := svc.Promote(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if ix.layerAsked != 0 {
		t.Errorf("layer question asked %d times despite TargetCatalogID, want 0", ix.layerAsked)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"low"}) {
		t.Errorf("changed layers = %v, want only [low]", got)
	}
	if result.TargetCatalogID != "low" {
		t.Errorf("TargetCatalogID = %q, want low", result.TargetCatalogID)
	}
}

// TestPromote_NewItem_NoChoice_ErrorsAndWritesNothing verifies a skipped layer question (the
// non-interactive case) in a multi-layer stack is an error and nothing is written.
func TestPromote_NewItem_NoChoice_ErrorsAndWritesNothing(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	_, err := svc.Promote(context.Background(), plNewItemRequest(src))

	// Assert
	if err == nil {
		t.Fatal("Promote succeeded without a target layer choice")
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the error: %v", got)
	}
}

// TestPromote_NewItem_CancelledLayerQuestion_ErrorsAndWritesNothing verifies a cancelled
// layer question is an error like a skipped one and nothing is written.
func TestPromote_NewItem_CancelledLayerQuestion_ErrorsAndWritesNothing(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ix := newPlInteraction("")
	ix.layerCancelled = true
	svc := app.New(ls.deps(t, ix))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	_, err := svc.Promote(context.Background(), plNewItemRequest(src))

	// Assert
	if err == nil {
		t.Fatal("Promote succeeded after the layer question was cancelled")
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the error: %v", got)
	}
}

// TestPromote_NewItem_UnknownTargetCatalogID_Rejected verifies an id naming no active layer
// is rejected and nothing is written.
func TestPromote_NewItem_UnknownTargetCatalogID_Rejected(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)
	req := plNewItemRequest(src)
	req.TargetCatalogID = "nope"

	// Act
	_, err := svc.Promote(context.Background(), req)

	// Assert
	if err == nil {
		t.Fatal("Promote accepted a layer id naming no active layer")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the rejected value", err)
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the error: %v", got)
	}
}

// TestPromote_NewItem_AnswerNamingUnknownLayer_Rejected verifies the interactive answer is
// checked against the active layers like the request field.
func TestPromote_NewItem_AnswerNamingUnknownLayer_Rejected(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	svc := app.New(ls.deps(t, newPlInteraction("ghost")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	_, err := svc.Promote(context.Background(), plNewItemRequest(src))

	// Assert
	if err == nil {
		t.Fatal("Promote accepted an answer naming no active layer")
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the error: %v", got)
	}
}

// TestPromote_NewItem_OneLayerStack_NeedsNoChoice verifies a one-layer stack writes to that
// layer without asking and without a flag.
func TestPromote_NewItem_OneLayerStack_NeedsNoChoice(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "only")
	ix := newPlInteraction("")
	svc := app.New(ls.deps(t, ix))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")

	// Act
	result, err := svc.Promote(context.Background(), plNewItemRequest(src))

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if ix.layerAsked != 0 {
		t.Errorf("layer question asked %d times for a one-layer stack, want 0", ix.layerAsked)
	}
	want := filepath.Join(ls.roots["only"], "Subagents", "Audit", plAgentKey+".md")
	if result.DestinationPath != want || result.TargetCatalogID != "only" {
		t.Errorf("result = (%q, %q), want (%q, only)", result.DestinationPath, result.TargetCatalogID, want)
	}
}

// TestPromote_NewItem_OneLayerStack_OtherIDRejected verifies that in a one-layer stack a
// layer id other than that layer's is rejected.
func TestPromote_NewItem_OneLayerStack_OtherIDRejected(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "only")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)
	req := plNewItemRequest(src)
	req.TargetCatalogID = "other"

	// Act
	_, err := svc.Promote(context.Background(), req)

	// Assert
	if err == nil {
		t.Fatal("Promote accepted a layer id other than the single layer's")
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed despite the error: %v", got)
	}
}

// TestPromote_PlainCatalog_NonEmptyTargetCatalogID_Rejected verifies a plain catalog has no
// layer ids, so any given value is rejected and nothing is written.
func TestPromote_PlainCatalog_NonEmptyTargetCatalogID_Rejected(t *testing.T) {
	// Arrange
	mosaicRoot := writeMosaicRoot(t)
	deps, _ := newPromoteDeps(t, newPlInteraction("").Stub, mosaicRoot)
	svc := app.New(deps)
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	req := plNewItemRequest(src)
	req.TargetCatalogID = "anything"

	// Act
	_, err := svc.Promote(context.Background(), req)

	// Assert
	if err == nil {
		t.Fatal("Promote accepted a layer id for a plain catalog")
	}
	dest := filepath.Join(mosaicRoot, "Catalog", "Subagents", "Audit", plAgentKey+".md")
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Errorf("file written at %q despite the error", dest)
	}
}
