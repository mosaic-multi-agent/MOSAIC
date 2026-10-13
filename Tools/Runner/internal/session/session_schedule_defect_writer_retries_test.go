package session_test

// A re-dispatch of an in-block writer that does not end in a clean SUCCESS
// (PARTIALLY_DONE, a tool-unavailable BLOCKED reply, or a harness failure) is
// retried like any other dispatch of that row: the run does not stop, the row
// runs again at the iteration it was dispatched at, and a corrected schedule
// then lets the block continue.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// TestSession_InBlockWriterRedispatchNotCleanSuccess_RetriesRowAndBlockContinues
// verifies, for a produced and for a seeded block, that when the writer's
// re-dispatch for a defective schedule first returns a non-SUCCESS result, the
// run still ends completed: the worker is dispatched again at the same
// iteration, writes the corrected schedule, and the block runs the iteration
// the corrected schedule added. Iteration A carries the iteration-level HITL
// flag in the schedule held before the writer's defective write, so every
// dispatch at A, the retried one included, must carry human_in_the_loop even
// though the live schedule is dropped while the writer is being retried.
func TestSession_InBlockWriterRedispatchNotCleanSuccess_RetriesRowAndBlockContinues(t *testing.T) {
	results := []struct {
		name  string
		entry harness.ScriptedEntry
	}{
		{"PARTIALLY_DONE", statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half done")},
		{"tool unavailable", e501Entry()},
		{"harness failure", failEntry("harness: subprocess timed out")},
	}
	blocks := []struct {
		name     string
		fixture  string
		workflow string
		seeded   bool
		prefix   []string
	}{
		{"produced block", "writer-produced-orch.md", "writer-produced", false, []string{"planner"}},
		{"seeded block", "writer-seeded-orch.md", "writer-seeded", true, nil},
	}
	for _, blk := range blocks {
		for _, res := range results {
			t.Run(blk.name+"/"+res.name, func(t *testing.T) {
				agents := append(append([]string{}, blk.prefix...), "worker", "closer")
				approved := &fixedApprovalReader{approval: domain.ApprovalTrue}
				rig := newBlockRig(t, blk.fixture, approved, agents...)
				hitlA := scheduleDoc(sdBlockName, sdSection, []scheduleRow{{id: "A", hitl: true}})
				if blk.seeded {
					rig.writeRunFile(t, "Plan.md", hitlA)
				} else {
					rig.queueProducer("planner", "planned", runFileWrite("Plan.md", hitlA))
				}
				rig.queueProducer("worker", "worked A", runFileWrite("Plan.md", dupSchedule()))
				rig.adapter.Queue("worker", res.entry)
				rig.queueProducer("worker", "worked A again", runFileWrite("Plan.md", okSchedule("A", "B")))
				rig.queueOK("worker")
				rig.queueOK("closer")

				got, err := rig.ses.Start(context.Background(), rig.config(blk.workflow, domain.ExecutionModeAuto))

				requireRunStatus(t, got, err, domain.RunCompleted)
				requireSequence(t, "iterations the worker ran at",
					rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "A", "B"})
				requireFlags(t, "worker", rig.hitlOf("worker"), []bool{true, true, true, false})
				if n := rig.invocationsOf("closer"); n != 1 {
					t.Errorf("want the closer dispatched once after the block, got %d", n)
				}
				if !blk.seeded {
					if n := rig.invocationsOf("planner"); n != 1 {
						t.Errorf("want the planner outside the block dispatched once, got %d", n)
					}
				}
			})
		}
	}
}
