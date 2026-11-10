package session_test

// Mode 1 sessions driven by the real orchestrator consultant over a scripted
// raw transport: the orchestrator's replies are JSON in the wire form of the
// consultant contract (`row`, `iteration` in the run artifact's value form,
// bare `agent`). A reply naming a block row and its iteration dispatches that
// row for that iteration; a reply naming a qualified row by its bare agent id
// dispatches that row; the former `stage` member is not accepted.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// scriptedRawInvoker implements domain.RawInvoker with raw replies consumed in
// FIFO order; once they are exhausted it fails like a dead transport.
type scriptedRawInvoker struct {
	replies []string
	calls   int
}

func (s *scriptedRawInvoker) InvokeRaw(_ context.Context, _ domain.AgentReference, _ []byte) ([]byte, error) {
	s.calls++
	if s.calls > len(s.replies) {
		return nil, errors.New("scriptedRawInvoker: no reply left")
	}
	return []byte(s.replies[s.calls-1]), nil
}

// writeNonNumericConsultRowStagePlan replaces the rig's Plan.md with two
// iterations whose IDs are not numbers: "A" (approach TDD) and "b-2"
// (approach Implementation-Only).
func writeNonNumericConsultRowStagePlan(t *testing.T, rig *rowStageRig) {
	t.Helper()
	const planContent = `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL | Approach |
|-------|------|------|------------|:----:|----------|
| A | Alpha | First stage | - | FALSE | TDD |
| b-2 | Beta | Second stage | A | FALSE | Implementation-Only |
`
	plan := filepath.Join(filepath.Dir(rig.cfg.OrchestratorFilePath), "Plan.md")
	if err := os.WriteFile(plan, []byte(planContent), 0600); err != nil {
		t.Fatalf("write Plan.md: %v", err)
	}
}

// ===== Block rows with non-numeric iteration IDs =====

func TestSession_Orchestrated_ReplyNamingBlockRowAndNonNumericIteration_DispatchesThatRowForThatIteration(t *testing.T) {
	raw := &scriptedRawInvoker{replies: []string{
		`{"action":"dispatch","agent":"test-writer","row":3,"iteration":"A.Test","task_description":"write the tests"}`,
		`{"action":"dispatch","agent":"implementer","row":4,"iteration":"b-2.Implementation","task_description":"implement it"}`,
		`{"action":"stop","reason":"done"}`,
	}}
	seed := rowStageSeed("plan-review", 2, "PLANNING", "")
	rig := newRowStageRig(t, &deviation.OrchestratorConsultant{Invoker: raw}, domain.ExecutionModeOrchestrated, seed)
	writeNonNumericConsultRowStagePlan(t, rig)
	queueSuccess(rig.f, "test-writer", "test-writer#3")
	queueSuccess(rig.f, "implementer", "implementer#4")

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	steps := workflowSteps(rig.store)
	if len(steps) != 2 {
		t.Fatalf("want 2 workflow steps, got %d: %+v", len(steps), steps)
	}
	requireStepAt(t, steps[0], "EXECUTION", 3, "A.Test")
	requireStepAt(t, steps[1], "EXECUTION", 4, "b-2.Implementation")
}

// The iteration is validated against the row's group: the value of another
// group, or without a group, is not dispatched and consumes the retry budget of
// malformed replies.
func TestSession_Orchestrated_ReplyWithWrongOrMissingGroupSegment_IsNotDispatched(t *testing.T) {
	cases := map[string]string{
		"group of another row": `"iteration":"A.Implementation"`,
		"group missing":        `"iteration":"A"`,
	}
	for name, member := range cases {
		t.Run(name, func(t *testing.T) {
			reply := `{"action":"dispatch","agent":"test-writer","row":3,` + member + `,"task_description":"write the tests"}`
			raw := &scriptedRawInvoker{replies: []string{reply, reply, reply}}
			seed := rowStageSeed("plan-review", 2, "PLANNING", "")
			rig := newRowStageRig(t, &deviation.OrchestratorConsultant{Invoker: raw}, domain.ExecutionModeOrchestrated, seed)
			writeNonNumericConsultRowStagePlan(t, rig)

			got, err := rig.ses.Start(context.Background(), rig.cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := len(rig.f.Invocations()); n != 0 {
				t.Errorf("want no dispatch for an invalid iteration, got %d", n)
			}
			if raw.calls != 3 {
				t.Errorf("want the 3 attempts of the malformed-reply budget, got %d", raw.calls)
			}
		})
	}
}

// The former `stage` member is not accepted, even when it names a valid
// iteration of the row's schedule in the old integer form.
func TestSession_Orchestrated_ReplyWithFormerStageMember_IsNotDispatched(t *testing.T) {
	reply := `{"action":"dispatch","agent":"test-writer","row":3,"stage":"A","task_description":"write the tests"}`
	raw := &scriptedRawInvoker{replies: []string{reply, reply, reply}}
	seed := rowStageSeed("plan-review", 2, "PLANNING", "")
	rig := newRowStageRig(t, &deviation.OrchestratorConsultant{Invoker: raw}, domain.ExecutionModeOrchestrated, seed)
	writeNonNumericConsultRowStagePlan(t, rig)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := len(rig.f.Invocations()); n != 0 {
		t.Errorf("want no dispatch for a reply carrying the former stage member, got %d", n)
	}
}

// ===== Mode-qualified rows named by their bare agent id =====

func TestSession_Orchestrated_ReplyNamingQualifiedRowsByBareAgentID_DispatchesTheNamedRows(t *testing.T) {
	raw := &scriptedRawInvoker{replies: []string{
		`{"action":"dispatch","agent":"planner","row":1,"task_description":"plan it"}`,
		`{"action":"dispatch","agent":"build-review","row":2,"task_description":"review the tests"}`,
		`{"action":"dispatch","agent":"implementer","row":3,"task_description":"implement it"}`,
		`{"action":"dispatch","agent":"build-review","row":4,"iteration":null,"task_description":"review the implementation"}`,
		`{"action":"stop","reason":"done"}`,
	}}
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "qualified-rows-orch.md")
	for _, agent := range []string{"planner", "implementer", "build-review"} {
		writeAgentFile(t, dir, agent)
	}
	f := harness.NewMockAdapter()
	store := &memStore{}
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    store,
		Routing:  &deviation.OrchestratorConsultant{Invoker: raw},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	queueSuccess(f, "planner", "planner#1")
	queueSuccess(f, "build-review", "build-review#2")
	queueSuccess(f, "implementer", "implementer#3")
	queueSuccess(f, "build-review", "build-review#4")
	cfg := domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           qualifiedWorkflowID,
		Task:                 "test task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	}

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	invs := f.Invocations()
	if len(invs) != 4 {
		t.Fatalf("want 4 dispatches, got %d", len(invs))
	}
	for i, wantAgent := range []string{"planner", "build-review", "implementer", "build-review"} {
		if invs[i].Agent.Identifier != wantAgent {
			t.Errorf("invocation %d: want agent %q, got %q", i, wantAgent, invs[i].Agent.Identifier)
		}
	}
	requireRecordedRows(t, store, 1, 2, 3, 4)
	if invs[3].Request.TaskDescription != "review the implementation" {
		t.Errorf("want the consultant's task description for the second qualified row, got %q",
			invs[3].Request.TaskDescription)
	}
}

// A reply naming the whole qualified cell as the agent is an agent-row
// mismatch: it is retried and never dispatched.
func TestSession_Orchestrated_ReplyNamingWholeQualifiedCell_IsNotDispatched(t *testing.T) {
	reply := `{"action":"dispatch","agent":"build-review(tests)","row":2,"task_description":"review the tests"}`
	raw := &scriptedRawInvoker{replies: []string{reply, reply, reply}}
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "qualified-rows-orch.md")
	for _, agent := range []string{"planner", "implementer", "build-review"} {
		writeAgentFile(t, dir, agent)
	}
	f := harness.NewMockAdapter()
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Routing:  &deviation.OrchestratorConsultant{Invoker: raw},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	cfg := domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           qualifiedWorkflowID,
		Task:                 "test task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	}

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := len(f.Invocations()); n != 0 {
		t.Errorf("want no dispatch for the whole qualified cell, got %d", n)
	}
	if raw.calls != 3 {
		t.Errorf("want the 3 attempts of the malformed-reply budget (agent-row mismatch is retried), got %d", raw.calls)
	}
}
