package analysis_test

// Tests for the run-level unattributed usage stream in aggregation.
//
// Rules under test:
//   - usage_records on the unattributed stream form their own actor, never the
//     orchestrator and never an agent instance; they count toward the run total
//     and the run's models.
//   - No agent_instance_id is required or read on that stream, so none of its
//     records raises an attribution finding.
//   - Single-stream ownership (by record_id, across all of the run's streams)
//     and last-wins supersede apply unchanged.
//   - Only usage_records count; every other event type is ignored without
//     findings and adds nothing to the run facts or the run model.
//   - A run without the stream has no unattributed usage.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// unattrStream builds a StreamRef for the run-level unattributed stream.
func unattrStream(run domain.RunRef) domain.StreamRef {
	return domain.StreamRef{Run: run, Kind: domain.StreamUnattributed}
}

// unattrInput builds an analysis input from the given streams.
func unattrInput(streams ...analysis.Stream) analysis.Input {
	return analysis.Input{Streams: streams}
}

func unattrIn(events ...domain.Event) analysis.Stream {
	return analysis.Stream{Ref: unattrStream(testRunRef), Events: events}
}

func inputOnly(n int64) domain.TokenUsage { return domain.TokenUsage{Input: domain.Tokens(n)} }

// requireUnattributed returns the run's unattributed actor, failing when absent.
func requireUnattributed(t *testing.T, run domain.RunAggregate) domain.ActorAggregate {
	t.Helper()
	if run.Unattributed == nil {
		t.Fatal("run.Unattributed is nil, want the unattributed usage of the run")
	}
	return *run.Unattributed
}

func TestAggregate_UnattributedUsage_IsItsOwnActorNotOrchestratorOrAgent(t *testing.T) {
	agg, _ := analysis.Aggregate(unattrInput(
		rfOrch(usageRecordEvent("", "o-1", "m-a", inputOnly(1000))),
		rfAgent("coder#1", usageRecordEvent("coder#1", "a-1", "m-a", inputOnly(2000))),
		unattrIn(usageRecordEvent("", "u-1", "m-a", inputOnly(500))),
	))

	run := requireOneRun(t, agg)

	un := requireUnattributed(t, run)
	assertTokenPresent(t, "unattributed input", un.Totals().Input, 500)
	if un.Actor.Kind != domain.ActorUnattributed {
		t.Errorf("unattributed actor kind = %v, want ActorUnattributed", un.Actor.Kind)
	}
	assertTokenPresent(t, "orchestrator input", run.Orchestrator.Totals().Input, 1000)
	if len(run.Agents) != 1 {
		t.Fatalf("got %d agents, want 1 (unattributed must not become an agent)", len(run.Agents))
	}
	assertTokenPresent(t, "agent input", run.Agents[0].Totals().Input, 2000)
}

func TestAggregate_UnattributedUsage_IsPartOfRunTotalsAndModels(t *testing.T) {
	agg, _ := analysis.Aggregate(unattrInput(
		rfOrch(usageRecordEvent("", "o-1", "m-a", inputOnly(1000))),
		unattrIn(usageRecordEvent("", "u-1", "m-only-unattributed", inputOnly(500))),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "run total input", run.Totals().Input, 1500)
	models := run.Models()
	if len(models) != 2 || models[0] != "m-a" || models[1] != "m-only-unattributed" {
		t.Errorf("run models = %v, want [m-a m-only-unattributed]", models)
	}
	if got := agg.Models(); len(got) != 2 {
		t.Errorf("aggregate models = %v, want both models", got)
	}
}

func TestAggregate_UnattributedUsage_RaisesNoAttributionFinding(t *testing.T) {
	_, findings := analysis.Aggregate(unattrInput(
		unattrIn(
			usageRecordEvent("", "u-1", "m-a", inputOnly(10)),
			usageRecordEvent("", "u-2", "m-a", inputOnly(20)),
		),
	))

	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none for unattributed usage_records", findings)
	}
}

func TestAggregate_UnattributedUsage_StreamWithoutAnyOtherStream_CreatesTheRun(t *testing.T) {
	agg, _ := analysis.Aggregate(unattrInput(
		unattrIn(usageRecordEvent("", "u-1", "m-a", inputOnly(10))),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "run total input", run.Totals().Input, 10)
	if !run.Provisional {
		t.Error("run with only an unattributed stream must stay provisional (no run_end seen)")
	}
}

func TestAggregate_UnattributedUsage_StrayAgentInstanceIDIsNotRead(t *testing.T) {
	agg, findings := analysis.Aggregate(unattrInput(
		unattrIn(usageRecordEvent("ghost#1", "u-1", "m-a", inputOnly(10))),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "unattributed input", requireUnattributed(t, run).Totals().Input, 10)
	if len(run.Agents) != 0 {
		t.Errorf("agents = %d, want 0: a stray agent_instance_id must not create an agent", len(run.Agents))
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none", findings)
	}
}

func TestAggregate_UnattributedUsage_ReobservedRecordID_LastObservationWins(t *testing.T) {
	agg, findings := analysis.Aggregate(unattrInput(
		unattrIn(
			usageRecordEvent("", "u-1", "m-a", inputOnly(500)),
			usageRecordEvent("", "u-2", "m-a", inputOnly(300)),
			usageRecordEvent("", "u-1", "m-a", inputOnly(600)),
		),
	))

	un := requireUnattributed(t, requireOneRun(t, agg))

	if len(un.Invocations) != 2 {
		t.Fatalf("records = %d, want 2 (re-observation must not add a record)", len(un.Invocations))
	}
	assertTokenPresent(t, "first record (superseded in place)", un.Invocations[0].Usage.Input, 600)
	assertTokenPresent(t, "second record", un.Invocations[1].Usage.Input, 300)
	assertTokenPresent(t, "unattributed total", un.Totals().Input, 900)
	if len(findings) != 0 {
		t.Errorf("re-observation raised findings: %+v", findings)
	}
}

func TestAggregate_UnattributedUsage_RecordAlreadyOnOrchestrator_CountsOnceWithOneFinding(t *testing.T) {
	agg, findings := analysis.Aggregate(unattrInput(
		rfOrch(usageRecordEvent("", "dup", "m-a", inputOnly(1000))),
		unattrIn(usageRecordEvent("", "dup", "m-a", inputOnly(1000))),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "run total input", run.Totals().Input, 1000)
	assertTokenPresent(t, "orchestrator input", run.Orchestrator.Totals().Input, 1000)
	if run.Unattributed != nil && len(run.Unattributed.Invocations) != 0 {
		t.Errorf("the duplicate must not be added to the unattributed total: %+v", run.Unattributed.Invocations)
	}
	if n := countFindingKind(findings, domain.FindingCrossStreamRecord); n != 1 {
		t.Errorf("cross-stream findings = %d, want exactly 1", n)
	}
}

func TestAggregate_UnattributedUsage_RecordAlreadyOnAgent_CountsOnceWithOneFinding(t *testing.T) {
	agg, findings := analysis.Aggregate(unattrInput(
		rfAgent("coder#1", usageRecordEvent("coder#1", "dup", "m-a", inputOnly(700))),
		unattrIn(usageRecordEvent("", "dup", "m-a", inputOnly(700))),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "run total input", run.Totals().Input, 700)
	if n := countFindingKind(findings, domain.FindingCrossStreamRecord); n != 1 {
		t.Errorf("cross-stream findings = %d, want exactly 1", n)
	}
}

func TestAggregate_UnattributedUsage_RecordFirstSeenUnattributed_LaterStreamIsTheDuplicate(t *testing.T) {
	agg, findings := analysis.Aggregate(unattrInput(
		unattrIn(usageRecordEvent("", "dup", "m-a", inputOnly(400))),
		rfOrch(usageRecordEvent("", "dup", "m-a", inputOnly(400))),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "run total input", run.Totals().Input, 400)
	assertTokenPresent(t, "unattributed input", requireUnattributed(t, run).Totals().Input, 400)
	if len(run.Orchestrator.Invocations) != 0 {
		t.Errorf("orchestrator must not receive a record first owned by the unattributed stream")
	}
	if n := countFindingKind(findings, domain.FindingCrossStreamRecord); n != 1 {
		t.Errorf("cross-stream findings = %d, want exactly 1", n)
	}
}

func TestAggregate_UnattributedUsage_SameRecordIDInAnotherRun_IsNotACrossStreamDuplicate(t *testing.T) {
	otherRun := domain.NamedRun("20260102T000000Z-eeee")
	agg, findings := analysis.Aggregate(unattrInput(
		rfOtherRunOrch(otherRun, usageRecordEvent("", "same", "m-a", inputOnly(5))),
		unattrIn(usageRecordEvent("", "same", "m-a", inputOnly(7))),
	))

	if len(agg.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(agg.Runs))
	}
	if n := countFindingKind(findings, domain.FindingCrossStreamRecord); n != 0 {
		t.Errorf("cross-stream findings = %d, want 0: ownership is per run", n)
	}
}

func TestAggregate_UnattributedUsage_RecordWithoutID_IsCountedWithOneIdentityFinding(t *testing.T) {
	agg, findings := analysis.Aggregate(unattrInput(
		unattrIn(usageRecordEvent("", "", "m-a", inputOnly(30))),
	))

	assertTokenPresent(t, "unattributed input", requireUnattributed(t, requireOneRun(t, agg)).Totals().Input, 30)
	if n := countFindingKind(findings, domain.FindingUnidentifiedEvent); n != 1 {
		t.Errorf("unidentified-event findings = %d, want exactly 1 (for the missing record_id)", n)
	}
}

func TestAggregate_UnattributedStream_OnlyUsageRecordsCount(t *testing.T) {
	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	agg, findings := analysis.Aggregate(unattrInput(
		unattrIn(
			usageRecordEvent("", "u-1", "m-a", inputOnly(10)),
			rfTurn(domain.TurnAssistant, at, inputOnly(9_999)),
			invEndEvent("coder#9", "m-a", inputOnly(8_888)),
			rfNotification(at, domain.NotificationFields{NotificationType: "idle"}),
			rmToolStart("t1", "Read", at),
			rmCompaction("auto", domain.Tokens(10), domain.Tokens(5), at),
		),
	))

	run := requireOneRun(t, agg)

	assertTokenPresent(t, "run total input", run.Totals().Input, 10)
	if len(run.Agents) != 0 {
		t.Errorf("agents = %d, want 0: invocation_end on this stream must not create an agent", len(run.Agents))
	}
	if len(run.Turns) != 0 || len(run.Notifications) != 0 {
		t.Errorf("turns = %d, notifications = %d, want none from the unattributed stream", len(run.Turns), len(run.Notifications))
	}
	if len(run.Activity.Invocations) != 0 || len(run.Activity.OrchestratorToolCalls) != 0 || len(run.Activity.Compactions) != 0 {
		t.Errorf("run model took activity from the unattributed stream: %+v", run.Activity)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none: other event types are ignored silently", findings)
	}
}

func TestAggregate_RunWithoutUnattributedStream_HasNoUnattributedUsage(t *testing.T) {
	agg, _ := analysis.Aggregate(unattrInput(
		rfOrch(usageRecordEvent("", "o-1", "m-a", inputOnly(1000))),
		rfAgent("coder#1", usageRecordEvent("coder#1", "a-1", "m-a", inputOnly(2000))),
	))

	run := requireOneRun(t, agg)

	if run.Unattributed != nil {
		t.Errorf("run.Unattributed = %+v, want nil when the run has no unattributed usage", run.Unattributed)
	}
	assertTokenPresent(t, "run total input", run.Totals().Input, 3000)
}

func TestAggregate_UnattributedStreamWithoutUsageRecords_HasNoUnattributedUsage(t *testing.T) {
	agg, _ := analysis.Aggregate(unattrInput(
		rfOrch(usageRecordEvent("", "o-1", "m-a", inputOnly(1000))),
		unattrIn(rmToolStart("t1", "Read", time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC))),
	))

	if requireOneRun(t, agg).Unattributed != nil {
		t.Error("run.Unattributed must be nil when the stream held no usage_record")
	}
}

func countFindingKind(findings []domain.Finding, kind domain.FindingKind) int {
	n := 0
	for _, f := range findings {
		if f.Kind == kind {
			n++
		}
	}
	return n
}
