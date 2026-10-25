package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// factsState accumulates the run-level facts, sessions, turns and notifications
// of one run. It only carries what the logs said; nothing here feeds pricing.
type factsState struct {
	harnesses      map[domain.Harness]bool
	schemaVersions map[string]bool

	adapterVersion string
	model          domain.ModelID
	provider       domain.ProviderID
	startedAt      time.Time
	endedAt        time.Time
	outcome        string

	sessionOrder []string
	sessions     map[string]*domain.RunSession
	sessionSeen  map[string]bool // session ids whose session_start timestamp is already fixed

	turns         []domain.Turn
	notifications []domain.Notification
}

// factsFor returns the run's facts state, creating it on first use.
func (rs *runState) factsFor() *factsState {
	if rs.facts == nil {
		rs.facts = &factsState{
			harnesses:      make(map[domain.Harness]bool),
			schemaVersions: make(map[string]bool),
			sessions:       make(map[string]*domain.RunSession),
			sessionSeen:    make(map[string]bool),
		}
	}
	return rs.facts
}

// recordFacts keeps the digest facts carried by ev. It runs for every event on
// every stream and never touches usage or findings.
func (a *Aggregator) recordFacts(rs *runState, ref domain.StreamRef, ev domain.Event) {
	fs := rs.factsFor()

	if ev.Harness != "" {
		fs.harnesses[ev.Harness] = true
	}
	if ev.SchemaVersion != "" {
		fs.schemaVersions[ev.SchemaVersion] = true
	}
	if ev.SessionID != "" {
		fs.session(ev.SessionID)
	}

	switch ev.Type {
	case domain.EventRunStart:
		fs.recordRunStart(ev)
	case domain.EventRunEnd:
		fs.recordRunEnd(ev)
	case domain.EventSessionStart:
		fs.recordSessionStart(ev)
	case domain.EventSessionEnd:
		fs.recordSessionEnd(ev)
	case domain.EventTurn:
		if actor, ok := streamActor(ref); ok && ev.Turn != nil {
			fs.turns = append(fs.turns, domain.Turn{Actor: actor, Role: ev.Turn.Role, At: ev.Timestamp})
		}
	case domain.EventNotification:
		if actor, ok := streamActor(ref); ok && ev.Notification != nil {
			n := ev.Notification
			fs.notifications = append(fs.notifications, domain.Notification{
				Actor:            actor,
				At:               ev.Timestamp,
				Type:             n.NotificationType,
				RequiresResponse: n.RequiresResponse,
				Resolution:       n.Resolution,
				ID:               n.NotificationID,
			})
		}
	}
}

// streamActor names the actor of a stream for turns and notifications. An
// agent stream without an instance hint has no identity and is skipped.
func streamActor(ref domain.StreamRef) (domain.ActorRef, bool) {
	switch ref.Kind {
	case domain.StreamOrchestrator:
		return domain.Orchestrator(), true
	case domain.StreamAgentInstance:
		if ref.InstanceHint != "" {
			return domain.AgentInstance(ref.InstanceHint), true
		}
	}
	return domain.ActorRef{}, false
}

func (fs *factsState) recordRunStart(ev domain.Event) {
	if !ev.Timestamp.IsZero() && (fs.startedAt.IsZero() || ev.Timestamp.Before(fs.startedAt)) {
		fs.startedAt = ev.Timestamp
	}
	rsf := ev.RunStart
	if rsf == nil {
		return
	}
	if rsf.AdapterVersion != "" {
		fs.adapterVersion = rsf.AdapterVersion
	}
	if !rsf.Model.IsEmpty() {
		fs.model = rsf.Model
	}
	if rsf.Provider != "" {
		fs.provider = rsf.Provider
	}
}

func (fs *factsState) recordRunEnd(ev domain.Event) {
	if ev.Timestamp.After(fs.endedAt) {
		fs.endedAt = ev.Timestamp
	}
	if ev.RunEnd != nil && ev.RunEnd.Outcome != "" {
		fs.outcome = ev.RunEnd.Outcome
	}
}

// session returns the entry for id, creating it at the end of the feed order.
func (fs *factsState) session(id string) *domain.RunSession {
	s, ok := fs.sessions[id]
	if !ok {
		s = &domain.RunSession{ID: id}
		fs.sessions[id] = s
		fs.sessionOrder = append(fs.sessionOrder, id)
	}
	return s
}

func (fs *factsState) recordSessionStart(ev domain.Event) {
	if ev.SessionID == "" {
		return
	}
	s := fs.session(ev.SessionID)
	if !fs.sessionSeen[ev.SessionID] {
		fs.sessionSeen[ev.SessionID] = true
		s.StartedAt = ev.Timestamp
	}
	sf := ev.SessionStart
	if sf == nil {
		return
	}
	if sf.Resumed.IsPresent() {
		s.Resumed = sf.Resumed
	}
	if !sf.Model.IsEmpty() {
		s.Model = sf.Model
	}
	if sf.Provider != "" {
		s.Provider = sf.Provider
	}
}

func (fs *factsState) recordSessionEnd(ev domain.Event) {
	if ev.SessionID == "" {
		return
	}
	s := fs.session(ev.SessionID)
	s.EndedAt = ev.Timestamp
	if ev.SessionEnd != nil && ev.SessionEnd.Reason != "" {
		s.Reason = ev.SessionEnd.Reason
	}
}

// applyFacts copies the accumulated facts into the run aggregate.
func applyFacts(rs *runState, out *domain.RunAggregate) {
	fs := rs.facts
	if fs == nil {
		return
	}

	for h := range fs.harnesses {
		out.Facts.Harnesses = append(out.Facts.Harnesses, h)
	}
	sort.Slice(out.Facts.Harnesses, func(i, j int) bool { return out.Facts.Harnesses[i] < out.Facts.Harnesses[j] })
	for v := range fs.schemaVersions {
		out.Facts.SchemaVersions = append(out.Facts.SchemaVersions, v)
	}
	sort.Strings(out.Facts.SchemaVersions)

	out.Facts.AdapterVersion = fs.adapterVersion
	out.Facts.OrchestratorModel = fs.model
	out.Facts.OrchestratorProvider = fs.provider
	out.Facts.StartedAt = fs.startedAt
	out.Facts.EndedAt = fs.endedAt
	out.Facts.Outcome = fs.outcome

	for _, id := range fs.sessionOrder {
		out.Sessions = append(out.Sessions, *fs.sessions[id])
	}
	out.Turns = fs.turns
	out.Notifications = fs.notifications
}
