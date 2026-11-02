package digest_test

// Tests for the compaction round trip.
//
// Rules under test:
//   - The digest version is 1.2.0; a compaction is written with ended_at (omitted
//     when zero) and shape (single|paired|pre_only|post_only; omitted for single).
//   - Reading a digest of version 1.2.0 or later restores EndedAt and Shape as
//     written and applies no pairing.
//   - Reading a digest below 1.2.0 applies the legacy pairing rule to each
//     actor's stored compactions, so the count matches the one read from logs:
//     a stored entry with no tokens_after pairs with the next entry of the same
//     actor when that entry carries neither trigger nor tokens_before.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
)

func compactionRun(cs ...domain.Compaction) domain.Digest {
	run := sparseRun()
	run.Activity.Compactions = cs
	return domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}}}
}

func writtenCompactions(t *testing.T, d domain.Digest) []map[string]any {
	t.Helper()
	doc := readJSONObject(t, writeDigest(t, d))
	runs, _ := doc["runs"].([]any)
	if len(runs) == 0 {
		t.Fatal("digest has no runs")
	}
	raw, _ := runs[0].(map[string]any)["compactions"].([]any)
	out := make([]map[string]any, len(raw))
	for i, c := range raw {
		out[i], _ = c.(map[string]any)
	}
	return out
}

func rawCompaction(actor map[string]any, atSec int, extra map[string]any) map[string]any {
	c := map[string]any{"actor": actor, "at": at(atSec).Format(time.RFC3339Nano)}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

func readCompactions(t *testing.T, version string, compactions ...any) []domain.Compaction {
	t.Helper()
	doc := validDoc(version)
	doc["runs"] = []any{map[string]any{"run_id": runA, "compactions": compactions}}
	got, findings := readDigest(t, writeRawDoc(t, doc))
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	return got.Aggregate.Runs[0].Activity.Compactions
}

var orchActor = map[string]any{"kind": "orchestrator"}

func TestRoundTrip_PairedPreOnlyPostOnlyAndSingleCompactionsKeepSpanAndShape(t *testing.T) {
	coder := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	want := compactionRun(
		domain.Compaction{Actor: domain.Orchestrator(), At: at(10), EndedAt: at(14), Trigger: "auto",
			TokensBefore: domain.Tokens(900), TokensAfter: domain.Tokens(100), Shape: domain.CompactionPaired},
		domain.Compaction{Actor: coder, At: at(20), Trigger: "manual", TokensBefore: domain.Tokens(5), Shape: domain.CompactionPreOnly},
		domain.Compaction{Actor: coder, At: at(30), TokensAfter: domain.Tokens(6), Shape: domain.CompactionPostOnly},
		domain.Compaction{Actor: domain.Orchestrator(), At: at(40), Trigger: "auto", Shape: domain.CompactionSingle},
	)

	got, findings := roundTrip(t, want)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireSame(t, "compactions", want.Aggregate.Runs[0].Activity.Compactions, got.Aggregate.Runs[0].Activity.Compactions)
}

func TestEncode_CompactionWritesEndedAtAndShapeOnlyWhenTheyCarryInformation(t *testing.T) {
	d := compactionRun(
		domain.Compaction{Actor: domain.Orchestrator(), At: at(10), EndedAt: at(14), Shape: domain.CompactionPaired},
		domain.Compaction{Actor: domain.Orchestrator(), At: at(20), Shape: domain.CompactionPreOnly},
		domain.Compaction{Actor: domain.Orchestrator(), At: at(30), Shape: domain.CompactionPostOnly},
		domain.Compaction{Actor: domain.Orchestrator(), At: at(40), Shape: domain.CompactionSingle},
	)

	cs := writtenCompactions(t, d)

	if len(cs) != 4 {
		t.Fatalf("written compactions = %d, want 4", len(cs))
	}
	for i, wantShape := range []string{"paired", "pre_only", "post_only", ""} {
		got, present := cs[i]["shape"]
		if wantShape == "" && present {
			t.Errorf("compaction %d: shape = %v written for a single compaction, want omitted", i, got)
		}
		if wantShape != "" && got != wantShape {
			t.Errorf("compaction %d: shape = %v, want %q", i, got, wantShape)
		}
	}
	if _, present := cs[0]["ended_at"]; !present {
		t.Error("paired compaction wrote no ended_at")
	}
	for i := 1; i < 4; i++ {
		if v, present := cs[i]["ended_at"]; present {
			t.Errorf("compaction %d: ended_at = %v written for a zero EndedAt, want omitted", i, v)
		}
	}
}

func TestRead_Version120KeepsStoredShapesAndAppliesNoPairing(t *testing.T) {
	cs := readCompactions(t, "1.2.0",
		rawCompaction(orchActor, 1, map[string]any{"trigger": "auto", "tokens_before": 100}),
		rawCompaction(orchActor, 2, map[string]any{"tokens_after": 20}),
		rawCompaction(orchActor, 3, map[string]any{"shape": "paired", "ended_at": at(9).Format(time.RFC3339Nano), "trigger": "manual"}),
		rawCompaction(orchActor, 20, map[string]any{"shape": "pre_only"}),
		rawCompaction(orchActor, 30, map[string]any{"shape": "post_only"}),
	)

	want := []domain.CompactionShape{domain.CompactionSingle, domain.CompactionSingle, domain.CompactionPaired,
		domain.CompactionPreOnly, domain.CompactionPostOnly}
	if len(cs) != len(want) {
		t.Fatalf("compactions = %d, want %d: a 1.2.0 digest is not re-paired", len(cs), len(want))
	}
	for i := range want {
		if cs[i].Shape != want[i] {
			t.Errorf("compaction %d: Shape = %v, want %v", i, cs[i].Shape, want[i])
		}
	}
	if !cs[2].EndedAt.Equal(at(9)) {
		t.Errorf("EndedAt = %v, want %v", cs[2].EndedAt, at(9))
	}
}

func TestRead_OlderDigestLegacyPairIsOneCompaction(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.1.0"} {
		t.Run(version, func(t *testing.T) {
			cs := readCompactions(t, version,
				rawCompaction(orchActor, 10, map[string]any{"trigger": "auto", "tokens_before": 1000}),
				rawCompaction(orchActor, 12, map[string]any{"tokens_after": 200}),
			)

			if len(cs) != 1 {
				t.Fatalf("compactions = %d, want 1 (the stored pair)", len(cs))
			}
			c := cs[0]
			if c.Shape != domain.CompactionPaired || !c.At.Equal(at(10)) || !c.EndedAt.Equal(at(12)) || c.Trigger != "auto" {
				t.Errorf("compaction = %+v, want paired at %v..%v with trigger auto", c, at(10), at(12))
			}
			if v, ok := c.TokensBefore.Value(); !ok || v != 1000 {
				t.Errorf("TokensBefore = %v present=%v, want 1000", v, ok)
			}
			if v, ok := c.TokensAfter.Value(); !ok || v != 200 {
				t.Errorf("TokensAfter = %v present=%v, want 200", v, ok)
			}
		})
	}
}

func TestRead_OlderDigest_NonPairingEntriesStayOneCompactionEach(t *testing.T) {
	cs := readCompactions(t, "1.1.0",
		rawCompaction(orchActor, 1, map[string]any{"trigger": "auto", "tokens_before": 100, "tokens_after": 20}),
		rawCompaction(orchActor, 2, map[string]any{"tokens_after": 5}),
		rawCompaction(orchActor, 3, map[string]any{"trigger": "manual", "tokens_before": 7}),
		rawCompaction(orchActor, 4, map[string]any{"trigger": "manual", "tokens_after": 1}),
	)

	if len(cs) != 4 {
		t.Fatalf("compactions = %d, want 4: the first has tokens_after, the others carry a trigger or follow no first half", len(cs))
	}
	for i, c := range cs {
		if c.Shape != domain.CompactionSingle {
			t.Errorf("compaction %d: Shape = %v, want single", i, c.Shape)
		}
	}
}

func TestRead_OlderDigest_PairsPerActorOnly(t *testing.T) {
	coder := map[string]any{"kind": "agent_instance", "instance_id": "coder#1", "agent_type": "coder"}

	cs := readCompactions(t, "1.1.0",
		rawCompaction(orchActor, 1, map[string]any{"trigger": "auto", "tokens_before": 100}),
		rawCompaction(coder, 2, map[string]any{"tokens_after": 20}),
		rawCompaction(coder, 3, map[string]any{"trigger": "auto", "tokens_before": 50}),
		rawCompaction(coder, 4, map[string]any{"tokens_after": 9}),
	)

	if len(cs) != 3 {
		t.Fatalf("compactions = %d, want 3: orchestrator first half alone, coder lone second half, coder pair", len(cs))
	}
	paired := 0
	for _, c := range cs {
		if c.Shape == domain.CompactionPaired {
			paired++
			if c.Actor.Instance != "coder#1" {
				t.Errorf("paired compaction actor = %+v, want coder#1", c.Actor)
			}
		}
	}
	if paired != 1 {
		t.Errorf("paired compactions = %d, want 1", paired)
	}
}

func TestRead_OlderDigest_UnattributableBucketIsPairedToo(t *testing.T) {
	doc := validDoc("1.1.0")
	doc["unattributable"] = map[string]any{"compactions": []any{
		rawCompaction(orchActor, 1, map[string]any{"trigger": "auto", "tokens_before": 100}),
		rawCompaction(orchActor, 2, map[string]any{"tokens_after": 20}),
	}}

	got, findings := readDigest(t, writeRawDoc(t, doc))

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	if got.Aggregate.Unattributable == nil || len(got.Aggregate.Unattributable.Activity.Compactions) != 1 {
		t.Fatalf("unattributable = %+v, want one paired compaction", got.Aggregate.Unattributable)
	}
}

func TestRead_CurrentVersionDigestWrittenByThisPackage_KeepsTheCountOfTheSourceRun(t *testing.T) {
	if digest.Version != "1.5.0" {
		t.Fatalf("digest.Version = %q, want 1.5.0", digest.Version)
	}
	want := compactionRun(
		// Two stored singles that rule R1 would pair if the digest were treated as legacy.
		domain.Compaction{Actor: domain.Orchestrator(), At: at(1), Trigger: "auto", TokensBefore: domain.Tokens(100), Shape: domain.CompactionSingle},
		domain.Compaction{Actor: domain.Orchestrator(), At: at(3), TokensAfter: domain.Tokens(20), Shape: domain.CompactionSingle},
	)

	got, _ := roundTrip(t, want)

	if n := len(got.Aggregate.Runs[0].Activity.Compactions); n != 2 {
		t.Errorf("compactions after round trip = %d, want 2 (a current digest is not re-paired)", n)
	}
}
