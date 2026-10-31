package analysis

import "mosaic-log-analyzer/internal/domain"

// handleAgentUsageRecord processes a usage_record event on a subagent instance
// stream, resolving the actor identity (event field wins over folder hint,
// same as invocation_end) and routing it into that actor's per-record total.
func (a *Aggregator) handleAgentUsageRecord(rs *runState, ref domain.StreamRef, ev domain.Event) {
	if ev.UsageRecord == nil {
		return
	}

	rec := ev.UsageRecord

	id := rec.AgentInstanceID
	if id == "" {
		id = ref.InstanceHint
	}
	if id == "" {
		a.findings = append(a.findings, domain.Finding{
			Kind:     domain.FindingUnidentifiedEvent,
			Severity: domain.SeverityWarning,
			Run:      ref.Run,
			Detail:   "usage_record with no usable agent_instance_id",
		})
		return
	}

	as := a.getOrCreateActorState(rs, id)
	a.handleUsageRecord(rs, ref, ev, as)
}

// handleUsageRecord applies two rules in order for each usage_record event:
//
//  1. Single-stream check (run-scoped): for a non-empty record_id, consult
//     the run's ownership registry. If the record is absent, register this
//     actor as owner and proceed to rule 2. If the record is already owned by
//     this same actor, it is ordinary re-observation — proceed to rule 2. If
//     the record is owned by a different actor, raise exactly one
//     FindingCrossStreamRecord (the first time only) and return without
//     contributing the record to this actor's total.
//
//  2. Supersede (per-actor): for a non-empty record_id already held by this
//     actor, replace the stored entry in place (last observation wins),
//     preserving file order and record count. For a record_id not yet held,
//     append in file order. For an empty record_id, append and raise
//     FindingUnidentifiedEvent.
func (a *Aggregator) handleUsageRecord(rs *runState, ref domain.StreamRef, ev domain.Event, as *actorState) {
	rec := ev.UsageRecord
	if rec == nil {
		return
	}

	entry := domain.InvocationUsage{
		Model:    rec.Model,
		Provider: rec.Provider,
		Effort:   rec.Effort,
		Harness:  ev.Harness,
		Usage:    rec.Usage,
		RecordID: rec.RecordID, At: ev.Timestamp, ServiceTier: rec.ServiceTier,
		Kind:      callKindOrMain(rec.CallKind),
		Label:     rec.CallLabel,
		SessionID: ev.SessionID,

		RequestedTools: rec.RequestedTools,
	}

	if rec.RecordID == "" {
		// Empty record_id: cannot enter either registry or supersede map.
		a.findings = append(a.findings, domain.Finding{
			Kind:     domain.FindingUnidentifiedEvent,
			Severity: domain.SeverityWarning,
			Run:      ref.Run,
			Actor:    as.actor,
			Detail:   "usage_record with no record_id",
		})
		as.records = append(as.records, entry)
		return
	}

	// Rule 1: single-stream invariant check.
	if owner, owned := rs.recordOwners[rec.RecordID]; owned {
		if !sameActorIdentity(owner, as.actor) {
			// Cross-stream violation: raise at most one finding per record.
			if !rs.crossStreamReported[rec.RecordID] {
				rs.crossStreamReported[rec.RecordID] = true
				a.findings = append(a.findings, domain.Finding{
					Kind:     domain.FindingCrossStreamRecord,
					Severity: domain.SeverityWarning,
					Path:     ref.Path,
					Line:     0,
					Run:      ref.Run,
					Actor:    as.actor,
					Detail: "usage_record " + rec.RecordID +
						" already attributed to " + owner.Label() +
						"; duplicate on " + as.actor.Label() + " ignored",
				})
			}
			return
		}
		// Same actor: fall through to per-actor supersede.
	} else {
		// First time this record_id is seen in this run: register ownership.
		rs.recordOwners[rec.RecordID] = as.actor
	}

	// Rule 2: per-actor supersede.
	if as.recordIndex == nil {
		as.recordIndex = make(map[string]int)
	}
	if idx, seen := as.recordIndex[rec.RecordID]; seen {
		// Last observation wins: replace in place, preserving file order and count.
		as.records[idx] = entry
		return
	}

	// First observation for this actor: append and record its position.
	as.recordIndex[rec.RecordID] = len(as.records)
	as.records = append(as.records, entry)
}

// sameActorIdentity reports whether two ActorRefs identify the same stream
// owner. It compares only Kind and Instance, deliberately ignoring Type (the
// agent_type hint from invocation_start) which is not part of stream identity.
func sameActorIdentity(a, b domain.ActorRef) bool {
	return a.Kind == b.Kind && a.Instance == b.Instance
}

// callKindOrMain treats an unset kind as a main-loop call.
func callKindOrMain(k domain.CallKind) domain.CallKind {
	if k == "" {
		return domain.CallMain
	}
	return k
}
