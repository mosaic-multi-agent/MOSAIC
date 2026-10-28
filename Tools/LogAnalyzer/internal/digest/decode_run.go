package digest

import (
	"mosaic-log-analyzer/internal/domain"
)

func convertRun(rd runDoc, ref domain.RunRef, version string) (domain.RunAggregate, error) {
	run := domain.RunAggregate{
		Run:         ref,
		Provisional: rd.Provisional,
		Facts: domain.RunFacts{
			SchemaVersions:       rd.SchemaVersions,
			AdapterVersion:       rd.AdapterVersion,
			OrchestratorModel:    domain.ModelID(rd.OrchestratorModel),
			OrchestratorProvider: domain.ProviderID(rd.OrchestratorProvider),
			StartedAt:            parseTime(rd.StartedAt),
			EndedAt:              parseTime(rd.EndedAt),
			Outcome:              rd.Outcome,
		},
		Orchestrator: domain.ActorAggregate{
			Actor:       domain.Orchestrator(),
			Invocations: decodeUsages(rd.Orchestrator.Usage),
		},
	}
	for _, h := range rd.Harnesses {
		run.Facts.Harnesses = append(run.Facts.Harnesses, domain.Harness(h))
	}
	for _, s := range rd.Sessions {
		run.Sessions = append(run.Sessions, decodeSession(s))
	}
	calls, err := decodeToolCalls(rd.Orchestrator.ToolCalls)
	if err != nil {
		return run, err
	}
	run.Activity.OrchestratorToolCalls = calls

	if rd.Unattributed != nil {
		run.Unattributed = &domain.ActorAggregate{
			Actor:       domain.Unattributed(),
			Invocations: decodeUsages(rd.Unattributed.Usage),
		}
	}

	agents := make(map[string]domain.ActorAggregate, len(rd.Agents))
	for _, a := range rd.Agents {
		agg := domain.ActorAggregate{
			Actor: domain.ActorRef{
				Kind:     domain.ActorAgentInstance,
				Instance: domain.AgentInstanceID(a.InstanceID),
				Type:     a.AgentType,
			},
			Invocations: decodeUsages(a.Usage),
		}
		run.Agents = append(run.Agents, agg)
		if _, dup := agents[a.InstanceID]; !dup {
			agents[a.InstanceID] = agg
		}
	}
	for _, id := range rd.Invocations {
		inv, err := decodeInvocation(id, agents)
		if err != nil {
			return run, err
		}
		run.Activity.Invocations = append(run.Activity.Invocations, inv)
	}
	return run, decodeEvents(rd, &run, version)
}

// decodeEvents fills turns, notifications and compactions.
func decodeEvents(rd runDoc, run *domain.RunAggregate, version string) error {
	for _, t := range rd.Turns {
		actor, err := decodeActor(t.Actor)
		if err != nil {
			return err
		}
		run.Turns = append(run.Turns, domain.Turn{Actor: actor, Role: domain.TurnRole(t.Role), At: parseTime(t.At)})
	}
	for _, n := range rd.Notifications {
		actor, err := decodeActor(n.Actor)
		if err != nil {
			return err
		}
		run.Notifications = append(run.Notifications, domain.Notification{
			Actor:            actor,
			At:               parseTime(n.At),
			Type:             n.NotificationType,
			RequiresResponse: decodeBool(n.RequiresResponse),
			Resolution:       n.Resolution,
			ID:               n.NotificationID,
		})
	}
	for _, c := range rd.Compactions {
		actor, err := decodeActor(c.Actor)
		if err != nil {
			return err
		}
		shape, err := decodeShape(c.Shape)
		if err != nil {
			return err
		}
		run.Activity.Compactions = append(run.Activity.Compactions, domain.Compaction{
			Actor:        actor,
			At:           parseTime(c.At),
			Trigger:      c.Trigger,
			TokensBefore: decodeCount(c.TokensBefore),
			TokensAfter:  decodeCount(c.TokensAfter),
			EndedAt:      parseTime(c.EndedAt),
			Shape:        shape,
		})
	}
	if !storesCompactionShape(version) {
		run.Activity.Compactions = pairStoredCompactions(run.Activity.Compactions)
	}
	return nil
}

// decodeShape maps a wire shape; absent decodes as single.
func decodeShape(s string) (domain.CompactionShape, error) {
	switch s {
	case "", shapeSingle:
		return domain.CompactionSingle, nil
	case shapePaired:
		return domain.CompactionPaired, nil
	case shapePreOnly:
		return domain.CompactionPreOnly, nil
	case shapePostOnly:
		return domain.CompactionPostOnly, nil
	}
	return domain.CompactionSingle, invalidf("a compaction has an unknown shape")
}

// pairStoredCompactions applies the legacy pairing rule to compactions of a
// digest written before shapes were stored. Each actor's entries, in stored
// order, are treated as legacy events of one stream; a pair replaces its first
// entry and drops its second.
func pairStoredCompactions(stored []domain.Compaction) []domain.Compaction {
	byActor := make(map[domain.ActorRef][]int)
	for i, c := range stored {
		byActor[c.Actor] = append(byActor[c.Actor], i)
	}
	drop := make(map[int]bool)
	replace := make(map[int]domain.Compaction)
	for actor, idx := range byActor {
		evs := make([]domain.CompactionEvent, len(idx))
		for k, i := range idx {
			c := stored[i]
			evs[k] = domain.CompactionEvent{
				SchemaVersion: "1.0.0",
				At:            c.At,
				Fields:        domain.CompactionFields{Trigger: c.Trigger, TokensBefore: c.TokensBefore, TokensAfter: c.TokensAfter},
			}
		}
		for _, pc := range domain.PairCompactionEvents(evs) {
			if pc.Second < 0 {
				continue
			}
			c := pc.Compaction
			c.Actor = actor
			replace[idx[pc.First]] = c
			drop[idx[pc.Second]] = true
		}
	}
	out := make([]domain.Compaction, 0, len(stored))
	for i, c := range stored {
		if drop[i] {
			continue
		}
		if p, ok := replace[i]; ok {
			c = p
		}
		out = append(out, c)
	}
	return out
}

func decodeSession(s sessionDoc) domain.RunSession {
	return domain.RunSession{
		ID:        s.SessionID,
		StartedAt: parseTime(s.StartedAt),
		EndedAt:   parseTime(s.EndedAt),
		Resumed:   decodeBool(s.Resumed),
		Reason:    s.Reason,
		Model:     domain.ModelID(s.Model),
		Provider:  domain.ProviderID(s.Provider),
	}
}

// decodeInvocation reads an invocation; its actor and usage come from the agents
// entry with the same instance id when there is one.
func decodeInvocation(id invocationDoc, agents map[string]domain.ActorAggregate) (domain.Invocation, error) {
	inv := domain.Invocation{
		Actor: domain.ActorRef{
			Kind:     domain.ActorAgentInstance,
			Instance: domain.AgentInstanceID(id.InstanceID),
			Type:     id.AgentType,
		},
		StatusCode: id.StatusCode,
		Model:      domain.ModelID(id.Model),
		Provider:   domain.ProviderID(id.Provider),
		StartedAt:  parseTime(id.StartedAt),
		EndedAt:    parseTime(id.EndedAt),
		EndEvents:  id.EndEvents,
	}
	switch id.State {
	case stateOpen:
		inv.State = domain.InvocationOpen
	case stateClosed:
		inv.State = domain.InvocationClosed
	default:
		return inv, invalidf("an invocation has a missing or unknown state")
	}
	if agent, ok := agents[id.InstanceID]; ok {
		inv.Actor = agent.Actor
		inv.Usage = agent.Invocations
	}
	calls, err := decodeToolCalls(id.ToolCalls)
	if err != nil {
		return inv, err
	}
	inv.ToolCalls = calls
	return inv, nil
}

func decodeToolCalls(docs []toolCallDoc) ([]domain.ToolCall, error) {
	var calls []domain.ToolCall
	for _, c := range docs {
		call := domain.ToolCall{
			CallID:    c.CallID,
			ToolName:  c.ToolName,
			Status:    domain.ToolCallStatus(c.Status),
			StartedAt: parseTime(c.StartedAt),
			EndedAt:   parseTime(c.EndedAt),
		}
		switch c.Pairing {
		case pairingPaired:
			call.Pairing = domain.ToolCallPaired
		case pairingStartOnly:
			call.Pairing = domain.ToolCallStartOnly
		case pairingEndOnly:
			call.Pairing = domain.ToolCallEndOnly
		default:
			return nil, invalidf("a tool call has a missing or unknown pairing")
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func decodeUsages(docs []usageDoc) []domain.InvocationUsage {
	var usages []domain.InvocationUsage
	for _, u := range docs {
		entry := domain.InvocationUsage{
			Model:       domain.ModelID(u.Model),
			Provider:    domain.ProviderID(u.Provider),
			Effort:      u.Effort,
			Harness:     domain.Harness(u.Harness),
			RecordID:    u.RecordID,
			At:          parseTime(u.At),
			ServiceTier: u.ServiceTier,
		}
		if t := u.Tokens; t != nil {
			entry.Usage = domain.TokenUsage{
				Input:           decodeCount(t.Input),
				CacheRead:       decodeCount(t.CacheRead),
				CacheCreation:   decodeCount(t.CacheCreation),
				Output:          decodeCount(t.Output),
				CacheCreation5m: decodeCount(t.CacheCreation5m),
				CacheCreation1h: decodeCount(t.CacheCreation1h),
				Reasoning:       decodeCount(t.Reasoning),
			}
		}
		usages = append(usages, entry)
	}
	return usages
}

func decodeCount(p *int64) domain.TokenCount {
	if p == nil {
		return domain.AbsentTokens()
	}
	return domain.Tokens(*p)
}

func decodeBool(p *bool) domain.OptionalBool {
	if p == nil {
		return domain.BoolAbsent
	}
	return domain.BoolOf(*p)
}
