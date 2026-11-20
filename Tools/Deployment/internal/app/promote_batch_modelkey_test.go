package app_test

// promote_batch_modelkey_test.go covers where PromoteBatch reads an agent's source model from:
// the model key the harness descriptor declares, not a fixed key name.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestPromoteBatch_Tier_SourceModelIsReadFromTheDescriptorModelKey verifies the tier question
// subject is the value of the descriptor's model key (here "llm"), that a decoy "model" key is
// ignored, and that an agent with only the decoy key counts as having no model.
func TestPromoteBatch_Tier_SourceModelIsReadFromTheDescriptorModelKey(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t,
		batchSrc{File: "a-agent.md", Model: "m1", Key: "llm"},
		batchSrc{File: "b-agent.md", Model: "m1", Key: "llm", Extra: "model: decoy-b\n"},
		batchSrc{File: "c-agent.md", Extra: "model: decoy-c\n"},
		batchSrc{File: "d-agent.md", Model: "m2", Key: "llm", Extra: "model: decoy-d\n"})
	files := []string{"a-agent.md", "b-agent.md", "c-agent.md", "d-agent.md"}
	spy := batchSpyFor(t, ls, files...)
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	mustRunBatch(t, withBatchHarnessKey(ls.deps(t, spy), false, "llm"), dir)

	// Assert
	want := map[string]int{"m1": 1, "m2": 1, "c-agent": 1, "a-agent": 0, "b-agent": 0, "d-agent": 0}
	for subject, n := range want {
		if got := spy.askedSubject(domain.QPromoteRecommendedTier, subject); got != n {
			t.Errorf("tier asked %d times for %q, want %d", got, subject, n)
		}
	}
	for _, decoy := range []string{"decoy-b", "decoy-c", "decoy-d"} {
		if got := spy.askedSubject(domain.QPromoteRecommendedTier, decoy); got != 0 {
			t.Errorf("tier asked for decoy model %q, want the model key of the descriptor only", decoy)
		}
	}
}
