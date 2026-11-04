package analysis_test

// Usage-entry identity (record id, timestamp, service tier) after the supersede
// rule and on the legacy path, and the invocation provider.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// rfUsageRecord builds a usage_record with identity fields set.
func rfUsageRecord(id domain.AgentInstanceID, recordID, tier string, at time.Time, input int64) domain.Event {
	ev := usageRecordEvent(id, recordID, "model-a", domain.TokenUsage{Input: domain.Tokens(input)})
	ev.Timestamp = at
	ev.UsageRecord.ServiceTier = tier
	return ev
}

// rfInvEndProvider builds an invocation_end carrying a provider.
func rfInvEndProvider(id domain.AgentInstanceID, provider domain.ProviderID, at time.Time) domain.Event {
	ev := invEndEvent(id, "model-a", domain.TokenUsage{Input: domain.Tokens(5)})
	ev.Timestamp = at
	ev.InvocationEnd.Provider = provider
	return ev
}

func TestUsageIdentity_OrchestratorUsageRecord_KeepsRecordIdTimestampAndTier(t *testing.T) {
	run := rfRun(t, rfOrch(rfUsageRecord("", "rec-1", "priority", rmAt(12), 10)))

	if len(run.Orchestrator.Invocations) != 1 {
		t.Fatalf("got %d entries, want 1", len(run.Orchestrator.Invocations))
	}
	e := run.Orchestrator.Invocations[0]
	if e.RecordID != "rec-1" || e.ServiceTier != "priority" || !e.At.Equal(rmAt(12)) {
		t.Errorf("entry identity = %q/%q/%v, want rec-1/priority/%v", e.RecordID, e.ServiceTier, e.At, rmAt(12))
	}
}

func TestUsageIdentity_AgentUsageRecord_KeepsRecordIdTimestampAndTier(t *testing.T) {
	run := rfRun(t, rfAgent("w#1", rfUsageRecord("w#1", "rec-a", "standard", rmAt(30), 10)))

	e := requireOneAgent(t, run).Invocations[0]
	if e.RecordID != "rec-a" || e.ServiceTier != "standard" || !e.At.Equal(rmAt(30)) {
		t.Errorf("entry identity = %q/%q/%v", e.RecordID, e.ServiceTier, e.At)
	}
}

func TestUsageIdentity_Supersede_EntryCarriesTheLastObservationsIdentity(t *testing.T) {
	run := rfRun(t, rfAgent("w#1",
		rfUsageRecord("w#1", "rec-a", "priority", rmAt(10), 10),
		rfUsageRecord("w#1", "rec-a", "standard", rmAt(20), 25),
	))

	entries := requireOneAgent(t, run).Invocations
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 after supersede", len(entries))
	}
	e := entries[0]
	if e.ServiceTier != "standard" || !e.At.Equal(rmAt(20)) || e.RecordID != "rec-a" {
		t.Errorf("identity = %q/%q/%v, want the last observation's standard/%v", e.RecordID, e.ServiceTier, e.At, rmAt(20))
	}
	assertTokenPresent(t, "usage input", e.Usage.Input, 25)
}

// The last observation wins by feed order, not by timestamp, and is not merged
// with earlier ones: an unlogged tier on the last observation reads as absent.
func TestUsageIdentity_Supersede_LastInFeedOrderWinsWithoutMerging(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfUsageRecord("", "rec-1", "priority", rmAt(50), 10),
		rfUsageRecord("", "rec-1", "", rmAt(5), 11),
	))

	entries := run.Orchestrator.Invocations
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].ServiceTier != "" || !entries[0].At.Equal(rmAt(5)) {
		t.Errorf("identity = %q/%v, want empty tier and the last fed timestamp %v", entries[0].ServiceTier, entries[0].At, rmAt(5))
	}
}

func TestUsageIdentity_SupersedeKeepsPositionAndOtherEntriesIdentity(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfUsageRecord("", "rec-1", "t1", rmAt(1), 1),
		rfUsageRecord("", "rec-2", "t2", rmAt(2), 2),
		rfUsageRecord("", "rec-1", "t1b", rmAt(3), 3),
	))

	entries := run.Orchestrator.Invocations
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].RecordID != "rec-1" || entries[0].ServiceTier != "t1b" || !entries[0].At.Equal(rmAt(3)) {
		t.Errorf("first entry = %+v", entries[0])
	}
	if entries[1].RecordID != "rec-2" || entries[1].ServiceTier != "t2" || !entries[1].At.Equal(rmAt(2)) {
		t.Errorf("second entry = %+v", entries[1])
	}
}

func TestUsageIdentity_CrossStreamDuplicate_FirstOwnersEntryKeepsItsIdentity(t *testing.T) {
	agg, findings := rfAggregate(
		rfOrch(rfUsageRecord("", "rec-x", "owner-tier", rmAt(1), 10)),
		rfAgent("w#1", rfUsageRecord("w#1", "rec-x", "intruder-tier", rmAt(2), 99)),
	)

	run := requireOneRun(t, agg)
	if !hasFindingKind(findings, domain.FindingCrossStreamRecord) {
		t.Fatalf("expected the cross-stream finding, got %v", findings)
	}
	e := run.Orchestrator.Invocations[0]
	if e.ServiceTier != "owner-tier" || !e.At.Equal(rmAt(1)) {
		t.Errorf("owner entry identity = %q/%v", e.ServiceTier, e.At)
	}
	if len(run.Agents) != 0 && len(run.Agents[0].Invocations) != 0 {
		t.Errorf("duplicate record added to the second stream: %+v", run.Agents)
	}
}

func TestUsageIdentity_EmptyRecordId_KeepsTimestampAndTierWithEmptyId(t *testing.T) {
	run := rfRun(t, rfOrch(rfUsageRecord("", "", "priority", rmAt(8), 10)))

	e := run.Orchestrator.Invocations[0]
	if e.RecordID != "" || e.ServiceTier != "priority" || !e.At.Equal(rmAt(8)) {
		t.Errorf("identity = %q/%q/%v", e.RecordID, e.ServiceTier, e.At)
	}
}

func TestUsageIdentity_RecordWithoutTokenCounts_StillCarriesIdentity(t *testing.T) {
	ev := usageRecordEvent("", "rec-m", "model-a", domain.TokenUsage{})
	ev.Timestamp = rmAt(4)
	run := rfRun(t, rfOrch(ev))

	e := run.Orchestrator.Invocations[0]
	if e.RecordID != "rec-m" || !e.At.Equal(rmAt(4)) {
		t.Errorf("identity = %q/%v, want rec-m/%v", e.RecordID, e.At, rmAt(4))
	}
}

func TestUsageIdentity_UnloggedTimestampAndTier_StayAbsent(t *testing.T) {
	run := rfRun(t, rfOrch(rfUsageRecord("", "rec-1", "", time.Time{}, 10)))

	e := run.Orchestrator.Invocations[0]
	if !e.At.IsZero() || e.ServiceTier != "" {
		t.Errorf("absent identity invented: %v / %q", e.At, e.ServiceTier)
	}
}

func TestUsageIdentity_LegacyOrchestratorTurn_CarriesTimestampOnly(t *testing.T) {
	run := rfRun(t, rfOrch(rfTurn(domain.TurnAssistant, rmAt(15), domain.TokenUsage{Input: domain.Tokens(10)})))

	if len(run.Orchestrator.Invocations) != 1 {
		t.Fatalf("got %d entries, want 1", len(run.Orchestrator.Invocations))
	}
	e := run.Orchestrator.Invocations[0]
	if !e.At.Equal(rmAt(15)) {
		t.Errorf("At = %v, want %v", e.At, rmAt(15))
	}
	if e.RecordID != "" || e.ServiceTier != "" {
		t.Errorf("legacy entry invented identity: %q/%q", e.RecordID, e.ServiceTier)
	}
}

func TestUsageIdentity_LegacyInvocationEnd_CarriesTimestampOnly(t *testing.T) {
	run := rfRun(t, rfAgent("w#1", rfInvEndProvider("w#1", "anthropic", rmAt(44))))

	e := requireOneAgent(t, run).Invocations[0]
	if !e.At.Equal(rmAt(44)) {
		t.Errorf("At = %v, want %v", e.At, rmAt(44))
	}
	if e.RecordID != "" || e.ServiceTier != "" {
		t.Errorf("legacy entry invented identity: %q/%q", e.RecordID, e.ServiceTier)
	}
}

func TestUsageIdentity_InvocationUsageMirrorsActorEntriesIncludingIdentity(t *testing.T) {
	run := rfRun(t, rfAgent("w#1",
		rmInvStart("w#1", "worker", rmAt(1)),
		rfUsageRecord("w#1", "rec-a", "priority", rmAt(5), 10),
		rmInvEnd("w#1", "success", "model-a", rmAt(9)),
	))

	inv := rmInvocation(t, run.Activity, "w#1")
	agent := requireOneAgent(t, run)
	if len(inv.Usage) != 1 || !reflect.DeepEqual(inv.Usage, agent.Invocations) {
		t.Errorf("Invocation.Usage = %+v, want identical to actor entries %+v", inv.Usage, agent.Invocations)
	}
	if inv.Usage[0].RecordID != "rec-a" || inv.Usage[0].ServiceTier != "priority" {
		t.Errorf("identity missing on Invocation.Usage: %+v", inv.Usage[0])
	}
}

func TestInvocationProvider_ComesFromTheLastInvocationEnd(t *testing.T) {
	run := rfRun(t, rfAgent("w#1",
		rmInvStart("w#1", "worker", rmAt(1)),
		rfInvEndProvider("w#1", "anthropic", rmAt(5)),
		rfInvEndProvider("w#1", "bedrock", rmAt(9)),
	))

	if got := rmInvocation(t, run.Activity, "w#1").Provider; got != "bedrock" {
		t.Errorf("Provider = %q, want the last invocation_end's bedrock", got)
	}
}

func TestInvocationProvider_LastEndWithoutProvider_IsAbsent(t *testing.T) {
	run := rfRun(t, rfAgent("w#1",
		rfInvEndProvider("w#1", "anthropic", rmAt(5)),
		rfInvEndProvider("w#1", "", rmAt(9)),
	))

	if got := rmInvocation(t, run.Activity, "w#1").Provider; got != "" {
		t.Errorf("Provider = %q, want empty when the last end logged none", got)
	}
}

func TestInvocationProvider_OpenInvocation_IsAbsent(t *testing.T) {
	run := rfRun(t, rfAgent("w#1", rmInvStart("w#1", "worker", rmAt(1))))

	inv := rmInvocation(t, run.Activity, "w#1")
	if inv.State != domain.InvocationOpen || inv.Provider != "" {
		t.Errorf("open invocation = state %v provider %q, want open and empty", inv.State, inv.Provider)
	}
}
