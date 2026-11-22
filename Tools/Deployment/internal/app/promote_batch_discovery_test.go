package app_test

// promote_batch_discovery_test.go covers how PromoteBatch finds and offers the promotable
// agents of an agents folder, the pre-run failures, and that exactly the selected agents are
// processed.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// selectionOptions runs a batch whose selection is cancelled and returns the options offered.
func selectionOptions(t *testing.T, deps func(*batchSpy) app.Deps, spy *batchSpy, dir string) domain.ChoiceQuestion {
	t.Helper()
	spy.many = domain.MultiChoiceAnswer{Status: domain.Cancelled}
	_, _ = runBatch(t, deps(spy), dir)
	asks := spy.asked(domain.QPromoteAgents)
	if len(asks) != 1 {
		t.Fatalf("QPromoteAgents asked %d times, want 1", len(asks))
	}
	return asks[0].Choice
}

// TestPromoteBatch_Selection_OffersEveryPromotableAgentSortedWithLabelsAndHints verifies the
// selection question lists promotable files only, sorted by file name, with the key as label,
// the file name as id and description, the holding layer as hint for keys already in the stack,
// and with no skip.
func TestPromoteBatch_Selection_OffersEveryPromotableAgentSortedWithLabelsAndHints(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	ls.addAgent(t, "low", "Audit", "b-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "b-agent.md"}, batchSrc{File: "a-agent.agent.md"})
	writeBatchFile(t, dir, "plain.md", ineligibleSourceWithNoTransformVersion())
	writeBatchFile(t, dir, "notes.txt", batchSrc{File: "notes.md"}.bytes())
	writeBatchFile(t, dir, "orchestrator.md", batchSrc{File: "orchestrator.md"}.bytes())
	writeBatchFile(t, dir, "orchestrator.agent.md", batchSrc{File: "orchestrator.agent.md"}.bytes())
	writeBatchFile(t, dir, filepath.Join("nested", "deep-agent.md"), batchSrc{File: "deep-agent.md"}.bytes())
	spy := batchSpyFor(t, ls)

	// Act
	q := selectionOptions(t, func(s *batchSpy) app.Deps { return ls.batchDeps(t, s) }, spy, dir)

	// Assert
	type opt struct{ id, label, desc, hint string }
	var got []opt
	for _, o := range q.Options {
		got = append(got, opt{o.ID, o.Label, o.Description, o.Hint})
	}
	want := []opt{
		{"a-agent.agent.md", "a-agent", "a-agent.agent.md", ""},
		{"b-agent.md", "b-agent", "b-agent.md", "already in " + layerLabel("low")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if q.AllowSkip || q.AllowSkipAll {
		t.Errorf("selection offers skip (AllowSkip=%v, AllowSkipAll=%v), want none", q.AllowSkip, q.AllowSkipAll)
	}
	if q.Subject != dir {
		t.Errorf("Subject = %q, want the agents folder %q", q.Subject, dir)
	}
	if q.Title != "Select the agents to promote" {
		t.Errorf("Title = %q, want %q", q.Title, "Select the agents to promote")
	}
}

// TestPromoteBatch_Selection_HintNamesTheLayerTheStackResolvesTheKeyFrom verifies the hint of an
// agent held by several layers names the resolving (highest) layer, and names the default catalog
// by its fixed label.
func TestPromoteBatch_Selection_HintNamesTheLayerTheStackResolvesTheKeyFrom(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	ls.addAgent(t, "top", "Audit", "shared-agent", "250")
	ls.addAgent(t, "low", "Audit", "shared-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "shared-agent.md"}, batchSrc{File: "default-agent.md"})
	ls.registerDefault(t)
	ls.addAgent(t, "plmosaicdefault", "Audit", "default-agent", "11")
	spy := batchSpyFor(t, ls)
	deps := ls.withDefaultLayer(t, spy)

	// Act
	q := selectionOptions(t, func(*batchSpy) app.Deps { return deps }, spy, dir)

	// Assert
	hints := map[string]string{}
	for _, o := range q.Options {
		hints[o.ID] = o.Hint
	}
	if want := "already in " + layerLabel("top"); hints["shared-agent.md"] != want {
		t.Errorf("hint of shared-agent = %q, want %q", hints["shared-agent.md"], want)
	}
	if want := "already in " + defaultLayerLabel; hints["default-agent.md"] != want {
		t.Errorf("hint of default-agent = %q, want %q", hints["default-agent.md"], want)
	}
}

// TestPromoteBatch_AgentsFolderNeverInfluencesTheStack verifies that a catalog tree inside the
// agents folder neither makes its keys exist nor adds a layer: the batch writes into the one
// layer of the service's stack, with no layer and no existing-agent question.
func TestPromoteBatch_AgentsFolderNeverInfluencesTheStack(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "solo-agent.md"})
	writeBatchFile(t, dir, filepath.Join("Subagents", "Audit", "solo-agent.md"), batchSrc{File: "solo-agent.md"}.bytes())
	writeBatchFile(t, dir, "catalog-identity.yaml", []byte("id: other\nname: Other\nversion: \"1.0.0\"\n"))
	spy := batchSpyFor(t, ls, "solo-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	requirePromoted(t, res, "solo-agent.md")
	if res.TargetCatalogID != "only" {
		t.Errorf("TargetCatalogID = %q, want only", res.TargetCatalogID)
	}
	for _, id := range []domain.QuestionID{domain.QPromoteTargetCatalog, domain.QPromoteExistingAgent} {
		if n := len(spy.asked(id)); n != 0 {
			t.Errorf("%s asked %d times, want 0", id, n)
		}
	}
	if got := spy.asked(domain.QPromoteAgents)[0].Choice.Options; len(got) != 1 || got[0].Hint != "" {
		t.Errorf("selection options = %+v, want the one agent without a hint", got)
	}
}

// TestPromoteBatch_OnlySelectedAgentsAreProcessed verifies the result and the stack hold the
// selected subset only.
func TestPromoteBatch_OnlySelectedAgentsAreProcessed(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.md"})
	spy := batchSpyFor(t, ls, "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if len(res.Agents) != 1 || filepath.Base(res.Agents[0].SourcePath) != "b-agent.md" {
		t.Fatalf("result agents = %+v, want only b-agent.md", res.Agents)
	}
	requirePromoted(t, res, "b-agent.md")
	for _, key := range []string{"a-agent", "c-agent"} {
		if got := keyFiles(t, ls.roots["only"], key); len(got) != 0 {
			t.Errorf("unselected agent %s was written: %v", key, got)
		}
	}
	if got := keyFiles(t, ls.roots["only"], "b-agent"); len(got) != 1 {
		t.Errorf("selected agent b-agent files = %v, want exactly one", got)
	}
}

// TestPromoteBatch_PreRunFailures_ReturnErrorZeroResultAndAskNothing verifies each pre-run
// failure returns its error, a zero result, and neither asks a question nor writes.
func TestPromoteBatch_PreRunFailures_ReturnErrorZeroResultAndAskNothing(t *testing.T) {
	ls := newBatchLayers(t, "only")
	good := writeBatchFolder(t, batchSrc{File: "a-agent.md"})
	empty := writeBatchFolder(t, batchSrc{File: "a-agent.md"})
	if err := os.Remove(filepath.Join(empty, "a-agent.md")); err != nil {
		t.Fatalf("remove fixture file: %v", err)
	}
	writeBatchFile(t, empty, "plain.md", ineligibleSourceWithNoTransformVersion())
	cases := []struct {
		name    string
		harness string
		dir     string
		want    error
	}{
		{"no harness", "", good, app.ErrPromoteHarnessRequired},
		{"unknown harness", "nope-harness", good, app.ErrPromoteHarnessUnresolvable},
		{"no folder", batchHarnessID, "", app.ErrPromoteAgentsDirRequired},
		{"missing folder", batchHarnessID, filepath.Join(good, "missing"), app.ErrPromoteAgentsDirInvalid},
		{"folder is a file", batchHarnessID, filepath.Join(good, "a-agent.md"), app.ErrPromoteAgentsDirInvalid},
		{"nothing promotable", batchHarnessID, empty, app.ErrPromoteNoPromotableAgents},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			spy := batchSpyFor(t, ls, "a-agent.md")
			before := ls.snapshotLayers(t)

			// Act
			res, err := app.New(ls.batchDeps(t, spy)).PromoteBatch(context.Background(),
				app.PromoteBatchRequest{HarnessID: tc.harness, AgentsDir: tc.dir})

			// Assert
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want one wrapping %v", err, tc.want)
			}
			if !reflect.DeepEqual(res, app.PromoteBatchResult{}) {
				t.Errorf("result = %+v, want the zero result", res)
			}
			if len(spy.asks) != 0 {
				t.Errorf("%d questions asked before a pre-run failure, want none", len(spy.asks))
			}
			if got := ls.changedLayers(t, before); len(got) != 0 {
				t.Errorf("layers changed: %v", got)
			}
			if tc.want == app.ErrPromoteNoPromotableAgents && !strings.Contains(err.Error(), tc.dir) {
				t.Errorf("error %q does not name the folder %q", err, tc.dir)
			}
		})
	}
}

// TestPromoteBatch_SelectionNotAnswered_FailsWithSelectionCancelled verifies a selection that is
// not an answer with a known option ends the run with ErrSelectionCancelled.
func TestPromoteBatch_SelectionNotAnswered_FailsWithSelectionCancelled(t *testing.T) {
	answers := map[string]domain.MultiChoiceAnswer{
		"cancelled":       {Status: domain.Cancelled},
		"skipped":         {Status: domain.SkippedOne},
		"unsupported":     {Status: domain.AnswerStatus("unsupported")},
		"empty selection": {Status: domain.Answered},
		"unknown option":  {Status: domain.Answered, OptionIDs: []string{"ghost.md"}},
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			// Arrange
			ls := newBatchLayers(t, "only")
			dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"})
			spy := batchSpyFor(t, ls)
			spy.many = answer
			before := ls.snapshotLayers(t)

			// Act
			res, err := runBatch(t, ls.batchDeps(t, spy), dir)

			// Assert
			if !errors.Is(err, app.ErrSelectionCancelled) {
				t.Fatalf("error = %v, want one wrapping ErrSelectionCancelled", err)
			}
			if !reflect.DeepEqual(res, app.PromoteBatchResult{}) {
				t.Errorf("result = %+v, want the zero result", res)
			}
			if got := ls.changedLayers(t, before); len(got) != 0 {
				t.Errorf("layers changed: %v", got)
			}
		})
	}
}

// TestPromoteBatch_SelectionInteractionError_WrapsThatErrorNotSelectionCancelled verifies an
// error from the interaction port on the selection is returned as is.
func TestPromoteBatch_SelectionInteractionError_WrapsThatErrorNotSelectionCancelled(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"})
	boom := errors.New("interaction failed")
	spy := batchSpyFor(t, ls)
	spy.manyErr = boom

	// Act
	_, err := runBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want one wrapping the interaction error", err)
	}
	if errors.Is(err, app.ErrSelectionCancelled) {
		t.Error("an interaction error was reported as ErrSelectionCancelled")
	}
}
