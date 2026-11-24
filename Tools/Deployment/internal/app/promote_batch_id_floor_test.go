package app_test

// promote_batch_id_floor_test.go covers the numeric ids PromoteBatch gives new agents: the first
// new agent gets the larger of 1000 and "largest id in the stack + 1" in any layer other than the
// default catalog, "largest id in the stack + 1" in the default catalog, later new agents follow
// on consecutively, and replaced agents keep their ids without shifting the new ones.

import (
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// idFloorBatch is a stack of the user layer "top" (largest id batchSeedID) over the default
// catalog, and three new agents to promote into the layer named target.
func idFloorBatch(t *testing.T, target string, extraTopID string) (app.PromoteBatchResult, *plLayerSet) {
	t.Helper()
	ls := newBatchLayers(t, "top")
	if extraTopID != "" {
		ls.addAgent(t, "top", "Audit", "big-agent", extraTopID)
	}
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "c-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen(target)
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return mustRunBatch(t, ls.withDefaultLayer(t, spy), dir), ls
}

func assertBatchIDs(t *testing.T, res app.PromoteBatchResult, want ...string) {
	t.Helper()
	for i, f := range []string{"a-agent.md", "b-agent.md", "c-agent.md"}[:len(want)] {
		if got := requirePromoted(t, res, f).Result.NumericID; got != want[i] {
			t.Errorf("%s id = %q, want %q", f, got, want[i])
		}
	}
}

// TestPromoteBatch_NewAgents_IntoUserLayer_StartAtFloorAndFollowOn verifies three new agents
// into a user layer on a stack with no id at or above 1000 get 1000, 1001, 1002.
func TestPromoteBatch_NewAgents_IntoUserLayer_StartAtFloorAndFollowOn(t *testing.T) {
	// Act
	res, _ := idFloorBatch(t, "top", "")

	// Assert
	assertBatchIDs(t, res, "1000", "1001", "1002")
}

// TestPromoteBatch_NewAgents_IntoUserLayer_StackHoldsIDAboveFloor_FollowOnFromLargest verifies
// the first id is the stack's largest + 1 when the stack already holds 1004.
func TestPromoteBatch_NewAgents_IntoUserLayer_StackHoldsIDAboveFloor_FollowOnFromLargest(t *testing.T) {
	// Act
	res, _ := idFloorBatch(t, "top", "1004")

	// Assert
	assertBatchIDs(t, res, "1005", "1006", "1007")
}

// TestPromoteBatch_NewAgents_IntoDefaultLayer_FollowOnFromLargest verifies a batch into the
// default catalog starts at the stack's largest id + 1, without the floor.
func TestPromoteBatch_NewAgents_IntoDefaultLayer_FollowOnFromLargest(t *testing.T) {
	// Act
	res, _ := idFloorBatch(t, "plmosaicdefault", "")

	// Assert
	assertBatchIDs(t, res, "901", "902", "903")
}

// TestPromoteBatch_NewAgents_IntoDefaultLayer_StackHoldsIDAboveFloor verifies the default
// catalog batch follows the largest id even when it is above 1000.
func TestPromoteBatch_NewAgents_IntoDefaultLayer_StackHoldsIDAboveFloor(t *testing.T) {
	// Act
	res, _ := idFloorBatch(t, "plmosaicdefault", "1004")

	// Assert
	assertBatchIDs(t, res, "1005", "1006", "1007")
}

// TestPromoteBatch_ReplacedAgentInUserLayer_KeepsLowIDAndNewAgentsStartAtFloor verifies a
// replaced agent keeps an id below 1000 and the new agents of the batch still start at 1000.
func TestPromoteBatch_ReplacedAgentInUserLayer_KeepsLowIDAndNewAgentsStartAtFloor(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top")
	ls.addAgent(t, "top", "Audit", "a-agent", "12")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "c-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("top")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)

	// Act
	res := mustRunBatch(t, ls.withDefaultLayer(t, spy), dir)

	// Assert
	assertBatchIDs(t, res, "12", "1000", "1001")
}
