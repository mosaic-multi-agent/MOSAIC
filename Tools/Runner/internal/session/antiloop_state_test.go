package session

import "testing"

// dispatchesUntilBlocked records dispatches of agentID at rowIndex until the
// guard blocks one, and returns how many were allowed. It gives up after limit
// attempts so a guard that never fires cannot hang the test.
func dispatchesUntilBlocked(a *antiLoopState, rowIndex int, agentID string, limit int) int {
	allowed := 0
	for i := 0; i < limit; i++ {
		if !a.recordDispatch(rowIndex, agentID) {
			return allowed
		}
		allowed++
	}
	return allowed
}

func TestAntiLoopState_SameAgentSameRow_IsBlockedAtTheCap(t *testing.T) {
	a := &antiLoopState{rowIndex: -1}

	got := dispatchesUntilBlocked(a, 0, "agent-a", 20)

	if got != maxConsecutiveSameAgentDispatches {
		t.Errorf("want %d allowed dispatches before the guard blocks, got %d",
			maxConsecutiveSameAgentDispatches, got)
	}
}

// The same agent dispatched at a new row starts from a fresh count: a counter
// scoped globally to the agent would block the first dispatch at the new row.
func TestAntiLoopState_RowChangeResetsConsecutiveCount(t *testing.T) {
	a := &antiLoopState{rowIndex: -1}
	dispatchesUntilBlocked(a, 0, "agent-a", 20)

	got := dispatchesUntilBlocked(a, 1, "agent-a", 20)

	if got != maxConsecutiveSameAgentDispatches {
		t.Errorf("want a fresh count of %d allowed dispatches at the new row, got %d",
			maxConsecutiveSameAgentDispatches, got)
	}
}

func TestAntiLoopState_AgentChangeOnSameRow_ResetsConsecutiveCount(t *testing.T) {
	a := &antiLoopState{rowIndex: -1}
	dispatchesUntilBlocked(a, 0, "agent-a", 20)

	got := dispatchesUntilBlocked(a, 0, "agent-b", 20)

	if got != maxConsecutiveSameAgentDispatches {
		t.Errorf("want a fresh count of %d allowed dispatches for the new agent, got %d",
			maxConsecutiveSameAgentDispatches, got)
	}
}
