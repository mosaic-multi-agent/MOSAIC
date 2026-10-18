package app_test

// promote_layer_category_id_test.go covers two consequences of the target layer: the
// category options come from the target layer's Subagents/ directory only, and the numeric
// id of a new agent is unique across the whole resolved stack.

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// TestPromote_CategoryOptions_ComeFromTargetLayer verifies that the categories offered are
// those of the chosen layer (plus the utility option), not those of another layer.
func TestPromote_CategoryOptions_ComeFromTargetLayer(t *testing.T) {
	cases := []struct {
		target string
		want   []string
	}{
		{"low", []string{"LowOnly", app.PromoteCategoryUtility}},
		{"mid", []string{"MidOnly", app.PromoteCategoryUtility}}, // neither highest nor lowest
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			// Arrange
			ls := newPlLayerSet(t, "top", "mid", "low")
			ls.addAgent(t, "top", "TopOnly", "top-agent", "11")
			ls.addAgent(t, "mid", "MidOnly", "mid-agent", "13")
			ls.addAgent(t, "low", "LowOnly", "low-agent", "12")
			src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
			ix := newPlInteraction("")
			ix.Stub = interactiontest.NewBuilder().
				AnswerSelectOne(domain.QPromoteCategory, src, tc.want[0]).Build()
			svc := app.New(ls.deps(t, ix))

			// Act
			_, err := svc.Promote(context.Background(), app.PromoteRequest{
				FilePath: src, HarnessID: "stub-harness", TargetCatalogID: tc.target,
			})

			// Assert
			if err != nil {
				t.Fatalf("Promote: %v", err)
			}
			got := optionIDList(ix.categoryOptions)
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("category options = %v, want %v", got, want)
			}
		})
	}
}

// TestPromote_CategoryOptions_ResolvedItem_ComeFromResolvedLayer verifies that for an item
// whose key resolves from a lower layer, the interactive category options are that layer's.
func TestPromote_CategoryOptions_ResolvedItem_ComeFromResolvedLayer(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "top", "TopOnly", "top-agent", "11")
	ls.addAgent(t, "low", "Audit", plAgentKey, "12")
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	ix := newPlInteraction("")
	ix.Stub = interactiontest.NewBuilder().
		AnswerSelectOne(domain.QPromoteCategory, src, "Audit").Build()
	svc := app.New(ls.deps(t, ix))

	// Act
	_, err := svc.Promote(context.Background(), app.PromoteRequest{
		FilePath: src, HarnessID: "stub-harness", Overwrite: true,
	})

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	got := optionIDList(ix.categoryOptions)
	sort.Strings(got)
	want := []string{"Audit", app.PromoteCategoryUtility}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("category options = %v, want %v (resolved layer's categories)", got, want)
	}
}

// TestPromote_NewAgentNumericID_UniqueAcrossStack verifies that the id assigned to a new
// agent differs from every numeric id in the resolved stack, whichever layer receives it,
// and that the stack still composes afterwards (its own numeric-id check passes).
func TestPromote_NewAgentNumericID_UniqueAcrossStack(t *testing.T) {
	cases := []struct {
		target string
		topIDs []string // numeric ids of agents in the top layer
		lowIDs []string // numeric ids of agents in the low layer
	}{
		{target: "low", topIDs: []string{"3"}, lowIDs: []string{"1", "2"}},
		{target: "top", topIDs: []string{"1"}, lowIDs: []string{"2", "3"}},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			// Arrange
			ls := newPlLayerSet(t, "top", "low")
			used := map[string]bool{}
			for i, id := range tc.topIDs {
				ls.addAgent(t, "top", "Audit", "top-agent-"+string(rune('a'+i)), id)
				used[id] = true
			}
			for i, id := range tc.lowIDs {
				ls.addAgent(t, "low", "Audit", "low-agent-"+string(rune('a'+i)), id)
				used[id] = true
			}
			src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
			svc := app.New(ls.deps(t, newPlInteraction("")))

			// Act
			result, err := svc.Promote(context.Background(), app.PromoteRequest{
				FilePath: src, Category: "Audit", HarnessID: "stub-harness",
				TargetCatalogID: tc.target,
			})

			// Assert
			if err != nil {
				t.Fatalf("Promote: %v", err)
			}
			if used[result.NumericID] {
				t.Errorf("new agent got numeric id %q, already used in the stack %v", result.NumericID, used)
			}
			layers := []stack.Layer{{Root: ls.roots["top"]}, {Root: ls.roots["low"]}}
			if _, stErr := stack.New(ls.mosaicRoot, layers); stErr != nil {
				t.Errorf("stack no longer composes after the promote: %v", stErr)
			}
		})
	}
}
