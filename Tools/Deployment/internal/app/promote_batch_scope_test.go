package app_test

// promote_batch_scope_test.go covers where PromoteBatch takes its inputs from: the agent
// extension the harness descriptor declares, the selected non-duplicate agents as the only
// subjects of once-per-batch questions, the chosen layer as the source of the offered
// categories, an existing-key skip that asks nothing more, and a model shared by agents of which
// only some carry a tier.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// TestPromoteBatch_Selection_OffersOnlyFilesWithTheExtensionTheDescriptorDeclares verifies the
// harness's declared agent extension (its final suffix, as single-agent promote applies it), not a
// fixed one, decides which files are promotable and
// what their key is.
func TestPromoteBatch_Selection_OffersOnlyFilesWithTheExtensionTheDescriptorDeclares(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t,
		batchSrc{File: "a-agent.prompt"}, batchSrc{File: "b-agent.md"}, batchSrc{File: "c-agent.agent.md"})
	spy := batchSpyFor(t, ls)
	deps := withBatchHarnessModule(ls.deps(t, spy), batchHarnessModule{
		toolResolutionHarnessModule: &toolResolutionHarnessModule{ref: toolResolutionHarness},
		agentExt:                    ".prompt"})

	// Act
	q := selectionOptions(t, func(*batchSpy) app.Deps { return deps }, spy, dir)

	// Assert
	if len(q.Options) != 1 {
		t.Fatalf("options = %+v, want only the .prompt file", q.Options)
	}
	if o := q.Options[0]; o.ID != "a-agent.prompt" || o.Label != "a-agent" {
		t.Errorf("option = %+v, want id a-agent.prompt labelled with the key a-agent", o)
	}
}

// TestPromoteBatch_DuplicateKeyFile_IsNotTheSubjectOfOncePerBatchQuestions verifies a tool or
// model declared only by the skipped duplicate file triggers no tool or tier question.
func TestPromoteBatch_DuplicateKeyFile_IsNotTheSubjectOfOncePerBatchQuestions(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t,
		batchSrc{File: "a-agent.md", Model: "m1"},
		batchSrc{File: "x.agent.md"},
		batchSrc{File: "x.md", Model: "dup-model", Tools: []string{"dup-tool"}})
	spy := batchSpyFor(t, ls, "a-agent.md", "x.agent.md", "x.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if a := agentResult(t, res, "x.md"); a.Status != app.PromoteBatchSkipped {
		t.Fatalf("x.md status = %q, want skipped as the duplicate", a.Status)
	}
	if n := spy.askedSubject(domain.QPromoteCustomTool, "dup-tool"); n != 0 {
		t.Errorf("tool question asked %d times for a tool only the duplicate declares, want 0", n)
	}
	if n := spy.askedSubject(domain.QPromoteRecommendedTier, "dup-model"); n != 0 {
		t.Errorf("tier question asked %d times for a model only the duplicate declares, want 0", n)
	}
}

// TestPromoteBatch_Category_OffersTheCategoriesOfTheChosenLayerOnly verifies the offered
// categories come from the layer the batch writes to, not from another layer.
func TestPromoteBatch_Category_OffersTheCategoriesOfTheChosenLayerOnly(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	mkdirBatch(t, filepath.Join(ls.roots["top"], "Subagents", "OnlyTop"))
	mkdirBatch(t, filepath.Join(ls.roots["low"], "Subagents", "OnlyLow"))
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	asks := spy.asked(domain.QPromoteCategory)
	if len(asks) != 1 {
		t.Fatalf("category asked %d times, want 1", len(asks))
	}
	ids := optionIDs(asks[0].Choice)
	if !batchContains(ids, "OnlyLow") {
		t.Errorf("category options %v lack OnlyLow of the chosen layer", ids)
	}
	if batchContains(ids, "OnlyTop") {
		t.Errorf("category options %v offer OnlyTop of the layer that was not chosen", ids)
	}
}

// TestPromoteBatch_ExistingKey_Skip_AsksNoFurtherQuestionForTheAgent verifies that an agent kept
// as it is, whose source lacks a rationale and skills, is asked nothing more.
func TestPromoteBatch_ExistingKey_Skip_AsksNoFurtherQuestionForTheAgent(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "top", "low")
	ls.addAgentIn(t, "low", "Audit", "a-agent", "301")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md"})
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteTargetCatalog] = chosen("low")
	spy.oneByID[domain.QPromoteExistingAgent] = chosen(app.PromoteExistingSkip)

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if a := agentResult(t, res, "a-agent.md"); a.Status != app.PromoteBatchSkipped {
		t.Fatalf("status = %q, want skipped", a.Status)
	}
	for _, id := range []domain.QuestionID{domain.QPromoteCategory, domain.QPromoteTierRationale, domain.QPromoteRequiredSkills} {
		if n := len(spy.asked(id)); n != 0 {
			t.Errorf("%s asked %d times for a kept agent, want 0", id, n)
		}
	}
}

// TestPromoteBatch_Tier_SharedModelWithOneCarryingAgentAsksOnceForTheOther verifies two agents of
// one model, only one carrying a tier, yield one tier question, and the carrying agent keeps its
// own tier.
func TestPromoteBatch_Tier_SharedModelWithOneCarryingAgentAsksOnceForTheOther(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t,
		batchSrc{File: "a-agent.md", Model: "m1", Full: true},
		batchSrc{File: "b-agent.md", Model: "m1"})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	spy.one[askKey{domain.QPromoteRecommendedTier, "m1"}] = chosen("LOW")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	if n := spy.askedSubject(domain.QPromoteRecommendedTier, "m1"); n != 1 {
		t.Errorf("tier asked %d times for m1, want once (for the agent lacking a tier)", n)
	}
	if n := spy.askedSubject(domain.QPromoteRecommendedTier, "a-agent"); n != 0 {
		t.Errorf("tier asked for a-agent, want its own tier kept")
	}
	a := requirePromoted(t, res, "a-agent.md").Result.DestinationPath
	got, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("read promoted a-agent: %v", err)
	}
	if !strings.Contains(string(got), "recommended_tier: MEDIUM") {
		t.Errorf("a-agent lost its own tier MEDIUM:\n%s", got)
	}
}
