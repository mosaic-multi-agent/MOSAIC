package engine_test

// For an agent that fills several EXECUTION rows, a missing, unreadable or
// inconsistent recorded row makes Next stop and report. It never dispatches a
// guessed row.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// requireStopNaming asserts a Stop whose reason names the agent instance and,
// when row is non-zero, the recorded row.
func requireStopNaming(t *testing.T, dec domain.EngineDecision, agentInstance string, row int) {
	t.Helper()
	stop := requireStop(t, dec)
	if !strings.Contains(stop.Reason, agentInstance) {
		t.Errorf("stop reason must name agent %q, got %q", agentInstance, stop.Reason)
	}
	// Strip the instance id first: its sequence number must not satisfy the row check.
	withoutAgent := strings.ReplaceAll(stop.Reason, agentInstance, "")
	if row != 0 && !strings.Contains(withoutAgent, strconv.Itoa(row)) {
		t.Errorf("stop reason must name recorded row %d, got %q", row, stop.Reason)
	}
}

// nextAfterBuildReviewEntry drives Next after a build-review step recorded with
// the given stage and row. build-review fills two EXECUTION rows (9 and 12).
func nextAfterBuildReviewEntry(t *testing.T, stage string, row int) domain.EngineDecision {
	t.Helper()
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", stage, domain.StatusSUCCESS, row)
	return nextAfterLogIDs(aw, singleSchedules("TDD"), buildVerifiedAgents(), atInvocationNine(log))
}

// atInvocationNine renumbers the last entry as invocation 9, the position the
// old invocation-count arithmetic maps to the Test-group build-review row.
// Stopping must come from the recorded row, not from that arithmetic failing.
func atInvocationNine(log *runLog) *runLog {
	last := &log.entries[len(log.entries)-1]
	last.Seq = 9
	last.Agent = "build-review#9"
	log.globalSeq = 9
	return log
}

// loopAgentAdmitted admits the fixture whose one agent fills three rows of the
// Test group and one row of the Implementation group. The stage names the group,
// so a missing row is ambiguous only for an agent that repeats within a group.
func loopAgentAdmitted(t *testing.T) domain.AdmittedWorkflow {
	t.Helper()
	return mustParseAndAdmitV216(t, findingsLoopContent, "staged-findings-loop", "1.0")
}

func TestNext_RepeatedAgentInGroup_EntryWithoutRecordedRow_StopsNamingAgent(t *testing.T) {
	aw := loopAgentAdmitted(t)
	log := &runLog{}
	log.
		workflowStep("loop-agent", "1.Test", domain.StatusSUCCESS, loopRowWriter).
		workflowStep("loop-agent", "1.Test", domain.StatusSUCCESS, int(domain.NoWorkflowRow))

	dec := nextAfterLogIDs(aw, singleSchedules("TDD"), newTestAgents("loop-agent"), log)

	requireStopNaming(t, dec, "loop-agent#2", 0)
}

func TestNext_MultiRowAgent_RecordedRowHoldsDifferentAgent_StopsNamingAgentAndRow(t *testing.T) {
	// Row 10 is tests-review-tdd, not build-review.
	dec := nextAfterBuildReviewEntry(t, "1.Test", bvRowTestsReview)

	requireStopNaming(t, dec, "build-review#9", bvRowTestsReview)
}

func TestNext_MultiRowAgent_RecordedRowInOtherGroupThanStage_StopsNamingAgentAndRow(t *testing.T) {
	// Row 9 is a Test-group row; the entry's stage names the Implementation group.
	dec := nextAfterBuildReviewEntry(t, "1.Implementation", bvRowTestBuild)

	requireStopNaming(t, dec, "build-review#9", bvRowTestBuild)
}

func TestNext_MultiRowAgent_UnreadableStage_StopsNamingAgentAndRow(t *testing.T) {
	for _, stage := range []string{"", "Stage-1", "garbage"} {
		t.Run("stage="+stage, func(t *testing.T) {
			dec := nextAfterBuildReviewEntry(t, stage, bvRowTestBuild)

			requireStopNaming(t, dec, "build-review#9", bvRowTestBuild)
		})
	}
}

func TestNext_MultiRowAgent_RecordedRowOutsideTable_StopsNamingAgentAndRow(t *testing.T) {
	// The build-verified workflow has 13 rows.
	for _, row := range []int{14, 99} {
		t.Run("row="+strconv.Itoa(row), func(t *testing.T) {
			dec := nextAfterBuildReviewEntry(t, "1.Test", row)

			requireStopNaming(t, dec, "build-review#9", row)
		})
	}
}

func TestNext_RepeatedAgentInGroup_NoLogEntryForLastAgent_StopsNamingAgent(t *testing.T) {
	aw := loopAgentAdmitted(t)
	log := &runLog{}
	log.workflowStep("loop-agent", "1.Test", domain.StatusSUCCESS, loopRowWriter)
	state := log.state()
	state.GlobalSequence = 2
	state.CurrentState.LastAgent = "loop-agent#2" // no log entry has this instance id

	dec := engineNextAutoIDs(aw, singleSchedules("TDD"), state, successResponse("loop-agent#2"))

	requireStopNaming(t, dec, "loop-agent#2", 0)
}

func TestNext_RepeatedAgentInGroup_OnlyEarlierInstanceLogged_StopsInsteadOfUsingItsRow(t *testing.T) {
	// Earlier instances of the same agent are logged with valid rows, but the
	// step that just ran (loop-agent#3) is not. The row of a different instance
	// must not be borrowed.
	aw := loopAgentAdmitted(t)
	log := &runLog{}
	log.
		workflowStep("loop-agent", "1.Test", domain.StatusSUCCESS, loopRowWriter).
		workflowStep("loop-agent", "1.Test", domain.StatusSUCCESS, loopRowGate)
	state := log.state()
	state.GlobalSequence = 3
	state.CurrentState.LastAgent = "loop-agent#3"

	dec := engineNextAutoIDs(aw, singleSchedules("TDD"), state, successResponse("loop-agent#3"))

	requireStopNaming(t, dec, "loop-agent#3", 0)
}
