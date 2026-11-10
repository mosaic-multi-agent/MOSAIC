package session_test

// Session tests of the WorkflowFormat 2.16 Input wildcard rule: an Input entry
// containing '*', of any form, is replaced by every artifact that exists in
// the run folder and matches it, in lexical order, at the moment of dispatch.
// The rule names no block and no artifact: no schedule is consulted. No match
// gives no entry and no error. These tests cover the engine-routed path; the
// other dispatch paths are in session_wildcard_paths_test.go.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Start_InputWildcardsOf216Table_ListExistingArtifactsInLexicalOrder
// verifies that Stage-*/Plan.md and *-Research.md are each replaced by the
// matching existing artifacts, in lexical order within each entry and in the
// order the entries are declared, and that an entry matching nothing leaves
// no entry.
func TestSession_Start_InputWildcardsOf216Table_ListExistingArtifactsInLexicalOrder(t *testing.T) {
	rig := newBlockRig(t, "wildcard-inputs-orch.md", nil, "agent-a", "agent-b")
	rig.writeRunFiles(t, append(append([]string{}, wildcardStageFiles...), wildcardResearchFiles...)...)
	rig.queueOK("agent-a")
	rig.queueOK("agent-b")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := wildcardRunPaths("Stage-A/Plan.md", "Stage-b-2/Plan.md", "alpha-Research.md", "beta-Research.md")
	rig.requireInputsOfEveryDispatch(t, "agent-b", 1, want)
}

// TestSession_Start_InputWildcardsOf216Table_InputsCellHoldsTheListedArtifacts
// verifies that the Inputs cell of the Execution Log row names the listed
// artifacts, not the declared patterns.
func TestSession_Start_InputWildcardsOf216Table_InputsCellHoldsTheListedArtifacts(t *testing.T) {
	rig := newBlockRig(t, "wildcard-inputs-orch.md", nil, "agent-a", "agent-b")
	rig.writeRunFiles(t, "Stage-A/Plan.md", "alpha-Research.md")
	rig.queueOK("agent-a")
	rig.queueOK("agent-b")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := strings.Join(wildcardRunPaths("Stage-A/Plan.md", "alpha-Research.md"), ", ")
	if cell := rig.inputsCellOf(t, "agent-b"); cell != want {
		t.Errorf("Inputs cell: want %q, got %q", want, cell)
	}
}

// TestSession_Start_InputWildcardWithNoMatch_YieldsNoEntryAndNoError verifies
// that wildcards matching nothing are not an error and leave no entry, with no
// schedule anywhere.
func TestSession_Start_InputWildcardWithNoMatch_YieldsNoEntryAndNoError(t *testing.T) {
	rig := newBlockRig(t, "wildcard-inputs-orch.md", nil, "agent-a", "agent-b")
	rig.queueOK("agent-a")
	rig.queueOK("agent-b")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	rig.requireInputsOfEveryDispatch(t, "agent-b", 1, nil)
}

// TestSession_Start_InputWildcard_ListsArtifactsWrittenByAnEarlierRow verifies
// that the listing is taken when the row is dispatched: artifacts an earlier
// row wrote during the run are listed, although none existed at the start.
func TestSession_Start_InputWildcard_ListsArtifactsWrittenByAnEarlierRow(t *testing.T) {
	rig := newBlockRig(t, "wildcard-inputs-orch.md", nil, "agent-a", "agent-b")
	rig.queueOK("agent-a",
		runFileWrite("Stage-b-2/Plan.md", "# b-2\n"),
		runFileWrite("Stage-A/Plan.md", "# A\n"),
		runFileWrite("gamma-Research.md", "# gamma\n"))
	rig.queueOK("agent-b")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := wildcardRunPaths("Stage-A/Plan.md", "Stage-b-2/Plan.md", "gamma-Research.md")
	rig.requireInputsOfEveryDispatch(t, "agent-b", 1, want)
}

// TestSession_Start_OutputWildcard_ReachesTheSubagentLiterally verifies that
// an Output entry containing '*' is passed to the subagent as declared, even
// when files that match it exist.
func TestSession_Start_OutputWildcard_ReachesTheSubagentLiterally(t *testing.T) {
	approvals := &blockApprovalRecorder{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "wildcard-outputs-hitl-orch.md", approvals, "agent-a")
	rig.writeRunFiles(t, "Report-1/notes.md", "Report-2/notes.md")
	rig.queueOK("agent-a")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-outputs-hitl", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	outputs := rig.dispatchedOutputs("agent-a")
	if len(outputs) != 1 {
		t.Fatalf("want one dispatch of agent-a, got %d", len(outputs))
	}
	requireSequence(t, "agent-a outputs", outputs[0], wildcardRunPaths("Report-*/notes.md", "summary.md"))
}

// TestSession_Start_InputWildcardInBlockNotNamedStage_ListsMatchingArtifacts
// verifies the rule for a workflow whose block is named Feature, for a row of
// the block and a run-scope row after it: *-Research.md lists the matching
// artifacts, which the planner wrote, and the block's schedule plays no part.
func TestSession_Start_InputWildcardInBlockNotNamedStage_ListsMatchingArtifacts(t *testing.T) {
	rig := newBlockRig(t, "wildcard-block-orch.md", nil, "planner", "builder", "reviewer")
	rig.queueOK("planner",
		runFileWrite("FeaturePlan.md", scheduleDoc("Feature", "Stages", ids("A", "B"))),
		runFileWrite("beta-Research.md", "# beta\n"),
		runFileWrite("alpha-Research.md", "# alpha\n"),
		runFileWrite("Research.md", "# not a match\n"))
	rig.queueOKTimes("builder", 2)
	rig.queueOK("reviewer")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	rig.requireInputsOfEveryDispatch(t, "builder", 2,
		wildcardRunPaths("FeaturePlan.md", "alpha-Research.md", "beta-Research.md"))
	rig.requireInputsOfEveryDispatch(t, "reviewer", 1,
		wildcardRunPaths("alpha-Research.md", "beta-Research.md"))
}

// TestSession_Start_InputWildcardWithTwoSiblingBlocks_ListsWithoutEitherSchedule
// verifies that a workflow with two sibling blocks expands an Input wildcard
// by listing the run folder, for a row inside the first block and for a row
// after both: neither block's schedule is consulted, and the listing is
// identical on both rows.
func TestSession_Start_InputWildcardWithTwoSiblingBlocks_ListsWithoutEitherSchedule(t *testing.T) {
	rig := newBlockRig(t, "wildcard-sibling-blocks-orch.md", nil,
		"planner", "alpha-worker", "bridge", "beta-worker", "finisher")
	rig.writeRunFiles(t, wildcardStageFiles...)
	rig.queueOK("planner", runFileWrite("AlphaPlan.md", scheduleDoc("Alpha", "Stages", ids("x1", "x2"))))
	rig.queueOKTimes("alpha-worker", 2)
	rig.queueOK("bridge", runFileWrite("BetaPlan.md", scheduleDoc("Beta", "Stages", ids("y1"))))
	rig.queueOK("beta-worker")
	rig.queueOK("finisher")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-sibling-blocks", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	listed := wildcardRunPaths("Stage-A/Plan.md", "Stage-b-2/Plan.md")
	rig.requireInputsOfEveryDispatch(t, "alpha-worker", 2, append(wildcardRunPaths("AlphaPlan.md"), listed...))
	rig.requireInputsOfEveryDispatch(t, "finisher", 1, listed)
}
