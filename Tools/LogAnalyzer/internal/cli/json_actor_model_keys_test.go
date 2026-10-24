package cli_test

// The report JSON names, per actor, every (provider, model) pair the actor's
// requests used, so a consumer can form "provider/model" exactly as logged.
//
// Shape under test: each actor object carries "model_keys", an array of
// {"provider": <string>, "model": <string>} entries, one per distinct pair,
// ordered by model id and then provider. A request that logged no provider
// yields an entry whose provider is the empty string (never defaulted).
//
// Rules under test:
//   - The orchestrator and every agent actor expose their pairs.
//   - One model used under two providers appears once per provider.
//   - The existing "models" list stays the distinct model ids, and the other
//     existing actor keys keep their values.
//   - `total --format json` for the same run stays free of the new field.

import (
	"bytes"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const modelKeysKey = "model_keys"

// actorPairsReport analyses a run shaped like an OpenCode session: the
// orchestrator uses openai/gpt-5; agent a#1 uses openai/gpt-5 and
// anthropic/claude-opus; agent a#2 uses "shared" under two providers and
// "bare" without any provider.
func actorPairsReport() domain.Report {
	return providerGapReportFrom(
		[]domain.Event{
			providerGapRecord("", "o1", "gpt-5", "openai"),
			providerGapRecord("", "o2", "gpt-5", "openai"),
		},
		map[domain.AgentInstanceID][]domain.Event{
			"a#1": {
				providerGapRecord("a#1", "r1", "gpt-5", "openai"),
				providerGapRecord("a#1", "r2", "claude-opus", "anthropic"),
			},
			"a#2": {
				providerGapRecord("a#2", "r3", "shared", "p2"),
				providerGapRecord("a#2", "r4", "shared", "p1"),
				providerGapRecord("a#2", "r5", "bare", ""),
			},
		})
}

// pairsOf decodes an actor object's model_keys into "provider|model"
// strings in the order emitted.
func pairsOf(t *testing.T, actor map[string]any) []string {
	t.Helper()
	items, ok := actor[modelKeysKey].([]any)
	if !ok {
		t.Fatalf("actor has no %q array: %v", modelKeysKey, actor)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		e := vwObj(t, it)
		provider, pok := e["provider"].(string)
		model, mok := e["model"].(string)
		if !pok || !mok {
			t.Fatalf("entry %v must carry string provider and model", e)
		}
		out = append(out, provider+"|"+model)
	}
	return out
}

func agentByLabel(t *testing.T, run map[string]any, label string) map[string]any {
	t.Helper()
	for _, a := range vwList(t, run, "agents") {
		if obj := vwObj(t, a); obj["actor"] == label {
			return obj
		}
	}
	t.Fatalf("agent %q not found in report", label)
	return nil
}

func TestEncodeReport_Orchestrator_ListsProviderWithModel(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, actorPairsReport()))

	got := pairsOf(t, objectAt(t, run, "orchestrator"))

	if want := []string{"openai|gpt-5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("orchestrator model_keys = %q, want %q", got, want)
	}
}

func TestEncodeReport_Agent_ListsEveryProviderModelPair(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, actorPairsReport()))

	got := pairsOf(t, agentByLabel(t, run, "a#1"))

	want := []string{"anthropic|claude-opus", "openai|gpt-5"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a#1 model_keys = %q, want %q", got, want)
	}
}

func TestEncodeReport_SameModelUnderTwoProviders_ListsBothAndKeepsOneModelEntry(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, actorPairsReport()))
	agent := agentByLabel(t, run, "a#2")

	got := pairsOf(t, agent)

	want := []string{"|bare", "p1|shared", "p2|shared"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a#2 model_keys = %q, want %q", got, want)
	}
	if models := stringList(t, agent["models"]); !reflect.DeepEqual(models, []string{"bare", "shared"}) {
		t.Errorf("a#2 models = %q, want [bare shared]", models)
	}
}

func TestEncodeReport_ActorWithoutProvider_KeepsEmptyProviderVisible(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, actorPairsReport()))

	entries := vwList(t, agentByLabel(t, run, "a#2"), modelKeysKey)

	first := vwObj(t, entries[0])
	if p, present := first["provider"]; !present || p != "" {
		t.Errorf("provider-less entry = %v, want provider key present and empty", first)
	}
	if first["model"] != "bare" {
		t.Errorf("provider-less entry model = %v, want bare", first["model"])
	}
}

func TestEncodeReport_Actor_KeepsExistingKeysAndReplacesModelProvidersWithModelKeys(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, actorPairsReport()))
	orch := objectAt(t, run, "orchestrator")

	got := sortedKeys(orch)

	want := []string{"actor", "effort", "model_keys", "models", "requests_without_model", "totals", "unpriced_models"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("orchestrator keys = %q, want %q", got, want)
	}
	if models := stringList(t, orch["models"]); !reflect.DeepEqual(models, []string{"gpt-5"}) {
		t.Errorf("orchestrator models = %q, want [gpt-5]", models)
	}
	tokens := objectAt(t, objectAt(t, orch, "totals"), "tokens")
	if tokens["input"] != float64(2000) {
		t.Errorf("orchestrator input tokens = %v, want 2000 (unchanged by the new fields)", tokens["input"])
	}
}

func TestEncodeRunTotal_WithProviders_DoesNotCarryModelKeysOrModelLessCount(t *testing.T) {
	report := actorPairsReport()
	total, ok := app.RunTotalFor(report, cliTestRunID)
	if !ok {
		t.Fatalf("run %s not found", cliTestRunID)
	}
	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}

	got := decodeObject(t, buf.Bytes())

	want := []string{"complete", "currency", "money", "provisional", "run_id", "schema_version", "tokens", "unpriced_models"}
	if keys := sortedKeys(got); !reflect.DeepEqual(keys, want) {
		t.Errorf("total keys = %q, want %q", keys, want)
	}
}

// providerGapReportFrom analyses one run from the orchestrator's events and
// each agent instance's events, with an empty pricing table.
func providerGapReportFrom(orch []domain.Event, agents map[domain.AgentInstanceID][]domain.Event) domain.Report {
	run := domain.NamedRun(cliTestRunID)
	streams := []analysis.Stream{{
		Ref:    domain.StreamRef{Run: run, Kind: domain.StreamOrchestrator},
		Events: orch,
	}}
	for id, events := range agents {
		streams = append(streams, analysis.Stream{
			Ref:    domain.StreamRef{Run: run, Kind: domain.StreamAgentInstance, InstanceHint: id},
			Events: events,
		})
	}
	return analysis.Analyze(analysis.Input{Streams: streams}, domain.EmptyPricingTable(),
		domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"}, nil)
}
