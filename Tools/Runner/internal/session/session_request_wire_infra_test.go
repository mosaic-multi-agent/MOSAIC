package session_test

// The serialized form of the requests a session sends to infrastructure
// agents: every trigger dispatch of every class the trigger evaluator
// dispatches (checkpoint, commit, review), on each trigger it supports, and the
// commit setup dispatch, carries input_artifacts and output_artifacts as JSON
// arrays, `[]` when empty, never null.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// requireTriggerRunWire asserts the serialized form of every dispatch of agent
// in rig, and that there were want of them.
func requireTriggerRunWire(t *testing.T, rig *blockRig, agent string, want int) {
	t.Helper()
	reqs := rig.requestsOf(agent)
	if len(reqs) != want {
		t.Fatalf("want %d dispatches of %s, got %d", want, agent, len(reqs))
	}
	for i, req := range reqs {
		requireWireArtifactArrays(t, agent+" dispatch "+string(rune('1'+i)), req)
	}
}

func TestSession_CheckpointTriggerDispatch_SerializesBothListsAsArrays(t *testing.T) {
	const agent = "checkpoint-manager-git"
	cases := []struct {
		trigger, fixture string
		dispatches       int
	}{
		{"ITERATION_END", "infra-contents-checkpoint-orch.md", 1},
		{"PHASE_END", "infra-contents-phase-end-orch.md", 2},
	}
	for _, tc := range cases {
		t.Run(tc.trigger, func(t *testing.T) {
			rig := runProductWithInfra(t, tc.fixture, agent, tc.dispatches)

			requireTriggerRunWire(t, rig, agent, tc.dispatches)
		})
	}
	t.Run("INVOCATION_INTERVAL", func(t *testing.T) {
		ses, f, _, orchPath := newIntervalAgentSession(t)
		for _, name := range []string{"agent-a", "agent-b"} {
			queueWireSuccess(f, name)
		}
		queueRepeated(f, agent, 2, statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))
		cfg := baseLinearConfig(orchPath)
		cfg.Checkpoints = true

		got, err := ses.Start(context.Background(), cfg)

		requireRunStatus(t, got, err, domain.RunCompleted)
		requireWireArraysOfAgent(t, f.Invocations(), agent)
	})
}

func TestSession_CommitTriggerDispatch_SerializesBothListsAsArrays(t *testing.T) {
	cases := []struct {
		trigger, fixture string
	}{
		{"ITERATION_END", "infra-contents-commit-two-triggers-orch.md"},
		{"INVOCATION_INTERVAL", "infra-contents-commit-interval-orch.md"},
	}
	for _, tc := range cases {
		t.Run(tc.trigger, func(t *testing.T) {
			rig := runProductWithInfra(t, tc.fixture, infraCommitAgent, 1)

			requireTriggerRunWire(t, rig, infraCommitAgent, 1)
		})
	}
}

func TestSession_ReviewTriggerDispatch_SerializesBothListsAsArrays(t *testing.T) {
	cases := []struct {
		trigger, fixture string
	}{
		{"ITERATION_END", "infra-contents-review-orch.md"},
		{"INVOCATION_INTERVAL", "infra-contents-review-interval-orch.md"},
		{"PHASE_END", "request-wire-review-phase-end-orch.md"},
	}
	for _, tc := range cases {
		t.Run(tc.trigger, func(t *testing.T) {
			rig := runProductWithReview(t, tc.fixture)

			requireWireArraysOfAgent(t, rig.adapter.Invocations(), infraReviewAgent)
		})
	}
}

// The commit setup dispatch of a run with commits enabled.
func TestSession_CommitSetupDispatch_SerializesBothListsAsArrays(t *testing.T) {
	ses, f, _, orchPath := newCommitSession(t)
	f.Queue(infraCommitAgent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: infraCommitAgent + "#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "branch ready [branch:mosaic/run/test-run-id]",
	}})
	queueWireSuccess(f, "agent-a", "agent-b")
	cfg := baseCommitConfig(orchPath)
	cfg.RunID = "test-run-id"

	got, err := ses.Start(context.Background(), cfg)

	if err != nil || got.Status == domain.RunRefused {
		t.Fatalf("want the run to start past the commit setup, got status %q, err %v", got.Status, err)
	}
	invs := f.Invocations()
	if len(invs) == 0 || !isCommitSetup(invs[0].Request) {
		t.Fatalf("want the commit setup as the first dispatch, got %+v", invokedAgents(f))
	}
	requireWireArtifactArrays(t, "commit setup", invs[0].Request)
}
