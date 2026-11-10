package deviation_test

// Tests for routing dispatch replies over a table that repeats one agent
// through mode qualifiers: the reply names the bare agent id, the row decides
// which qualified row is meant, and a whole qualified cell is an agent-row
// mismatch (retried), never an unknown agent.

import (
	"context"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
)

// qualifiedRoutingTable returns a four-row table (1-based row numbers):
//
//	1 planner                (outside every block)
//	2 build-review(tests)    (outside every block)
//	3 implementer            (outside every block)
//	4 build-review(impl)     (block Stage, group Implementation)
func qualifiedRoutingTable() domain.RoutingTable {
	return domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, Phase: "PLANNING", Agent: "planner", AgentID: "planner",
			PhaseParsed: domain.PhaseParsed{Name: "PLANNING"}},
		{Index: 1, Phase: "PLANNING", Agent: "build-review(tests)", AgentID: "build-review", Qualifier: "tests",
			PhaseParsed: domain.PhaseParsed{Name: "PLANNING"}},
		{Index: 2, Phase: "EXECUTION", Agent: "implementer", AgentID: "implementer",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION"}},
		{Index: 3, Phase: "REVIEW", Agent: "build-review(impl)", AgentID: "build-review", Qualifier: "impl",
			PhaseParsed: domain.PhaseParsed{Name: "REVIEW", IsStaged: true, Group: "Implementation",
				Blocks: []domain.BlockName{"Stage"}}},
	}}
}

func qualifiedSchedules() domain.BlockSchedules {
	return domain.BlockSchedules{"Stage": scheduleOf("Stage", "1", "A")}
}

// consultQualified runs one consultation over the qualified table and reports
// the dispatch, the invocations used and the error.
func consultQualified(t *testing.T, agent, extra string) (*domain.DispatchInstruction, int, error) {
	t.Helper()
	reply := sequencedReply{reply: dispatchReply(agent, extra)}
	invoker := &sequencedRawInvoker{replies: []sequencedReply{reply, reply, reply, reply}}
	c := newTestOrchestratorConsultant(invoker, qualifiedRoutingTable())

	instr, err := c.ConsultRouting(context.Background(), rowStageRequest(qualifiedSchedules()))

	return instr.Dispatch, invoker.pos, err
}

func TestConsultRouting_QualifiedRow_BareAgentIDIsAcceptedAndDispatched(t *testing.T) {
	cases := []struct {
		name      string
		extra     string
		wantIndex int
		wantID    domain.IterationID
	}{
		{"qualified row outside every block", `"row":2,"iteration":null`, 1, ""},
		{"qualified row without iteration member", `"row":2`, 1, ""},
		{"qualified row in a block", `"row":4,"iteration":"A.Implementation"`, 3, "A"},
		{"qualified row in a block, ID in other letter case", `"row":4,"iteration":"a.Implementation"`, 3, "A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp, _, err := consultQualified(t, "build-review", tc.extra)

			if err != nil {
				t.Fatalf("want the reply accepted, got %v", err)
			}
			if disp == nil {
				t.Fatal("want a dispatch, got nil")
			}
			if disp.Agent != "build-review" {
				t.Errorf("want the bare agent id dispatched, got %q", disp.Agent)
			}
			if disp.RowIndex != tc.wantIndex {
				t.Errorf("want RowIndex=%d (the named row, not the first row of the agent), got %d", tc.wantIndex, disp.RowIndex)
			}
			if disp.Iteration != tc.wantID {
				t.Errorf("want Iteration=%q, got %q", tc.wantID, disp.Iteration)
			}
		})
	}
}

func TestConsultRouting_QualifiedRow_WholeCellIsAnAgentRowMismatchNotAnUnknownAgent(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		extra string
	}{
		{"whole cell of the named row", "build-review(tests)", `"row":2`},
		{"whole cell of the row in a block", "build-review(impl)", `"row":4,"iteration":"A.Implementation"`},
		{"whole cell of another qualified row", "build-review(impl)", `"row":2`},
		{"bare id on a row of another agent", "build-review", `"row":3`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp, calls, err := consultQualified(t, tc.agent, tc.extra)

			requireRejected(t, disp, calls, err)
		})
	}
}

// An agent that is neither a bare id nor a whole cell is unknown and not
// retried; the available agents are listed as bare ids, one per row.
func TestConsultRouting_QualifiedRow_UnknownAgentListsBareIDsInTableOrder(t *testing.T) {
	disp, calls, err := consultQualified(t, "nobody", `"row":2`)

	ce := assertConsultationError(t, err, domain.ConsultFailUnknownAgent)
	if disp != nil {
		t.Errorf("want no dispatch, got %+v", disp)
	}
	if calls != 1 {
		t.Errorf("want 1 InvokeRaw call (no retry), got %d", calls)
	}
	want := []string{"planner", "build-review", "implementer", "build-review"}
	if !slices.Equal(ce.Agents, want) {
		t.Errorf("want Agents %v (bare ids, one per row), got %v", want, ce.Agents)
	}
}
