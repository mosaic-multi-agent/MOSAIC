package app_test

// transform_models_test.go specifies the model questions of the whole-deployment transform:
// one question per source model group, only for agent files that will be written, with custom
// entry offered; skip leaves one group unset, skip-all this and every later group; existing
// pre-answers keep working; cancelling or failing a question aborts the run with nothing written.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// Source models of the agent fixtures. Plan order of the groups (file-name order) is
// sonnet (harness-only), opus (orchestrator-script, orchestrator), unset (standalone),
// sonnet again (subagent) and haiku (utility).
const (
	mSonnet = "claude-sonnet-4-5"
	mOpus   = "claude-opus-4-5"
	mHaiku  = "claude-haiku-4-5"
	mUnset  = app.UnsetSourceModel
)

// trModelGroups is the distinct source models of the seeded agents in first-seen order.
var trModelGroups = []string{mSonnet, mOpus, mUnset, mHaiku}

// trKeysByModel maps each source model group to the agent keys of its fixtures.
var trKeysByModel = map[string][]string{
	mSonnet: {"harness-only", "subagent"},
	mOpus:   {"orchestrator-script", "orchestrator"},
	mUnset:  {"standalone"},
	mHaiku:  {"utility"},
}

// trAssertModel fails unless the written agent carries a model line containing want.
func trAssertModel(t *testing.T, r *trRun, key, want string) {
	t.Helper()
	line, ok := trModelLine(r.read(r.dest(key)))
	if !ok || !strings.Contains(line, want) {
		t.Errorf("agent %q model line = %q (present %v), want it to contain %q", key, line, ok, want)
	}
}

// trAssertNoModel fails when the written agent carries any model key, empty or not.
func trAssertNoModel(t *testing.T, r *trRun, key string) {
	t.Helper()
	if line, ok := trModelLine(r.read(r.dest(key))); ok {
		t.Errorf("agent %q has model line %q, want no model key", key, line)
	}
}

func trModelRun(t *testing.T, script *interactiontest.Stub) *trRun {
	t.Helper()
	r := trNew(t, script, hClaude, hOpen)
	r.seed(trAgentFixtures...)
	return r
}

func TestTransformModels_AskedOncePerSourceModelGroupInPlanOrder(t *testing.T) {
	r := trModelRun(t, nil)

	r.mustRun(r.request())

	if got := r.spy.modelSubjects(); !slices.Equal(got, trModelGroups) {
		t.Errorf("model question subjects = %q, want %q", got, trModelGroups)
	}
}

func TestTransformModels_QuestionOffersSkipSkipAllAndCustomEntry(t *testing.T) {
	r := trModelRun(t, nil)

	r.mustRun(r.request())

	qs := r.spy.modelQuestions()
	if len(qs) == 0 {
		t.Fatal("no model question was asked")
	}
	for _, q := range qs {
		if !q.AllowCustom || !q.AllowSkip || !q.AllowSkipAll {
			t.Errorf("question for %q: AllowCustom=%v AllowSkip=%v AllowSkipAll=%v, want all true",
				q.Subject, q.AllowCustom, q.AllowSkip, q.AllowSkipAll)
		}
	}
}

func TestTransformModels_FileThatIsNotAnAgent_ContributesNoGroup(t *testing.T) {
	r := trModelRun(t, nil)
	r.put(hClaude, "notes", "---\nname: notes\nmodel: some-other-model\n---\n\nJust notes.\n")

	r.mustRun(r.request())

	if got := r.spy.modelSubjects(); !slices.Equal(got, trModelGroups) {
		t.Errorf("groups asked = %q, want %q (a file that is not an agent adds no group)", got, trModelGroups)
	}
}

func TestTransformModels_DryRun_AsksModelQuestionsAndWritesNothing(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.DryRun = true
	before := r.snapshot()

	r.mustRun(req)

	if got := r.spy.modelSubjects(); !slices.Equal(got, trModelGroups) {
		t.Errorf("groups asked in a dry run = %q, want %q", got, trModelGroups)
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("dry run changed the workspace: %s", diff)
	}
}

func TestTransformModels_OnlyGroupsOfWrittenFilesAreAsked(t *testing.T) {
	r := trModelRun(t, nil)
	r.put(hOpen, "utility", "existing destination") // utility is skipped: destination exists

	r.mustRun(r.request())

	if got, want := r.spy.modelSubjects(), []string{mSonnet, mOpus, mUnset}; !slices.Equal(got, want) {
		t.Errorf("groups asked = %q, want %q (the haiku group's only file is skipped)", got, want)
	}
}

func TestTransformModels_ExistingDestinationWithOverwrite_IsAskedAgain(t *testing.T) {
	r := trModelRun(t, nil)
	r.put(hOpen, "utility", "existing destination")
	req := r.request()
	req.Overwrite = true

	r.mustRun(req)

	if !slices.Contains(r.spy.modelSubjects(), mHaiku) {
		t.Errorf("the haiku group was not asked although its file is overwritten: %q", r.spy.modelSubjects())
	}
}

func TestTransformModels_CustomAnswer_IsWrittenForThatGroupOnly(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOneCustom(domain.QTransformTargetModel, mSonnet, "my-org/custom-model").Build()
	r := trModelRun(t, script)

	r.mustRun(r.request())

	for _, key := range trKeysByModel[mSonnet] {
		trAssertModel(t, r, key, "my-org/custom-model")
	}
	for _, key := range trKeysByModel[mOpus] {
		trAssertNoModel(t, r, key)
	}
}

func TestTransformModels_SkippedGroup_LeavesOnlyThatGroupWithoutModel(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOneCustom(domain.QTransformTargetModel, mSonnet, "my-org/s").
		AnswerSelectOneCustom(domain.QTransformTargetModel, mUnset, "my-org/u").
		AnswerSelectOneCustom(domain.QTransformTargetModel, mHaiku, "my-org/h").Build()
	r := trModelRun(t, script) // the opus group is unscripted, which the stub answers with a skip

	r.mustRun(r.request())

	for _, key := range trKeysByModel[mOpus] {
		trAssertNoModel(t, r, key)
	}
	trAssertModel(t, r, "subagent", "my-org/s")
	trAssertModel(t, r, "standalone", "my-org/u")
	trAssertModel(t, r, "utility", "my-org/h")
}

func TestTransformModels_SkipAll_LeavesThisAndLaterGroupsUnsetWithoutAskingThem(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOneCustom(domain.QTransformTargetModel, mSonnet, "my-org/s").
		AnswerSkipAll(domain.QTransformTargetModel, mOpus).Build()
	r := trModelRun(t, script)

	r.mustRun(r.request())

	if got, want := r.spy.modelSubjects(), []string{mSonnet, mOpus}; !slices.Equal(got, want) {
		t.Errorf("groups asked = %q, want %q (later groups are not asked after skip-all)", got, want)
	}
	trAssertModel(t, r, "subagent", "my-org/s")
	for _, key := range []string{"orchestrator", "orchestrator-script", "standalone", "utility"} {
		trAssertNoModel(t, r, key)
	}
}

func TestTransformModels_TargetModel_AnswersEveryGroupWithoutQuestions(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.TargetModel = "pre/answered"

	r.mustRun(req)

	if got := r.spy.modelSubjects(); len(got) != 0 {
		t.Errorf("model questions asked despite TargetModel: %q", got)
	}
	for _, key := range trAgentFixtures {
		trAssertModel(t, r, key, "pre/answered")
	}
}

func TestTransformModels_ModelMap_AnswersItsGroupAndWinsOverTargetModel(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.TargetModel = "pre/default"
	req.ModelMap = app.TransformModelMapping{mOpus: "pre/opus"}

	r.mustRun(req)

	for _, key := range trKeysByModel[mOpus] {
		trAssertModel(t, r, key, "pre/opus")
	}
	for _, key := range trKeysByModel[mSonnet] {
		trAssertModel(t, r, key, "pre/default")
	}
}

func TestTransformModels_ModelMapWithEmptyValue_LeavesTheGroupUnsetWithoutAsking(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.ModelMap = app.TransformModelMapping{mSonnet: ""}

	r.mustRun(req)

	if got, want := r.spy.modelSubjects(), []string{mOpus, mUnset, mHaiku}; !slices.Equal(got, want) {
		t.Errorf("groups asked = %q, want %q (ModelMap answered the sonnet group)", got, want)
	}
	for _, key := range trKeysByModel[mSonnet] {
		trAssertNoModel(t, r, key)
	}
}

func TestTransformModels_SkipAllRequest_AsksNothingAndWritesNoModel(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.SkipAll = map[domain.QuestionID]bool{domain.QTransformTargetModel: true}

	r.mustRun(req)

	if got := r.spy.modelSubjects(); len(got) != 0 {
		t.Errorf("model questions asked despite SkipAll: %q", got)
	}
	for _, key := range trAgentFixtures {
		trAssertNoModel(t, r, key)
	}
}

func TestTransformModels_CancelledQuestion_AbortsWithNothingWritten(t *testing.T) {
	// Groups before the cancelled one are unscripted, which the stub answers with a skip.
	for _, group := range trModelGroups {
		name := group
		if group == mUnset {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			script := interactiontest.NewBuilder().
				AnswerCancelledChoice(domain.QTransformTargetModel, group).Build()
			r := trModelRun(t, script)
			before := r.snapshot()

			res, err := r.run(r.request())

			if !errors.Is(err, app.ErrModelSelectionCancelled) {
				t.Fatalf("error = %v, want ErrModelSelectionCancelled", err)
			}
			if !res.Cancelled {
				t.Error("Cancelled = false, want true")
			}
			if res.SourceHarnessID != hClaude || res.TargetHarnessID != hOpen || res.Workspace != r.ws {
				t.Errorf("cancelled result = %q -> %q in %q, want %q -> %q in %q",
					res.SourceHarnessID, res.TargetHarnessID, res.Workspace, hClaude, hOpen, r.ws)
			}
			if diff := trSameTree(before, r.snapshot()); diff != "" {
				t.Errorf("cancelled run left a trace (no file may exist after a cancel): %s", diff)
			}
			if len(r.spy.reviews) != 0 {
				t.Error("the plan review was asked after a cancelled model question")
			}
		})
	}
}

func TestTransformModels_FailedQuestion_AbortsAsCancelledWithNothingWritten(t *testing.T) {
	r := trModelRun(t, nil)
	r.spy.failOn = domain.QTransformTargetModel
	before := r.snapshot()

	res, err := r.run(r.request())

	if !errors.Is(err, app.ErrModelSelectionCancelled) {
		t.Fatalf("error = %v, want ErrModelSelectionCancelled", err)
	}
	if !res.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("failed question left a trace: %s", diff)
	}
}
