package session

// Tests for the run-schedule to block-schedules adapter: the run's schedule is
// held under every block that has a staged row, so a row of any block finds
// its own block's schedule.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// scheduleOf builds a schedule holding one iteration per id, in order.
func scheduleOf(ids ...domain.IterationID) *domain.BlockSchedule {
	sched := &domain.BlockSchedule{Block: "Stage"}
	for _, id := range ids {
		sched.Iterations = append(sched.Iterations, domain.ScheduleIteration{ID: id})
	}
	return sched
}

func twoIterationSchedule() *domain.BlockSchedule { return scheduleOf("1", "2") }

// stagedRowIn returns a row of block with the given group.
func stagedRowIn(idx int, group string, block domain.BlockName) domain.RoutingRow {
	parsed := domain.PhaseParsed{
		Name: "EXECUTION", IsStaged: true, Group: domain.GroupName(group), Blocks: []domain.BlockName{block},
	}
	return domain.RoutingRow{Index: idx, Phase: "EXECUTION", Agent: "dev", AgentID: "dev", PhaseParsed: parsed}
}

func idsOf(sched domain.BlockSchedule) []domain.IterationID {
	var ids []domain.IterationID
	for _, it := range sched.Iterations {
		ids = append(ids, it.ID)
	}
	return ids
}

func TestStageSchedules_StagedRowsInTwoBlocks_KeyEveryBlock(t *testing.T) {
	workflow := domain.AdmittedWorkflow{Table: domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, Phase: "PLANNING", Agent: "planner", AgentID: "planner", PhaseParsed: domain.PhaseParsed{Name: "PLANNING"}},
		stagedRowIn(1, "Test", "Alpha"),
		stagedRowIn(2, "Impl", "Beta"),
		stagedRowIn(3, "Impl", "Beta"),
	}}}

	got := stageSchedules(workflow, twoIterationSchedule())

	if len(got) != 2 {
		t.Fatalf("want one schedule per block with a staged row (Alpha, Beta), got %v", got)
	}
	for _, block := range []domain.BlockName{"Alpha", "Beta"} {
		sched, ok := got[block]
		if !ok {
			t.Fatalf("want a schedule for block %q, got blocks %v", block, got)
		}
		if sched.Block != block {
			t.Errorf("block %q: want the schedule to name its own block, got %q", block, sched.Block)
		}
		if ids := idsOf(sched); len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
			t.Errorf("block %q: want iterations 1,2, got %v", block, ids)
		}
	}
}

func TestStageSchedules_OneDeclaredBlock_KeysExactlyThatBlock(t *testing.T) {
	workflow := domain.AdmittedWorkflow{Table: domain.RoutingTable{Rows: []domain.RoutingRow{
		stagedRowIn(0, "Test", "Stage"),
		stagedRowIn(1, "Impl", "Stage"),
	}}}

	got := stageSchedules(workflow, twoIterationSchedule())

	if _, ok := got["Stage"]; !ok || len(got) != 1 {
		t.Fatalf("want exactly the schedule of block Stage, got %v", got)
	}
}

func TestStageSchedules_NoSchedule_GivesNoSchedules(t *testing.T) {
	workflow := domain.AdmittedWorkflow{Table: domain.RoutingTable{Rows: []domain.RoutingRow{
		stagedRowIn(0, "Test", "Alpha"),
	}}}

	got := stageSchedules(workflow, nil)

	if len(got) != 0 {
		t.Errorf("want no schedules without a schedule, got %v", got)
	}
}
