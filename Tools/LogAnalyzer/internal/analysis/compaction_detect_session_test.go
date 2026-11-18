package analysis_test

// Tests for detected compactions across the sessions of one actor: a context
// drop is only a compaction between two calls of the same session. Calls
// without a session id join the session of the nearest preceding call that has
// one, or of the nearest following one when none precedes.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// detSessionSpec is an orchestrator whose calls carry the given session ids,
// one id per point.
func detSessionSpec(sessions []string, pts ...detPt) ctxRunSpec {
	calls := detCalls("o", pts...)
	for i := range calls {
		calls[i].SessionID = sessions[i]
	}
	return ctxRunSpec{orchUsage: calls}
}

func TestDetectedCompaction_DropAtASessionBoundary_IsNotDetected(t *testing.T) {
	spec := detSessionSpec([]string{"s1", "s1", "s2", "s2"},
		detPt{0, 200000}, detPt{60, 210000}, detPt{120, 80000}, detPt{180, 90000})

	run, findings := detPrice(t, spec)

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0 (the drop pairs two sessions)", n)
	}
	detRequireMismatches(t, findings, 0)
}

func TestDetectedCompaction_DropInsideOneSession_IsStillDetectedAfterASessionStart(t *testing.T) {
	spec := detSessionSpec([]string{"s1", "s2", "s2"},
		detPt{0, 300000}, detPt{60, 250000}, detPt{120, 100000})

	run, _ := detPrice(t, spec)

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	detAssertMark(t, marks[0], 1, 2, 250000, 100000, ctxAt(120))
}

func TestDetectedCompaction_FirstCallOfASession_IsNeverTheAfterCallOfAMark(t *testing.T) {
	spec := detSessionSpec([]string{"s1", "s2", "s3"},
		detPt{0, 400000}, detPt{60, 100000}, detPt{120, 20000})

	run, _ := detPrice(t, spec)

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_SingleNamedSession_YieldsTheSameMarksAsBefore(t *testing.T) {
	spec := detSessionSpec([]string{"s1", "s1", "s1"},
		detPt{0, 300000}, detPt{60, 150000}, detPt{120, 70000})

	run, _ := detPrice(t, spec)

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 2 {
		t.Fatalf("detected marks = %d, want 2", len(marks))
	}
	detAssertMark(t, marks[0], 0, 1, 300000, 150000, ctxAt(60))
	detAssertMark(t, marks[1], 1, 2, 150000, 70000, ctxAt(120))
}

func TestDetectedCompaction_NoSessionIdsAtAll_IsOneSession(t *testing.T) {
	spec := detSessionSpec([]string{"", "", ""},
		detPt{0, 200000}, detPt{60, 80000}, detPt{120, 90000})

	run, _ := detPrice(t, spec)

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	detAssertMark(t, marks[0], 0, 1, 200000, 80000, ctxAt(60))
}

func TestDetectedCompaction_CallsWithoutIdBeforeTheFirstId_JoinTheFollowingSession(t *testing.T) {
	spec := detSessionSpec([]string{"", "s1"}, detPt{0, 200000}, detPt{60, 80000})

	run, _ := detPrice(t, spec)

	if n := run.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Errorf("detected = %d, want 1 (leading call without id belongs to s1)", n)
	}
}

func TestDetectedCompaction_CallsWithoutIdAfterAnId_JoinThePrecedingSession(t *testing.T) {
	spec := detSessionSpec([]string{"s1", ""}, detPt{0, 200000}, detPt{60, 80000})

	run, _ := detPrice(t, spec)

	if n := run.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Errorf("detected = %d, want 1 (trailing call without id belongs to s1)", n)
	}
}

func TestDetectedCompaction_CallWithoutIdBetweenSessions_BelongsToThePrecedingOne(t *testing.T) {
	spec := detSessionSpec([]string{"s1", "", "s2"}, detPt{0, 200000}, detPt{60, 190000}, detPt{120, 50000})

	run, _ := detPrice(t, spec)

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0 (the drop to s2 crosses a session boundary)", n)
	}
}

func TestDetectedCompaction_SessionsOfASubagent_AreSeparatedToo(t *testing.T) {
	agent := detAgent("worker#1", detPt{0, 200000}, detPt{60, 210000}, detPt{120, 80000})
	agent.usage[2].SessionID = "resumed"
	agent.usage[0].SessionID = "first"
	agent.usage[1].SessionID = "first"

	run, _ := detPrice(t, ctxRunSpec{agents: []ctxAgent{agent}})

	if n := ctxRequireAgent(t, run, "worker#1").DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
}

func TestDetectedCompaction_LoggedCompactionIsMatchedToTheSameSessionDrop(t *testing.T) {
	// A same-session drop at 360 s matched by a logged compaction at 330 s; a
	// cross-session drop at 700 s must neither be detected nor raise a finding.
	calls := sessJoin(
		sessAll("s1",
			ctxCall("o1", 0, 200000, 0, 0, 10),
			ctxCall("o2", 300, 200000, 0, 0, 10),
			ctxCall("o3", 360, 80000, 0, 0, 10)),
		sessAll("s2", ctxCall("o4", 700, 20000, 0, 0, 10)),
	)
	spec := ctxRunSpec{
		orchUsage:   calls,
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 330)},
	}

	run, findings := detPrice(t, spec)

	detRequireCounts(t, run.Context.Orchestrator, 1, 1)
	detRequireMismatches(t, findings, 0)
}

func TestDetectedCompaction_LoggedCompactionAtASessionBoundaryDrop_RaisesNoDropMismatch(t *testing.T) {
	spec := detSessionSpec([]string{"s1", "s1", "s2", "s2"},
		detPt{0, 200000}, detPt{300, 200000}, detPt{360, 80000}, detPt{600, 80000})
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 330)}

	run, findings := detPrice(t, spec)

	detRequireCounts(t, run.Context.Orchestrator, 1, 0)
	got := detRequireMismatches(t, findings, 1)
	detAssertDetailContains(t, got[0], detNoDropNear)
}
