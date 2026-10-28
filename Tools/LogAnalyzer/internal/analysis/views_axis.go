package analysis

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// runAxis is the run's logged span: the earliest and latest non-zero timestamp
// of its facts, sessions, invocations, tool calls, per-call usage, turns,
// notifications and compactions. ok is false when none was logged.
func runAxis(run domain.RunAggregate) (span domain.TimeInterval, ok bool) {
	add := func(ts ...time.Time) {
		for _, t := range ts {
			if t.IsZero() {
				continue
			}
			if !ok || t.Before(span.Start) {
				span.Start = t
			}
			if !ok || t.After(span.End) {
				span.End = t
			}
			ok = true
		}
	}
	addCalls := func(calls []domain.ToolCall) {
		for _, c := range calls {
			add(c.StartedAt, c.EndedAt)
		}
	}
	addUsage := func(usage []domain.InvocationUsage) {
		for _, u := range usage {
			add(u.At)
		}
	}
	add(run.Facts.StartedAt, run.Facts.EndedAt)
	for _, s := range run.Sessions {
		add(s.StartedAt, s.EndedAt)
	}
	for _, inv := range run.Activity.Invocations {
		add(inv.StartedAt, inv.EndedAt)
		addCalls(inv.ToolCalls)
	}
	addCalls(run.Activity.OrchestratorToolCalls)
	addUsage(run.Orchestrator.Invocations)
	for _, a := range run.Agents {
		addUsage(a.Invocations)
	}
	if run.Unattributed != nil {
		addUsage(run.Unattributed.Invocations)
	}
	for _, t := range run.Turns {
		add(t.At)
	}
	for _, n := range run.Notifications {
		add(n.At)
	}
	for _, c := range run.Activity.Compactions {
		add(c.At, c.EndedAt)
	}
	return span, ok
}
