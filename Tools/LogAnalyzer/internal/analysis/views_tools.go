package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// toolFigures computes exact per-tool figures: tools ascending by name, the
// unnamed tool last. Durations come only from paired calls with usable timestamps.
func toolFigures(calls []domain.ToolCall) []domain.ToolFigures {
	index := make(map[string]int)
	var out []domain.ToolFigures
	for _, c := range calls {
		i, ok := index[c.ToolName]
		if !ok {
			i = len(out)
			index[c.ToolName] = i
			out = append(out, domain.ToolFigures{Tool: c.ToolName})
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
	}
	for i := range out {
		if n := out[i].Durations.Count; n > 0 {
			out[i].Durations.Average = out[i].Durations.Total / time.Duration(n)
		}
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
// each agent type that made tool calls.
func buildToolsBreakdown(act domain.RunActivity, groups []typeGroup) domain.ToolsBreakdown {
	all := append([]domain.ToolCall(nil), act.OrchestratorToolCalls...)
	for _, inv := range act.Invocations {
		all = append(all, inv.ToolCalls...)
	}
	b := domain.ToolsBreakdown{
		Run:          toolFigures(all),
		Orchestrator: toolFigures(act.OrchestratorToolCalls),
	}
	for _, g := range groups {
		var calls []domain.ToolCall
		for _, d := range g.dispatches {
			calls = append(calls, d.Invocation.ToolCalls...)
		}
		if len(calls) == 0 {
			continue
		}
		b.ByAgentType = append(b.ByAgentType, domain.AgentTypeTools{AgentType: g.agentType, Tools: toolFigures(calls)})
	}
	return b
}
