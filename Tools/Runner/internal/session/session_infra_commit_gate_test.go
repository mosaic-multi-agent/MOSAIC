package session_test

// The commit class is gated by the run's commits setting, as the checkpoint
// class is gated by checkpoints: with commits disabled no commit-class agent is
// dispatched by trigger evaluation, whatever triggers it declares or an
// infrastructure_overrides entry gives it, and no Execution Log row exists for
// it. With commits enabled it fires as before. On resume the setting recorded in
// the artifact decides. Review-class agents are not gated.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

const (
	commitGateFixture  = "iteration-end-product-block-orch.md"
	commitGateWorkflow = "product-block"
)

// newCommitGateRig is the Product block rig over fixture with iterations A and
// B scheduled, the iterations' builder and checker replies queued, and the
// commit agent's definition present.
func newCommitGateRig(t *testing.T, fixture string) *blockRig {
	t.Helper()
	rig := newProductRig(t, fixture, infraCommitAgent, "A", "B")
	queueProductIteration(rig, "A")
	queueProductIteration(rig, "B")
	return rig
}

// requireIterationsRan asserts the builder and checker ran once per iteration.
func requireIterationsRan(t *testing.T, rig *blockRig, iterations int) {
	t.Helper()
	for _, agent := range []string{"builder", "checker"} {
		if n := rig.invocationsOf(agent); n != iterations {
			t.Errorf("want %d dispatches of %s, got %d", iterations, agent, n)
		}
	}
}

// requireCommitTriggerDispatches asserts the commit agent was dispatched by
// trigger evaluation want times, with a row for each and no commit setup.
func requireCommitTriggerDispatches(t *testing.T, rig *blockRig, want int, setupExpected bool) {
	t.Helper()
	if n := len(rig.requestsOf(infraCommitAgent)); n != want {
		t.Errorf("want %d trigger dispatches of %s, got %d", want, infraCommitAgent, n)
	}
	if rows := infraRowsOf(rig.store, infraCommitAgent); len(rows) != want {
		t.Errorf("want %d Execution Log rows of %s, got %d", want, infraCommitAgent, len(rows))
	}
	setups := 0
	for _, inv := range rig.adapter.Invocations() {
		if isCommitSetup(inv.Request) {
			setups++
		}
	}
	if (setups == 1) != setupExpected || setups > 1 {
		t.Errorf("want commit setup dispatched: %v, got %d setup dispatches", setupExpected, setups)
	}
}

// ===== New run =====

func TestSession_Start_CommitsDisabled_IterationEndCommitAgentNotDispatched(t *testing.T) {
	rig := newCommitGateRig(t, commitGateFixture)

	got, err := rig.ses.Start(context.Background(), rig.config(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireNoCommitActivity(t, rig)
	if n := rig.invocationsOf(infraCommitAgent); n != 0 {
		t.Errorf("want no dispatch of %s with commits disabled, got %d", infraCommitAgent, n)
	}
	requireIterationsRan(t, rig, 2)
}

func TestSession_Start_CommitsEnabled_IterationEndCommitAgentDispatchedAtEachIterationEnd(t *testing.T) {
	rig := newCommitGateRig(t, commitGateFixture)
	rig.queueCommitSetup()
	rig.queueOKTimes(infraCommitAgent, 2)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireCommitTriggerDispatches(t, rig, 2, true)
	requireIterationsRan(t, rig, 2)
}

func TestSession_Start_CommitsDisabled_ReviewAgentStillDispatchedAtIterationEnd(t *testing.T) {
	rig := newBlockRig(t, "commit-gate-review-orch.md", nil, "planner", "builder", "checker", infraCommitAgent, infraReviewAgent)
	rig.queueOK("planner", runFileWrite("ProductPlan.md", scheduleDoc("Product", "Products", ids("A"))))
	queueProductIteration(rig, "A")
	rig.queueOK(infraReviewAgent)
	rig.consult.queueStop("review done")

	got, err := rig.ses.Start(context.Background(), rig.config(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf(infraReviewAgent); n != 1 {
		t.Errorf("want the review agent dispatched at the iteration end, got %d dispatches", n)
	}
	requireNoCommitActivity(t, rig)
	if n := rig.invocationsOf(infraCommitAgent); n != 0 {
		t.Errorf("want no dispatch of %s with commits disabled, got %d", infraCommitAgent, n)
	}
}

func TestSession_Start_CommitsDisabled_CheckpointAgentsFireAndEveryCommitAgentIsSkipped(t *testing.T) {
	ses, f, store, orchPath := newTwoGatedClassesSession(t)
	for _, agent := range []string{"agent-a", "agent-b", "checkpoint-manager-git"} {
		queueCond(f, agent, domain.StatusSUCCESS)
	}
	queueCond(f, "checkpoint-manager-git", domain.StatusSUCCESS)
	cfg := baseLinearConfig(orchPath)
	cfg.Checkpoints = true
	cfg.InfraClassSelections = map[string]string{
		"checkpoint": "checkpoint-manager-git",
		"commit":     "commit-manager-git",
	}

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	agents := invokedAgents(f)
	if countAgent(agents, "checkpoint-manager-git") == 0 {
		t.Errorf("want the checkpoint agent dispatched (checkpoints enabled), got %v", agents)
	}
	for _, commitAgent := range []string{"commit-manager-git", "commit-manager-alt"} {
		if n := countAgent(agents, commitAgent); n != 0 {
			t.Errorf("want no dispatch of %s with commits disabled, got %d: %v", commitAgent, n, agents)
		}
		if rows := infraRowsOf(store, commitAgent); len(rows) != 0 {
			t.Errorf("want no Execution Log row of %s, got %+v", commitAgent, rows)
		}
	}
}

// ===== Resume: the recorded commits setting decides =====

// resumeCommitGateRig is the gate rig resumed from a run that completed
// nothing yet, recording the given commits setting and, when overrideTriggers
// is not empty, an infrastructure_overrides entry giving the commit agent those
// triggers.
func resumeCommitGateRig(t *testing.T, fixture string, commitsRecorded bool, overrideTriggers ...string) *blockRig {
	t.Helper()
	rig := newCommitGateRig(t, fixture)
	rig.resumeFrom(commitGateWorkflow, domain.ExecutionModeAuto)
	if commitsRecorded {
		rig.recordCommitsEnabled()
		rig.queueOKTimes(infraCommitAgent, 2)
	}
	if len(overrideTriggers) > 0 {
		rig.store.state.InfrastructureOverrides = []domain.InfrastructureOverride{
			overrideOf(infraCommitAgent, overrideTriggers...),
		}
	}
	return rig
}

func TestSession_Start_ResumeRecordingCommitsDisabled_CommitAgentNotDispatched(t *testing.T) {
	rig := resumeCommitGateRig(t, commitGateFixture, false)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireNoCommitActivity(t, rig)
	if n := rig.invocationsOf(infraCommitAgent); n != 0 {
		t.Errorf("want no dispatch of %s on a resume recording commits disabled, got %d", infraCommitAgent, n)
	}
	requireIterationsRan(t, rig, 2)
}

func TestSession_Start_ResumeRecordingCommitsEnabledWithBranch_CommitAgentDispatchedWithoutSetup(t *testing.T) {
	rig := resumeCommitGateRig(t, commitGateFixture, true)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireCommitTriggerDispatches(t, rig, 2, false)
	requireIterationsRan(t, rig, 2)
}

// ===== Triggers given by an infrastructure_overrides entry =====

func TestSession_Start_ResumeOverrideGivesCommitAgentIterationEnd_CommitsDisabled_NotDispatched(t *testing.T) {
	rig := resumeCommitGateRig(t, "commit-gate-manual-orch.md", false, domain.InfraTriggerIterationEnd)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireNoCommitActivity(t, rig)
	if n := rig.invocationsOf(infraCommitAgent); n != 0 {
		t.Errorf("want no dispatch of %s despite its overridden ITERATION_END, got %d", infraCommitAgent, n)
	}
	requireIterationsRan(t, rig, 2)
}

func TestSession_Start_ResumeOverrideGivesCommitAgentIterationEnd_CommitsEnabled_DispatchedAtEachIterationEnd(t *testing.T) {
	rig := resumeCommitGateRig(t, "commit-gate-manual-orch.md", true, domain.InfraTriggerIterationEnd)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireCommitTriggerDispatches(t, rig, 2, false)
	requireIterationsRan(t, rig, 2)
}
