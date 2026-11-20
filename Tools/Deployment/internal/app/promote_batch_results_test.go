package app_test

// promote_batch_results_test.go covers how PromoteBatch accounts for the selected agents: a
// failing agent does not stop the batch, a cancel aborts it (keeping earlier promotions), every
// selected agent has a status, ids and keys are distinct within a batch, and source files are
// never changed.

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

// removeAfterSelection makes the spy delete the agent file once the selection is answered, so
// the file can no longer be decoded when the batch starts on it.
func removeAfterSelection(t *testing.T, spy *batchSpy, path string) {
	t.Helper()
	spy.after = func(id domain.QuestionID, _ string) {
		if id == domain.QPromoteAgents {
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove fixture file: %v", err)
			}
		}
	}
}

func statuses(res app.PromoteBatchResult) map[string]app.PromoteBatchStatus {
	out := map[string]app.PromoteBatchStatus{}
	for _, a := range res.Agents {
		out[filepath.Base(a.SourcePath)] = a.Status
	}
	return out
}

// TestPromoteBatch_UnreadableAgent_FailsThatAgentAndTheBatchContinues verifies a selected file
// that cannot be decoded is Failed with its error, is the subject of no question, and the next
// agent is promoted.
func TestPromoteBatch_UnreadableAgent_FailsThatAgentAndTheBatchContinues(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	removeAfterSelection(t, spy, filepath.Join(dir, "a-agent.md"))

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	a := agentResult(t, res, "a-agent.md")
	if a.Status != app.PromoteBatchFailed || a.Err == nil || a.Reason == "" {
		t.Errorf("a-agent = %+v, want failed with an error and a reason", a)
	}
	requirePromoted(t, res, "b-agent.md")
	if res.Aborted {
		t.Error("batch aborted, want it to continue after a failed agent")
	}
	if n := len(spy.asked(domain.QPromoteCategory)); n != 1 {
		t.Errorf("category asked %d times, want 1 (only for b-agent)", n)
	}
	if n := spy.askedSubject(domain.QPromoteTierRationale, "a-agent"); n != 0 {
		t.Errorf("rationale asked %d times for the failed agent, want 0", n)
	}
}

// batchAbortCase is one per-agent question whose cancel aborts the batch while the second of
// three agents is being processed.
type batchAbortCase struct {
	name  string
	setup func(t *testing.T, ls *plLayerSet, spy *batchSpy)
}

var batchAbortCases = []batchAbortCase{
	{"category", func(_ *testing.T, _ *plLayerSet, spy *batchSpy) {
		spy.oneSeq[domain.QPromoteCategory] = []domain.ChoiceAnswer{chosen("Audit"), status(domain.Cancelled)}
	}},
	{"rationale", func(_ *testing.T, _ *plLayerSet, spy *batchSpy) {
		spy.text[askKey{domain.QPromoteTierRationale, "b-agent"}] = textStatus(domain.Cancelled)
	}},
	{"skills", func(_ *testing.T, _ *plLayerSet, spy *batchSpy) {
		spy.text[askKey{domain.QPromoteRequiredSkills, "b-agent"}] = textStatus(domain.Cancelled)
	}},
	{"existing agent", func(t *testing.T, ls *plLayerSet, spy *batchSpy) {
		ls.addAgent(t, "only", "Audit", "b-agent", "301")
		spy.one[askKey{domain.QPromoteExistingAgent, "b-agent"}] = status(domain.Cancelled)
	}},
	{"interaction error", func(_ *testing.T, _ *plLayerSet, spy *batchSpy) {
		spy.err[askKey{domain.QPromoteTierRationale, "b-agent"}] = errors.New("interaction failed")
	}},
}

// TestPromoteBatch_AbortOnPerAgentQuestion_KeepsEarlierPromotionsAndReportsTheRestNotProcessed
// verifies an abort while processing the second agent keeps the first promotion, reports the
// second and third as not processed with nothing written for them, and returns no error.
func TestPromoteBatch_AbortOnPerAgentQuestion_KeepsEarlierPromotionsAndReportsTheRestNotProcessed(t *testing.T) {
	for _, tc := range batchAbortCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls := newBatchLayers(t, "only")
			dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.md"})
			spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "c-agent.md")
			spy.oneSeq[domain.QPromoteCategory] = []domain.ChoiceAnswer{chosen("Audit")}
			tc.setup(t, ls, spy)
			srcBefore := snapshotTree(t, dir)

			// Act
			res, err := runBatch(t, ls.batchDeps(t, spy), dir)

			// Assert
			if err != nil {
				t.Fatalf("PromoteBatch returned %v, want a populated result and no error", err)
			}
			if !res.Aborted {
				t.Error("Aborted = false, want true")
			}
			want := map[string]app.PromoteBatchStatus{
				"a-agent.md": app.PromoteBatchPromoted,
				"b-agent.md": app.PromoteBatchNotProcessed,
				"c-agent.md": app.PromoteBatchNotProcessed,
			}
			if got := statuses(res); !reflect.DeepEqual(got, want) {
				t.Errorf("statuses = %v, want %v", got, want)
			}
			if len(res.Agents) != 3 {
				t.Errorf("result lists %d agents, want all 3 selected", len(res.Agents))
			}
			if got := keyFiles(t, ls.roots["only"], "a-agent"); len(got) != 1 {
				t.Errorf("the agent promoted before the abort has files %v, want exactly one", got)
			}
			if got := keyFiles(t, ls.roots["only"], "c-agent"); len(got) != 0 {
				t.Errorf("an agent after the abort was written: %v", got)
			}
			if !reflect.DeepEqual(srcBefore, snapshotTree(t, dir)) {
				t.Error("source files changed")
			}
		})
	}
}

// TestPromoteBatch_ContextDone_AbortsLikeACancel verifies a context that ends during the batch
// aborts it: the agent being processed and every later agent are not processed. The context
// ends right after a valid answer, so the implementation must re-check it after every answer,
// not only before asking; that post-answer check is intended.
func TestPromoteBatch_ContextDone_AbortsLikeACancel(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	spy.after = func(id domain.QuestionID, _ string) {
		if id == domain.QPromoteCategory {
			cancel()
		}
	}

	// Act
	res, err := app.New(ls.batchDeps(t, spy)).PromoteBatch(ctx, app.PromoteBatchRequest{HarnessID: batchHarnessID, AgentsDir: dir})

	// Assert
	if err != nil {
		t.Fatalf("PromoteBatch returned %v, want a populated result and no error", err)
	}
	want := map[string]app.PromoteBatchStatus{"a-agent.md": app.PromoteBatchNotProcessed, "b-agent.md": app.PromoteBatchNotProcessed}
	if !res.Aborted || !reflect.DeepEqual(statuses(res), want) {
		t.Errorf("aborted = %v statuses = %v, want aborted with %v", res.Aborted, statuses(res), want)
	}
	if got := keyFiles(t, ls.roots["only"], "a-agent"); len(got) != 0 {
		t.Errorf("agent written after the context ended: %v", got)
	}
}

// TestPromoteBatch_AbortAfterSelection_KeepsOutcomesKnownFromTheSelection verifies a duplicate
// key stays skipped and an undecodable file stays failed when a later question aborts; only the
// agent being processed and the agents after it are not processed.
func TestPromoteBatch_AbortAfterSelection_KeepsOutcomesKnownFromTheSelection(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "x.agent.md"}, batchSrc{File: "x.md"}, batchSrc{File: "z-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "x.agent.md", "x.md", "z-agent.md")
	spy.oneSeq[domain.QPromoteCategory] = []domain.ChoiceAnswer{status(domain.Cancelled)}
	removeAfterSelection(t, spy, filepath.Join(dir, "a-agent.md"))

	// Act
	res, err := runBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if err != nil {
		t.Fatalf("PromoteBatch returned %v, want a populated result and no error", err)
	}
	want := map[string]app.PromoteBatchStatus{
		"a-agent.md": app.PromoteBatchFailed,
		"x.agent.md": app.PromoteBatchNotProcessed,
		"x.md":       app.PromoteBatchSkipped,
		"z-agent.md": app.PromoteBatchNotProcessed,
	}
	if !res.Aborted || !reflect.DeepEqual(statuses(res), want) {
		t.Errorf("aborted = %v statuses = %v, want aborted with %v", res.Aborted, statuses(res), want)
	}
}

// onceCancelCase is a once-per-batch question that is cancelled.
type onceCancelCase struct {
	name       string
	setup      func(spy *batchSpy)
	wantTarget string // TargetCatalogID: empty when the abort comes before the layer is chosen
}

var onceCancelCases = []onceCancelCase{
	{"layer", func(s *batchSpy) { s.oneByID[domain.QPromoteTargetCatalog] = status(domain.Cancelled) }, ""},
	{"unmapped tool", func(s *batchSpy) {
		s.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
		s.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = textStatus(domain.Cancelled)
	}, "low"},
	{"tier", func(s *batchSpy) {
		s.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
		s.one[askKey{domain.QPromoteRecommendedTier, "m1"}] = status(domain.Cancelled)
	}, "low"},
}

// TestPromoteBatch_AbortOnOncePerBatchQuestion_WritesNothingAndReportsEveryAgentNotProcessed
// verifies a cancel on a once-per-batch question leaves every layer untouched and every selected
// agent not processed, with no error.
func TestPromoteBatch_AbortOnOncePerBatchQuestion_WritesNothingAndReportsEveryAgentNotProcessed(t *testing.T) {
	for _, tc := range onceCancelCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls := newBatchLayers(t, "top", "low")
			dir := writeBatchFolder(t,
				batchSrc{File: "a-agent.md", Model: "m1", Tools: []string{"tool-one"}},
				batchSrc{File: "b-agent.md", Model: "m1"})
			spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
			spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
			tc.setup(spy)
			before := ls.snapshotLayers(t)

			// Act
			res, err := runBatch(t, ls.batchDeps(t, spy), dir)

			// Assert
			if err != nil {
				t.Fatalf("PromoteBatch returned %v, want a populated result and no error", err)
			}
			want := map[string]app.PromoteBatchStatus{"a-agent.md": app.PromoteBatchNotProcessed, "b-agent.md": app.PromoteBatchNotProcessed}
			if !res.Aborted || !reflect.DeepEqual(statuses(res), want) {
				t.Errorf("aborted = %v statuses = %v, want aborted with %v", res.Aborted, statuses(res), want)
			}
			if got := ls.changedLayers(t, before); len(got) != 0 {
				t.Errorf("layers changed: %v", got)
			}
			if res.TargetCatalogID != tc.wantTarget {
				t.Errorf("TargetCatalogID = %q, want %q", res.TargetCatalogID, tc.wantTarget)
			}
			if n := len(spy.asked(domain.QPromoteCategory)); n != 0 {
				t.Errorf("category asked %d times after an abort in the once-per-batch questions, want 0", n)
			}
		})
	}
}

// TestPromoteBatch_EverySelectedAgentIsAccountedFor verifies one entry per selected agent, in
// selection order, with the source path, key, status and the reason or error that goes with it.
func TestPromoteBatch_EverySelectedAgentIsAccountedFor(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	ls.addAgent(t, "only", "Audit", "b-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.md"},
		batchSrc{File: "x.agent.md"}, batchSrc{File: "x.md"})
	files := []string{"a-agent.md", "b-agent.md", "c-agent.md", "x.agent.md", "x.md"}
	spy := batchSpyFor(t, ls, files...)
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	removeAfterSelection(t, spy, filepath.Join(dir, "c-agent.md"))

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if len(res.Agents) != len(files) {
		t.Fatalf("result lists %d agents, want %d", len(res.Agents), len(files))
	}
	wantStatus := []app.PromoteBatchStatus{app.PromoteBatchPromoted, app.PromoteBatchSkipped, app.PromoteBatchFailed,
		app.PromoteBatchPromoted, app.PromoteBatchSkipped}
	for i, a := range res.Agents {
		if a.SourcePath != filepath.Join(dir, files[i]) || a.Key != batchKey(files[i]) || a.Status != wantStatus[i] {
			t.Errorf("entry %d = {%q %q %q}, want {%q %q %q}", i, a.SourcePath, a.Key, a.Status,
				filepath.Join(dir, files[i]), batchKey(files[i]), wantStatus[i])
		}
		switch a.Status {
		case app.PromoteBatchPromoted:
			if a.Reason != "" || a.Err != nil || a.Result.DestinationPath == "" || a.Result.NumericID == "" {
				t.Errorf("%s promoted entry = %+v, want a result and no reason or error", files[i], a)
			}
		case app.PromoteBatchFailed:
			if a.Err == nil || a.Reason == "" {
				t.Errorf("%s failed entry = %+v, want an error and a reason", files[i], a)
			}
		case app.PromoteBatchSkipped:
			if a.Reason == "" || a.Err != nil {
				t.Errorf("%s skipped entry = %+v, want a reason and no error", files[i], a)
			}
		}
	}
	if want := "another selected file has the same key x"; res.Agents[4].Reason != want {
		t.Errorf("duplicate reason = %q, want %q", res.Agents[4].Reason, want)
	}
}

// TestPromoteBatch_NewAgents_GetDistinctIDsAboveTheStackAndDistinctKeys verifies new agents get
// consecutive ids above the stack's largest, no id of the stack is reused, and two selected files
// with the same key write only the first of them (in selection order) and ask nothing for the
// second.
func TestPromoteBatch_NewAgents_GetDistinctIDsAboveTheStackAndDistinctKeys(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	ls.addAgent(t, "top", "Audit", "other-agent", "400")
	ls.addAgent(t, "low", "Audit", "old-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"},
		batchSrc{File: "x.agent.md", Desc: "first of the duplicates"}, batchSrc{File: "x.md", Desc: "second of the duplicates"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "x.agent.md", "x.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	ids := map[string]string{}
	for _, f := range []string{"a-agent.md", "b-agent.md", "x.agent.md"} {
		ids[f] = requirePromoted(t, res, f).Result.NumericID
	}
	want := map[string]string{"a-agent.md": "901", "b-agent.md": "902", "x.agent.md": "903"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v, want consecutive ids above the stack's largest %v", ids, want)
	}
	dup := agentResult(t, res, "x.md")
	if dup.Status != app.PromoteBatchSkipped || !strings.Contains(dup.Reason, "same key x") {
		t.Errorf("x.md = %+v, want skipped because another selected file has the same key x", dup)
	}
	xs := keyFiles(t, ls.roots["low"], "x")
	if len(xs) != 1 {
		t.Fatalf("layer holds %v for key x, want exactly one", xs)
	}
	if got, _ := parseFrontmatterFieldSvc(t, xs[0], "description"); got != "first of the duplicates" {
		t.Errorf("x description = %q, want the first selected file's", got)
	}
	if n := len(spy.asked(domain.QPromoteCategory)); n != 3 {
		t.Errorf("category asked %d times, want 3 (the duplicate is not asked about)", n)
	}
}

// TestPromoteBatch_SourceFilesAreNeverChanged verifies promoted, skipped and duplicate source
// files are byte-identical after the batch and the folder gains no file.
func TestPromoteBatch_SourceFilesAreNeverChanged(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	ls.addAgent(t, "only", "Audit", "b-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"},
		batchSrc{File: "x.agent.md"}, batchSrc{File: "x.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "x.agent.md", "x.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingReplace)
	before := snapshotTree(t, dir)

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	requirePromoted(t, res, "a-agent.md")
	requirePromoted(t, res, "b-agent.md")
	if after := snapshotTree(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("agents folder changed:\nbefore %v\nafter  %v", before, after)
	}
}
