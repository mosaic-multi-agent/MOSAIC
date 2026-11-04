package analysis_test

// Run-level facts: harnesses, schema versions, run_start metadata, start and
// end timestamps and outcome, including resumed runs with several
// run_start/run_end pairs.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func TestRunFacts_SingleRun_KeepsEveryLoggedFact(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "0.9.1", Model: "orch-model", Provider: "anthropic"}),
		rfRunEnd(rmAt(100), "completed"),
	))

	f := run.Facts
	if f.AdapterVersion != "0.9.1" || f.OrchestratorModel != "orch-model" || f.OrchestratorProvider != "anthropic" {
		t.Errorf("run_start metadata = %q/%q/%q", f.AdapterVersion, f.OrchestratorModel, f.OrchestratorProvider)
	}
	if !f.StartedAt.Equal(rmAt(0)) || !f.EndedAt.Equal(rmAt(100)) {
		t.Errorf("StartedAt/EndedAt = %v/%v, want %v/%v", f.StartedAt, f.EndedAt, rmAt(0), rmAt(100))
	}
	if f.Outcome != "completed" {
		t.Errorf("Outcome = %q, want completed", f.Outcome)
	}
}

func TestRunFacts_ResumedRun_StartIsEarliestEndIsLatest(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "v1"}),
		rfRunEnd(rmAt(50), "interrupted"),
		rfRunStart(rmAt(200), domain.RunStartFields{AdapterVersion: "v2"}),
		rfRunEnd(rmAt(300), "completed"),
	))

	if !run.Facts.StartedAt.Equal(rmAt(0)) {
		t.Errorf("StartedAt = %v, want the first run_start %v", run.Facts.StartedAt, rmAt(0))
	}
	if !run.Facts.EndedAt.Equal(rmAt(300)) {
		t.Errorf("EndedAt = %v, want the last run_end %v", run.Facts.EndedAt, rmAt(300))
	}
}

func TestRunFacts_ResumedRun_MetadataAndOutcomeComeFromLastInFeedOrder(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "v1", Model: "m1", Provider: "p1"}),
		rfRunEnd(rmAt(50), "interrupted"),
		rfRunStart(rmAt(200), domain.RunStartFields{AdapterVersion: "v2", Model: "m2", Provider: "p2"}),
		rfRunEnd(rmAt(300), "completed"),
	))

	f := run.Facts
	if f.AdapterVersion != "v2" || f.OrchestratorModel != "m2" || f.OrchestratorProvider != "p2" {
		t.Errorf("metadata = %q/%q/%q, want the last run_start v2/m2/p2", f.AdapterVersion, f.OrchestratorModel, f.OrchestratorProvider)
	}
	if f.Outcome != "completed" {
		t.Errorf("Outcome = %q, want the last run_end's completed", f.Outcome)
	}
}

// Feed order and timestamp order disagree: "earliest/latest" compares
// timestamps, "last" is feed order.
func TestRunFacts_FeedOrderDiffersFromTimeOrder_TimesByTimestampValuesByFeed(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(rmAt(200), domain.RunStartFields{AdapterVersion: "fed-first"}),
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "fed-last"}),
		rfRunEnd(rmAt(300), "fed-first"),
		rfRunEnd(rmAt(100), "fed-last"),
	))

	f := run.Facts
	if !f.StartedAt.Equal(rmAt(0)) {
		t.Errorf("StartedAt = %v, want earliest timestamp %v", f.StartedAt, rmAt(0))
	}
	if !f.EndedAt.Equal(rmAt(300)) {
		t.Errorf("EndedAt = %v, want latest timestamp %v", f.EndedAt, rmAt(300))
	}
	if f.AdapterVersion != "fed-last" {
		t.Errorf("AdapterVersion = %q, want the last in feed order", f.AdapterVersion)
	}
	if f.Outcome != "fed-last" {
		t.Errorf("Outcome = %q, want the last in feed order", f.Outcome)
	}
}

func TestRunFacts_LaterEventWithoutValue_DoesNotEraseEarlierValue(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "v1", Model: "m1", Provider: "p1"}),
		rfRunEnd(rmAt(50), "completed"),
		rfRunStart(rmAt(60), domain.RunStartFields{}),
		rfRunEnd(rmAt(70), ""),
	))

	f := run.Facts
	if f.AdapterVersion != "v1" || f.OrchestratorModel != "m1" || f.OrchestratorProvider != "p1" {
		t.Errorf("metadata = %q/%q/%q, want the earlier values kept", f.AdapterVersion, f.OrchestratorModel, f.OrchestratorProvider)
	}
	if f.Outcome != "completed" {
		t.Errorf("Outcome = %q, want the earlier completed kept", f.Outcome)
	}
}

func TestRunFacts_UnparseableTimestamps_AreNotUsedAsStartOrEnd(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(time.Time{}, domain.RunStartFields{AdapterVersion: "v1"}),
		rfRunStart(rmAt(40), domain.RunStartFields{}),
		rfRunEnd(time.Time{}, "completed"),
		rfRunEnd(rmAt(90), ""),
	))

	f := run.Facts
	if !f.StartedAt.Equal(rmAt(40)) {
		t.Errorf("StartedAt = %v, want the only parseable run_start %v", f.StartedAt, rmAt(40))
	}
	if !f.EndedAt.Equal(rmAt(90)) {
		t.Errorf("EndedAt = %v, want the only parseable run_end %v", f.EndedAt, rmAt(90))
	}
	if f.AdapterVersion != "v1" || f.Outcome != "completed" {
		t.Errorf("values from events with unparseable timestamps lost: %q/%q", f.AdapterVersion, f.Outcome)
	}
}

func TestRunFacts_OptionalFactsAbsent_StayAbsent(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfRunStart(time.Time{}, domain.RunStartFields{}),
		usageRecordEvent("", "rec-1", "m", domain.TokenUsage{Input: domain.Tokens(1)}),
	))

	f := run.Facts
	if f.AdapterVersion != "" || f.OrchestratorModel != "" || f.OrchestratorProvider != "" || f.Outcome != "" {
		t.Errorf("absent strings were invented: %+v", f)
	}
	if !f.StartedAt.IsZero() || !f.EndedAt.IsZero() {
		t.Errorf("absent times were invented: %v / %v", f.StartedAt, f.EndedAt)
	}
	if len(f.Harnesses) != 1 || f.Harnesses[0] != rfCC {
		t.Errorf("Harnesses = %v, want just the one the envelope carried", f.Harnesses)
	}
}

func TestRunFacts_RunWithNoLifecycleOrEnvelopeFacts_HasEmptyFacts(t *testing.T) {
	run := rfRun(t, rfOrch(usageRecordEvent("", "rec-1", "m", domain.TokenUsage{Input: domain.Tokens(1)})))

	f := run.Facts
	if len(f.Harnesses) != 0 || len(f.SchemaVersions) != 0 {
		t.Errorf("Harnesses/SchemaVersions = %v/%v, want empty", f.Harnesses, f.SchemaVersions)
	}
	if f.AdapterVersion != "" || f.Outcome != "" || !f.StartedAt.IsZero() || !f.EndedAt.IsZero() {
		t.Errorf("facts invented for a run without lifecycle events: %+v", f)
	}
}

func TestRunFacts_Harnesses_DistinctNonEmptySortedVerbatim(t *testing.T) {
	ev := func(h domain.Harness) domain.Event {
		return rfEnv(domain.Event{Type: domain.EventOther}, rmAt(1), h, "", "")
	}
	run := rfRun(t,
		rfOrch(ev("opencode"), ev(""), ev(rfCC), ev("opencode")),
		rfAgent("w#1", ev(rfCC), ev("vendor-x")),
	)

	got := rfStrings(run.Facts.Harnesses)
	want := []string{"claude-code", "opencode", "vendor-x"}
	if !rmEqualStrings(got, want) {
		t.Errorf("Harnesses = %v, want %v", got, want)
	}
}

func TestRunFacts_SchemaVersions_DistinctNonEmptySortedIncludingUnsupported(t *testing.T) {
	ev := func(typ domain.EventType, v string) domain.Event {
		return rfEnv(domain.Event{Type: typ}, rmAt(1), rfCC, v, "")
	}
	run := rfRun(t,
		rfOrch(ev(domain.EventOther, "1.3.0"), ev(domain.EventNotification, "1.0.0"), ev(domain.EventOther, "")),
		rfAgent("w#1", ev(domain.EventOther, "9.9.9"), ev(domain.EventOther, "1.3.0")),
	)

	got := run.Facts.SchemaVersions
	want := []string{"1.0.0", "1.3.0", "9.9.9"}
	if !rmEqualStrings(got, want) {
		t.Errorf("SchemaVersions = %v, want %v", got, want)
	}
}

func TestRunFacts_ComeFromEventsOnAnyStream(t *testing.T) {
	run := rfRun(t,
		rfOrch(rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "v1"})),
		rfAgent("w#1", rfRunEnd(rmAt(30), "completed")),
	)

	if run.Facts.AdapterVersion != "v1" || run.Facts.Outcome != "completed" || !run.Facts.EndedAt.Equal(rmAt(30)) {
		t.Errorf("facts from the agent stream run_end were lost: %+v", run.Facts)
	}
}

func TestRunFacts_AreKeptPerRun(t *testing.T) {
	other := domain.NamedRun("20260102T000000Z-beef")
	agg, _ := rfAggregate(
		rfOrch(rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "first"}), rfRunEnd(rmAt(5), "completed")),
		rfOtherRunOrch(other, rfRunStart(rmAt(500), domain.RunStartFields{AdapterVersion: "second"})),
	)

	if len(agg.Runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(agg.Runs))
	}
	first, second := agg.Runs[0], agg.Runs[1]
	if first.Facts.AdapterVersion != "first" || first.Facts.Outcome != "completed" {
		t.Errorf("first run facts = %+v", first.Facts)
	}
	if second.Facts.AdapterVersion != "second" || second.Facts.Outcome != "" || !second.Facts.EndedAt.IsZero() {
		t.Errorf("second run facts leaked or invented: %+v", second.Facts)
	}
}

func TestRunFacts_UnattributableBucket_KeepsItsOwnFacts(t *testing.T) {
	agg, _ := rfAggregate(
		rfOrch(rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "named"})),
		rfUnattributable(
			rfRunStart(rmAt(10), domain.RunStartFields{AdapterVersion: "stray", Model: "sm", Provider: "sp"}),
			rfRunEnd(rmAt(20), "aborted"),
		),
	)

	if agg.Unattributable == nil {
		t.Fatal("unattributable bucket missing")
	}
	f := agg.Unattributable.Facts
	if f.AdapterVersion != "stray" || f.OrchestratorModel != "sm" || f.OrchestratorProvider != "sp" || f.Outcome != "aborted" {
		t.Errorf("bucket facts = %+v", f)
	}
	if !f.StartedAt.Equal(rmAt(10)) || !f.EndedAt.Equal(rmAt(20)) {
		t.Errorf("bucket times = %v/%v", f.StartedAt, f.EndedAt)
	}
	if agg.Runs[0].Facts.AdapterVersion != "named" || agg.Runs[0].Facts.Outcome != "" {
		t.Errorf("bucket facts leaked into the named run: %+v", agg.Runs[0].Facts)
	}
}
