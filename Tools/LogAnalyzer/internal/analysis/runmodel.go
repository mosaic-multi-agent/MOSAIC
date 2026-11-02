package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// toolEvent is one tool_call_start or tool_call_end as observed, in stream
// order. Only the fields the run model reports are kept; payload bodies never
// are.
type toolEvent struct {
	callID string
	isEnd  bool
	name   string
	status domain.ToolCallStatus
	at     time.Time
}

// instanceActivity is the run-model state of one actor stream: the tool-call
// events it logged and, for subagent instances, the invocation lifecycle.
type instanceActivity struct {
	tools []toolEvent

	started   bool
	startedAt time.Time // first invocation_start timestamp as logged
	endEvents int
	endedAt   time.Time
	status    string
	model     domain.ModelID
	provider  domain.ProviderID
}

// runActivityState accumulates the run model of one run.
type runActivityState struct {
	orchestrator instanceActivity
	instances    map[domain.AgentInstanceID]*instanceActivity
	compactions  []compactionFeedEntry // feed order
}

// activityState returns the run's activity state, creating it on first use.
func (rs *runState) activityState() *runActivityState {
	if rs.activity == nil {
		rs.activity = &runActivityState{instances: make(map[domain.AgentInstanceID]*instanceActivity)}
	}
	return rs.activity
}

func (ra *runActivityState) instance(id domain.AgentInstanceID) *instanceActivity {
	ia, ok := ra.instances[id]
	if !ok {
		ia = &instanceActivity{}
		ra.instances[id] = ia
	}
	return ia
}

// recordToolOrCompaction feeds tool-call and compaction events into the run
// model. It reports whether ev was one of those types; such events carry no
// usage and are not subject to any other attribution rule.
func (a *Aggregator) recordToolOrCompaction(rs *runState, ref domain.StreamRef, ev domain.Event) bool {
	switch ev.Type {
	case domain.EventToolCallStart, domain.EventToolCallEnd, domain.EventCompaction:
	default:
		return false
	}

	var target *instanceActivity
	actor := domain.Orchestrator()
	switch ref.Kind {
	case domain.StreamOrchestrator:
		target = &rs.activityState().orchestrator
	case domain.StreamAgentInstance:
		// Events are attributed to the stream's instance hint. A stream with
		// no identity cannot be attributed: the event is left out of the model
		// and the gap is reported.
		if ref.InstanceHint == "" {
			a.findings = append(a.findings, domain.Finding{
				Kind:     domain.FindingUnidentifiedEvent,
				Severity: domain.SeverityWarning,
				Run:      ref.Run,
				Detail:   string(ev.Type) + " with no usable agent_instance_id",
			})
			return true
		}
		target = rs.activityState().instance(ref.InstanceHint)
		actor = domain.AgentInstance(ref.InstanceHint)
	default:
		return true
	}
	ra := rs.activityState()

	switch {
	case ev.ToolCallStart != nil:
		target.tools = append(target.tools, toolEvent{
			callID: ev.ToolCallStart.CallID, name: ev.ToolCallStart.ToolName, at: ev.Timestamp,
		})
	case ev.ToolCallEnd != nil:
		target.tools = append(target.tools, toolEvent{
			callID: ev.ToolCallEnd.CallID, name: ev.ToolCallEnd.ToolName, isEnd: true, status: ev.ToolCallEnd.Status, at: ev.Timestamp,
		})
	case ev.Compaction != nil:
		ra.compactions = append(ra.compactions, newCompactionFeedEntry(ref, actor, ev))
	}
	return true
}

// recordInvocationStart notes an invocation_start for the resolved instance.
func (rs *runState) recordInvocationStart(id domain.AgentInstanceID, at time.Time) {
	ia := rs.activityState().instance(id)
	if !ia.started {
		ia.started = true
		ia.startedAt = at
	}
}

// recordInvocationEnd notes an invocation_end for the resolved instance. The
// last end wins for status, model and end time; absence on the last end is
// kept rather than filled from an earlier one.
func (rs *runState) recordInvocationEnd(id domain.AgentInstanceID, end *domain.InvocationEndFields, at time.Time) {
	ia := rs.activityState().instance(id)
	ia.endEvents++
	ia.endedAt = at
	ia.status = end.StatusCode
	ia.model = end.Model
	ia.provider = end.Provider
}

// buildActivity converts the accumulated state into the run model. It reads
// the usage and agent type from the already-built per-actor state so both
// views always agree.
func buildActivity(rs *runState) domain.RunActivity {
	var out domain.RunActivity
	ra := rs.activity

	ids := make(map[domain.AgentInstanceID]bool, len(rs.agents))
	for id := range rs.agents {
		ids[id] = true
	}
	if ra != nil {
		for id := range ra.instances {
			ids[id] = true
		}
		out.OrchestratorToolCalls = pairToolCalls(ra.orchestrator.tools)
		out.Compactions = sortedCompactions(pairCompactionFeed(ra.compactions))
	}

	sorted := make([]domain.AgentInstanceID, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	for _, id := range sorted {
		inv := domain.Invocation{Actor: domain.AgentInstance(id), State: domain.InvocationOpen}
		if as, ok := rs.agents[id]; ok {
			inv.Actor = as.actor
			inv.Usage = as.effective()
		}
		if ra != nil {
			if ia, ok := ra.instances[id]; ok {
				applyInstanceActivity(&inv, ia)
			}
		}
		out.Invocations = append(out.Invocations, inv)
	}
	return out
}

func applyInstanceActivity(inv *domain.Invocation, ia *instanceActivity) {
	inv.StartedAt = ia.startedAt
	inv.ToolCalls = pairToolCalls(ia.tools)
	if ia.endEvents == 0 {
		return
	}
	inv.State = domain.InvocationClosed
	inv.EndEvents = ia.endEvents
	inv.EndedAt = ia.endedAt
	inv.StatusCode = ia.status
	inv.Model = ia.model
	inv.Provider = ia.provider
}

// pairToolCalls pairs starts and ends by call_id. A call pairs only when its
// non-empty id has exactly one start and one end in the stream; anything else
// stays unpaired. Calls are ordered by the position of their first event.
func pairToolCalls(events []toolEvent) []domain.ToolCall {
	if len(events) == 0 {
		return nil
	}
	type slots struct{ start, end, starts, ends int }
	byID := make(map[string]*slots)
	for i, e := range events {
		if e.callID == "" {
			continue
		}
		s := byID[e.callID]
		if s == nil {
			s = &slots{}
			byID[e.callID] = s
		}
		if e.isEnd {
			s.end, s.ends = i, s.ends+1
		} else {
			s.start, s.starts = i, s.starts+1
		}
	}

	calls := make([]domain.ToolCall, 0, len(events))
	for i, e := range events {
		if s := byID[e.callID]; e.callID != "" && s.starts == 1 && s.ends == 1 {
			if i == min(s.start, s.end) {
				calls = append(calls, pairedCall(events[s.start], events[s.end]))
			}
			continue
		}
		calls = append(calls, unpairedCall(e))
	}
	return calls
}

func pairedCall(start, end toolEvent) domain.ToolCall {
	return domain.ToolCall{
		CallID:    start.callID,
		ToolName:  start.name,
		Status:    end.status,
		StartedAt: start.at,
		EndedAt:   end.at,
		Pairing:   domain.ToolCallPaired,
	}
}

func unpairedCall(e toolEvent) domain.ToolCall {
	if e.isEnd {
		return domain.ToolCall{CallID: e.callID, ToolName: e.name, Status: e.status, EndedAt: e.at, Pairing: domain.ToolCallEndOnly}
	}
	return domain.ToolCall{CallID: e.callID, ToolName: e.name, StartedAt: e.at, Pairing: domain.ToolCallStartOnly}
}

// sortedCompactions orders compactions by time (zero time last), then actor
// label, keeping feed order for ties.
func sortedCompactions(in []domain.Compaction) []domain.Compaction {
	if len(in) == 0 {
		return nil
	}
	out := append([]domain.Compaction(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.At.IsZero() != b.At.IsZero() {
			return !a.At.IsZero()
		}
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		return a.Actor.Label() < b.Actor.Label()
	})
	return out
}
