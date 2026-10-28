package digest_test

// Behaviour of the run digest for run-level unattributed usage: a digest of a
// run that has it carries it, reopening the digest reproduces the same run
// totals, overview share and by-agent row as the original aggregate, and a run
// without it (or a digest written before the field existed) is unchanged.

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
)

const unattributedModel = domain.ModelID("m-a")

func unattributedPricing(t *testing.T) domain.PricingTable {
	t.Helper()
	rate := func(s string) domain.Rate {
		r, err := domain.ParseRate(s)
		if err != nil {
			t.Fatalf("ParseRate(%q): %v", s, err)
		}
		return r
	}
	return domain.NewPricingTable([]domain.ModelPricing{{
		Model: unattributedModel,
		Standard: domain.RateSet{
			Input: rate("1.00"), CacheRead: rate("0.10"), CacheWrite5m: rate("1.25"),
			CacheWrite1h: rate("2.00"), Output: rate("5.00"),
		},
	}})
}

// runWithUnattributed is a named run whose orchestrator and one agent have
// usage plus run-level unattributed usage of its own.
func runWithUnattributed() domain.RunAggregate {
	return domain.RunAggregate{
		Run: domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: []domain.InvocationUsage{
			{Model: unattributedModel, RecordID: "o-1", At: at(1), Usage: domain.TokenUsage{Input: domain.Tokens(1000), Output: domain.Tokens(200)}},
		}},
		Agents: []domain.ActorAggregate{{
			Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"},
			Invocations: []domain.InvocationUsage{
				{Model: unattributedModel, RecordID: "a-1", At: at(2), Usage: domain.TokenUsage{Input: domain.Tokens(2000), Output: domain.Tokens(100)}},
			},
		}},
		Unattributed: &domain.ActorAggregate{Actor: domain.Unattributed(), Invocations: []domain.InvocationUsage{
			{Model: unattributedModel, Provider: "github-copilot", Harness: domain.HarnessVSCodeGHCP, RecordID: "u-1", At: at(3),
				Usage: domain.TokenUsage{Input: domain.Tokens(1000), CacheRead: domain.Tokens(0), Output: domain.Tokens(100)}},
			{Model: "m-other", RecordID: "u-2", At: at(4), Usage: domain.TokenUsage{Output: domain.Tokens(7), Reasoning: domain.Tokens(3)}},
		}},
	}
}

func digestOf(runs ...domain.RunAggregate) domain.Digest {
	return domain.Digest{Aggregate: domain.Aggregate{Runs: runs}}
}

func TestRoundTrip_UnattributedUsageIsReproduced(t *testing.T) {
	want := digestOf(runWithUnattributed())

	got, findings := roundTrip(t, want)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	if got.Aggregate.Runs[0].Unattributed == nil {
		t.Fatal("unattributed usage was lost")
	}
	requireSame(t, "aggregate", want.Aggregate, got.Aggregate)
}

func TestRoundTrip_UnattributedUsageStaysOutOfOrchestratorAndAgents(t *testing.T) {
	want := runWithUnattributed()

	got, _ := roundTrip(t, digestOf(want))

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	run := got.Aggregate.Runs[0]
	if len(run.Orchestrator.Invocations) != len(want.Orchestrator.Invocations) {
		t.Errorf("orchestrator usage entries = %d, want %d", len(run.Orchestrator.Invocations), len(want.Orchestrator.Invocations))
	}
	if len(run.Agents) != 1 {
		t.Errorf("agents = %d, want 1", len(run.Agents))
	}
	if run.Unattributed == nil || run.Unattributed.Actor.Kind != domain.ActorUnattributed {
		t.Fatalf("unattributed actor = %+v", run.Unattributed)
	}
	if len(run.Unattributed.Invocations) != 2 {
		t.Errorf("unattributed usage entries = %d, want 2", len(run.Unattributed.Invocations))
	}
}

func TestRoundTrip_UnattributedUsageKeepsRunTotalsAndModels(t *testing.T) {
	want := runWithUnattributed()

	got, _ := roundTrip(t, digestOf(want))

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	requireSame(t, "run totals", want.Totals(), got.Aggregate.Runs[0].Totals())
	if in, ok := got.Aggregate.Runs[0].Totals().Input.Value(); !ok || in != 4000 {
		t.Errorf("run input = %v present=%v, want 4000 including unattributed", in, ok)
	}
	if !reflect.DeepEqual(want.Models(), got.Aggregate.Runs[0].Models()) {
		t.Errorf("models = %v, want %v", got.Aggregate.Runs[0].Models(), want.Models())
	}
}

func TestRoundTrip_UnattributedUsageReproducesOverviewShareAndByAgentRow(t *testing.T) {
	table := unattributedPricing(t)
	want := runWithUnattributed()
	src := domain.Source{Kind: domain.SourceDigestFile, Path: "x"}
	before, _ := analysis.Price(domain.Aggregate{Runs: []domain.RunAggregate{want}}, table, src)

	got, _ := roundTrip(t, digestOf(want))
	after, _ := analysis.Price(got.Aggregate, table, src)

	requireAtLeast(t, "priced runs", len(after.Runs), 1)
	b, a := before.Runs[0], after.Runs[0]
	if a.Unattributed == nil || b.Unattributed == nil {
		t.Fatalf("by-agent unattributed row missing: before %v, after %v", b.Unattributed != nil, a.Unattributed != nil)
	}
	if !reflect.DeepEqual(b.Unattributed.Totals, a.Unattributed.Totals) {
		t.Errorf("unattributed totals differ: %+v vs %+v", b.Unattributed.Totals, a.Unattributed.Totals)
	}
	if !reflect.DeepEqual(b.Totals, a.Totals) {
		t.Errorf("run totals differ: %+v vs %+v", b.Totals, a.Totals)
	}
	if a.Overview.Unattributed == nil || !reflect.DeepEqual(b.Overview.Unattributed, a.Overview.Unattributed) {
		t.Errorf("overview unattributed totals differ: %+v vs %+v", b.Overview.Unattributed, a.Overview.Unattributed)
	}
	if a.Overview.UnattributedShare == (domain.Ratio{}) || a.Overview.UnattributedShare != b.Overview.UnattributedShare {
		t.Errorf("overview share = %+v, want %+v (non-zero)", a.Overview.UnattributedShare, b.Overview.UnattributedShare)
	}
	if !reflect.DeepEqual(b.ByAgentType, a.ByAgentType) {
		t.Errorf("agent-type groups changed: %+v vs %+v", b.ByAgentType, a.ByAgentType)
	}
}

func TestRoundTrip_RunWithoutUnattributedUsageStaysWithout(t *testing.T) {
	want := digestOf(richRun())

	got, _ := roundTrip(t, want)

	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	if got.Aggregate.Runs[0].Unattributed != nil {
		t.Errorf("decoded unattributed usage that was never written: %+v", got.Aggregate.Runs[0].Unattributed)
	}
	requireSame(t, "aggregate", want.Aggregate, got.Aggregate)
}

func TestEncode_UnattributedUsageIsWrittenUnderItsOwnRunKey(t *testing.T) {
	doc := readJSONObject(t, writeDigest(t, digestOf(runWithUnattributed())))

	runs, _ := doc["runs"].([]any)
	requireAtLeast(t, "runs", len(runs), 1)
	run, _ := runs[0].(map[string]any)
	un, _ := run["unattributed"].(map[string]any)
	if un == nil {
		t.Fatalf("run has no unattributed key: %v", run)
	}
	if usage, _ := un["usage"].([]any); len(usage) != 2 {
		t.Errorf("unattributed usage = %v, want 2 entries", un["usage"])
	}
	orch, _ := run["orchestrator"].(map[string]any)
	if usage, _ := orch["usage"].([]any); len(usage) != 1 {
		t.Errorf("orchestrator usage = %v, want only its own entry", orch["usage"])
	}
}

func TestEncode_RunWithoutUnattributedUsageOmitsTheKey(t *testing.T) {
	doc := readJSONObject(t, writeDigest(t, digestOf(richRun(), sparseRun())))

	runs, _ := doc["runs"].([]any)
	for i, r := range runs {
		if _, ok := r.(map[string]any)["unattributed"]; ok {
			t.Errorf("run %d carries an unattributed key without unattributed usage", i)
		}
	}
}

func TestVersion_AdditiveFieldBumpsTheMinorVersion(t *testing.T) {
	if digest.Version != "1.1.0" {
		t.Errorf("digest.Version = %q, want 1.1.0 (additive change of major 1)", digest.Version)
	}
	doc := readJSONObject(t, writeDigest(t, digestOf(runWithUnattributed())))
	if doc["digest_version"] != "1.1.0" {
		t.Errorf("written digest_version = %v, want 1.1.0", doc["digest_version"])
	}
}

func TestRead_OlderDigestWithoutUnattributedKeyLoadsWithoutIt(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "digest", "older", "older.digest.json")

	got, findings := readDigest(t, path)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	run := got.Aggregate.Runs[0]
	if run.Unattributed != nil {
		t.Errorf("older digest yielded unattributed usage: %+v", run.Unattributed)
	}
	if in, ok := run.Totals().Input.Value(); !ok || in != 120 {
		t.Errorf("run input = %v present=%v, want 120", in, ok)
	}
}

func TestRead_UnattributedKeyIsDecodedFromRawDocument(t *testing.T) {
	doc := validDoc("1.1.0")
	doc["runs"] = []any{map[string]any{
		"run_id": runA,
		"unattributed": map[string]any{"usage": []any{
			map[string]any{"record_id": "u-1", "model": "m-a", "tokens": map[string]any{"input_tokens": 40, "output_tokens": 2}},
		}},
	}}

	got, findings := readDigest(t, writeRawDoc(t, doc))

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireAtLeast(t, "runs", len(got.Aggregate.Runs), 1)
	un := got.Aggregate.Runs[0].Unattributed
	if un == nil || un.Actor.Kind != domain.ActorUnattributed {
		t.Fatalf("unattributed = %+v", un)
	}
	requireAtLeast(t, "unattributed usage", len(un.Invocations), 1)
	if v, ok := un.Invocations[0].Usage.Input.Value(); !ok || v != 40 || un.Invocations[0].RecordID != "u-1" {
		t.Errorf("entry = %+v", un.Invocations[0])
	}
	if v, ok := got.Aggregate.Runs[0].Totals().Input.Value(); !ok || v != 40 {
		t.Errorf("run input = %v present=%v, want 40", v, ok)
	}
}
