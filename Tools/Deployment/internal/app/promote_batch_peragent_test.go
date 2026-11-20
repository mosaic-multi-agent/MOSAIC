package app_test

// promote_batch_peragent_test.go covers the per-agent questions of PromoteBatch that do not
// concern an existing key: the category (always asked, no skip) and the tier rationale and
// required skills (skippable for one agent or for the rest of the batch).

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// TestPromoteBatch_Category_AskedPerAgentWithoutSkipAndWritesToChosenCategory verifies the
// category question is asked once per agent, offers no skip, offers the layer's categories and
// the utility sentinel, and each agent lands in the category it was given.
func TestPromoteBatch_Category_AskedPerAgentWithoutSkipAndWritesToChosenCategory(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true}, batchSrc{File: "b-agent.md", Full: true},
		batchSrc{File: "c-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md", "c-agent.md")
	spy.oneSeq[domain.QPromoteCategory] = []domain.ChoiceAnswer{chosen("Audit"), chosen("Build"), chosen(app.PromoteCategoryUtility)}

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	asks := spy.asked(domain.QPromoteCategory)
	if len(asks) != 3 {
		t.Fatalf("category asked %d times, want once per agent (3)", len(asks))
	}
	for _, a := range asks {
		if a.Choice.AllowSkip || a.Choice.AllowSkipAll {
			t.Error("category question offers skip, want none")
		}
		ids := optionIDs(a.Choice)
		for _, want := range []string{"Audit", "Build", app.PromoteCategoryUtility} {
			if !batchContains(ids, want) {
				t.Errorf("category options %v lack %q", ids, want)
			}
		}
	}
	want := map[string]string{
		"a-agent.md": filepath.Join(ls.roots["only"], "Subagents", "Audit", "a-agent.md"),
		"b-agent.md": filepath.Join(ls.roots["only"], "Subagents", "Build", "b-agent.md"),
		"c-agent.md": filepath.Join(ls.roots["only"], "UtilityAgents", "c-agent.md"),
	}
	for f, path := range want {
		if got := requirePromoted(t, res, f).Result.DestinationPath; got != path {
			t.Errorf("%s written to %q, want %q", f, got, path)
		}
	}
}

// TestPromoteBatch_Category_NotAnOfferedOption_FailsThatAgentAndContinues verifies a category
// that is not one of the offered options fails the agent, writes nothing for it, and the next
// agent is still promoted.
func TestPromoteBatch_Category_NotAnOfferedOption_FailsThatAgentAndContinues(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true}, batchSrc{File: "b-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneSeq[domain.QPromoteCategory] = []domain.ChoiceAnswer{chosen("NoSuchCategory"), chosen("Audit")}

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	a := agentResult(t, res, "a-agent.md")
	if a.Status != app.PromoteBatchFailed || a.Err == nil || a.Reason == "" {
		t.Errorf("a-agent = %+v, want failed with an error and a reason", a)
	}
	requirePromoted(t, res, "b-agent.md")
	if got := keyFiles(t, ls.roots["only"], "a-agent"); len(got) != 0 {
		t.Errorf("failed agent was written: %v", got)
	}
	if res.Aborted {
		t.Error("batch aborted, want it to continue after a failed agent")
	}
}

// TestPromoteBatch_Category_NotAnAnswer_FailsThatAgentWithCategoryRequired verifies a skipped
// category (no skip is offered) fails the agent with ErrPromoteCategoryRequired, and the batch
// continues.
func TestPromoteBatch_Category_NotAnAnswer_FailsThatAgentWithCategoryRequired(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true}, batchSrc{File: "b-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md", "b-agent.md")
	spy.oneSeq[domain.QPromoteCategory] = []domain.ChoiceAnswer{status(domain.SkippedOne), chosen("Audit")}

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	a := agentResult(t, res, "a-agent.md")
	if a.Status != app.PromoteBatchFailed || !errors.Is(a.Err, app.ErrPromoteCategoryRequired) {
		t.Errorf("a-agent = %+v, want failed with ErrPromoteCategoryRequired", a)
	}
	requirePromoted(t, res, "b-agent.md")
}

// optionalTextCase is one of the two optional text questions of the per-agent loop.
type optionalTextCase struct {
	name  string
	id    domain.QuestionID
	key   string // frontmatter key written
	value string // answer
	want  any    // value expected in the file
}

var optionalTextCases = []optionalTextCase{
	{"tier rationale", domain.QPromoteTierRationale, "tier_rationale", "needs careful reading", "needs careful reading"},
	{"required skills", domain.QPromoteRequiredSkills, "required_skills", "lean-tdd, efficient-file-reading",
		[]string{"lean-tdd", "efficient-file-reading"}},
}

// frontmatterValue reads key of the promoted file as a list when want is a list, else a scalar.
func frontmatterValue(t *testing.T, path, key string, want any) (any, bool) {
	t.Helper()
	if _, isList := want.([]string); isList {
		l := parseFrontmatterListSvc(t, path, key)
		return l, l != nil
	}
	v, ok := parseFrontmatterFieldSvc(t, path, key)
	return v, ok
}

func newOptionalTextBatch(t *testing.T) (*plLayerSet, *batchSpy, string) {
	t.Helper()
	ls := newBatchLayers(t, "only")
	files := []string{"a-agent.md", "b-agent.md"}
	dir := writeBatchFolder(t, batchSrc{File: files[0]}, batchSrc{File: files[1]})
	spy := batchSpyFor(t, ls, files...)
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return ls, spy, dir
}

// TestPromoteBatch_OptionalText_AskedPerAgentWithSkipAndWritten verifies the rationale and the
// skills are asked per agent (subject the key) with skip and skip-all, and the answer is written.
func TestPromoteBatch_OptionalText_AskedPerAgentWithSkipAndWritten(t *testing.T) {
	for _, tc := range optionalTextCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls, spy, dir := newOptionalTextBatch(t)
			spy.textByID[tc.id] = typed(tc.value)

			// Act
			res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

			// Assert
			for _, k := range []string{"a-agent", "b-agent"} {
				if n := spy.askedSubject(tc.id, k); n != 1 {
					t.Errorf("%s asked %d times for %s, want 1", tc.id, n, k)
				}
			}
			for _, a := range spy.asked(tc.id) {
				if !a.Text.AllowSkip || !a.Text.AllowSkipAll {
					t.Errorf("%s offers AllowSkip=%v AllowSkipAll=%v, want both", a.Subject, a.Text.AllowSkip, a.Text.AllowSkipAll)
				}
			}
			for _, f := range []string{"a-agent.md", "b-agent.md"} {
				path := requirePromoted(t, res, f).Result.DestinationPath
				if got, _ := frontmatterValue(t, path, tc.key, tc.want); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("%s %s = %v, want %v", f, tc.key, got, tc.want)
				}
			}
		})
	}
}

// TestPromoteBatch_OptionalText_SkipForOneAgentOmitsOnlyThatAgentsField verifies a skip applies
// to the one agent: its field is omitted, the next agent is asked and keeps its answer.
func TestPromoteBatch_OptionalText_SkipForOneAgentOmitsOnlyThatAgentsField(t *testing.T) {
	for _, tc := range optionalTextCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls, spy, dir := newOptionalTextBatch(t)
			spy.text[askKey{tc.id, "a-agent"}] = textStatus(domain.SkippedOne)
			spy.text[askKey{tc.id, "b-agent"}] = typed(tc.value)

			// Act
			res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

			// Assert
			a := requirePromoted(t, res, "a-agent.md").Result.DestinationPath
			if got, present := frontmatterValue(t, a, tc.key, tc.want); present {
				t.Errorf("a-agent %s = %v, want it omitted", tc.key, got)
			}
			b := requirePromoted(t, res, "b-agent.md").Result.DestinationPath
			if got, _ := frontmatterValue(t, b, tc.key, tc.want); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("b-agent %s = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

// TestPromoteBatch_OptionalText_SkipAllOmitsFieldAndStopsAsking verifies skip-all omits the field
// for this and every later agent and asks that question no more.
func TestPromoteBatch_OptionalText_SkipAllOmitsFieldAndStopsAsking(t *testing.T) {
	for _, tc := range optionalTextCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls, spy, dir := newOptionalTextBatch(t)
			spy.text[askKey{tc.id, "a-agent"}] = textStatus(domain.SkippedAll)

			// Act
			res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

			// Assert
			if n := len(spy.asked(tc.id)); n != 1 {
				t.Errorf("%s asked %d times, want 1 (skipped for the rest of the batch)", tc.id, n)
			}
			for _, f := range []string{"a-agent.md", "b-agent.md"} {
				path := requirePromoted(t, res, f).Result.DestinationPath
				if got, present := frontmatterValue(t, path, tc.key, tc.want); present {
					t.Errorf("%s %s = %v, want it omitted", f, tc.key, got)
				}
			}
		})
	}
}

// TestPromoteBatch_OptionalText_NotAskedWhenSourceCarriesField verifies a source that carries the
// rationale and the skills is not asked for them.
func TestPromoteBatch_OptionalText_NotAskedWhenSourceCarriesField(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t, batchSrc{File: "a-agent.md", Full: true})
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	for _, id := range []domain.QuestionID{domain.QPromoteTierRationale, domain.QPromoteRequiredSkills} {
		if n := len(spy.asked(id)); n != 0 {
			t.Errorf("%s asked %d times for a source carrying the field, want 0", id, n)
		}
	}
	path := requirePromoted(t, res, "a-agent.md").Result.DestinationPath
	if got, _ := parseFrontmatterFieldSvc(t, path, "tier_rationale"); got != "moderate capability" {
		t.Errorf("tier_rationale = %q, want the source's", got)
	}
}

// TestPromoteBatch_OptionalText_AnsweredEmptyOmitsField verifies an empty answer leaves the key
// out of the generic agent.
func TestPromoteBatch_OptionalText_AnsweredEmptyOmitsField(t *testing.T) {
	// Arrange
	ls, spy, dir := newOptionalTextBatch(t)
	spy.textByID[domain.QPromoteTierRationale] = typed("   ")
	spy.textByID[domain.QPromoteRequiredSkills] = typed(" , ")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	doc, raw := parsePromoted(t, requirePromoted(t, res, "a-agent.md").Result.DestinationPath)
	assertKeysAbsent(t, doc, raw, "tier_rationale", "required_skills")
}
