package app_test

// transform_custom_tool_list_test.go specifies the custom tool question of the whole-deployment
// transform in the list syntax: the same question text and validation as the deploy flows, every
// entry of an answer reaches the rebuilt agent in the target's form, the tool is not left
// unmapped, and the result's custom tools list names the entries.

import (
	"slices"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trListRun runs the transform of the subagent fixture with the given answer to its custom
// tool question and returns the run and the result.
func trListRun(t *testing.T, answer string) (*trRun, app.TransformHarnessResult) {
	t.Helper()
	script := interactiontest.NewBuilder().
		AnswerText(domain.QCustomTool, trUnmappedGeneric, answer).Build()
	r := trCustomToolRun(t, script)
	return r, r.mustRun(r.request())
}

func TestTransformCustomTool_Question_TeachesTheListSyntaxAndValidates(t *testing.T) {
	r, _ := trListRun(t, listFlowAnswer)

	if len(r.spy.texts) == 0 {
		t.Fatal("QCustomTool was not asked")
	}
	q := r.spy.texts[0]
	assertListSyntaxQuestion(t, q)
	if q.Validate == nil {
		t.Fatal("the question carries no Validate hook")
	}
	assertListSyntaxValidate(t, q.Validate)
}

func TestTransformCustomTool_MixedAnswer_EveryEntryIsWrittenInTheTargetForm(t *testing.T) {
	r, res := trListRun(t, listFlowAnswer)

	written := r.read(r.dest("subagent"))

	names := rbToolNames(t, written, "permission")
	for _, want := range []string{"github_get_pr", "jira_*"} {
		if countName(names, want) != 1 {
			t.Errorf("the written agent must allow %q once as its own entry, got %v", want, names)
		}
	}
	if got := trFile(t, res, "subagent").UnmappedTools; len(got) != 0 {
		t.Errorf("UnmappedTools = %q, want none for an answered tool", got)
	}
	if r.spy.Stub.CountAsked(domain.QCustomTool, trUnmappedGeneric) != 1 {
		t.Error("the question must be asked once for the generic tool")
	}
}

func TestTransformCustomTool_MixedAnswer_ResultListsTheEntries(t *testing.T) {
	_, res := trListRun(t, listFlowAnswer)

	got := trFile(t, res, "subagent").CustomTools

	if want := []string{"github/get_pr", "jira"}; !slices.Equal(got, want) {
		t.Errorf("customTools = %q, want %q", got, want)
	}
}

func TestTransformCustomTool_SingleBareServer_ResultListsTheServerAndNothingIsUnmapped(t *testing.T) {
	_, res := trListRun(t, "my-grep-server")

	f := trFile(t, res, "subagent")

	if want := []string{"my-grep-server"}; !slices.Equal(f.CustomTools, want) {
		t.Errorf("customTools = %q, want %q", f.CustomTools, want)
	}
	if len(f.UnmappedTools) != 0 {
		t.Errorf("UnmappedTools = %q, want none", f.UnmappedTools)
	}
}

func TestTransformCustomTool_WholeServerForm_NormalisesToTheServer(t *testing.T) {
	_, res := trListRun(t, " github/* , jira ")

	got := trFile(t, res, "subagent").CustomTools

	if want := []string{"github", "jira"}; !slices.Equal(got, want) {
		t.Errorf("customTools = %q, want %q", got, want)
	}
}
