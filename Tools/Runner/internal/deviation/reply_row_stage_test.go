package deviation_test

// Tests for the row and stage a routing dispatch reply must name: a valid
// reply resolves to exactly that row and stage; an invalid one is retried with
// the malformed-JSON budget and never returned as a dispatch.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// stagedRoutingTable returns a four-row table (1-based row numbers):
//
//	1 researcher (RESEARCH, non-staged)
//	2 dev        (EXECUTION.[Stage].Test, staged)
//	3 dev        (EXECUTION.[Stage].Implementation, staged)
//	4 reviewer   (REVIEW, non-staged)
//
// The agent "dev" fills two rows, so the agent name alone is ambiguous.
func stagedRoutingTable() domain.RoutingTable {
	return domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, Phase: "RESEARCH", Agent: "researcher", AgentID: "researcher", PhaseParsed: domain.PhaseParsed{Name: "RESEARCH"}},
		{Index: 1, Phase: "EXECUTION.[Stage].Test", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Test", Blocks: []domain.BlockName{"Stage"}}},
		{Index: 2, Phase: "EXECUTION.[Stage].Implementation", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Implementation", Blocks: []domain.BlockName{"Stage"}}},
		{Index: 3, Phase: "REVIEW", Agent: "reviewer", AgentID: "reviewer", PhaseParsed: domain.PhaseParsed{Name: "REVIEW"}},
	}}
}

// schedulesUpTo returns the schedules of the staged rows' block holding the
// iterations "1".."n".
func schedulesUpTo(n int) domain.BlockSchedules {
	sched := domain.BlockSchedule{Block: "Stage"}
	for i := 1; i <= n; i++ {
		sched.Iterations = append(sched.Iterations, domain.ScheduleIteration{ID: domain.IterationID(strconv.Itoa(i))})
	}
	return domain.BlockSchedules{"Stage": sched}
}

// rowStageRequest is a routing request carrying the current block schedules.
func rowStageRequest(schedules domain.BlockSchedules) domain.ConsultationRequest {
	req := validRoutingRequest("Orchestration-abc/Orchestration.md")
	req.Schedules = schedules
	return req
}

// dispatchReply builds a dispatch reply for agent with extra raw JSON members
// (for example `"row":3,"iteration":"2.Implementation"`) between the agent and
// the task.
func dispatchReply(agent, extra string) []byte {
	if extra != "" {
		extra += ","
	}
	return []byte(fmt.Sprintf(`{"action":"dispatch","agent":%q,%s"task_description":"do the thing"}`, agent, extra))
}

func TestConsultRouting_ValidRowAndStage_ResolveToTheNamedRowAndStage(t *testing.T) {
	cases := []struct {
		name      string
		agent     string
		extra     string
		wantIndex int
		wantStage domain.IterationID
	}{
		{"non-staged row", "reviewer", `"row":4`, 3, ""},
		{"first row of a multi-row agent", "dev", `"row":2,"iteration":"1.Test"`, 1, "1"},
		{"second row of a multi-row agent", "dev", `"row":3,"iteration":"2.Implementation"`, 2, "2"},
		{"explicit null iteration on non-staged row", "researcher", `"row":1,"iteration":null`, 0, ""},
		{"absent iteration on non-staged row", "researcher", `"row":1`, 0, ""},
		{"empty iteration on non-staged row is absent", "reviewer", `"row":4,"iteration":""`, 3, ""},
		{"null stage member is not a stage", "reviewer", `"row":4,"stage":null`, 3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRawInvoker{reply: dispatchReply(tc.agent, tc.extra)}
			c := newTestOrchestratorConsultant(fake, stagedRoutingTable())

			instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))

			if err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if instr.Dispatch == nil {
				t.Fatal("want Dispatch instruction, got nil")
			}
			if instr.Dispatch.RowIndex != tc.wantIndex {
				t.Errorf("want RowIndex=%d (the named row), got %d", tc.wantIndex, instr.Dispatch.RowIndex)
			}
			if instr.Dispatch.Iteration != tc.wantStage {
				t.Errorf("want Iteration=%q, got %q", tc.wantStage, instr.Dispatch.Iteration)
			}
		})
	}
}

// The row may arrive as a decimal string; it decodes to the same target.
func TestConsultRouting_LenientRowForm_DecodesToTheSameTarget(t *testing.T) {
	cases := []struct {
		name  string
		extra string
	}{
		{"row as integer", `"row":3,"iteration":"2.Implementation"`},
		{"row as string", `"row":"3","iteration":"2.Implementation"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRawInvoker{reply: dispatchReply("dev", tc.extra)}
			c := newTestOrchestratorConsultant(fake, stagedRoutingTable())

			instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))

			if err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if instr.Dispatch == nil || instr.Dispatch.RowIndex != 2 || instr.Dispatch.Iteration != "2" {
				t.Fatalf("want dispatch at RowIndex=2 Iteration=2, got %+v", instr.Dispatch)
			}
		})
	}
}

// Every invalid reply consumes the whole attempt budget (3 invocations), ends
// with the malformed-JSON class and never yields a dispatch.
func TestConsultRouting_InvalidRowOrStage_RetriedThenMalformedJSON(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		extra string
	}{
		{"missing row", "reviewer", ``},
		{"null row", "reviewer", `"row":null`},
		{"row beyond the table", "reviewer", `"row":9`},
		{"row zero", "reviewer", `"row":0`},
		{"negative row", "reviewer", `"row":-1`},
		{"iteration zero on a staged row", "dev", `"row":3,"iteration":"0.Implementation"`},
		{"iteration with a negative ID", "dev", `"row":3,"iteration":"-1.Implementation"`},
		{"row of another agent", "reviewer", `"row":1`},
		{"staged row without iteration", "dev", `"row":3`},
		{"staged row with null iteration", "dev", `"row":3,"iteration":null`},
		{"staged row with empty iteration", "dev", `"row":3,"iteration":""`},
		{"iteration outside the current set", "dev", `"row":3,"iteration":"4.Implementation"`},
		{"iteration on a non-staged row", "reviewer", `"row":4,"iteration":"1"`},
		{"dash iteration on a non-staged row", "reviewer", `"row":4,"iteration":"-"`},
		{"group of another group", "dev", `"row":3,"iteration":"1.Test"`},
		{"group missing on a grouped row", "dev", `"row":3,"iteration":"2"`},
		{"too many segments", "dev", `"row":3,"iteration":"2.Implementation.x"`},
		{"ID outside the character set", "dev", `"row":3,"iteration":"bad_id.Implementation"`},
		{"row as boolean", "reviewer", `"row":true`},
		{"row as fraction", "reviewer", `"row":1.5`},
		{"row as non-numeric string", "reviewer", `"row":"abc"`},
		{"iteration as integer", "dev", `"row":3,"iteration":2`},
		{"iteration as boolean", "dev", `"row":3,"iteration":true`},
		{"iteration as object", "dev", `"row":3,"iteration":{"id":"2"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := sequencedReply{reply: dispatchReply(tc.agent, tc.extra)}
			invoker := &sequencedRawInvoker{replies: []sequencedReply{bad, bad, bad, bad}}
			c := newTestOrchestratorConsultant(invoker, stagedRoutingTable())

			instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))

			assertConsultationError(t, err, domain.ConsultFailMalformedJSON)
			if instr.Dispatch != nil {
				t.Errorf("want no dispatch for an invalid reply, got %+v", instr.Dispatch)
			}
			if invoker.pos != 3 {
				t.Errorf("want 3 InvokeRaw calls (same budget as malformed JSON), got %d", invoker.pos)
			}
		})
	}
}

func TestConsultRouting_InvalidRowThenValidReply_SucceedsOnSecondAttempt(t *testing.T) {
	invoker := &sequencedRawInvoker{replies: []sequencedReply{
		{reply: dispatchReply("dev", `"row":3`)}, // staged row, no iteration
		{reply: dispatchReply("dev", `"row":3,"iteration":"2.Implementation"`)},
	}}
	c := newTestOrchestratorConsultant(invoker, stagedRoutingTable())

	instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))

	if err != nil {
		t.Fatalf("want success on the retry, got %v", err)
	}
	if instr.Dispatch == nil || instr.Dispatch.RowIndex != 2 || instr.Dispatch.Iteration != "2" {
		t.Fatalf("want dispatch at RowIndex=2 Iteration=2, got %+v", instr.Dispatch)
	}
	if invoker.pos != 2 {
		t.Errorf("want 2 InvokeRaw calls, got %d", invoker.pos)
	}
}

// Invalid replies and malformed JSON share one budget of three attempts.
func TestConsultRouting_InvalidAndMalformedReplies_ShareOneRetryBudget(t *testing.T) {
	invoker := &sequencedRawInvoker{replies: []sequencedReply{
		malformedSchemaReply(),
		{reply: dispatchReply("reviewer", ``)},
		{reply: dispatchReply("dev", `"row":3`)},
		{reply: dispatchReply("dev", `"row":3,"iteration":"2.Implementation"`)}, // would be valid, but the budget is spent
	}}
	c := newTestOrchestratorConsultant(invoker, stagedRoutingTable())

	instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))

	assertConsultationError(t, err, domain.ConsultFailMalformedJSON)
	if instr.Dispatch != nil {
		t.Errorf("want no dispatch once the budget is spent, got %+v", instr.Dispatch)
	}
	if invoker.pos != 3 {
		t.Errorf("want 3 InvokeRaw calls, got %d", invoker.pos)
	}
}

// The stage set that counts is the one on the request: a stage added mid-run
// is accepted, and the same reply is invalid against the earlier set.
func TestConsultRouting_StageValidatedAgainstTheRequestsCurrentStageSet(t *testing.T) {
	reply := dispatchReply("dev", `"row":3,"iteration":"3.Implementation"`)

	grown := &fakeRawInvoker{reply: reply}
	instr, err := newTestOrchestratorConsultant(grown, stagedRoutingTable()).
		ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))
	if err != nil {
		t.Fatalf("want stage 3 accepted when the request's stage set holds it, got %v", err)
	}
	if instr.Dispatch == nil || instr.Dispatch.Iteration != "3" {
		t.Fatalf("want dispatch at Iteration=3, got %+v", instr.Dispatch)
	}

	stale := &sequencedRawInvoker{replies: []sequencedReply{{reply: reply}, {reply: reply}, {reply: reply}}}
	_, err = newTestOrchestratorConsultant(stale, stagedRoutingTable()).
		ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(2)))
	assertConsultationError(t, err, domain.ConsultFailMalformedJSON)
}

func TestConsultRouting_StagedRowWithoutAnyStageSet_IsInvalid(t *testing.T) {
	reply := sequencedReply{reply: dispatchReply("dev", `"row":3,"iteration":"1.Implementation"`)}
	invoker := &sequencedRawInvoker{replies: []sequencedReply{reply, reply, reply}}
	c := newTestOrchestratorConsultant(invoker, stagedRoutingTable())

	instr, err := c.ConsultRouting(context.Background(), rowStageRequest(nil))

	assertConsultationError(t, err, domain.ConsultFailMalformedJSON)
	if instr.Dispatch != nil {
		t.Errorf("want no dispatch without a stage set, got %+v", instr.Dispatch)
	}
}

// A stop reply needs neither row nor stage.
func TestConsultRouting_StopReplyNeedsNoRowOrStage(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, stagedRoutingTable())

	instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(1)))

	if err != nil {
		t.Fatalf("want no error for a stop reply, got %v", err)
	}
	if instr.Stop == nil {
		t.Fatal("want Stop instruction, got nil")
	}
}

// An unknown agent keeps its own failure class and is not retried, even when
// the reply carries a row.
func TestConsultRouting_UnknownAgentWithRow_StaysUnknownAgentWithoutRetry(t *testing.T) {
	bad := sequencedReply{reply: dispatchReply("nobody", `"row":1`)}
	invoker := &sequencedRawInvoker{replies: []sequencedReply{bad, bad, bad}}
	c := newTestOrchestratorConsultant(invoker, stagedRoutingTable())

	_, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(1)))

	assertConsultationError(t, err, domain.ConsultFailUnknownAgent)
	if invoker.pos != 1 {
		t.Errorf("want 1 InvokeRaw call (no retry), got %d", invoker.pos)
	}
}

// An unknown agent without any row stays an unknown-agent failure (no retry):
// agent and task checks come before row validation.
func TestConsultRouting_UnknownAgentWithoutRow_StaysUnknownAgentWithoutRetry(t *testing.T) {
	fake := &fakeRawInvoker{reply: dispatchReply("not-in-table", ``)}
	c := newTestOrchestratorConsultant(fake, stagedRoutingTable())

	_, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3)))

	assertConsultationError(t, err, domain.ConsultFailUnknownAgent)
}

// The stage set is session context for validation only; it must not appear in
// the payload sent to the orchestrator.
func TestConsultRouting_StageSetIsNotSerialisedOntoTheWire(t *testing.T) {
	fake := &fakeRawInvoker{reply: dispatchReply("dev", `"row":3,"iteration":"2.Implementation"`)}
	c := newTestOrchestratorConsultant(fake, stagedRoutingTable())

	if _, err := c.ConsultRouting(context.Background(), rowStageRequest(schedulesUpTo(3))); err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(fake.sent, &raw); err != nil {
		t.Fatalf("want valid JSON wire payload, got %v", err)
	}
	for key := range raw {
		if k := strings.ToLower(key); strings.Contains(k, "stage") || strings.Contains(k, "schedule") {
			t.Errorf("want no stage-set field in the wire payload, found key %q", key)
		}
	}
}
