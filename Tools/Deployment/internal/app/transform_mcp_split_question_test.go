package app_test

// transform_mcp_split_question_test.go specifies the split question of the whole-deployment
// transform: an OpenCode name that mapping and declared servers leave ambiguous is asked once
// per distinct name for the agents that will be written, with the possible splits listed and
// the split at the first "_" offered first. An answer decides the split; skip and skip-all leave
// the name unmapped; cancelling aborts the run with nothing written.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trDeclareAzure makes only the Azure DevOps name resolvable, leaving the tracker name ambiguous.
func trDeclareAzure(t *testing.T, r *trRun) {
	t.Helper()
	trPutOpenCodeConfig(t, r, "declared-azure.json", "opencode.json")
}

// trTrackerQuestion returns the split question asked for the tracker name.
func trTrackerQuestion(t *testing.T, r *trRun) domain.ChoiceQuestion {
	t.Helper()
	for _, q := range r.spy.splitQuestions() {
		if q.Subject == trSplitTracker {
			return q
		}
	}
	t.Fatalf("no split question for %s; asked: %q", trSplitTracker, r.spy.splitSubjects())
	return domain.ChoiceQuestion{}
}

func TestTransformMcpSplit_AmbiguousNameIsAskedWithTheFirstUnderscoreSplitFirst(t *testing.T) {
	r := trMCPRun(t, nil)
	trDeclareAzure(t, r)

	r.mustRun(r.request())

	q := trTrackerQuestion(t, r)
	var ids, labels []string
	for _, o := range q.Options {
		ids = append(ids, o.ID)
		labels = append(labels, o.Label)
	}
	if want := []string{"tracker", "tracker_get"}; !slices.Equal(ids, want) {
		t.Errorf("option ids = %q, want %q (shortest server first)", ids, want)
	}
	if want := []string{"tracker / get_issue", "tracker_get / issue"}; !slices.Equal(labels, want) {
		t.Errorf("option labels = %q, want %q", labels, want)
	}
	if !q.AllowSkip || !q.AllowSkipAll {
		t.Errorf("the question must allow skip and skip-all, got AllowSkip=%v AllowSkipAll=%v", q.AllowSkip, q.AllowSkipAll)
	}
}

func TestTransformMcpSplit_NameDecidedByTheDeclaredServerIsNotAsked(t *testing.T) {
	r := trMCPRun(t, nil)
	trDeclareAzure(t, r)

	r.mustRun(r.request())

	if got, want := r.spy.splitSubjects(), []string{trSplitTracker}; !slices.Equal(got, want) {
		t.Errorf("split questions = %q, want %q", got, want)
	}
}

func TestTransformMcpSplit_NameIsAskedOnceForSeveralAgentsThatGrantIt(t *testing.T) {
	r := trMCPRun(t, nil)
	trDeclareAzure(t, r)
	r.put(hOpen, "second-agent", trOpenCodeFixture(t, "opencode-agents", trMCPAgentKey+".md"))

	r.mustRun(r.request())

	if n := strings.Count(strings.Join(r.spy.splitSubjects(), "\n"), trSplitTracker); n != 1 {
		t.Errorf("the tracker name must be asked once, asked %d times: %q", n, r.spy.splitSubjects())
	}
}

func TestTransformMcpSplit_NotAskedForAnAgentThatIsNotWritten(t *testing.T) {
	r := trMCPRun(t, nil)
	r.put(hCLI, trMCPAgentKey, "existing destination") // skipped: destination exists

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); len(got) != 0 {
		t.Errorf("split questions asked for an agent that is not written: %q", got)
	}
}

func TestTransformMcpSplit_AnsweredSplitIsWrittenInGhcpSyntaxAndNotUnmapped(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOne(domain.QMCPNameSplit, trSplitTracker, "tracker_get").Build()
	r := trMCPRun(t, script)
	trDeclareAzure(t, r)

	res := r.mustRun(r.request())

	out := trMCPOutput(r)
	if !strings.Contains(out, "tracker_get/issue") {
		t.Errorf("the answered split must be written as tracker_get/issue:\n%s", out)
	}
	if strings.Contains(out, trSplitTracker) {
		t.Errorf("the source form of the name must not be carried:\n%s", out)
	}
	if got := trUnmapped(t, res); slices.Contains(got, trSplitTracker) {
		t.Errorf("UnmappedTools = %q, an answered name is not unmapped", got)
	}
}

func TestTransformMcpSplit_SkippedName_IsReportedUnmappedAndTheAgentStillWritten(t *testing.T) {
	r := trMCPRun(t, nil) // the stub skips an unscripted question
	trDeclareAzure(t, r)

	res := r.mustRun(r.request())

	f := trFile(t, res, trMCPAgentKey)
	if f.Status != app.StatusTransformed {
		t.Errorf("status = %q, want transformed: a skipped name does not fail the file", f.Status)
	}
	if !slices.Contains(f.UnmappedTools, trSplitTracker) {
		t.Errorf("UnmappedTools = %q, want it to contain %q", f.UnmappedTools, trSplitTracker)
	}
	out := trMCPOutput(r)
	for _, bad := range []string{trSplitTracker, "tracker/get_issue", "tracker_get/issue"} {
		if strings.Contains(out, bad) {
			t.Errorf("a skipped name must not be written, found %q:\n%s", bad, out)
		}
	}
}

func TestTransformMcpSplit_SkipAllRequest_AsksNothingAndReportsTheNamesUnmapped(t *testing.T) {
	r := trMCPRun(t, nil)
	req := r.request()
	req.SkipAll = map[domain.QuestionID]bool{domain.QMCPNameSplit: true}

	res := r.mustRun(req)

	if got := r.spy.splitSubjects(); len(got) != 0 {
		t.Errorf("split questions asked despite SkipAll: %q", got)
	}
	got := trUnmapped(t, res)
	for _, want := range []string{trSplitTracker, trSplitAzure} {
		if !slices.Contains(got, want) {
			t.Errorf("UnmappedTools = %q, want it to contain %q", got, want)
		}
	}
}

func TestTransformMcpSplit_SkipAllAnswer_StopsFurtherQuestions(t *testing.T) {
	// The first question asked is for trSplitTracker because the fixture lists the tracker
	// permission before the azure one; reordering the fixture requires changing this script.
	script := interactiontest.NewBuilder().
		AnswerSkipAll(domain.QMCPNameSplit, trSplitTracker).Build()
	r := trMCPRun(t, script)

	res := r.mustRun(r.request())

	if got := r.spy.splitSubjects(); len(got) != 1 {
		t.Errorf("after skip-all no further split question may be asked, asked: %q", got)
	}
	got := trUnmapped(t, res)
	for _, want := range []string{trSplitTracker, trSplitAzure} {
		if !slices.Contains(got, want) {
			t.Errorf("UnmappedTools = %q, want it to contain %q", got, want)
		}
	}
}

func TestTransformMcpSplit_PreAnswer_SuppressesTheQuestionAndIsWritten(t *testing.T) {
	r := trMCPRun(t, nil)
	trDeclareAzure(t, r)
	req := r.request()
	req.MCPSplits = map[string]string{trSplitTracker: "tracker"}

	res := r.mustRun(req)

	if got := r.spy.splitSubjects(); len(got) != 0 {
		t.Errorf("a pre-answered name must not be asked, asked: %q", got)
	}
	if out := trMCPOutput(r); !strings.Contains(out, "tracker/get_issue") {
		t.Errorf("the pre-answer must be written as tracker/get_issue:\n%s", out)
	}
	if got := trUnmapped(t, res); len(got) != 0 {
		t.Errorf("UnmappedTools = %q, want none", got)
	}
}

func TestTransformMcpSplit_PreAnswerNamingNoOfferedSplit_IsASkipReportedUnmapped(t *testing.T) {
	r := trMCPRun(t, nil)
	trDeclareAzure(t, r)
	req := r.request()
	req.MCPSplits = map[string]string{trSplitTracker: "nonsense"}

	res := r.mustRun(req)

	if got := trUnmapped(t, res); !slices.Contains(got, trSplitTracker) {
		t.Errorf("UnmappedTools = %q, want it to contain %q", got, trSplitTracker)
	}
	if out := trMCPOutput(r); strings.Contains(out, "nonsense") {
		t.Errorf("a pre-answer naming no offered split must not be written:\n%s", out)
	}
}

func TestTransformMcpSplit_CancelledQuestion_AbortsWithNothingWritten(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerCancelledChoice(domain.QMCPNameSplit, trSplitTracker).Build()
	r := trMCPRun(t, script)
	trDeclareAzure(t, r)
	before := r.snapshot()

	res, err := r.run(r.request())

	if !errors.Is(err, app.ErrSelectionCancelled) {
		t.Fatalf("error = %v, want ErrSelectionCancelled", err)
	}
	if !res.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("cancelled run left a trace: %s", diff)
	}
	if len(r.spy.reviews) != 0 {
		t.Error("the plan review was asked after a cancelled split question")
	}
}

func TestTransformMcpSplit_DryRunAsksTheQuestionAndWritesNothing(t *testing.T) {
	r := trMCPRun(t, nil)
	trDeclareAzure(t, r)
	before := r.snapshot()
	req := r.request()
	req.DryRun = true

	r.mustRun(req)

	if got := r.spy.splitSubjects(); !slices.Contains(got, trSplitTracker) {
		t.Errorf("a dry run asks the same questions, asked: %q", got)
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("dry run left a trace: %s", diff)
	}
}
