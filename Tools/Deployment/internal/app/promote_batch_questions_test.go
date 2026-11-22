package app_test

// promote_batch_questions_test.go covers the questions PromoteBatch asks once for the whole
// batch (target layer, unmapped tool names, recommended tier per source model) and that all of
// them are asked before any agent is written.

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// optionIDs returns the IDs of the options of a question.
func optionIDs(q domain.ChoiceQuestion) []string {
	var ids []string
	for _, o := range q.Options {
		ids = append(ids, o.ID)
	}
	return ids
}

// TestPromoteBatch_MultiLayerStack_AsksLayerOnceAndWritesEveryAgentThere verifies one layer
// question for the whole batch, without skip, and every agent written to the chosen layer.
func TestPromoteBatch_MultiLayerStack_AsksLayerOnceAndWritesEveryAgentThere(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "c-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	baseline := spy.fileCount()

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	layerAsks := spy.asked(domain.QPromoteTargetCatalog)
	if len(layerAsks) != 1 {
		t.Fatalf("layer question asked %d times, want 1", len(layerAsks))
	}
	if layerAsks[0].Choice.AllowSkip || layerAsks[0].Choice.AllowSkipAll {
		t.Error("layer question offers skip, want none")
	}
	if layerAsks[0].Files != baseline {
		t.Errorf("layers held %d files at the layer question, want the %d they held before the batch", layerAsks[0].Files, baseline)
	}
	if got := optionIDs(layerAsks[0].Choice); !reflect.DeepEqual(got, []string{"top", "low"}) {
		t.Errorf("layer options = %v, want [top low]", got)
	}
	for _, f := range []string{"a-agent.md", "b-agent.md", "c-agent.md"} {
		want := filepath.Join(ls.roots["low"], "Subagents", "Audit", batchKey(f)+".md")
		if got := requirePromoted(t, res, f).Result.DestinationPath; got != want {
			t.Errorf("%s written to %q, want %q", f, got, want)
		}
	}
	if res.TargetCatalogID != "low" || res.TargetLayer != layerLabel("low") {
		t.Errorf("target = %q / %q, want low / %q", res.TargetCatalogID, res.TargetLayer, layerLabel("low"))
	}
	if len(keyFiles(t, ls.roots["top"], "a-agent")) != 0 {
		t.Error("an agent was written to the layer that was not chosen")
	}
}

// TestPromoteBatch_SingleLayerStack_AsksNoLayerQuestion verifies a one-layer stack receives the
// batch without a layer question.
func TestPromoteBatch_SingleLayerStack_AsksNoLayerQuestion(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"}, batchSrc{File: "b-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Build")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if n := len(spy.asked(domain.QPromoteTargetCatalog)); n != 0 {
		t.Errorf("layer question asked %d times for a one-layer stack, want 0", n)
	}
	want := filepath.Join(ls.roots["only"], "Subagents", "Build", "a-agent.md")
	if got := requirePromoted(t, res, "a-agent.md").Result.DestinationPath; got != want {
		t.Errorf("a-agent written to %q, want %q", got, want)
	}
	if res.TargetCatalogID != "only" {
		t.Errorf("TargetCatalogID = %q, want only", res.TargetCatalogID)
	}
}

// TestPromoteBatch_PlainCatalog_WritesToItsRootWithoutLayerQuestion verifies a catalog that is
// not a stack receives the batch at its root, with no layer id reported.
func TestPromoteBatch_PlainCatalog_WritesToItsRootWithoutLayerQuestion(t *testing.T) {
	// Arrange
	mosaicRoot := writeMosaicRoot(t)
	mkdirBatch(t, filepath.Join(mosaicRoot, "Catalog", "Subagents", "Audit"))
	spy := newBatchSpy(t, mosaicRoot).selecting("a-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	deps, _ := newPromoteDeps(t, spy.Stub, mosaicRoot)
	deps.Interaction = spy
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"})

	// Act
	res := mustRunBatch(t, withBatchHarness(deps, false), dir)

	// Assert
	want := filepath.Join(mosaicRoot, "Catalog", "Subagents", "Audit", "a-agent.md")
	if got := requirePromoted(t, res, "a-agent.md").Result.DestinationPath; got != want {
		t.Errorf("written to %q, want %q", got, want)
	}
	if n := len(spy.asked(domain.QPromoteTargetCatalog)); n != 0 {
		t.Errorf("layer question asked %d times for a plain catalog, want 0", n)
	}
	if res.TargetCatalogID != "" {
		t.Errorf("TargetCatalogID = %q, want empty for a plain catalog", res.TargetCatalogID)
	}
}

// batchToolFolder is two agents: a declares two unmapped tools and the mapped bash, b shares
// one unmapped tool and declares a third.
func batchToolFolder(t *testing.T) string {
	t.Helper()
	return writeBatchFolder(t,
		batchSrc{File: "a-agent.md", Tools: []string{"bash", "tool-one", "tool-two"}, Full: true},
		batchSrc{File: "b-agent.md", Tools: []string{"tool-one", "tool-three"}, Full: true})
}

func newToolBatch(t *testing.T) (*plLayerSet, *batchSpy) {
	t.Helper()
	ls := newBatchLayers(t, "only")
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return ls, spy
}

// TestPromoteBatch_UnmappedTool_AskedOnceAndAnswerReachesEveryAgentDeclaringIt verifies a tool
// name shared by several agents is asked once, before any write, with skip and skip-all, and the
// answer lands in every agent that declares it; a mapped tool is never asked.
func TestPromoteBatch_UnmappedTool_AskedOnceAndAnswerReachesEveryAgentDeclaringIt(t *testing.T) {
	// Arrange
	ls, spy := newToolBatch(t)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = typed("file-reader")
	baseline := spy.fileCount()

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))

	// Assert
	for _, tool := range []string{"tool-one", "tool-two", "tool-three"} {
		if n := spy.askedSubject(domain.QPromoteCustomTool, tool); n != 1 {
			t.Errorf("%s asked %d times, want 1", tool, n)
		}
	}
	if n := spy.askedSubject(domain.QPromoteCustomTool, "bash"); n != 0 {
		t.Errorf("mapped tool bash asked %d times, want 0", n)
	}
	for _, a := range spy.asked(domain.QPromoteCustomTool) {
		if !a.Text.AllowSkip || !a.Text.AllowSkipAll {
			t.Errorf("%s offers AllowSkip=%v AllowSkipAll=%v, want both", a.Subject, a.Text.AllowSkip, a.Text.AllowSkipAll)
		}
		if a.Files != baseline {
			t.Errorf("%s asked with %d files in the layers, want %d (before any write)", a.Subject, a.Files, baseline)
		}
	}
	for _, f := range []string{"a-agent.md", "b-agent.md"} {
		if tools := requirePromoted(t, res, f).Result.Tools; !batchContains(tools, "file-reader") {
			t.Errorf("%s tools = %v, want the shared answer file-reader", f, tools)
		}
	}
}

// TestPromoteBatch_UnmappedTool_SkipCarriesNameVerbatimToEveryAgent verifies a skipped tool name
// reaches every agent that declares it unchanged, and does not skip the other tool questions.
func TestPromoteBatch_UnmappedTool_SkipCarriesNameVerbatimToEveryAgent(t *testing.T) {
	// Arrange
	ls, spy := newToolBatch(t)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = textStatus(domain.SkippedOne)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-two"}] = typed("writer")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))

	// Assert
	for _, f := range []string{"a-agent.md", "b-agent.md"} {
		if tools := requirePromoted(t, res, f).Result.Tools; !batchContains(tools, "tool-one") {
			t.Errorf("%s tools = %v, want tool-one verbatim", f, tools)
		}
	}
	if tools := agentResult(t, res, "a-agent.md").Result.Tools; !batchContains(tools, "writer") {
		t.Errorf("a-agent tools = %v, want the answered writer", tools)
	}
	if n := spy.askedSubject(domain.QPromoteCustomTool, "tool-three"); n != 1 {
		t.Errorf("tool-three asked %d times after a single skip, want 1", n)
	}
}

// TestPromoteBatch_UnmappedTool_SkipAllStopsFurtherToolQuestions verifies skip-all keeps this and
// every later tool name verbatim and asks no further tool question.
func TestPromoteBatch_UnmappedTool_SkipAllStopsFurtherToolQuestions(t *testing.T) {
	// Arrange
	ls, spy := newToolBatch(t)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = textStatus(domain.SkippedAll)

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))

	// Assert
	if n := len(spy.asked(domain.QPromoteCustomTool)); n != 1 {
		t.Errorf("%d tool questions asked, want exactly the one that was skipped for all", n)
	}
	a := requirePromoted(t, res, "a-agent.md").Result.Tools
	b := requirePromoted(t, res, "b-agent.md").Result.Tools
	for _, tool := range []string{"tool-one", "tool-two"} {
		if !batchContains(a, tool) {
			t.Errorf("a-agent tools = %v, want %s verbatim", a, tool)
		}
	}
	for _, tool := range []string{"tool-one", "tool-three"} {
		if !batchContains(b, tool) {
			t.Errorf("b-agent tools = %v, want %s verbatim", b, tool)
		}
	}
}

// TestPromoteBatch_UnrecoverableToolInfo_KeepsPerAgentToolsQuestion verifies a harness whose
// tool information is unrecoverable is asked its tools question per agent, never the unmapped
// tool question.
func TestPromoteBatch_UnrecoverableToolInfo_KeepsPerAgentToolsQuestion(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.textByID[domain.QPromoteNonRecoverableTools] = typed("bash")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true}, batchSrc{File: "b-agent.md", Full: true})

	// Act
	res := mustRunBatch(t, withBatchHarness(ls.deps(t, spy), true), dir)

	// Assert
	if n := len(spy.asked(domain.QPromoteNonRecoverableTools)); n != 2 {
		t.Errorf("tools question asked %d times, want once per agent (2)", n)
	}
	if n := len(spy.asked(domain.QPromoteCustomTool)); n != 0 {
		t.Errorf("unmapped tool question asked %d times, want 0", n)
	}
	for _, f := range []string{"a-agent.md", "b-agent.md"} {
		if tools := requirePromoted(t, res, f).Result.Tools; !reflect.DeepEqual(tools, []string{"bash"}) {
			t.Errorf("%s tools = %v, want [bash]", f, tools)
		}
	}
}

// TestPromoteBatch_UnrecoverableToolInfo_CancelAbortsTheBatch verifies a cancel on the tools
// question of the second agent aborts like any per-agent question: the first agent stays
// promoted, the second and third are not processed and nothing is written for them.
func TestPromoteBatch_UnrecoverableToolInfo_CancelAbortsTheBatch(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "c-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.textByID[domain.QPromoteNonRecoverableTools] = typed("bash")
	spy.text[askKey{domain.QPromoteNonRecoverableTools, "b-agent"}] = textStatus(domain.Cancelled)
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true}, batchSrc{File: "b-agent.md", Full: true},
		batchSrc{File: "c-agent.md", Full: true})

	// Act
	res, err := runBatch(t, withBatchHarness(ls.deps(t, spy), true), dir)

	// Assert
	if err != nil {
		t.Fatalf("PromoteBatch returned %v, want a populated result and no error", err)
	}
	want := map[string]app.PromoteBatchStatus{
		"a-agent.md": app.PromoteBatchPromoted,
		"b-agent.md": app.PromoteBatchNotProcessed,
		"c-agent.md": app.PromoteBatchNotProcessed,
	}
	if !res.Aborted {
		t.Error("Aborted = false, want true")
	}
	if got := statuses(res); !reflect.DeepEqual(got, want) {
		t.Errorf("statuses = %v, want %v", got, want)
	}
	if got := keyFiles(t, ls.roots["only"], "b-agent"); len(got) != 0 {
		t.Errorf("the agent whose tools question was cancelled was written: %v", got)
	}
}

// TestPromoteBatch_UnrecoverableToolInfo_SkipPromotesWithEmptyTools verifies a skip on the
// tools question is not an abort: the agent is promoted with an empty tools list.
func TestPromoteBatch_UnrecoverableToolInfo_SkipPromotesWithEmptyTools(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.textByID[domain.QPromoteNonRecoverableTools] = typed("bash")
	spy.text[askKey{domain.QPromoteNonRecoverableTools, "b-agent"}] = textStatus(domain.SkippedOne)
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true}, batchSrc{File: "b-agent.md", Full: true})

	// Act
	res := mustRunBatch(t, withBatchHarness(ls.deps(t, spy), true), dir)

	// Assert
	if res.Aborted {
		t.Error("Aborted = true, want false for a skipped tools question")
	}
	if tools := requirePromoted(t, res, "a-agent.md").Result.Tools; !reflect.DeepEqual(tools, []string{"bash"}) {
		t.Errorf("a-agent tools = %v, want [bash]", tools)
	}
	if tools := requirePromoted(t, res, "b-agent.md").Result.Tools; len(tools) != 0 {
		t.Errorf("b-agent tools = %v, want empty", tools)
	}
}

func batchContains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// batchModelFolder holds agents on models m1 (two agents) and m2, plus two without a model.
func batchModelFolder(t *testing.T) string {
	t.Helper()
	return writeBatchFolder(t,
		batchSrc{File: "a-agent.md", Model: "m1"}, batchSrc{File: "b-agent.md", Model: "m1"},
		batchSrc{File: "c-agent.md", Model: "m2"}, batchSrc{File: "d-agent.md"}, batchSrc{File: "e-agent.md"})
}

var batchModelFiles = []string{"a-agent.md", "b-agent.md", "c-agent.md", "d-agent.md", "e-agent.md"}

func newModelBatch(t *testing.T) (*plLayerSet, *batchSpy) {
	t.Helper()
	ls := newBatchLayers(t, "only")
	spy := batchSpyFor(t, ls, batchModelFiles...)
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return ls, spy
}

func recommendedTier(t *testing.T, res app.PromoteBatchResult, file string) (string, bool) {
	t.Helper()
	return parseFrontmatterFieldSvc(t, requirePromoted(t, res, file).Result.DestinationPath, "recommended_tier")
}

// TestPromoteBatch_Tier_AskedOncePerDistinctModelAndIndividuallyWithoutModel verifies one tier
// question per distinct source model (subject the model id), one per agent without a model
// (subject the agent key), asked before any write with skip, skip-all and a custom entry, and
// the answer reaching every agent of the model.
func TestPromoteBatch_Tier_AskedOncePerDistinctModelAndIndividuallyWithoutModel(t *testing.T) {
	// Arrange
	ls, spy := newModelBatch(t)
	spy.one[askKey{domain.QPromoteRecommendedTier, "m1"}] = chosen("HIGH")
	spy.one[askKey{domain.QPromoteRecommendedTier, "m2"}] = chosenCustom("XL")
	spy.one[askKey{domain.QPromoteRecommendedTier, "d-agent"}] = chosen("HIGH")
	baseline := spy.fileCount()

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchModelFolder(t))

	// Assert
	for subject, want := range map[string]int{"m1": 1, "m2": 1, "d-agent": 1, "e-agent": 1, "a-agent": 0, "b-agent": 0} {
		if n := spy.askedSubject(domain.QPromoteRecommendedTier, subject); n != want {
			t.Errorf("tier asked %d times for %q, want %d", n, subject, want)
		}
	}
	for _, a := range spy.asked(domain.QPromoteRecommendedTier) {
		q := a.Choice
		if !q.AllowSkip || !q.AllowSkipAll || !q.AllowCustom || q.CustomPrompt == "" {
			t.Errorf("tier question %q flags: skip=%v skipAll=%v custom=%v prompt=%q; want all offered with a prompt",
				a.Subject, q.AllowSkip, q.AllowSkipAll, q.AllowCustom, q.CustomPrompt)
		}
		if a.Files != baseline {
			t.Errorf("tier question %q asked with %d files in the layers, want %d", a.Subject, a.Files, baseline)
		}
	}
	for file, want := range map[string]string{"a-agent.md": "HIGH", "b-agent.md": "HIGH", "c-agent.md": "XL", "d-agent.md": "HIGH"} {
		if got, _ := recommendedTier(t, res, file); got != want {
			t.Errorf("%s recommended_tier = %q, want %q", file, got, want)
		}
	}
	if got, present := recommendedTier(t, res, "e-agent.md"); present {
		t.Errorf("e-agent recommended_tier = %q, want none (its tier question was skipped)", got)
	}
}

// TestPromoteBatch_Tier_SkipForOneModelLeavesOtherModelsAsked verifies a skip leaves that model's
// agents without a tier and does not skip the other questions.
func TestPromoteBatch_Tier_SkipForOneModelLeavesOtherModelsAsked(t *testing.T) {
	// Arrange
	ls, spy := newModelBatch(t)
	spy.one[askKey{domain.QPromoteRecommendedTier, "m1"}] = status(domain.SkippedOne)
	spy.one[askKey{domain.QPromoteRecommendedTier, "m2"}] = chosen("HIGH")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchModelFolder(t))

	// Assert
	for _, f := range []string{"a-agent.md", "b-agent.md"} {
		if got, present := recommendedTier(t, res, f); present {
			t.Errorf("%s recommended_tier = %q, want none after the model was skipped", f, got)
		}
	}
	if got, _ := recommendedTier(t, res, "c-agent.md"); got != "HIGH" {
		t.Errorf("c-agent recommended_tier = %q, want HIGH", got)
	}
}

// TestPromoteBatch_Tier_SkipAllStopsFurtherTierQuestions verifies skip-all leaves every later
// agent without a tier and asks no further tier question.
func TestPromoteBatch_Tier_SkipAllStopsFurtherTierQuestions(t *testing.T) {
	// Arrange
	ls, spy := newModelBatch(t)
	spy.one[askKey{domain.QPromoteRecommendedTier, "m1"}] = status(domain.SkippedAll)

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchModelFolder(t))

	// Assert
	if n := len(spy.asked(domain.QPromoteRecommendedTier)); n != 1 {
		t.Errorf("%d tier questions asked, want only the one skipped for all", n)
	}
	for _, f := range batchModelFiles {
		if got, present := recommendedTier(t, res, f); present {
			t.Errorf("%s recommended_tier = %q, want none after skip-all", f, got)
		}
	}
}

// TestPromoteBatch_Tier_NotAskedWhenSourceCarriesTier verifies a source that already carries a
// recommended_tier is not asked and keeps it.
func TestPromoteBatch_Tier_NotAskedWhenSourceCarriesTier(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Model: "m1", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if n := len(spy.asked(domain.QPromoteRecommendedTier)); n != 0 {
		t.Errorf("tier asked %d times for a source carrying its tier, want 0", n)
	}
	if got, _ := recommendedTier(t, res, "a-agent.md"); got != "MEDIUM" {
		t.Errorf("recommended_tier = %q, want the source's MEDIUM", got)
	}
}

// TestPromoteBatch_OncePerBatchQuestions_AllAskedInOrderBeforeAnyPerAgentQuestion verifies the
// order layer, tool names, tier, and that every one of them precedes the first per-agent question
// and the first write.
func TestPromoteBatch_OncePerBatchQuestions_AllAskedInOrderBeforeAnyPerAgentQuestion(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	dir := writeBatchFolder(t,
		batchSrc{File: "a-agent.md", Model: "m1", Tools: []string{"tool-one"}},
		batchSrc{File: "b-agent.md", Model: "m2"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	baseline := spy.fileCount()

	// Act
	mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	var order []domain.QuestionID
	for _, a := range spy.asks {
		order = append(order, a.ID)
	}
	wantPrefix := []domain.QuestionID{
		domain.QPromoteAgents, domain.QPromoteTargetCatalog, domain.QPromoteCustomTool,
		domain.QPromoteRecommendedTier, domain.QPromoteRecommendedTier,
	}
	if len(order) < len(wantPrefix) || !reflect.DeepEqual(order[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("question order = %v, want it to start with %v", order, wantPrefix)
	}
	for _, a := range spy.asks[:len(wantPrefix)] {
		if a.Files != baseline {
			t.Errorf("%s asked with %d files in the layers, want %d", a.ID, a.Files, baseline)
		}
	}
	for _, a := range spy.asks[len(wantPrefix):] {
		switch a.ID {
		case domain.QPromoteTargetCatalog, domain.QPromoteCustomTool, domain.QPromoteRecommendedTier:
			t.Errorf("%s asked again after the once-per-batch questions", a.ID)
		}
	}
}
