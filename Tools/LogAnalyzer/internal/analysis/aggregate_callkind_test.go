package analysis_test

// Tests that call kind, call label and the session id survive aggregation.
//
// Rules under test:
//   - A usage_record unit carries the kind and label of its event and the
//     envelope session id; an event without a kind is a main call.
//   - Turn and invocation_end units are main calls with no label.
//   - The last observation of a record_id wins, kind and label included.
//   - Calls of every kind are kept as usage units (totals count them all).
//   - The run model's Invocation.Usage carries the same values.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func kindRecord(id domain.AgentInstanceID, recordID string, kind domain.CallKind, label string, input int64) domain.Event {
	ev := usageRecordEvent(id, recordID, v130Model, domain.TokenUsage{Input: domain.Tokens(input)})
	ev.UsageRecord.CallKind = kind
	ev.UsageRecord.CallLabel = label
	return ev
}

func requireKindLabel(t *testing.T, u domain.InvocationUsage, kind domain.CallKind, label string) {
	t.Helper()
	if u.Kind != kind || u.Label != label {
		t.Errorf("unit = {Kind:%q Label:%q}, want {Kind:%q Label:%q}", u.Kind, u.Label, kind, label)
	}
}

func TestAggregate_UsageRecord_CarriesCallKindAndLabel(t *testing.T) {
	got := orchestratorUsages(t,
		kindRecord("", "r1", domain.CallMain, "panel/editAgent", 10),
		kindRecord("", "r2", domain.CallCompaction, "summarizeConversationHistory", 20),
		kindRecord("", "r3", domain.CallAuxiliary, "title", 30),
	)

	if len(got) != 3 {
		t.Fatalf("got %d usage units, want 3 (every kind is kept)", len(got))
	}
	requireKindLabel(t, got[0], domain.CallMain, "panel/editAgent")
	requireKindLabel(t, got[1], domain.CallCompaction, "summarizeConversationHistory")
	requireKindLabel(t, got[2], domain.CallAuxiliary, "title")
}

func TestAggregate_Agent_UsageRecord_CarriesCallKindAndLabel(t *testing.T) {
	got := agentUsages(t, kindRecord("a#1", "r1", domain.CallAuxiliary, "progressMessages", 10))

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	requireKindLabel(t, got[0], domain.CallAuxiliary, "progressMessages")
}

func TestAggregate_UsageRecord_WithoutKind_IsMainWithEmptyLabel(t *testing.T) {
	// The zero kind is what an event built without the field carries.
	got := orchestratorUsages(t, kindRecord("", "r1", "", "", 10))

	requireKindLabel(t, got[0], domain.CallMain, "")
}

func TestAggregate_TurnAndInvocationEnd_AreMainCalls(t *testing.T) {
	turn := orchestratorUsages(t, turnEvent(v130Model, withReasoning(100, domain.AbsentTokens())))
	end := agentUsages(t, invEndEvent("a#1", v130Model, withReasoning(100, domain.AbsentTokens())))

	requireKindLabel(t, turn[0], domain.CallMain, "")
	requireKindLabel(t, end[0], domain.CallMain, "")
}

func TestAggregate_SupersededRecord_LastObservationWinsForKindAndLabel(t *testing.T) {
	got := orchestratorUsages(t,
		kindRecord("", "r1", domain.CallMain, "panel/editAgent", 10),
		kindRecord("", "r1", domain.CallCompaction, "summarizeConversationHistory", 10),
	)

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	requireKindLabel(t, got[0], domain.CallCompaction, "summarizeConversationHistory")
}

func TestAggregate_UsageRecord_CarriesEnvelopeSessionID(t *testing.T) {
	withSession := kindRecord("", "r1", domain.CallMain, "", 10)
	withSession.SessionID = "sess-1"
	without := kindRecord("", "r2", domain.CallMain, "", 10)

	got := orchestratorUsages(t, withSession, without)

	if got[0].SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", got[0].SessionID)
	}
	if got[1].SessionID != "" {
		t.Errorf("SessionID = %q, want empty when the envelope has none", got[1].SessionID)
	}
}

func TestAggregate_RunModelInvocationUsage_CarriesCallKindAndLabel(t *testing.T) {
	agg, _ := analysis.Aggregate(oneAgentStream(testRunRef, "a#1",
		kindRecord("a#1", "r1", domain.CallCompaction, "summarizeConversationHistory", 10),
	))

	inv := rmInvocation(t, requireOneRun(t, agg).Activity, "a#1")
	if len(inv.Usage) != 1 {
		t.Fatalf("Invocation.Usage has %d units, want 1", len(inv.Usage))
	}
	requireKindLabel(t, inv.Usage[0], domain.CallCompaction, "summarizeConversationHistory")
}
