package digest_test

// Call kind and call label in the run digest.
//
// Rules under test:
//   - Kind and label of every usage entry survive a write and a read, for the
//     orchestrator and for agents.
//   - A main call stores no call_kind; an empty label stores no call_label.
//   - A digest-loaded run gives the same context, compaction and cost figures
//     as the run it was written from.
//   - A digest written without the fields, or with an unknown kind, loads with
//     every call main and an empty label.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func kindUsage(id string, sec int, input int64, kind domain.CallKind, label string) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model: "model-a", Harness: domain.HarnessClaudeCode, RecordID: id, At: at(sec),
		Kind: kind, Label: label,
		Usage: domain.TokenUsage{Input: domain.Tokens(input), CacheRead: domain.Tokens(0), CacheCreation: domain.Tokens(0), Output: domain.Tokens(10)},
	}
}

// kindAggregate has main calls 100k -> 40k (a detectable drop) with helper calls
// of other kinds between them, for the orchestrator and for one agent.
func kindAggregate() domain.Aggregate {
	calls := func(prefix string) []domain.InvocationUsage {
		return []domain.InvocationUsage{
			kindUsage(prefix+"1", 10, 100_000, domain.CallMain, ""),
			kindUsage(prefix+"2", 20, 600, domain.CallAuxiliary, "title"),
			kindUsage(prefix+"3", 30, 900_000, domain.CallCompaction, "summarizeConversationHistory"),
			kindUsage(prefix+"4", 40, 800, domain.CallAuxiliary, ""),
			kindUsage(prefix+"5", 50, 40_000, domain.CallMain, "panel/editAgent"),
		}
	}
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	agentUsage := calls("a")
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:          domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: calls("o")},
		Agents:       []domain.ActorAggregate{{Actor: agent, Invocations: agentUsage}},
		Activity: domain.RunActivity{Invocations: []domain.Invocation{{
			Actor: agent, State: domain.InvocationClosed, StartedAt: at(0), EndedAt: at(100), Usage: agentUsage,
		}}},
	}}}
}

func kindPricing() domain.PricingTable {
	rate := func(s string) domain.Rate {
		r, err := domain.ParseRate(s)
		if err != nil {
			panic(err)
		}
		return r
	}
	return domain.NewPricingTable([]domain.ModelPricing{{
		Model: "model-a",
		Standard: domain.RateSet{
			Input: rate("3.00"), CacheRead: rate("0.30"), CacheWrite5m: rate("3.75"),
			CacheWrite1h: rate("6.00"), Output: rate("15.00"),
		},
	}})
}

func TestRoundTrip_CallKindAndLabel_SurviveForOrchestratorAndAgents(t *testing.T) {
	loaded, findings := roundTrip(t, domain.Digest{Aggregate: kindAggregate()})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}

	run := loaded.Aggregate.Runs[0]
	want := []struct {
		kind  domain.CallKind
		label string
	}{
		{domain.CallMain, ""},
		{domain.CallAuxiliary, "title"},
		{domain.CallCompaction, "summarizeConversationHistory"},
		{domain.CallAuxiliary, ""},
		{domain.CallMain, "panel/editAgent"},
	}
	for name, usage := range map[string][]domain.InvocationUsage{
		"orchestrator": run.Orchestrator.Invocations,
		"agent":        run.Agents[0].Invocations,
	} {
		requireAtLeast(t, name+" usage", len(usage), len(want))
		for i, w := range want {
			if usage[i].Kind != w.kind || usage[i].Label != w.label {
				t.Errorf("%s usage %d = {%q %q}, want {%q %q}", name, i, usage[i].Kind, usage[i].Label, w.kind, w.label)
			}
		}
	}
}

func TestWrite_MainCallStoresNoCallKind_AndEmptyLabelStoresNoCallLabel(t *testing.T) {
	doc := readJSONObject(t, writeDigest(t, domain.Digest{Aggregate: kindAggregate()}))

	runs := doc["runs"].([]any)
	orch := runs[0].(map[string]any)["orchestrator"].(map[string]any)
	usage := orch["usage"].([]any)
	if len(usage) != 5 {
		t.Fatalf("usage entries = %d, want 5", len(usage))
	}
	main := usage[0].(map[string]any)
	if _, has := main["call_kind"]; has {
		t.Errorf("main call stores call_kind = %v, want the key omitted", main["call_kind"])
	}
	if _, has := main["call_label"]; has {
		t.Errorf("call without a label stores call_label = %v, want the key omitted", main["call_label"])
	}
	aux := usage[1].(map[string]any)
	if aux["call_kind"] != "auxiliary" || aux["call_label"] != "title" {
		t.Errorf("auxiliary entry = %v, want call_kind auxiliary and call_label title", aux)
	}
	comp := usage[2].(map[string]any)
	if comp["call_kind"] != "compaction" {
		t.Errorf("compaction entry call_kind = %v, want compaction", comp["call_kind"])
	}
	labelledMain := usage[4].(map[string]any)
	if _, has := labelledMain["call_kind"]; has || labelledMain["call_label"] != "panel/editAgent" {
		t.Errorf("labelled main entry = %v, want a call_label and no call_kind", labelledMain)
	}
}

func TestRoundTrip_ContextCompactionAndCostFigures_EqualThoseOfTheLiveRun(t *testing.T) {
	agg := kindAggregate()
	loaded, _ := roundTrip(t, domain.Digest{Aggregate: agg})
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}

	live, _ := analysis.Price(agg, kindPricing(), domain.DefaultAnalysisSettings(), src)
	restored, _ := analysis.Price(loaded.Aggregate, kindPricing(), domain.DefaultAnalysisSettings(), src)

	want, got := live.Runs[0], restored.Runs[0]
	if !reflect.DeepEqual(got.Context, want.Context) {
		t.Errorf("digest-loaded context differs from the live run:\n got %+v\nwant %+v", got.Context, want.Context)
	}
	if !reflect.DeepEqual(got.Totals, want.Totals) {
		t.Errorf("digest-loaded totals differ from the live run:\n got %+v\nwant %+v", got.Totals, want.Totals)
	}
	// The comparison above is vacuous unless helper calls changed the figures.
	if v, _ := want.Context.Orchestrator.Peak.Tokens.Value(); v != 100_000 {
		t.Errorf("live peak = %d, want 100000 (main calls only)", v)
	}
	if n := want.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Errorf("live detected compactions = %d, want 1", n)
	}
	if v, _ := got.Context.Orchestrator.Peak.Tokens.Value(); v != 100_000 {
		t.Errorf("digest-loaded peak = %d, want 100000 (main calls only)", v)
	}
	if n := got.Context.Orchestrator.DetectedCount(); n != 1 {
		t.Errorf("digest-loaded detected compactions = %d, want 1", n)
	}
}

func kindDocWithUsage(version string, entry map[string]any) map[string]any {
	entry["record_id"] = "r1"
	entry["model"] = "m"
	entry["tokens"] = map[string]any{"input_tokens": 5}
	doc := validDoc(version)
	doc["runs"] = []any{map[string]any{
		"run_id":       runA,
		"orchestrator": map[string]any{"usage": []any{entry}},
	}}
	return doc
}

func TestRead_DigestWithoutCallKindFields_LoadsEveryCallAsMain(t *testing.T) {
	path := writeRawDoc(t, kindDocWithUsage("1.2.0", map[string]any{}))

	got, findings := readDigest(t, path)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	usage := got.Aggregate.Runs[0].Orchestrator.Invocations
	requireAtLeast(t, "orchestrator usage", len(usage), 1)
	if usage[0].Kind != domain.CallMain || usage[0].Label != "" {
		t.Errorf("usage = {%q %q}, want {main \"\"}", usage[0].Kind, usage[0].Label)
	}
}

func TestRead_UnknownCallKind_LoadsAsMain(t *testing.T) {
	path := writeRawDoc(t, kindDocWithUsage("1.3.0", map[string]any{"call_kind": "background", "call_label": "x"}))

	got, _ := readDigest(t, path)

	usage := got.Aggregate.Runs[0].Orchestrator.Invocations
	requireAtLeast(t, "orchestrator usage", len(usage), 1)
	if usage[0].Kind != domain.CallMain {
		t.Errorf("Kind = %q, want main for an unknown kind", usage[0].Kind)
	}
}

func TestRead_CallKindInADigest_IsRestored(t *testing.T) {
	path := writeRawDoc(t, kindDocWithUsage("1.3.0", map[string]any{"call_kind": "compaction", "call_label": "summarizeConversationHistory"}))

	got, _ := readDigest(t, path)

	usage := got.Aggregate.Runs[0].Orchestrator.Invocations
	requireAtLeast(t, "orchestrator usage", len(usage), 1)
	if usage[0].Kind != domain.CallCompaction || usage[0].Label != "summarizeConversationHistory" {
		t.Errorf("usage = {%q %q}, want {compaction summarizeConversationHistory}", usage[0].Kind, usage[0].Label)
	}
}
