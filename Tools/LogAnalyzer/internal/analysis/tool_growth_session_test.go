package analysis_test

// Tests for tool context growth across the sessions of one actor: a singular
// call is only measured against the actor's next timed main call when that call
// belongs to the same session. Calls without a session id follow the shared
// grouping rule (nearest preceding id, else nearest following id).

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func tgNoTools() domain.RequestedTools { return domain.NewRequestedTools(nil) }

func TestToolGrowth_NextMainCallInAnotherSession_IsNotSingular(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		sessCall("s2", tgCall("m2", 20, 1500, 50, tgNoTools())),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "m1 across a session boundary", got)
}

func TestToolGrowth_NextMainCallInTheSameSession_IsMeasuredAsBefore(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		sessCall("s1", tgCall("m2", 20, 1500, 50, tgNoTools())),
		sessCall("s2", tgCall("m3", 30, 1700, 50, tgNoTools())),
	)

	a := ctxRequireAgent(t, run, ctxAgentID)

	tgAssertGrowth(t, "m1", tgCallGrowth(t, a, "m1"), domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_HelperCallOfAnotherSession_IsSkippedToTheNextMainCallOfTheSession(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		sessCall("s2", tgKinded(tgCall("x1", 12, 9000, 5, tgNoTools()), domain.CallAuxiliary)),
		sessCall("s1", tgCall("m2", 20, 1500, 50, tgNoTools())),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_HelperCallBeforeTheNextSession_DoesNotMakeTheNextMainCallSameSession(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		sessCall("s1", tgKinded(tgCall("x1", 12, 1100, 5, tgNoTools()), domain.CallAuxiliary)),
		sessCall("s2", tgCall("m2", 20, 1500, 50, tgNoTools())),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "m1 whose next main call is in another session", got)
}

func TestToolGrowth_SessionBoundaryPairs_CountInNoGrowthFigure(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		sessCall("s2", tgCall("m2", 20, 1500, 50, tgNoTools())),
	)

	for _, tool := range run.Tools.Run {
		if n := tool.Growth.Samples + tool.Growth.Negative + tool.Growth.NotMeasurable; n != 0 {
			t.Errorf("tool %q growth %+v counts a cross-session pair", tool.Tool, tool.Growth)
		}
	}
}

func TestToolGrowth_SameSessionPair_StillCountsAsASample(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		sessCall("s1", tgCall("m2", 20, 1500, 50, tgNoTools())),
	)

	var samples int
	for _, tool := range run.Tools.Run {
		if tool.Tool == "read_file" {
			samples = int(tool.Growth.Samples)
		}
	}

	if samples != 1 {
		t.Errorf("read_file samples = %d, want 1", samples)
	}
}

func TestToolGrowth_CrossSessionContextDrop_IsNotCountedAsNegativeGrowth(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 2000, 100, tgOne("read_file"))),
		sessCall("s2", tgCall("m2", 20, 500, 50, tgNoTools())),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "m1 across a session boundary with a drop", got)
	for _, tool := range run.Tools.Run {
		if tool.Growth.Negative != 0 {
			t.Errorf("tool %q negative growth = %d, want 0", tool.Tool, tool.Growth.Negative)
		}
	}
}

func TestToolGrowth_AllCallsWithoutSessionId_AreOneSession(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, tgOne("read_file")),
		tgCall("m2", 20, 1500, 50, tgNoTools()),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_NextCallWithoutIdAfterAnId_JoinsThePrecedingSession(t *testing.T) {
	run := tgAgentRun(t,
		sessCall("s1", tgCall("m1", 10, 1000, 100, tgOne("read_file"))),
		tgCall("m2", 20, 1500, 50, tgNoTools()),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_LeadingCallWithoutId_JoinsTheFollowingSession(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, tgOne("read_file")),
		sessCall("s1", tgCall("m2", 20, 1500, 50, tgNoTools())),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_OrchestratorSessions_AreSeparatedToo(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{orchUsage: sessJoin(
		sessAll("s1", tgCall("o1", 10, 1000, 100, tgOne("read_file"))),
		sessAll("s2", tgCall("o2", 20, 1500, 50, tgNoTools())),
	)}})

	tgAssertNotSingular(t, "o1", tgCallGrowth(t, run.Context.Orchestrator, "o1"))
}
