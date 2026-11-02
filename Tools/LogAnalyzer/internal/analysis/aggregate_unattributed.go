package analysis

import "mosaic-log-analyzer/internal/domain"

// addUnattributed handles one event of a run's unattributed stream. Only
// usage_record events count; they follow the same single-stream and supersede
// rules as every other usage_record, but carry no agent identity, so any
// agent_instance_id on them is not read. The stream feeds neither the run
// facts nor the run model.
func (a *Aggregator) addUnattributed(rs *runState, ref domain.StreamRef, ev domain.Event) {
	if ev.Type != domain.EventUsageRecord {
		return
	}
	if rs.unattributed == nil {
		rs.unattributed = &actorState{actor: domain.Unattributed()}
	}
	a.handleUsageRecord(rs, ref, ev, rs.unattributed)
}

// unattributedAggregate returns the run's unattributed usage, or nil when the
// run has none.
func unattributedAggregate(rs *runState) *domain.ActorAggregate {
	if rs.unattributed == nil {
		return nil
	}
	return &domain.ActorAggregate{
		Actor:       rs.unattributed.actor,
		Invocations: rs.unattributed.effective(),
	}
}
