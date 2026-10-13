package deviation_test

// Tests for the iteration value of a routing dispatch reply: it is the run
// artifact's value form of the named row, its ID is checked against the
// schedule of the block the row belongs to, and an accepted ID is carried as
// the string the schedule spells it.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// twoBlockTable returns a three-row table (1-based row numbers):
//
//	1 dev      (block Alpha, group Test)
//	2 dev      (block Beta, group Impl)
//	3 reviewer (outside every block)
func twoBlockTable() domain.RoutingTable {
	return domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, Phase: "EXECUTION", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Test", Blocks: []domain.BlockName{"Alpha"}}},
		{Index: 1, Phase: "EXECUTION", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Impl", Blocks: []domain.BlockName{"Beta"}}},
		{Index: 2, Phase: "REVIEW", Agent: "reviewer", AgentID: "reviewer", PhaseParsed: domain.PhaseParsed{Name: "REVIEW"}},
	}}
}

// scheduleOf builds the schedule of block with the given iteration IDs.
func scheduleOf(block domain.BlockName, ids ...string) domain.BlockSchedule {
	sched := domain.BlockSchedule{Block: block}
	for _, id := range ids {
		sched.Iterations = append(sched.Iterations, domain.ScheduleIteration{ID: domain.IterationID(id)})
	}
	return sched
}

// twoBlockSchedules gives Alpha the IDs "1","2","setup-b" and Beta "b1","B2".
func twoBlockSchedules() domain.BlockSchedules {
	return domain.BlockSchedules{
		"Alpha": scheduleOf("Alpha", "1", "2", "setup-b"),
		"Beta":  scheduleOf("Beta", "b1", "B2"),
	}
}

// consultTwoBlock runs one routing consultation over the two-block table with a
// reply naming agent "dev" or "reviewer" plus the given raw members, and
// reports the dispatch (nil when the reply was never accepted) and the number
// of invocations used.
func consultTwoBlock(t *testing.T, agent, extra string, schedules domain.BlockSchedules) (*domain.DispatchInstruction, int, error) {
	t.Helper()
	reply := sequencedReply{reply: dispatchReply(agent, extra)}
	invoker := &sequencedRawInvoker{replies: []sequencedReply{reply, reply, reply, reply}}
	c := newTestOrchestratorConsultant(invoker, twoBlockTable())

	instr, err := c.ConsultRouting(context.Background(), rowStageRequest(schedules))

	return instr.Dispatch, invoker.pos, err
}

func requireRejected(t *testing.T, disp *domain.DispatchInstruction, calls int, err error) {
	t.Helper()
	assertConsultationError(t, err, domain.ConsultFailMalformedJSON)
	if disp != nil {
		t.Errorf("want no dispatch for an invalid reply, got %+v", disp)
	}
	if calls != 3 {
		t.Errorf("want 3 InvokeRaw calls (the malformed-JSON budget), got %d", calls)
	}
}

func TestConsultRouting_IterationKnownInTheRowsBlockSchedule_IsAcceptedAsTheScheduleSpellsIt(t *testing.T) {
	cases := []struct {
		name      string
		extra     string
		wantIndex int
		wantID    domain.IterationID
	}{
		{"numeric ID with its group", `"row":1,"iteration":"2.Test"`, 0, "2"},
		{"non-numeric ID", `"row":1,"iteration":"setup-b.Test"`, 0, "setup-b"},
		{"ID in other letter case is carried as the schedule spells it", `"row":2,"iteration":"b2.Impl"`, 1, "B2"},
		{"upper-case spelling of a lower-case ID", `"row":1,"iteration":"SETUP-B.Test"`, 0, "setup-b"},
		{"ID of the second block", `"row":2,"iteration":"b1.Impl"`, 1, "b1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp, _, err := consultTwoBlock(t, "dev", tc.extra, twoBlockSchedules())

			if err != nil {
				t.Fatalf("want the reply accepted, got %v", err)
			}
			if disp == nil || disp.RowIndex != tc.wantIndex {
				t.Fatalf("want dispatch at RowIndex=%d, got %+v", tc.wantIndex, disp)
			}
			if disp.Iteration != tc.wantID {
				t.Errorf("want Iteration=%q, got %q", tc.wantID, disp.Iteration)
			}
		})
	}
}

func TestConsultRouting_IterationNotValidForTheRow_IsRejectedWithTheMalformedClass(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		extra string
	}{
		{"unknown numeric ID", "dev", `"row":1,"iteration":"9.Test"`},
		{"unknown non-numeric ID", "dev", `"row":1,"iteration":"nope.Test"`},
		{"ID that exists only in the other block", "dev", `"row":1,"iteration":"b1.Test"`},
		{"ID of the first block on a row of the second", "dev", `"row":2,"iteration":"setup-b.Impl"`},
		{"numeric ID missing from the second block", "dev", `"row":2,"iteration":"1.Impl"`},
		{"negative ID", "dev", `"row":1,"iteration":"-1.Test"`},
		{"group of another row", "dev", `"row":1,"iteration":"1.Impl"`},
		{"group missing", "dev", `"row":1,"iteration":"1"`},
		{"group compared exactly", "dev", `"row":1,"iteration":"1.test"`},
		{"iteration on a row outside every block", "reviewer", `"row":3,"iteration":"1"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp, calls, err := consultTwoBlock(t, tc.agent, tc.extra, twoBlockSchedules())

			requireRejected(t, disp, calls, err)
		})
	}
}

// The former stage member is no longer part of the reply: a reply carrying a
// non-null one is malformed even when it also carries a valid iteration.
func TestConsultRouting_StageMember_IsNotAccepted(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		extra string
	}{
		{"stage as integer instead of iteration", "dev", `"row":1,"stage":2`},
		{"stage as string instead of iteration", "dev", `"row":1,"stage":"2"`},
		{"stage in group form instead of iteration", "dev", `"row":1,"stage":"Test.2"`},
		{"valid iteration beside a stage", "dev", `"row":1,"iteration":"2.Test","stage":2`},
		{"stage on a row outside every block", "reviewer", `"row":3,"stage":1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp, calls, err := consultTwoBlock(t, tc.agent, tc.extra, twoBlockSchedules())

			requireRejected(t, disp, calls, err)
		})
	}
}

func TestConsultRouting_BlockWithoutAnySchedule_RejectsEveryIteration(t *testing.T) {
	onlyAlpha := domain.BlockSchedules{"Alpha": scheduleOf("Alpha", "1", "2")}

	disp, calls, err := consultTwoBlock(t, "dev", `"row":2,"iteration":"1.Impl"`, onlyAlpha)

	requireRejected(t, disp, calls, err)
}

// Zero is an ID like any other: accepted when the schedule holds it, unknown
// otherwise.
func TestConsultRouting_ZeroIDOnABlockRow_FollowsTheSchedule(t *testing.T) {
	withZero := domain.BlockSchedules{"Alpha": scheduleOf("Alpha", "0", "1")}

	accepted, _, errAccepted := consultTwoBlock(t, "dev", `"row":1,"iteration":"0.Test"`, withZero)
	rejected, calls, errRejected := consultTwoBlock(t, "dev", `"row":1,"iteration":"0.Test"`, twoBlockSchedules())

	if errAccepted != nil || accepted == nil || accepted.Iteration != "0" {
		t.Errorf("want 0 accepted when the schedule holds it, got %+v err=%v", accepted, errAccepted)
	}
	requireRejected(t, rejected, calls, errRejected)
}

// A row outside every block takes no iteration: whatever value is given,
// including the on-disk marker "-" and zero, is rejected.
func TestConsultRouting_AnyIterationOnARowOutsideEveryBlock_IsRejected(t *testing.T) {
	cases := []struct {
		name  string
		extra string
	}{
		{"zero", `"row":3,"iteration":"0"`},
		{"dash", `"row":3,"iteration":"-"`},
		{"ID", `"row":3,"iteration":"2"`},
		{"grouped value", `"row":3,"iteration":"2.Test"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp, calls, err := consultTwoBlock(t, "reviewer", tc.extra, twoBlockSchedules())

			requireRejected(t, disp, calls, err)
		})
	}
}

// A dash is never an iteration of a block row either.
func TestConsultRouting_DashOnABlockRow_IsRejected(t *testing.T) {
	disp, calls, err := consultTwoBlock(t, "dev", `"row":1,"iteration":"-"`, twoBlockSchedules())

	requireRejected(t, disp, calls, err)
}

// A schedule may spell an ID with a leading zero: the reply naming that exact
// spelling is carried as the schedule spells it.
func TestConsultRouting_ScheduleIDWithLeadingZero_IsMatchedAsSpelled(t *testing.T) {
	schedules := domain.BlockSchedules{"Alpha": scheduleOf("Alpha", "02")}

	disp, _, err := consultTwoBlock(t, "dev", `"row":1,"iteration":"02.Test"`, schedules)

	if err != nil || disp == nil {
		t.Fatalf("want the reply accepted, got %+v err=%v", disp, err)
	}
	if disp.Iteration != "02" {
		t.Errorf("want Iteration=%q as the schedule spells it, got %q", "02", disp.Iteration)
	}
}

// Each block is checked against its own schedule: a reply is valid for one row
// and invalid for the other when the ID lives in one schedule only.
func TestConsultRouting_SameIDIsValidOnlyForTheRowOfTheBlockThatSchedulesIt(t *testing.T) {
	schedules := domain.BlockSchedules{
		"Alpha": scheduleOf("Alpha", "x1"),
		"Beta":  scheduleOf("Beta", "y1"),
	}

	onAlpha, _, errAlpha := consultTwoBlock(t, "dev", `"row":1,"iteration":"x1.Test"`, schedules)
	onBeta, callsBeta, errBeta := consultTwoBlock(t, "dev", `"row":2,"iteration":"x1.Impl"`, schedules)

	if errAlpha != nil || onAlpha == nil || onAlpha.Iteration != "x1" {
		t.Errorf("want x1 accepted on the Alpha row, got %+v err=%v", onAlpha, errAlpha)
	}
	requireRejected(t, onBeta, callsBeta, errBeta)
}
