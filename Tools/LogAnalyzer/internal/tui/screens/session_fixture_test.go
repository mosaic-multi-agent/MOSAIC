package screens_test

// Fixture and helpers of the Session screen tests. The run is the standard Context
// run with the figures the Session screen adds on top of it: every actor has a
// priced cost split, and the invocations have a dispatch with status, model, effort
// and duration. Run cost is 100.00, so each actor's share is a whole percent:
//
//	actor         cost    share  input / cache read / cache write / output (percent of the actor)
//	orchestrator  40.00   40%    25% / 30% / 35% / 10%      span 1h2m5s, model-o
//	builder#1     10.00   10%    15% / 25% / 40% / 20%      BLOCKED, 2m5s, effort high
//	coder#1       25.00   25%    20% / 28% / 36% / 16%      SUCCESS
//	reviewer#1    20.00   20%    50% / 49% / <1% / 1%       PARTIALLY_DONE
//	tester#1      unpriced       no status, no type
//	ghost#1       no data        no per-call records
//
// The per-call requested tools and growth are set on builder#1 and coder#1 only;
// the other actors model older logs that do not provide them.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sessSplit is a priced cost split in cents; the total is the sum of the four parts.
func sessSplit(input, cacheRead, cacheWrite, output int64) domain.Totals {
	return domain.Totals{Money: domain.CategoryMoney{
		Input: known(input), CacheRead: known(cacheRead), CacheCreation: known(cacheWrite), Output: known(output),
		Total: known(input + cacheRead + cacheWrite + output), Complete: true,
	}}
}

// sessUnpriced is the cost split of an actor whose model has no price.
func sessUnpriced() domain.Totals {
	u := domain.UnpricedMoney()
	return domain.Totals{Money: domain.CategoryMoney{Input: u, CacheRead: u, CacheCreation: u, Output: u, Total: u}}
}

// sessNoData is the cost split of an actor that logged nothing priceable.
func sessNoData() domain.Totals {
	n := domain.NoMoneyData()
	return domain.Totals{Money: domain.CategoryMoney{Input: n, CacheRead: n, CacheCreation: n, Output: n, Total: n}}
}

// sessHighEffort is the effort of builder#1.
var sessHighEffort = domain.EffortLevels{{Level: "high", Records: 3}}

// sessDispatch is a dispatch of one invocation.
func sessDispatch(ref domain.ActorRef, status string, secs int, models []domain.ModelID, effort domain.EffortLevels, totals domain.Totals) domain.Dispatch {
	d := domain.Dispatch{
		Invocation: domain.Invocation{Actor: ref, State: domain.InvocationClosed, StatusCode: status, EndEvents: 1},
		Models:     models,
		Totals:     totals,
		Effort:     effort,
	}
	if secs > 0 {
		d.Invocation.StartedAt = dispatchEpoch
		d.Invocation.EndedAt = dispatchEpoch.Add(time.Duration(secs) * time.Second)
	}
	return d
}

// sessOneTool is the requested tools of a call that asked for one tool once.
func sessOneTool(tool string) domain.RequestedTools {
	return domain.NewRequestedTools(map[string]int{tool: 1})
}

// withSessionCalls sets the requested tools and growth of the calls of one invocation.
func withSessionCalls(run *domain.RunReport, id string, set func(calls []domain.ContextCall)) {
	for i := range run.Context.Invocations {
		if run.Context.Invocations[i].Actor.Label() == id {
			set(run.Context.Invocations[i].Calls)
		}
	}
}

// Growth of the singular calls of coder#1 and builder#1.
const (
	sessMeasuredGrowth = int64(12_345)
	sessCoderGrowth    = int64(7_000)
)

// sessionRun is the Context run with dispatches, cost splits and per-call tools.
func sessionRun() domain.RunReport {
	run := contextRun()
	run.Totals.Money.Total = known(10_000)
	run.WallTime.Span = domain.TimeInterval{Start: ctxBase, End: ctxBase.Add(3725 * time.Second)}
	run.WallTime.HasSpan = true

	run.Orchestrator = domain.ActorReport{
		Actor: domain.Orchestrator(), Models: []domain.ModelID{"model-o"}, Totals: sessSplit(1000, 1200, 1400, 400),
	}
	dispatches := []domain.Dispatch{
		sessDispatch(ctxAgent("builder#1", "builder"), "BLOCKED", 125, []domain.ModelID{"model-b"}, sessHighEffort, sessSplit(150, 250, 400, 200)),
		sessDispatch(ctxAgent("coder#1", "coder"), "SUCCESS", 60, []domain.ModelID{"model-c"}, nil, sessSplit(500, 700, 900, 400)),
		sessDispatch(ctxAgent("ghost#1", "ghost"), "", 0, nil, nil, sessNoData()),
		sessDispatch(ctxAgent("reviewer#1", "reviewer"), "PARTIALLY_DONE", 90, []domain.ModelID{"model-r"}, nil, sessSplit(1000, 980, 8, 12)),
		sessDispatch(ctxAgent("tester#1", ""), "", 30, []domain.ModelID{"model-t"}, nil, sessUnpriced()),
	}
	run.Agents, run.ByAgentType = nil, nil
	for _, d := range dispatches {
		run.Agents = append(run.Agents, domain.ActorReport{Actor: d.Invocation.Actor, Models: d.Models, Totals: d.Totals, Effort: d.Effort})
		run.ByAgentType = append(run.ByAgentType, domain.AgentTypeGroup{
			AgentType: d.Invocation.Actor.Type, Figures: domain.GroupFigures{Invocations: 1, Totals: d.Totals},
			PerInvocation: stats(1, 1, 1, 1), Dispatches: []domain.Dispatch{d},
		})
	}

	// builder#1, calls at 10s, 70s, 130s and 190s.
	withSessionCalls(&run, "builder#1", func(c []domain.ContextCall) {
		c[0].RequestedTools = domain.NewRequestedTools(map[string]int{"read_file": 2, "glob": 1})
		c[1].RequestedTools = sessOneTool("edit")
		c[1].Growth = domain.CallGrowth{State: domain.GrowthMeasured, Tool: "edit", Tokens: sessMeasuredGrowth}
		c[2].RequestedTools = domain.NewRequestedTools(nil)
		// c[3] stays not provided.
	})
	// coder#1, calls at 30s, 90s, 150s, 210s and 270s.
	withSessionCalls(&run, "coder#1", func(c []domain.ContextCall) {
		c[0].RequestedTools = sessOneTool("bash")
		c[0].Growth = domain.CallGrowth{State: domain.GrowthNegative, Tool: "bash", Tokens: -500}
		c[1].RequestedTools = sessOneTool("bash")
		c[1].Growth = domain.CallGrowth{State: domain.GrowthNotMeasurable, Tool: "bash"}
		c[2].RequestedTools = sessOneTool("glob")
		c[2].Growth = domain.CallGrowth{State: domain.GrowthMeasured, Tool: "glob", Tokens: sessCoderGrowth}
		c[3].RequestedTools = domain.NewRequestedTools(map[string]int{"read_file": 1, "glob": 1})
		// c[4] stays not provided.
	})
	return run
}

// Clock seconds of the first call of each actor of the standard run.
var sessFirstCall = map[string]int{
	"orchestrator": 0, "builder#1": 10, "coder#1": 30, "reviewer#1": 20, "tester#1": 40,
}

// sessRef is the actor reference of one actor of the standard run.
func sessRef(id string) domain.ActorRef {
	switch id {
	case "orchestrator":
		return domain.Orchestrator()
	case "builder#1":
		return ctxAgent(id, "builder")
	case "tester#1":
		return ctxAgent(id, "")
	}
	return ctxAgent(id, strings.TrimSuffix(id, "#1"))
}

// sessOpenFrom opens the Session screen of an actor with colour off.
func sessOpenFrom(t *testing.T, run domain.RunReport, id string, opener screens.ScreenID, w, h int) *screens.SessionScreen {
	t.Helper()
	colourOff(t)
	return screens.NewSessionScreen(run, sessRef(id), opener, w, h, paletteStyles())
}

// sessOpen opens the Session screen of an actor of the standard run, as from Context.
func sessOpen(t *testing.T, id string) *screens.SessionScreen {
	t.Helper()
	return sessOpenFrom(t, sessionRun(), id, screens.ScreenContext, tableW, tallH)
}

// sessView is the styling-free view of the actor's Session screen.
func sessView(t *testing.T, id string) string {
	t.Helper()
	return stripStyling(sessOpen(t, id).View(tableW, tallH))
}

// sessAbove is the part of the view above the first per-call row: the head, the cost
// split and the curve, without the frame's header and tab bar. A view without per-call rows is returned whole.
func sessAbove(view string, firstCallSec int) string {
	label := ctxAt(firstCallSec).Format("15:04:05")
	lines := viewLines(view)
	if len(lines) > screens.FrameChromeLines {
		lines = lines[2:] // the frame's header and tab bar are not part of the screen
	}
	for i, l := range lines {
		if strings.Contains(l, label) {
			return strings.Join(lines[:i], "\n")
		}
	}
	return strings.Join(lines, "\n")
}

// selectedLine is the line the table cursor marks, or "".
func selectedLine(view string) string {
	for _, l := range viewLines(view) {
		if strings.HasPrefix(strings.TrimSpace(l), ">") {
			return l
		}
	}
	return ""
}
