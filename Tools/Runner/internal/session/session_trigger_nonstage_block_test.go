package session_test

// Tests that ITERATION_END fires after every iteration of a block whatever its
// name: the block here is Product, not Stage.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Start_IterationEnd_BlockNotNamedStage_FiresAfterEachIteration
// verifies that a commit agent on ITERATION_END is dispatched once after the
// last row of each iteration of a block named Product, and not after the
// intermediate row.
func TestSession_Start_IterationEnd_BlockNotNamedStage_FiresAfterEachIteration(t *testing.T) {
	rig := newBlockRig(t, "iteration-end-product-block-orch.md", nil,
		"planner", "builder", "checker", "commit-manager-git")
	rig.queueOK("planner", runFileWrite("ProductPlan.md", scheduleDoc("Product", "Products", ids("A", "B"))))
	rig.queueOKTimes("builder", 2)
	rig.queueOKTimes("checker", 2)
	rig.queueOKTimes("commit-manager-git", 2)

	got, err := rig.ses.Start(context.Background(), rig.config("product-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAgentSequence(t, invokedAgents(rig.adapter), []string{
		"planner",
		"builder", "checker", "commit-manager-git",
		"builder", "checker", "commit-manager-git",
	})
}

// TestSession_Start_IterationEnd_BlockNotNamedStage_SingleIterationFiresOnce
// verifies the boundary of a one-iteration block of that name: the commit
// agent fires once, after the checker.
func TestSession_Start_IterationEnd_BlockNotNamedStage_SingleIterationFiresOnce(t *testing.T) {
	rig := newBlockRig(t, "iteration-end-product-block-orch.md", nil,
		"planner", "builder", "checker", "commit-manager-git")
	rig.queueOK("planner", runFileWrite("ProductPlan.md", scheduleDoc("Product", "Products", ids("only"))))
	rig.queueOK("builder")
	rig.queueOK("checker")
	rig.queueOK("commit-manager-git")

	got, err := rig.ses.Start(context.Background(), rig.config("product-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAgentSequence(t, invokedAgents(rig.adapter),
		[]string{"planner", "builder", "checker", "commit-manager-git"})
}
