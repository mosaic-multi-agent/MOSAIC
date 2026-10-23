package cli_test

// Tests that JSON outputs keep `unpriced_models` as a list of model-id strings
// for a run whose gaps are tracked per (provider, model).
//
// Rules under test:
//   - `total --format json` lists each model id once (distinct, sorted), even
//     when the same model is unpriced under several providers; entries never
//     carry the provider.
//   - A model priced under one provider and unpriced under another is still
//     listed (it is a gap), once.
//   - The report JSON follows the same rule for its unpriced_models arrays.

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func providerGapRecord(id domain.AgentInstanceID, recordID string, model domain.ModelID, provider domain.ProviderID) domain.Event {
	usage := domain.TokenUsage{Input: domain.Tokens(1000), Output: domain.Tokens(100)}
	return domain.Event{
		Type: domain.EventUsageRecord,
		UsageRecord: &domain.UsageRecordFields{
			AgentInstanceID: id, RecordID: recordID, Model: model, Provider: provider,
			Usage: usage, HasUsage: true,
		},
	}
}

// providerGapReport analyses a run in which "beta" is used under providers p1
// and p2, "alpha" under p1 only and "gamma" without provider.
func providerGapReport(table domain.PricingTable) domain.Report {
	run := domain.NamedRun(cliTestRunID)
	agent := func(id domain.AgentInstanceID, events ...domain.Event) analysis.Stream {
		return analysis.Stream{
			Ref:    domain.StreamRef{Run: run, Kind: domain.StreamAgentInstance, InstanceHint: id},
			Events: events,
		}
	}
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: domain.StreamRef{Run: run, Kind: domain.StreamOrchestrator}, Events: []domain.Event{
			providerGapRecord("", "o1", "beta", "p1"),
		}},
		agent("a#1",
			providerGapRecord("a#1", "r1", "beta", "p2"),
			providerGapRecord("a#1", "r2", "alpha", "p1"),
			providerGapRecord("a#1", "r3", "gamma", "")),
	}}
	return analysis.Analyze(in, table, domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"}, nil)
}

func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, raw)
	}
	return m
}

func stringList(t *testing.T, v any) []string {
	t.Helper()
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("unpriced_models = %#v, want an array", v)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			t.Fatalf("unpriced_models item %#v is not a string", it)
		}
		out = append(out, s)
	}
	return out
}

func TestEncodeRunTotal_ProviderGaps_ListsEachModelIDOnce(t *testing.T) {
	report := providerGapReport(domain.EmptyPricingTable())
	total, ok := app.RunTotalFor(report, cliTestRunID)
	if !ok {
		t.Fatalf("run %s not found", cliTestRunID)
	}

	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}

	got := stringList(t, decodeObject(t, buf.Bytes())["unpriced_models"])
	if want := []string{"alpha", "beta", "gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("unpriced_models = %q, want %q", got, want)
	}
}

func TestEncodeRunTotal_ModelPricedUnderOtherProvider_StillListedOnce(t *testing.T) {
	betaP1 := flatPricing("beta")
	betaP1.Provider = "p1"
	report := providerGapReport(domain.NewPricingTable([]domain.ModelPricing{betaP1}))
	total, _ := app.RunTotalFor(report, cliTestRunID)

	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}

	got := stringList(t, decodeObject(t, buf.Bytes())["unpriced_models"])
	if want := []string{"alpha", "beta", "gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("unpriced_models = %q, want %q (beta stays a gap under p2)", got, want)
	}
}

func TestEncodeReport_ProviderGaps_UnpricedModelsAreModelIDStrings(t *testing.T) {
	report := providerGapReport(domain.EmptyPricingTable())

	var buf bytes.Buffer
	if err := cli.EncodeReport(&buf, report); err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}

	doc := decodeObject(t, buf.Bytes())
	want := []string{"alpha", "beta", "gamma"}
	if got := stringList(t, doc["unpriced_models"]); !reflect.DeepEqual(got, want) {
		t.Errorf("report unpriced_models = %q, want %q", got, want)
	}
	runs, _ := doc["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	run, _ := runs[0].(map[string]any)
	if got := stringList(t, run["unpriced_models"]); !reflect.DeepEqual(got, want) {
		t.Errorf("run unpriced_models = %q, want %q", got, want)
	}
}
