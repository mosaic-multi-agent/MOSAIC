package app_test

// promote_batch_existing_test.go covers PromoteBatch for an agent whose key already exists in the
// stack: the replace-or-skip question (default skip, naming the holding layer), a skip leaving
// every layer byte-identical, and a replacement taking the old agent's place (same numeric id,
// chosen layer and category, one agent with the key in that layer, no other layer modified).

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// existingBatch is a stack whose lower layer holds "a-agent" (id 301), and a one-agent selection.
type existingBatch struct {
	ls  *plLayerSet
	spy *batchSpy
	dir string
}

func newExistingBatch(t *testing.T, existingCategory string) existingBatch {
	t.Helper()
	ls := newBatchLayers(t, "top", "low")
	ls.addAgentIn(t, "low", existingCategory, "a-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return existingBatch{ls, spy, dir}
}

// TestPromoteBatch_ExistingKey_AsksReplaceOrSkipWithSkipAsDefault verifies the question shape: it
// concerns only an agent whose key exists, names the holding layer, has the two options in
// order with skip as the default, and offers no skip action of its own.
func TestPromoteBatch_ExistingKey_AsksReplaceOrSkipWithSkipAsDefault(t *testing.T) {
	// Arrange
	b := newExistingBatch(t, "Audit")
	writeBatchFile(t, b.dir, "b-agent.md", batchSrc{File: "b-agent.md", Full: true}.bytes())
	b.spy.selecting("a-agent.md", "b-agent.md")

	// Act
	mustRunBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

	// Assert
	asks := b.spy.asked(domain.QPromoteExistingAgent)
	if len(asks) != 1 {
		t.Fatalf("existing-agent question asked %d times, want once (only a-agent exists)", len(asks))
	}
	q := asks[0].Choice
	if q.Subject != "a-agent" {
		t.Errorf("Subject = %q, want the agent key a-agent", q.Subject)
	}
	if want := "Agent a-agent already exists in " + layerLabel("low"); q.Title != want {
		t.Errorf("Title = %q, want %q", q.Title, want)
	}
	if q.AllowSkip || q.AllowSkipAll {
		t.Error("existing-agent question offers a skip action, want only its two options")
	}
	if q.DefaultOptionID != app.PromoteExistingSkip {
		t.Errorf("DefaultOptionID = %q, want %q", q.DefaultOptionID, app.PromoteExistingSkip)
	}
	type opt struct{ id, label string }
	var got []opt
	for _, o := range q.Options {
		got = append(got, opt{o.ID, o.Label})
	}
	want := []opt{
		{app.PromoteExistingSkip, "Skip this agent"},
		{app.PromoteExistingReplace, "Replace the existing agent in " + layerLabel("low")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %+v, want %+v", got, want)
	}
}

// TestPromoteBatch_ExistingKey_NotReplacedUnlessUserChoosesReplace verifies skip, skip-one and
// skip-all all keep the existing agent: the agent is skipped with the holding layer in the
// reason, no category is asked for it, and every layer stays byte-identical.
func TestPromoteBatch_ExistingKey_NotReplacedUnlessUserChoosesReplace(t *testing.T) {
	answers := map[string]domain.ChoiceAnswer{
		"skip option": chosen(app.PromoteExistingSkip),
		"skipped":     status(domain.SkippedOne),
		"skipped all": status(domain.SkippedAll),
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			// Arrange
			b := newExistingBatch(t, "Audit")
			b.spy.oneByID[domain.QPromoteExistingAgent] = answer
			before := b.ls.snapshotLayers(t)

			// Act
			res := mustRunBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

			// Assert
			a := agentResult(t, res, "a-agent.md")
			if a.Status != app.PromoteBatchSkipped {
				t.Fatalf("status = %q, want skipped", a.Status)
			}
			if want := "kept the existing agent in " + layerLabel("low"); a.Reason != want {
				t.Errorf("Reason = %q, want %q", a.Reason, want)
			}
			if got := b.ls.changedLayers(t, before); len(got) != 0 {
				t.Errorf("layers changed after declining the replacement: %v", got)
			}
			if n := len(b.spy.asked(domain.QPromoteCategory)); n != 0 {
				t.Errorf("category asked %d times for a skipped agent, want 0", n)
			}
		})
	}
}

// TestPromoteBatch_ExistingKey_Replace_KeepsIDInChosenCategoryAndTouchesOnlyThatLayer verifies a
// replacement in the batch layer: one agent with the key remains, under the chosen category, with
// the replaced agent's id, and no other layer is modified.
func TestPromoteBatch_ExistingKey_Replace_KeepsIDInChosenCategoryAndTouchesOnlyThatLayer(t *testing.T) {
	for _, tc := range replaceCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			b := newExistingBatch(t, tc.existing)
			b.spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)
			b.spy.oneByID[domain.QPromoteCategory] = chosen(tc.chosen)
			before := b.ls.snapshotLayers(t)
			srcBefore := readBytes(t, filepath.Join(b.dir, "a-agent.md"))

			// Act
			res := mustRunBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

			// Assert
			a := requirePromoted(t, res, "a-agent.md")
			want := b.ls.agentPath("low", tc.chosen, "a-agent")
			if got := keyFiles(t, b.ls.roots["low"], "a-agent"); !reflect.DeepEqual(got, []string{want}) {
				t.Errorf("layer holds %v for the key, want exactly [%s]", got, want)
			}
			if a.Result.DestinationPath != want || a.Result.NumericID != "301" {
				t.Errorf("result = %q id %q, want %q id 301", a.Result.DestinationPath, a.Result.NumericID, want)
			}
			if id, _ := parseFrontmatterFieldSvc(t, want, "id"); id != "301" {
				t.Errorf("written id = %q, want the replaced agent's 301", id)
			}
			if a.ReplacedLayer != layerLabel("low") {
				t.Errorf("ReplacedLayer = %q, want %q", a.ReplacedLayer, layerLabel("low"))
			}
			if got := b.ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"low"}) {
				t.Errorf("changed layers = %v, want only [low]", got)
			}
			if !bytes.Equal(srcBefore, readBytes(t, filepath.Join(b.dir, "a-agent.md"))) {
				t.Error("the source file was modified")
			}
		})
	}
}

// TestPromoteBatch_ExistingKey_ReplaceAgentOfAnotherLayer_WritesChosenLayerAndLeavesTheOtherAlone
// verifies that when the existing agent lives in another layer, the new agent is written to the
// batch layer with the old agent's id, the question names the other layer, and the other layer
// is left byte-identical.
func TestPromoteBatch_ExistingKey_ReplaceAgentOfAnotherLayer_WritesChosenLayerAndLeavesTheOtherAlone(t *testing.T) {
	// Arrange
	b := newExistingBatch(t, "Audit")
	b.spy.oneByID[domain.QPromoteTargetCatalog] = chosen("top")
	b.spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)
	before := b.ls.snapshotLayers(t)

	// Act
	res := mustRunBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

	// Assert
	a := requirePromoted(t, res, "a-agent.md")
	want := b.ls.agentPath("top", "Audit", "a-agent")
	if a.Result.DestinationPath != want || a.Result.NumericID != "301" {
		t.Errorf("result = %q id %q, want %q id 301", a.Result.DestinationPath, a.Result.NumericID, want)
	}
	if a.ReplacedLayer != layerLabel("low") {
		t.Errorf("ReplacedLayer = %q, want the layer of the replaced agent %q", a.ReplacedLayer, layerLabel("low"))
	}
	if got := b.ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"top"}) {
		t.Errorf("changed layers = %v, want only [top]", got)
	}
	if want := "Agent a-agent already exists in " + layerLabel("low"); b.spy.asked(domain.QPromoteExistingAgent)[0].Choice.Title != want {
		t.Errorf("Title = %q, want %q", b.spy.asked(domain.QPromoteExistingAgent)[0].Choice.Title, want)
	}
}

// TestPromoteBatch_ExistingKey_BatchLayerHoldsKeyAndHigherLayerResolvesIt_ReplacesTheBatchLayersOwn
// verifies the agent replaced is the batch layer's own agent when it holds the key, even if a
// higher layer resolves the key, and the question names the batch layer.
func TestPromoteBatch_ExistingKey_BatchLayerHoldsKeyAndHigherLayerResolvesIt_ReplacesTheBatchLayersOwn(t *testing.T) {
	// Arrange
	b := newExistingBatch(t, "Audit")
	b.ls.addAgent(t, "top", "Audit", "a-agent", "250")
	b.spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)
	before := b.ls.snapshotLayers(t)

	// Act
	res := mustRunBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

	// Assert
	a := requirePromoted(t, res, "a-agent.md")
	if a.Result.NumericID != "301" || a.ReplacedLayer != layerLabel("low") {
		t.Errorf("id %q replaced layer %q, want 301 in %q", a.Result.NumericID, a.ReplacedLayer, layerLabel("low"))
	}
	if got := b.ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"low"}) {
		t.Errorf("changed layers = %v, want only [low]", got)
	}
	if want := "Agent a-agent already exists in " + layerLabel("low"); b.spy.asked(domain.QPromoteExistingAgent)[0].Choice.Title != want {
		t.Errorf("Title = %q, want it to name the batch layer %q", b.spy.asked(domain.QPromoteExistingAgent)[0].Choice.Title, layerLabel("low"))
	}
}

// TestPromoteBatch_ExistingKey_InDefaultCatalog_NamesItAndReplacesIntoChosenLayer verifies the
// question names the MOSAIC default catalog, and a replacement into another layer takes the
// default agent's id and leaves the default catalog byte-identical.
func TestPromoteBatch_ExistingKey_InDefaultCatalog_NamesItAndReplacesIntoChosenLayer(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top")
	ls.registerDefault(t)
	ls.addAgent(t, "plmosaicdefault", "Audit", "a-agent", "11")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("top")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)
	before := ls.snapshotLayers(t)

	// Act
	res := mustRunBatch(t, ls.withDefaultLayer(t, spy), dir)

	// Assert
	q := spy.asked(domain.QPromoteExistingAgent)[0].Choice
	if want := "Agent a-agent already exists in " + defaultLayerLabel; q.Title != want {
		t.Errorf("Title = %q, want %q", q.Title, want)
	}
	if want := "Replace the existing agent in " + defaultLayerLabel; q.Options[1].Label != want {
		t.Errorf("replace option = %q, want %q", q.Options[1].Label, want)
	}
	a := requirePromoted(t, res, "a-agent.md")
	if a.Result.NumericID != "11" || a.ReplacedLayer != defaultLayerLabel {
		t.Errorf("id %q replaced layer %q, want 11 in %q", a.Result.NumericID, a.ReplacedLayer, defaultLayerLabel)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"top"}) {
		t.Errorf("changed layers = %v, want only [top]", got)
	}
}

// TestPromoteBatch_ExistingKey_ReplacementDoesNotShiftNewIDs verifies a replacement keeps its
// own id while a new agent of the same batch still gets the floored first id of a user layer.
func TestPromoteBatch_ExistingKey_ReplacementDoesNotShiftNewIDs(t *testing.T) {
	// Arrange
	b := newExistingBatch(t, "Audit")
	writeBatchFile(t, b.dir, "b-agent.md", batchSrc{File: "b-agent.md", Full: true}.bytes())
	b.spy.selecting("a-agent.md", "b-agent.md")
	b.spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)

	// Act
	res := mustRunBatch(t, b.ls.batchDeps(t, b.spy), b.dir)

	// Assert
	if got := requirePromoted(t, res, "a-agent.md").Result.NumericID; got != "301" {
		t.Errorf("a-agent id = %q, want the replaced 301", got)
	}
	if got := requirePromoted(t, res, "b-agent.md").Result.NumericID; got != "1000" {
		t.Errorf("b-agent id = %q, want 1000 (the floor; the stack's largest is %s)", got, batchSeedID)
	}
}
