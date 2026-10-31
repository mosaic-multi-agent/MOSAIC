package analysis_test

// Tests for singular-call detection on the per-call context model.
//
// Rules under test:
//   - A call is singular only when it requested exactly one tool once (the
//     same tool twice, two tools, no tools and an absent field are not).
//   - A singular call needs a next main call of the same actor; helper calls
//     in between are skipped, and the last call of an actor has none.
//   - A compaction (logged mark, or a compaction call) or a user turn of the
//     same actor between the two calls makes the call not singular.
//   - Singular is decided from recorded figures, never from timestamps.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func TestToolGrowth_OneToolOnce_IsSingularAndCarriesTheTool(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 1000, 100, tgOne("read_file")),
		tgCall("c2", 20, 1500, 50, domain.NewRequestedTools(nil)),
	)

	a := ctxRequireAgent(t, run, ctxAgentID)

	tgAssertGrowth(t, "c1", tgCallGrowth(t, a, "c1"), domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_RequestShapes_OnlyOneToolOnceIsSingular(t *testing.T) {
	cases := []struct {
		name     string
		tools    domain.RequestedTools
		singular bool
	}{
		{"one tool once", tgOne("read_file"), true},
		{"same tool twice", domain.NewRequestedTools(map[string]int{"read_file": 2}), false},
		{"two tools", domain.NewRequestedTools(map[string]int{"read_file": 1, "glob": 1}), false},
		{"no tools", domain.NewRequestedTools(nil), false},
		{"field absent", domain.RequestedTools{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := tgAgentRun(t,
				tgCall("c1", 10, 1000, 100, tc.tools),
				tgCall("c2", 20, 1500, 50, domain.NewRequestedTools(nil)),
			)

			got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

			if (got.State != domain.GrowthNotSingular) != tc.singular {
				t.Errorf("growth = %+v, singular want %v", got, tc.singular)
			}
		})
	}
}

func TestToolGrowth_LastCallOfAnActor_IsNotSingular(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 1000, 100, domain.NewRequestedTools(nil)),
		tgCall("c2", 20, 1500, 50, tgOne("read_file")),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c2")

	tgAssertNotSingular(t, "last call", got)
}

func TestToolGrowth_NextCallOfAnotherActor_DoesNotCount(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{tgCall("o1", 10, 1000, 100, tgOne("read_file"))},
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("a1", 20, 5000, 100, domain.NewRequestedTools(nil)),
		}}},
	}})

	tgAssertNotSingular(t, "only orchestrator call", tgCallGrowth(t, run.Context.Orchestrator, "o1"))
}

func TestToolGrowth_NonMainNextCalls_AreSkippedToTheNextMainCall(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, tgOne("read_file")),
		tgKinded(tgCall("x1", 12, 9000, 5, domain.NewRequestedTools(nil)), domain.CallAuxiliary),
		tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	// Paired with m2 (1500), not with the auxiliary call (9000).
	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_NonMainCall_IsNeverSingularItself(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, domain.NewRequestedTools(nil)),
		tgKinded(tgCall("x1", 12, 1100, 5, tgOne("read_file")), domain.CallAuxiliary),
		tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "x1")

	tgAssertNotSingular(t, "auxiliary call", got)
}

func TestToolGrowth_OnlyHelperCallsFollow_IsNotSingular(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, tgOne("read_file")),
		tgKinded(tgCall("x1", 12, 1100, 5, domain.NewRequestedTools(nil)), domain.CallAuxiliary),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "call without a next main call", got)
}

func TestToolGrowth_CompactionCallBetween_IsNotSingular(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, tgOne("read_file")),
		tgKinded(tgCall("k1", 12, 1100, 5, domain.NewRequestedTools(nil)), domain.CallCompaction),
		tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "call across a compaction call", got)
}

func TestToolGrowth_LoggedCompactionBetween_IsNotSingular(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, tgOne("read_file")),
			tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
		}}},
		compactions: []domain.Compaction{tgLoggedCompaction(tgAgentActor(), 15)},
	}})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "call across a logged compaction", got)
}

func TestToolGrowth_CompactionOfAnotherActor_DoesNotMatter(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, tgOne("read_file")),
			tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
		}}},
		compactions: []domain.Compaction{tgLoggedCompaction(domain.Orchestrator(), 15)},
	}})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_CompactionOutsideTheTwoCalls_DoesNotMatter(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, tgOne("read_file")),
			tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
			tgCall("m3", 40, 2000, 50, domain.NewRequestedTools(nil)),
		}}},
		compactions: []domain.Compaction{tgLoggedCompaction(tgAgentActor(), 30)},
	}})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_UserTurnOfTheSameActorBetween_IsNotSingular(t *testing.T) {
	run := tgPrice(t, tgRunSpec{
		ctxRunSpec: ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, tgOne("read_file")),
			tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
		}}}},
		turns: []domain.Turn{tgUserTurn(tgAgentActor(), 15)},
	})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "call across a user turn", got)
}

func TestToolGrowth_UserTurnExactlyAtEitherCall_IsNotSingular(t *testing.T) {
	for _, sec := range []int{10, 20} {
		run := tgPrice(t, tgRunSpec{
			ctxRunSpec: ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
				tgCall("m1", 10, 1000, 100, tgOne("read_file")),
				tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
			}}}},
			turns: []domain.Turn{tgUserTurn(tgAgentActor(), sec)},
		})

		got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

		if got.State != domain.GrowthNotSingular {
			t.Errorf("user turn at %ds: growth = %+v, want not singular (closed interval)", sec, got)
		}
	}
}

func TestToolGrowth_UserTurnsThatDoNotCount_LeaveTheCallSingular(t *testing.T) {
	cases := map[string]domain.Turn{
		"before the first call":   tgUserTurn(tgAgentActor(), 5),
		"after the second call":   tgUserTurn(tgAgentActor(), 25),
		"another actor":           tgUserTurn(domain.Orchestrator(), 15),
		"assistant role":          {Actor: tgAgentActor(), Role: domain.TurnAssistant, At: ctxAt(15)},
		"unparseable (zero) time": {Actor: tgAgentActor(), Role: domain.TurnUser},
	}
	for name, turn := range cases {
		t.Run(name, func(t *testing.T) {
			run := tgPrice(t, tgRunSpec{
				ctxRunSpec: ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
					tgCall("m1", 10, 1000, 100, tgOne("read_file")),
					tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
				}}}},
				turns: []domain.Turn{turn},
			})

			got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

			tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
		})
	}
}

func TestToolGrowth_UserTurnOfTheOrchestrator_BreaksOrchestratorPairsOnly(t *testing.T) {
	run := tgPrice(t, tgRunSpec{
		ctxRunSpec: ctxRunSpec{orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 100, tgOne("read_file")),
			tgCall("o2", 20, 1500, 50, domain.NewRequestedTools(nil)),
		}},
		turns: []domain.Turn{tgUserTurn(domain.Orchestrator(), 15)},
	})

	tgAssertNotSingular(t, "o1", tgCallGrowth(t, run.Context.Orchestrator, "o1"))
}

func TestToolGrowth_CallWithoutATimestamp_IsNeverSingularAndNeverTheNextCall(t *testing.T) {
	noTime := tgCall("z1", 0, 1200, 10, domain.NewRequestedTools(nil))
	noTime.At = time.Time{}
	untimedSingular := tgCall("z2", 0, 1300, 10, tgOne("read_file"))
	untimedSingular.At = time.Time{}

	run := tgAgentRun(t,
		tgCall("m1", 10, 1000, 100, tgOne("read_file")),
		noTime,
		untimedSingular,
		tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
	)

	a := ctxRequireAgent(t, run, ctxAgentID)

	tgAssertNotSingular(t, "untimed singular call", tgCallGrowth(t, a, "z2"))
	// The untimed call is not the next call of m1: m1 pairs with m2.
	tgAssertGrowth(t, "m1", tgCallGrowth(t, a, "m1"), domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_CallsWithTheSameStart_PairInRecordedOrder(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("t1", 10, 1000, 100, tgOne("read_file")),
		tgCall("t2", 10, 1300, 50, tgOne("glob")),
		tgCall("t3", 30, 1800, 50, domain.NewRequestedTools(nil)),
	)

	a := ctxRequireAgent(t, run, ctxAgentID)

	tgAssertGrowth(t, "t1", tgCallGrowth(t, a, "t1"), domain.GrowthMeasured, "read_file", 200)
	tgAssertGrowth(t, "t2", tgCallGrowth(t, a, "t2"), domain.GrowthMeasured, "glob", 450)
}

func TestToolGrowth_SingularIsDecidedFromRecordedFigures_NotFromTimestamps(t *testing.T) {
	// Two calls a millisecond apart and two a minute apart behave alike.
	near := tgAgentRun(t,
		tgCall("c1", 10, 1000, 100, tgOne("read_file")),
		tgCall("c2", 10, 1500, 50, domain.NewRequestedTools(nil)),
	)
	far := tgAgentRun(t,
		tgCall("c1", 10, 1000, 100, tgOne("read_file")),
		tgCall("c2", 70, 1500, 50, domain.NewRequestedTools(nil)),
	)

	gc := tgCallGrowth(t, ctxRequireAgent(t, near, ctxAgentID), "c1")
	gf := tgCallGrowth(t, ctxRequireAgent(t, far, ctxAgentID), "c1")

	if gc != gf || gc.State != domain.GrowthMeasured {
		t.Errorf("near growth %+v, far growth %+v, want equal and measured", gc, gf)
	}
}

func TestToolGrowth_UnattributedStream_IsNeverMeasured(t *testing.T) {
	agg := ctxAggregate(ctxRunSpec{})
	agg.Runs[0].Unattributed = &domain.ActorAggregate{Invocations: []domain.InvocationUsage{
		tgCall("u1", 10, 1000, 100, tgOne("read_file")),
		tgCall("u2", 20, 1500, 50, domain.NewRequestedTools(nil)),
	}}

	run := ctxPrice(t, agg, domain.DefaultAnalysisSettings())

	for _, tool := range run.Tools.Run {
		if tool.Growth.Samples+tool.Growth.Negative+tool.Growth.NotMeasurable != 0 {
			t.Errorf("tool %q has growth figures %+v from the unattributed stream", tool.Tool, tool.Growth)
		}
	}
}
