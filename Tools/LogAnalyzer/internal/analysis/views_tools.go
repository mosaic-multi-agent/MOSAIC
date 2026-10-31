package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// harnessedCall is a tool call with the harness of the actor that made it.
type harnessedCall struct {
	call    domain.ToolCall
	harness domain.Harness
}

func tagCalls(calls []domain.ToolCall, h domain.Harness) []harnessedCall {
	out := make([]harnessedCall, len(calls))
	for i, c := range calls {
		out[i] = harnessedCall{call: c, harness: h}
	}
	return out
}

// toolFigures computes exact per-tool figures: tools ascending by name, the
// unnamed tool last. Durations come only from paired calls with usable timestamps.
// Each row lists the distinct non-empty harnesses of its calls, sorted. A tool
// seen only through context growth gets a row with no calls.
func toolFigures(calls []harnessedCall, growth map[string]*growthAcc) []domain.ToolFigures {
	index := make(map[string]int)
	seen := make(map[string]map[domain.Harness]bool)
	var out []domain.ToolFigures
	for _, hc := range calls {
		c := hc.call
		i, ok := index[c.ToolName]
		if !ok {
			i = len(out)
			index[c.ToolName] = i
			out = append(out, domain.ToolFigures{Tool: c.ToolName})
			seen[c.ToolName] = map[domain.Harness]bool{}
		}
		f := &out[i]
		f.Calls++
		if c.Status == domain.ToolCallError {
			f.Errors++
		}
		if c.Pairing != domain.ToolCallPaired {
			f.Unpaired++
		}
		if d, ok := c.Duration(); ok {
			addDuration(&f.Durations, d)
		}
		if hc.harness != "" && !seen[c.ToolName][hc.harness] {
			seen[c.ToolName][hc.harness] = true
			f.Harnesses = append(f.Harnesses, hc.harness)
		}
	}
	for tool := range growth {
		if _, ok := index[tool]; !ok {
			index[tool] = len(out)
			out = append(out, domain.ToolFigures{Tool: tool})
		}
	}
	for i := range out {
		if acc := growth[out[i].Tool]; acc != nil {
			out[i].Growth = acc.ToolGrowth
		}
		if n := out[i].Durations.Count; n > 0 {
			out[i].Durations.Average = out[i].Durations.Total / time.Duration(n)
		}
		sort.Slice(out[i].Harnesses, func(a, b int) bool { return out[i].Harnesses[a] < out[i].Harnesses[b] })
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Tool, out[j].Tool
		if (a == "") != (b == "") {
			return b == ""
		}
		return a < b
	})
	return out
}

func addDuration(s *domain.DurationStats, d time.Duration) {
	if s.Count == 0 || d < s.Min {
		s.Min = d
	}
	if s.Count == 0 || d > s.Max {
		s.Max = d
	}
	s.Count++
	s.Total += d
}

// buildToolsBreakdown builds the tools view for the run, its orchestrator and
// each agent type that made tool calls. Harnesses come from the same actor
// attribution the wall-time split uses.
func buildToolsBreakdown(run domain.RunAggregate, groups []typeGroup, rc domain.RunContext) domain.ToolsBreakdown {
	harnesses := resolveActorHarnesses(run)
	orch := tagCalls(run.Activity.OrchestratorToolCalls, harnesses.orchestrator)
	all := append([]harnessedCall(nil), orch...)
	for _, inv := range run.Activity.Invocations {
		all = append(all, tagCalls(inv.ToolCalls, harnesses.of(inv.Actor))...)
	}
	allActors := append([]domain.ActorContext{rc.Orchestrator}, rc.Invocations...)
	b := domain.ToolsBreakdown{
		Run:          toolFigures(all, growthByTool(allActors)),
		Orchestrator: toolFigures(orch, growthByTool([]domain.ActorContext{rc.Orchestrator})),
	}
	for _, g := range groups {
		var calls []harnessedCall
		for _, d := range g.dispatches {
			calls = append(calls, tagCalls(d.Invocation.ToolCalls, harnesses.of(d.Invocation.Actor))...)
		}
		var actors []domain.ActorContext
		for _, a := range rc.Invocations {
			if a.Actor.Type == g.agentType {
				actors = append(actors, a)
			}
		}
		tools := toolFigures(calls, growthByTool(actors))
		if len(tools) == 0 {
			continue
		}
		b.ByAgentType = append(b.ByAgentType, domain.AgentTypeTools{AgentType: g.agentType, Tools: tools})
	}
	return b
}
