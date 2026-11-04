package analysis_test

// Tests for the stretches the Timeline collapses: one wait of one actor, or one
// between-sessions interval, longer than 10 minutes, with no other invocation
// working during it. The run spans 0..5000 seconds unless a test says otherwise.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// collapseSpec is a run over 0..5000 seconds.
func collapseSpec() wtSpec { return newTestWallSpec(0, 5000) }

func requireCollapsible(t *testing.T, got domain.WallTimeSplit, want [][2]int) {
	t.Helper()
	wtAssertIntervals(t, "Collapsible", got.Collapsible, want)
}

func TestCollapsible_OrchestratorWaitLongerThanTenMinutes_Collapses(t *testing.T) {
	spec := collapseSpec()
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}

	requireCollapsible(t, wtSplit(t, spec), [][2]int{{1000, 2000}})
}

func TestCollapsible_LimitIsTenMinutesStrictly(t *testing.T) {
	cases := []struct {
		name string
		end  int
		want [][2]int
	}{
		{"exactly 10 minutes", 1600, nil},
		{"one second more", 1601, [][2]int{{1000, 1601}}},
		{"one second less", 1599, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := collapseSpec()
			spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, tc.end)}

			requireCollapsible(t, wtSplit(t, spec), tc.want)
		})
	}
}

func TestCollapsible_BetweenSessionsIntervalLimitIsTenMinutesStrictly(t *testing.T) {
	cases := []struct {
		name     string
		nextFrom int
		want     [][2]int
	}{
		{"exactly 10 minutes", 1600, nil},
		{"one second more", 1601, [][2]int{{1000, 1601}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := collapseSpec()
			spec.sessions = []domain.RunSession{wtSession("s1", 0, 1000), wtSession("s2", tc.nextFrom, 5000)}

			requireCollapsible(t, wtSplit(t, spec), tc.want)
		})
	}
}

func TestCollapsible_LongBetweenSessionsInterval_Collapses(t *testing.T) {
	spec := collapseSpec()
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 1000), wtSession("s2", 3000, 5000)}

	requireCollapsible(t, wtSplit(t, spec), [][2]int{{1000, 3000}})
}

func TestCollapsible_SubagentsOwnLongWait_CollapsesWhileItsOwnBarIsOpen(t *testing.T) {
	spec := collapseSpec()
	spec.agents = []wtAgent{{id: "a#1", start: 500, end: 3000, tools: []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}}}

	requireCollapsible(t, wtSplit(t, spec), [][2]int{{1000, 2000}})
}

func TestCollapsible_WaitDuringAnotherAgentsWork_StaysToScale(t *testing.T) {
	cases := []struct {
		name string
		spec func() wtSpec
	}{
		{"orchestrator waits while a subagent works part of the time", func() wtSpec {
			spec := collapseSpec()
			spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}
			spec.agents = []wtAgent{{id: "a#1", start: 1500, end: 1600}}
			return spec
		}},
		{"subagent waits while another subagent works", func() wtSpec {
			spec := collapseSpec()
			spec.agents = []wtAgent{
				{id: "a#1", start: 500, end: 3000, tools: []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}},
				{id: "b#2", start: 1900, end: 2500},
			}
			return spec
		}},
		{"another invocation is still open", func() wtSpec {
			spec := collapseSpec()
			spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}
			spec.agents = []wtAgent{{id: "a#1", start: 1500, end: wtNone}}
			return spec
		}},
		{"another invocation spans the whole wait", func() wtSpec {
			spec := collapseSpec()
			spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}
			spec.agents = []wtAgent{{id: "a#1", start: 100, end: 4000}}
			return spec
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCollapsible(t, wtSplit(t, tc.spec()), nil)
		})
	}
}

func TestCollapsible_AgentThatOnlyTouchesTheWaitForNoLength_DoesNotKeepItToScale(t *testing.T) {
	cases := []struct {
		name  string
		start int
		end   int
	}{
		{"ends when the wait starts", 800, 1000},
		{"starts when the wait ends", 2000, 2500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := collapseSpec()
			spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}
			spec.agents = []wtAgent{{id: "a#1", start: tc.start, end: tc.end}}

			requireCollapsible(t, wtSplit(t, spec), [][2]int{{1000, 2000}})
		})
	}
}

func TestCollapsible_OrchestratorAndSubagentWaitingTogether_OnlyTheSubagentsWaitCollapses(t *testing.T) {
	spec := collapseSpec()
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 2000)}
	spec.agents = []wtAgent{{id: "a#1", start: 500, end: 3000, tools: []domain.ToolCall{wtTool("w2", ask, 1200, 1900)}}}

	requireCollapsible(t, wtSplit(t, spec), [][2]int{{1200, 1900}})
}

func TestCollapsible_TwoTouchingSixMinuteStretches_DoNotCollapse(t *testing.T) {
	t.Run("between sessions then a wait", func(t *testing.T) {
		spec := collapseSpec()
		spec.sessions = []domain.RunSession{wtSession("s1", 0, 1000), wtSession("s2", 1360, 5000)}
		spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1360, 1720)}

		got := wtSplit(t, spec)

		wtAssertIntervals(t, "BetweenSessionIntervals", got.BetweenSessionIntervals, [][2]int{{1000, 1360}})
		requireCollapsible(t, got, nil)
	})
	t.Run("two actors' waits", func(t *testing.T) {
		spec := collapseSpec()
		spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 1000, 1360)}
		spec.agents = []wtAgent{{id: "a#1", start: 1360, end: 3000, tools: []domain.ToolCall{wtTool("w2", ask, 1360, 1720)}}}

		requireCollapsible(t, wtSplit(t, spec), nil)
	})
}

func TestCollapsible_TouchingLongStretches_StayTwoSeparateIntervals(t *testing.T) {
	spec := collapseSpec()
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 1000), wtSession("s2", 2000, 5000)}
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 2000, 3000)}

	requireCollapsible(t, wtSplit(t, spec), [][2]int{{1000, 2000}, {2000, 3000}})
}

func TestCollapsible_IsSortedWhateverTheSourceOfTheStretches(t *testing.T) {
	spec := collapseSpec()
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 1000), wtSession("s2", 2000, 5000)}
	spec.orchTools = []domain.ToolCall{wtTool("w2", ask, 3500, 4500), wtTool("w1", ask, 2200, 2400)}
	spec.agents = []wtAgent{{id: "a#1", start: 2500, end: 3400, tools: []domain.ToolCall{wtTool("w3", ask, 2600, 3300)}}}

	requireCollapsible(t, wtSplit(t, spec), [][2]int{{1000, 2000}, {2600, 3300}, {3500, 4500}})
}

func TestCollapsible_StretchTouchingTheStartOrEndOfTheSpan_IsLeftOut(t *testing.T) {
	cases := []struct {
		name       string
		start, end int
	}{
		{"wait from the span start", 0, 1000},
		{"wait to the span end", 4000, 5000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := collapseSpec()
			spec.orchTools = []domain.ToolCall{wtTool("w1", ask, tc.start, tc.end)}

			got := wtSplit(t, spec)

			requireCollapsible(t, got, nil)
			if got.Waiting != 1000*time.Second {
				t.Errorf("Waiting = %v, want the real 1000s", got.Waiting)
			}
		})
	}
}

func TestCollapsible_DoesNotChangeTheRealWallTimeFigures(t *testing.T) {
	spec := collapseSpec()
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 1000), wtSession("s2", 3000, 5000)}
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 3500, 4500)}

	got := wtSplit(t, spec)

	requireCollapsible(t, got, [][2]int{{1000, 3000}, {3500, 4500}})
	wtAssertSplit(t, got, 1000, 0, 2000, 2000)
	wtAssertIntervals(t, "WaitIntervals", got.WaitIntervals, [][2]int{{3500, 4500}})
	wtAssertIntervals(t, "BetweenSessionIntervals", got.BetweenSessionIntervals, [][2]int{{1000, 3000}})
	wtAssertIntervals(t, "ActorWaits", mustWaitsOf(t, got, domain.Orchestrator()), [][2]int{{3500, 4500}})
	if got.Span.End.Sub(got.Span.Start) != 5000*time.Second {
		t.Errorf("Span = %v, want the real 5000s", got.Span)
	}
}

func mustWaitsOf(t *testing.T, split domain.WallTimeSplit, actor domain.ActorRef) []domain.TimeInterval {
	t.Helper()
	waits, ok := wtWaitsOf(split, actor)
	if !ok {
		t.Fatalf("no waits for %s", actor.Label())
	}
	return waits
}

func TestCollapsible_RunWithoutLongStretches_IsEmpty(t *testing.T) {
	spec := collapseSpec()
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 2000), wtSession("s2", 2300, 5000)}
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 3000, 3400)}
	spec.agents = []wtAgent{{id: "a#1", start: 100, end: 900}}

	requireCollapsible(t, wtSplit(t, spec), nil)
}
