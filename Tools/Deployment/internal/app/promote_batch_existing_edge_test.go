package app_test

// promote_batch_existing_edge_test.go covers edge cases of PromoteBatch for keys that already
// exist in the stack: a duplicate key within the batch whose key also exists in the stack, and an
// answer to the replace-or-skip question that is neither of its two options.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// TestPromoteBatch_DuplicateKeyThatExistsInStack_ReplacementIsNotOverwrittenByTheSecondFile
// verifies that when two selected files share a key already in the stack and the user replaces
// for the first, the second is skipped as a duplicate without its own existing-agent question,
// and the replacement written for the first file stays.
func TestPromoteBatch_DuplicateKeyThatExistsInStack_ReplacementIsNotOverwrittenByTheSecondFile(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	ls.addAgent(t, "only", "Audit", "x", "301")
	dir := writeBatchFolder(t,
		batchSrc{File: "x.agent.md", Desc: "first of the duplicates"},
		batchSrc{File: "x.md", Desc: "second of the duplicates"})
	spy := batchSpyFor(t, ls, "x.agent.md", "x.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	first := requirePromoted(t, res, "x.agent.md")
	if first.Result.NumericID != "301" {
		t.Errorf("x.agent.md id = %q, want the replaced agent's 301", first.Result.NumericID)
	}
	dup := agentResult(t, res, "x.md")
	if dup.Status != app.PromoteBatchSkipped || !strings.Contains(dup.Reason, "same key x") {
		t.Errorf("x.md = %+v, want skipped because another selected file has the same key x", dup)
	}
	if n := len(spy.asked(domain.QPromoteExistingAgent)); n != 1 {
		t.Errorf("existing-agent question asked %d times, want once (not for the duplicate)", n)
	}
	files := keyFiles(t, ls.roots["only"], "x")
	if len(files) != 1 {
		t.Fatalf("layer holds %v for key x, want exactly one", files)
	}
	if got, _ := parseFrontmatterFieldSvc(t, files[0], "description"); got != "first of the duplicates" {
		t.Errorf("x description = %q, want the first selected file's", got)
	}
}

// TestPromoteBatch_ExistingKey_UnknownOptionAnswer_DoesNotReplaceTheAgent verifies an answer
// that is not one of the offered options is never treated as a replace: the question is asked,
// the agent is not promoted and every layer stays byte-identical.
func TestPromoteBatch_ExistingKey_UnknownOptionAnswer_DoesNotReplaceTheAgent(t *testing.T) {
	// Arrange
	b := newExistingBatch(t, "Audit")
	b.spy.oneByID[domain.QPromoteExistingAgent] = chosen("not-an-offered-option")
	before := b.ls.snapshotLayers(t)

	// Act
	res, err := runBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

	// Assert
	if n := len(b.spy.asked(domain.QPromoteExistingAgent)); n != 1 {
		t.Fatalf("existing-agent question asked %d times, want once", n)
	}
	if err != nil {
		t.Fatalf("PromoteBatch returned %v, want a result with the agent not promoted", err)
	}
	if a := agentResult(t, res, "a-agent.md"); a.Status != app.PromoteBatchSkipped && a.Status != app.PromoteBatchFailed {
		t.Errorf("a-agent status = %q after an unknown option, want skipped or failed", a.Status)
	}
	if got := b.ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("layers changed after an unknown option answer: %v", got)
	}
}
