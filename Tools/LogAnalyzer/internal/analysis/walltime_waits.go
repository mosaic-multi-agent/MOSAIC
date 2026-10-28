package analysis

import (
	"sort"
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

// waitCollector gathers the run's waits on the user.
type waitCollector struct {
	settings  domain.AnalysisSettings
	intervals []domain.TimeInterval
	open      []domain.OpenWait
	counted   map[domain.CountedTool]int
}

// actorHarnesses maps each actor to the harness named by its usage records;
// an actor without one takes the run's only harness, or "" when the run has
// several.
type actorHarnesses struct {
	orchestrator domain.Harness
	agents       map[domain.AgentInstanceID]domain.Harness
}

func firstHarness(usage []domain.InvocationUsage) domain.Harness {
	for _, u := range usage {
		if u.Harness != "" {
			return u.Harness
		}
	}
	return ""
}

func resolveActorHarnesses(run domain.RunAggregate) actorHarnesses {
	var fallback domain.Harness
	if len(run.Facts.Harnesses) == 1 {
		fallback = run.Facts.Harnesses[0]
	}
	pick := func(usage []domain.InvocationUsage) domain.Harness {
		if h := firstHarness(usage); h != "" {
			return h
		}
		return fallback
	}
	out := actorHarnesses{orchestrator: pick(run.Orchestrator.Invocations), agents: map[domain.AgentInstanceID]domain.Harness{}}
	for _, a := range run.Agents {
		out.agents[a.Actor.Instance] = pick(a.Invocations)
	}
	for _, inv := range run.Activity.Invocations {
		if _, ok := out.agents[inv.Actor.Instance]; !ok {
			out.agents[inv.Actor.Instance] = pick(inv.Usage)
		}
	}
	return out
}

func (h actorHarnesses) of(actor domain.ActorRef) domain.Harness {
	if actor.Kind == domain.ActorAgentInstance {
		if v, ok := h.agents[actor.Instance]; ok {
			return v
		}
	}
	return h.orchestrator
}

// addToolCalls records the user-interaction calls among calls.
func (w *waitCollector) addToolCalls(actor domain.ActorRef, h domain.Harness, calls []domain.ToolCall) {
	for _, c := range calls {
		if !w.settings.IsUserInteractionTool(h, c.ToolName) {
			continue
		}
		w.counted[domain.CountedTool{Tool: c.ToolName, Harness: h}]++
		if _, ok := c.Duration(); !ok {
			w.open = append(w.open, domain.OpenWait{Actor: actor, At: c.StartedAt, Kind: domain.OpenWaitToolCall, Tool: c.ToolName})
			continue
		}
		w.intervals = append(w.intervals, domain.TimeInterval{Start: c.StartedAt, End: c.EndedAt})
	}
}

func isResolutionType(typ string) bool {
	return strings.HasSuffix(typ, "_resolved") || strings.HasSuffix(typ, "_complete") || strings.HasSuffix(typ, "_completed")
}

// opensWait reports whether a notification asks the user for something.
func opensWait(n domain.Notification) bool {
	if isResolutionType(n.Type) {
		return false
	}
	return strings.HasPrefix(n.Type, "permission") || strings.HasPrefix(n.Type, "elicitation") ||
		n.RequiresResponse == domain.BoolTrue
}

// addNotifications records waits from notifications. Only notifications that
// share a notification id pair into an interval (first to last time); a
// notification asking for a response without a partner is open-ended, and so is
// every idle_prompt.
func (w *waitCollector) addNotifications(notifs []domain.Notification) {
	byID := map[string][]domain.Notification{}
	var ids []string
	for _, n := range notifs {
		if n.Type == "idle_prompt" {
			w.open = append(w.open, domain.OpenWait{Actor: n.Actor, At: n.At, Kind: domain.OpenWaitIdlePrompt})
			continue
		}
		if n.ID == "" {
			w.addUnpaired(n)
			continue
		}
		if _, ok := byID[n.ID]; !ok {
			ids = append(ids, n.ID)
		}
		byID[n.ID] = append(byID[n.ID], n)
	}
	for _, id := range ids {
		w.addGroup(byID[id])
	}
}

func (w *waitCollector) addUnpaired(n domain.Notification) {
	if opensWait(n) {
		w.open = append(w.open, domain.OpenWait{Actor: n.Actor, At: n.At, Kind: domain.OpenWaitPermission})
	}
}

func (w *waitCollector) addGroup(group []domain.Notification) {
	opens := false
	for _, n := range group {
		opens = opens || opensWait(n)
	}
	if !opens {
		return
	}
	var first, last domain.Notification
	timed := 0
	for _, n := range group {
		if n.At.IsZero() {
			continue
		}
		if timed == 0 || n.At.Before(first.At) {
			first = n
		}
		if timed == 0 || n.At.After(last.At) {
			last = n
		}
		timed++
	}
	if len(group) < 2 || timed < 2 {
		for _, n := range group {
			w.addUnpaired(n)
		}
		return
	}
	w.intervals = append(w.intervals, domain.TimeInterval{Start: first.At, End: last.At})
}

// countedTools lists the counted tool names sorted by tool, then harness.
func (w *waitCollector) countedTools() []domain.CountedTool {
	out := make([]domain.CountedTool, 0, len(w.counted))
	for k, calls := range w.counted {
		k.Calls = calls
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tool != out[j].Tool {
			return out[i].Tool < out[j].Tool
		}
		return out[i].Harness < out[j].Harness
	})
	return out
}

// openWaits returns the open-ended waits sorted by time, zero times last.
func (w *waitCollector) openWaits() []domain.OpenWait {
	out := w.open
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].At, out[j].At
		if a.IsZero() != b.IsZero() {
			return !a.IsZero()
		}
		return a.Before(b)
	})
	return out
}
