package digest

import (
	"bytes"
	"encoding/json"

	"mosaic-log-analyzer/internal/domain"
)

// encode renders d as digest file bytes at this package's own version.
func encode(d domain.Digest) ([]byte, error) {
	doc := digestDoc{
		Format:        formatName,
		DigestVersion: Version,
		ToolVersion:   d.Header.ToolVersion,
		WrittenAt:     formatTime(d.Header.WrittenAt),
		Runs:          make([]runDoc, 0, len(d.Aggregate.Runs)),
	}
	for _, run := range d.Aggregate.Runs {
		doc.Runs = append(doc.Runs, encodeRun(run))
	}
	if d.Aggregate.Unattributable != nil {
		bucket := encodeRun(*d.Aggregate.Unattributable)
		bucket.RunID = ""
		doc.Unattributable = &bucket
	}
	for _, f := range d.Findings {
		doc.Findings = append(doc.Findings, encodeFinding(f))
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeRun(r domain.RunAggregate) runDoc {
	doc := runDoc{
		RunID:                r.Run.ID,
		Provisional:          r.Provisional,
		SchemaVersions:       r.Facts.SchemaVersions,
		AdapterVersion:       r.Facts.AdapterVersion,
		OrchestratorModel:    string(r.Facts.OrchestratorModel),
		OrchestratorProvider: string(r.Facts.OrchestratorProvider),
		StartedAt:            formatTime(r.Facts.StartedAt),
		EndedAt:              formatTime(r.Facts.EndedAt),
		Outcome:              r.Facts.Outcome,
		Orchestrator: orchestratorDoc{
			Usage:     encodeUsages(r.Orchestrator.Invocations),
			ToolCalls: encodeToolCalls(r.Activity.OrchestratorToolCalls),
		},
	}
	for _, h := range r.Facts.Harnesses {
		doc.Harnesses = append(doc.Harnesses, string(h))
	}
	for _, s := range r.Sessions {
		doc.Sessions = append(doc.Sessions, encodeSession(s))
	}
	for _, a := range r.Agents {
		doc.Agents = append(doc.Agents, agentDoc{
			InstanceID: string(a.Actor.Instance),
			AgentType:  a.Actor.Type,
			Usage:      encodeUsages(a.Invocations),
		})
	}
	for _, inv := range r.Activity.Invocations {
		doc.Invocations = append(doc.Invocations, encodeInvocation(inv))
	}
	for _, t := range r.Turns {
		doc.Turns = append(doc.Turns, turnDoc{Actor: encodeActor(t.Actor), Role: string(t.Role), At: formatTime(t.At)})
	}
	for _, n := range r.Notifications {
		doc.Notifications = append(doc.Notifications, notificationDoc{
			Actor:            encodeActor(n.Actor),
			At:               formatTime(n.At),
			NotificationType: n.Type,
			RequiresResponse: encodeBool(n.RequiresResponse),
			Resolution:       n.Resolution,
			NotificationID:   n.ID,
		})
	}
	for _, c := range r.Activity.Compactions {
		doc.Compactions = append(doc.Compactions, compactionDoc{
			Actor:        encodeActor(c.Actor),
			At:           formatTime(c.At),
			Trigger:      c.Trigger,
			TokensBefore: encodeCount(c.TokensBefore),
			TokensAfter:  encodeCount(c.TokensAfter),
		})
	}
	return doc
}

func encodeSession(s domain.RunSession) sessionDoc {
	return sessionDoc{
		SessionID: s.ID,
		StartedAt: formatTime(s.StartedAt),
		EndedAt:   formatTime(s.EndedAt),
		Resumed:   encodeBool(s.Resumed),
		Reason:    s.Reason,
		Model:     string(s.Model),
		Provider:  string(s.Provider),
	}
}

func encodeInvocation(inv domain.Invocation) invocationDoc {
	state := stateClosed
	if inv.State == domain.InvocationOpen {
		state = stateOpen
	}
	return invocationDoc{
		InstanceID: string(inv.Actor.Instance),
		AgentType:  inv.Actor.Type,
		State:      state,
		StatusCode: inv.StatusCode,
		Model:      string(inv.Model),
		Provider:   string(inv.Provider),
		StartedAt:  formatTime(inv.StartedAt),
		EndedAt:    formatTime(inv.EndedAt),
		EndEvents:  inv.EndEvents,
		ToolCalls:  encodeToolCalls(inv.ToolCalls),
	}
}

func encodeToolCalls(calls []domain.ToolCall) []toolCallDoc {
	var docs []toolCallDoc
	for _, c := range calls {
		pairing := pairingPaired
		switch c.Pairing {
		case domain.ToolCallStartOnly:
			pairing = pairingStartOnly
		case domain.ToolCallEndOnly:
			pairing = pairingEndOnly
		}
		docs = append(docs, toolCallDoc{
			CallID:    c.CallID,
			ToolName:  c.ToolName,
			Status:    string(c.Status),
			StartedAt: formatTime(c.StartedAt),
			EndedAt:   formatTime(c.EndedAt),
			Pairing:   pairing,
		})
	}
	return docs
}

func encodeUsages(usages []domain.InvocationUsage) []usageDoc {
	var docs []usageDoc
	for _, u := range usages {
		docs = append(docs, usageDoc{
			RecordID:    u.RecordID,
			At:          formatTime(u.At),
			Model:       string(u.Model),
			Provider:    string(u.Provider),
			Effort:      u.Effort,
			ServiceTier: u.ServiceTier,
			Harness:     string(u.Harness),
			Tokens:      encodeTokens(u.Usage),
		})
	}
	return docs
}

// encodeTokens returns nil when no count is present (a missing-usage marker).
func encodeTokens(u domain.TokenUsage) *tokensDoc {
	doc := tokensDoc{
		Input:           encodeCount(u.Input),
		CacheRead:       encodeCount(u.CacheRead),
		CacheCreation:   encodeCount(u.CacheCreation),
		CacheCreation5m: encodeCount(u.CacheCreation5m),
		CacheCreation1h: encodeCount(u.CacheCreation1h),
		Output:          encodeCount(u.Output),
		Reasoning:       encodeCount(u.Reasoning),
	}
	if doc == (tokensDoc{}) {
		return nil
	}
	return &doc
}

func encodeCount(c domain.TokenCount) *int64 {
	v, ok := c.Value()
	if !ok {
		return nil
	}
	return &v
}

func encodeBool(b domain.OptionalBool) *bool {
	v, ok := b.Value()
	if !ok {
		return nil
	}
	return &v
}

func encodeActor(a domain.ActorRef) *actorDoc {
	if a.Kind == domain.ActorAgentInstance {
		return &actorDoc{Kind: actorAgentInstance, InstanceID: string(a.Instance), AgentType: a.Type}
	}
	return &actorDoc{Kind: actorOrchestrator}
}

// encodeFinding keeps only kind, severity, run scope and actor; path, line and
// detail text are never written.
func encodeFinding(f domain.Finding) findingDoc {
	doc := findingDoc{
		Kind:     string(f.Kind),
		Severity: severityInfo,
		Scope:    scopeGeneral,
		Actor:    encodeActor(f.Actor),
	}
	if f.Severity == domain.SeverityWarning {
		doc.Severity = severityWarning
	}
	if f.Run.IsUnattributable() {
		doc.Scope = scopeUnattributable
	} else if f.Run.ID != "" {
		doc.Scope = scopeRun
		doc.RunID = f.Run.ID
	}
	return doc
}
