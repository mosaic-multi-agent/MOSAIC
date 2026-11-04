package session_test

// What a commit-class agent receives when ITERATION_END fires it: the ended
// iteration's own artifacts that exist in the run folder, run-scoped, in table
// order, and a task description naming the block and the iteration.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_Start_CommitOnIterationEnd_StageBlock_CarriesOnlyExistingIterationFiles(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, map[string]string{
		"test-writer":  "Stage-2/PlanProgress.md",
		"tests-review": "Stage-2/tests-review-tdd.md",
	})
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	// implementation-review.md is a fill-in entry whose file does not exist;
	// ContractsDesign.md exists but names no iteration.
	requireSequence(t, "commit input_artifacts", req.InputArtifacts, wildcardRunPaths(
		"Stage-2/Plan.md", "Stage-2/PlanProgress.md", "Stage-2/tests-review-tdd.md"))
}

func TestSession_Start_CommitOnIterationEnd_StageBlock_AllFilesExist_FollowTableOrderWithoutShared(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, map[string]string{
		"test-writer":  "Stage-2/PlanProgress.md",
		"tests-review": "Stage-2/tests-review-tdd.md",
		"impl-review":  "Stage-2/implementation-review.md",
	})
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	requireSequence(t, "commit input_artifacts", req.InputArtifacts, wildcardRunPaths(
		"Stage-2/Plan.md", "Stage-2/PlanProgress.md", "Stage-2/tests-review-tdd.md", "Stage-2/implementation-review.md"))
}

func TestSession_Start_CommitOnIterationEnd_TaskDescriptionNamesBlockAndIterationWithoutGroup(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	queueStageRows(rig, nil)
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	requireDescriptionNames(t, req, "Stage 2", infraCommitAgent)
	// The last row of the iteration ran in the Implementation group; the
	// recorded value "2.Implementation" is not what the commit is told.
	if strings.Contains(req.TaskDescription, "Implementation") {
		t.Errorf("want the group suffix dropped from the task description, got %q", req.TaskDescription)
	}
}

func TestSession_Start_CommitOnIterationEnd_CarriesNoOutputsAndNoHITL(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, nil)
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	requireTriggerDispatchShape(t, req)
	if len(req.InputArtifacts) == 0 {
		t.Error("want the existing iteration file as input, got none")
	}
}

func TestSession_Start_CommitOnIterationEnd_RecordsSentInputsOnTheInfrastructureRow(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, map[string]string{"test-writer": "Stage-2/PlanProgress.md"})
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	rows := infraRowsOf(rig.store, infraCommitAgent)
	if len(rows) != 1 {
		t.Fatalf("want 1 commit infrastructure row, got %d", len(rows))
	}
	want := dispatchPath("Stage-2/Plan.md") + ", " + dispatchPath("Stage-2/PlanProgress.md")
	if rows[0].Inputs != want {
		t.Errorf("want row Inputs %q, got %q", want, rows[0].Inputs)
	}
}

func TestSession_Start_CommitOnIterationEnd_NoIterationFileExists_StillDispatchedWithEmptyInputs(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	queueStageRows(rig, nil)
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	requireTriggerDispatchShape(t, req)
	if len(req.InputArtifacts) != 0 {
		t.Errorf("want no input_artifacts when none of the iteration's files exist, got %q", req.InputArtifacts)
	}
	requireDescriptionNames(t, req, "Stage 2")
	rows := infraRowsOf(rig.store, infraCommitAgent)
	if len(rows) != 1 || rows[0].Inputs != "" {
		t.Errorf("want one commit row with empty Inputs, got %+v", rows)
	}
}

func TestSession_Start_CommitOnIterationEnd_NonStageBlock_CarriesEachIterationsOwnFiles(t *testing.T) {
	rig := newProductRig(t, "iteration-end-product-block-orch.md", infraCommitAgent, "A", "B")
	rig.queueCommitSetup()
	queueProductIteration(rig, "A")
	rig.queueOK(infraCommitAgent)
	queueProductIteration(rig, "B")
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("product-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	reqs := rig.requestsOf(infraCommitAgent)
	if len(reqs) != 2 {
		t.Fatalf("want 2 commit dispatches (one per iteration), got %d", len(reqs))
	}
	// ProductPlan.md exists but names no iteration; iteration B's commit does
	// not receive iteration A's files, which also exist by then.
	requireSequence(t, "commit of A", reqs[0].InputArtifacts, wildcardRunPaths("built-A.md", "checked-A.md"))
	requireSequence(t, "commit of B", reqs[1].InputArtifacts, wildcardRunPaths("built-B.md", "checked-B.md"))
	requireDescriptionNames(t, reqs[0], "Product A")
	requireDescriptionNames(t, reqs[1], "Product B")
}

func TestSession_Start_CommitOnIterationEnd_FillInWildcard_CarriesOnlyTheEndedIterationsMatches(t *testing.T) {
	rig := newBlockRig(t, "infra-contents-topic-wildcard-orch.md", nil,
		"planner", "researcher", "synthesizer", infraCommitAgent)
	rig.queueCommitSetup()
	rig.queueOK("planner", runFileWrite("TopicPlan.md", scheduleDoc("Topic", "Topics", ids("Pricing", "Support"))))
	rig.writeRunFiles(t,
		"A-Pricing-Findings.md", "B-Pricing-Findings.md", "A-Support-Findings.md", "X-ProductMap.md")
	rig.queueOK("researcher", runFileWrite("notes-Pricing.md", "# notes\n"))
	rig.queueOK("synthesizer")
	rig.queueOK(infraCommitAgent)
	rig.queueOK("researcher", runFileWrite("notes-Support.md", "# notes\n"))
	rig.queueOK("synthesizer")
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("topic-wildcard", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	reqs := rig.requestsOf(infraCommitAgent)
	if len(reqs) != 2 {
		t.Fatalf("want 2 commit dispatches (one per topic), got %d", len(reqs))
	}
	// The wildcard entry contributes the existing matches of its resolved
	// pattern in lexical order, at its table position; *-ProductMap.md has no
	// fill-in, so X-ProductMap.md is never sent.
	requireSequence(t, "commit of Pricing", reqs[0].InputArtifacts, wildcardRunPaths(
		"notes-Pricing.md", "A-Pricing-Findings.md", "B-Pricing-Findings.md"))
	requireSequence(t, "commit of Support", reqs[1].InputArtifacts, wildcardRunPaths(
		"notes-Support.md", "A-Support-Findings.md"))
}
