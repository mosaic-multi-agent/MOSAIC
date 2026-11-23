package app_test

// promote_id_floor_test.go covers the numeric id a single-file promote gives a new agent: into
// any layer other than the default catalog the larger of 1000 and "largest id in the stack + 1",
// into the default catalog "largest id in the stack + 1", and a replaced agent keeps its id.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/stack"
)

// depsWithDefault installs the stack of the user layers of ls over the MOSAIC default catalog
// (marked Default) as the run's catalog.
func (ls *plLayerSet) depsWithDefault(t *testing.T) app.Deps {
	t.Helper()
	deps := ls.deps(t, newPlInteraction(""))
	layers := []stack.Layer{}
	for _, id := range ls.order {
		layers = append(layers, stack.Layer{Root: ls.roots[id]})
	}
	layers = append(layers, defaultStackLayer(ls.mosaicRoot))
	st, err := stack.New(ls.mosaicRoot, layers)
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	deps.Catalog = st
	return deps
}

// TestPromote_NewAgent_NumericIDFloorDependsOnTargetLayer verifies the id of a new agent for
// every combination of target layer and highest id of the stack.
func TestPromote_NewAgent_NumericIDFloorDependsOnTargetLayer(t *testing.T) {
	cases := []struct {
		name      string
		target    string
		defaultID string // id of the agent in the default catalog
		userID    string // id of the agent in the user layer; empty = none
		want      string
	}{
		{"user layer over default with 67", "top", "67", "", "1000"},
		{"user layer, highest id 999", "top", "999", "", "1000"},
		{"user layer, highest id 1000", "top", "67", "1000", "1001"},
		{"user layer, stack holds 1004", "top", "67", "1004", "1005"},
		{"default layer with 67", "plmosaicdefault", "67", "", "68"},
		{"default layer, highest id 1004 in a user layer", "plmosaicdefault", "67", "1004", "1005"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls := newPlLayerSet(t, "top")
			ls.seedDefaultAgent(t, "default-agent", tc.defaultID)
			if tc.userID != "" {
				ls.addAgent(t, "top", "Audit", "user-agent", tc.userID)
			}
			svc := app.New(ls.depsWithDefault(t))
			src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")

			// Act
			result, err := svc.Promote(context.Background(), app.PromoteRequest{
				FilePath: src, Category: "Audit", HarnessID: "stub-harness", TargetCatalogID: tc.target,
			})

			// Assert
			if err != nil {
				t.Fatalf("Promote: %v", err)
			}
			if result.NumericID != tc.want {
				t.Errorf("result NumericID = %q, want %q", result.NumericID, tc.want)
			}
			if id, _ := parseFrontmatterFieldSvc(t, result.DestinationPath, "id"); id != tc.want {
				t.Errorf("written id = %q, want %q", id, tc.want)
			}
		})
	}
}

// seedDefaultAgent registers the default catalog of ls and gives it a worker agent that carries
// numericID.
func (ls *plLayerSet) seedDefaultAgent(t *testing.T, key, numericID string) {
	t.Helper()
	ls.registerDefault(t)
	ls.addAgent(t, "plmosaicdefault", "Audit", key, numericID)
}

// TestPromote_Replace_InUserLayerOverDefault_KeepsLowIDWithoutFloor verifies replacing an agent
// whose id is below 1000 keeps that id in a user layer: the floor applies to new agents only.
func TestPromote_Replace_InUserLayerOverDefault_KeepsLowIDWithoutFloor(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top")
	ls.seedDefaultAgent(t, "default-agent", "67")
	ls.addAgent(t, "top", "Audit", plAgentKey, "12")
	svc := app.New(ls.depsWithDefault(t))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")

	// Act
	result, err := svc.Promote(context.Background(), replaceRequest(src, "Audit", true))

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if result.NumericID != "12" {
		t.Errorf("NumericID = %q, want the replaced agent's 12", result.NumericID)
	}
	if id, _ := parseFrontmatterFieldSvc(t, result.DestinationPath, "id"); id != "12" {
		t.Errorf("written id = %q, want the replaced agent's 12", id)
	}
}

// TestPromote_Replace_InDefaultLayer_KeepsItsID verifies replacing an agent of the default
// catalog keeps its id.
func TestPromote_Replace_InDefaultLayer_KeepsItsID(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top")
	ls.seedDefaultAgent(t, "default-agent", "67")
	ls.addAgent(t, "plmosaicdefault", "Audit", plAgentKey, "11")
	svc := app.New(ls.depsWithDefault(t))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	req := replaceRequest(src, "Audit", true)
	req.TargetCatalogID = "plmosaicdefault"

	// Act
	result, err := svc.Promote(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if result.NumericID != "11" {
		t.Errorf("NumericID = %q, want the replaced agent's 11", result.NumericID)
	}
}
