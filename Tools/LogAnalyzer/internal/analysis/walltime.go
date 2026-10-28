package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// buildWallTime splits the run's logged span (the timeline axis) into waiting on
// the user, subagents working, orchestrator only and between sessions. Each
// instant counts once, in that order of precedence.
func buildWallTime(run domain.RunAggregate, settings domain.AnalysisSettings, axis domain.TimeInterval, hasAxis bool) domain.WallTimeSplit {
	harnesses := resolveActorHarnesses(run)
	w := &waitCollector{settings: settings, counted: map[domain.CountedTool]int{}}
	w.addToolCalls(domain.Orchestrator(), harnesses.orchestrator, run.Activity.OrchestratorToolCalls)
	for _, inv := range run.Activity.Invocations {
		w.addToolCalls(inv.Actor, harnesses.of(inv.Actor), inv.ToolCalls)
	}
	w.addNotifications(run.Notifications)

	split := domain.WallTimeSplit{
		Span:         axis,
		HasSpan:      hasAxis,
		CountedTools: w.countedTools(),
		OpenWaits:    w.openWaits(),
		SessionGaps:  sessionGaps(run.Sessions),
	}
	split.UnconfiguredTools = unconfiguredTools(run, settings, harnesses)
	split.MissingContactToolsHint = len(split.UnconfiguredTools) > 0
	if !hasAxis {
		return split
	}
	waits := clipIntervals(mergeIntervals(w.intervals), axis)
	subagents := subtractIntervals(clipIntervals(subagentIntervals(run.Activity.Invocations, axis.End), axis), waits)
	gaps := subtractIntervals(subtractIntervals(clipIntervals(mergeIntervals(split.SessionGaps), axis), waits), subagents)

	split.WaitIntervals = waits
	split.Waiting = totalIntervals(waits)
	split.Subagents = totalIntervals(subagents)
	split.BetweenSessions = totalIntervals(gaps)
	split.OrchestratorOnly = axis.End.Sub(axis.Start) - split.Waiting - split.Subagents - split.BetweenSessions
	return split
}

// subagentIntervals returns the merged union of the invocations' intervals; an
// open invocation runs to axisEnd.
func subagentIntervals(invs []domain.Invocation, axisEnd time.Time) []domain.TimeInterval {
	var all []domain.TimeInterval
	for _, inv := range invs {
		if iv, ok := invocationInterval(inv, axisEnd); ok {
			all = append(all, iv)
		}
	}
	return mergeIntervals(all)
}

// unconfiguredTools lists the distinct tool names that are neither configured
// (user-interaction or write-class) nor built in for the actor's harness. A
// harness without a captured built-in list is skipped.
func unconfiguredTools(run domain.RunAggregate, settings domain.AnalysisSettings, harnesses actorHarnesses) []string {
	seen := map[string]bool{}
	check := func(h domain.Harness, calls []domain.ToolCall) {
		builtin := domain.HarnessBuiltinTools(h)
		if len(builtin) == 0 {
			return
		}
		for _, c := range calls {
			if c.ToolName == "" || settings.IsUserInteractionTool(h, c.ToolName) || settings.IsWriteTool(h, c.ToolName) || containsString(builtin, c.ToolName) {
				continue
			}
			seen[c.ToolName] = true
		}
	}
	check(harnesses.orchestrator, run.Activity.OrchestratorToolCalls)
	for _, inv := range run.Activity.Invocations {
		check(harnesses.of(inv.Actor), inv.ToolCalls)
	}
	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
