package digest_test

// End-to-end tests: a run read from logs and the same run loaded from a digest
// report the same compactions.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func e2eCompaction(version string, phase domain.CompactionPhase, trigger string, before, after domain.TokenCount, sec int) domain.Event {
	return domain.Event{
		Type:          domain.EventCompaction,
		SchemaVersion: version,
		Timestamp:     at(sec),
		Compaction:    &domain.CompactionFields{Trigger: trigger, TokensBefore: before, TokensAfter: after, Phase: phase},
	}
}

func e2eAggregate(t *testing.T, events ...domain.Event) domain.Aggregate {
	t.Helper()
	ref := domain.StreamRef{Run: domain.NamedRun(runA), Kind: domain.StreamOrchestrator}
	agg, _ := analysis.Aggregate(analysis.Input{Streams: []analysis.Stream{{Ref: ref, Events: events}}})
	return agg
}

func e2eRunCompactions(t *testing.T, agg domain.Aggregate) []domain.Compaction {
	t.Helper()
	requireAtLeast(t, "runs", len(agg.Runs), 1)
	return agg.Runs[0].Activity.Compactions
}

func TestEndToEnd_PhasedAndLegacyPairs_DigestLoadedRunHasTheSameCompactions(t *testing.T) {
	agg := e2eAggregate(t,
		e2eCompaction("1.5.0", domain.CompactionPhasePre, "auto", domain.Tokens(900), domain.TokenCount{}, 10),
		e2eCompaction("1.5.0", domain.CompactionPhasePost, "", domain.TokenCount{}, domain.Tokens(100), 14),
		e2eCompaction("1.2.0", domain.CompactionPhaseNone, "manual", domain.Tokens(500), domain.TokenCount{}, 20),
		e2eCompaction("1.2.0", domain.CompactionPhaseNone, "", domain.TokenCount{}, domain.Tokens(50), 22),
		e2eCompaction("1.5.0", domain.CompactionPhasePre, "auto", domain.Tokens(7), domain.TokenCount{}, 30),
	)
	fromLogs := e2eRunCompactions(t, agg)

	loaded, findings := roundTrip(t, domain.Digest{Aggregate: agg})

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	if len(fromLogs) != 3 {
		t.Fatalf("compactions read from logs = %d, want 3 (pair, legacy pair, lone pre)", len(fromLogs))
	}
	requireSame(t, "compactions", fromLogs, e2eRunCompactions(t, loaded.Aggregate))
}

func TestEndToEnd_DigestWrittenBeforePairing_LoadsTheCountOfTheSameRunReadFromLogs(t *testing.T) {
	fromLogs := e2eRunCompactions(t, e2eAggregate(t,
		e2eCompaction("1.2.0", domain.CompactionPhaseNone, "auto", domain.Tokens(1000), domain.TokenCount{}, 10),
		e2eCompaction("1.2.0", domain.CompactionPhaseNone, "", domain.TokenCount{}, domain.Tokens(200), 12),
	))
	// The same two events as an older digest stored them: one entry per event.
	stored := readCompactions(t, "1.1.0",
		rawCompaction(orchActor, 10, map[string]any{"trigger": "auto", "tokens_before": 1000}),
		rawCompaction(orchActor, 12, map[string]any{"tokens_after": 200}),
	)

	if len(fromLogs) != 1 || len(stored) != 1 {
		t.Fatalf("compactions: from logs %d, from the older digest %d; want 1 and 1", len(fromLogs), len(stored))
	}
	a, b := fromLogs[0], stored[0]
	if a.Shape != b.Shape || !a.At.Equal(b.At) || !a.EndedAt.Equal(b.EndedAt) || a.Trigger != b.Trigger ||
		a.TokensBefore != b.TokensBefore || a.TokensAfter != b.TokensAfter {
		t.Errorf("older digest compaction = %+v, want the one read from logs %+v", b, a)
	}
}

func TestRead_UnknownCompactionShape_IsAnInvalidDigest(t *testing.T) {
	doc := validDoc("1.2.0")
	doc["runs"] = []any{map[string]any{"run_id": runA, "compactions": []any{
		rawCompaction(orchActor, 1, map[string]any{"shape": "triple", "at": at(1).Format(time.RFC3339Nano)}),
	}}}
	path := writeRawDoc(t, doc)

	got, findings := readDigest(t, path)

	assertOneInvalidFinding(t, path, got, findings)
}

func TestRead_EmptyOrAbsentCompactionShape_DecodesAsSingle(t *testing.T) {
	cs := readCompactions(t, "1.2.0",
		rawCompaction(orchActor, 1, map[string]any{"shape": "", "trigger": "a"}),
		rawCompaction(orchActor, 2, map[string]any{"trigger": "b"}),
	)

	requireAtLeast(t, "compactions", len(cs), 2)
	if cs[0].Shape != domain.CompactionSingle || cs[1].Shape != domain.CompactionSingle {
		t.Errorf("shapes = %v, %v; want single, single", cs[0].Shape, cs[1].Shape)
	}
}
