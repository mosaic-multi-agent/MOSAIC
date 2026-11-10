package digest_test

// Behaviour of the run digest for requested tools: the field of every usage
// entry is written exactly when provided, reading the digest back reproduces
// provided, empty and not-provided apart, the digest version is the one that
// introduced the field, and older digests decode with the field not provided.

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func toolsCall(id string, sec int, in int64, tools domain.RequestedTools) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model: "model-a", Harness: domain.HarnessClaudeCode, RecordID: id, At: at(sec),
		Kind: domain.CallMain,
		Usage: domain.TokenUsage{
			Input: domain.Tokens(in), CacheRead: domain.Tokens(0), CacheCreation: domain.Tokens(0),
			Output: domain.Tokens(10),
		},
		RequestedTools: tools,
	}
}

// requestedToolsRun has provided, empty and not-provided calls on the
// orchestrator and on one invocation.
func requestedToolsRun() domain.RunAggregate {
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	orch := []domain.InvocationUsage{
		toolsCall("o1", 10, 100, domain.NewRequestedTools(map[string]int{"read_file": 2, "glob": 1})),
		toolsCall("o2", 20, 120, domain.NewRequestedTools(nil)),
		toolsCall("o3", 30, 140, domain.RequestedTools{}),
	}
	usage := []domain.InvocationUsage{
		toolsCall("a1", 15, 200, domain.NewRequestedTools(map[string]int{"edit": 1})),
		toolsCall("a2", 25, 220, domain.RequestedTools{}),
	}
	return domain.RunAggregate{
		Run:          domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: orch},
		Agents:       []domain.ActorAggregate{{Actor: agent, Invocations: usage}},
		Activity: domain.RunActivity{Invocations: []domain.Invocation{{
			Actor: agent, State: domain.InvocationClosed, StartedAt: at(0), EndedAt: at(100), Usage: usage,
		}}},
	}
}

func TestRoundTrip_RequestedToolsOfEveryUsageEntry_AreReproduced(t *testing.T) {
	want := domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{requestedToolsRun()}}}

	got, findings := roundTrip(t, want)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	run := got.Aggregate.Runs[0]
	orch := run.Orchestrator.Invocations
	if len(orch) != 3 {
		t.Fatalf("orchestrator usage entries = %d, want 3", len(orch))
	}
	if !orch[0].RequestedTools.IsProvided() || orch[0].RequestedTools.Count("read_file") != 2 || orch[0].RequestedTools.Count("glob") != 1 {
		t.Errorf("o1 requested tools = %v, want read_file 2 and glob 1", orch[0].RequestedTools.Counts())
	}
	if !orch[1].RequestedTools.IsProvided() || orch[1].RequestedTools.Total() != 0 {
		t.Errorf("o2 provided %v total %d, want provided and empty", orch[1].RequestedTools.IsProvided(), orch[1].RequestedTools.Total())
	}
	if orch[2].RequestedTools.IsProvided() {
		t.Error("o3 reads as provided, want not provided")
	}
	agentUsage := run.Agents[0].Invocations
	if len(agentUsage) != 2 || agentUsage[0].RequestedTools.Count("edit") != 1 || agentUsage[1].RequestedTools.IsProvided() {
		t.Errorf("agent usage requested tools = %v / provided %v, want edit 1 then not provided",
			agentUsage[0].RequestedTools.Counts(), agentUsage[1].RequestedTools.IsProvided())
	}
	requireSame(t, "round trip", want.Aggregate.Runs[0].Orchestrator.Invocations, orch)
	requireSame(t, "round trip agent", want.Aggregate.Runs[0].Agents[0].Invocations, agentUsage)
}

func TestWrite_RequestedToolsKey_IsWrittenExactlyWhenProvided(t *testing.T) {
	d := domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{requestedToolsRun()}}}

	doc := readJSONObject(t, writeDigest(t, d))

	runs := doc["runs"].([]any)
	usage := runs[0].(map[string]any)["orchestrator"].(map[string]any)["usage"].([]any)
	first, second, third := usage[0].(map[string]any), usage[1].(map[string]any), usage[2].(map[string]any)
	if !reflect.DeepEqual(first["requested_tools"], map[string]any{"read_file": float64(2), "glob": float64(1)}) {
		t.Errorf("provided key = %#v, want read_file 2 and glob 1", first["requested_tools"])
	}
	if v, ok := second["requested_tools"].(map[string]any); !ok || len(v) != 0 {
		t.Errorf("empty value written as %#v, want {}", second["requested_tools"])
	}
	if _, ok := third["requested_tools"]; ok {
		t.Error("a not-provided value is written as a key, want the key omitted")
	}
}

func TestWrite_DigestVersion_IsTheOneThatIntroducedRequestedTools(t *testing.T) {
	d := domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{requestedToolsRun()}}}

	doc := readJSONObject(t, writeDigest(t, d))

	if doc["digest_version"] != "1.5.0" {
		t.Errorf("digest_version = %v, want 1.5.0", doc["digest_version"])
	}
}

func TestRead_OlderDigestWithoutRequestedTools_DecodesWithFieldNotProvided(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "requestedtools", "older.digest.json")

	got, findings := readDigest(t, path)

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	if len(got.Aggregate.Runs) != 1 || len(got.Aggregate.Runs[0].Orchestrator.Invocations) != 1 {
		t.Fatalf("runs = %+v", got.Aggregate.Runs)
	}
	u := got.Aggregate.Runs[0].Orchestrator.Invocations[0]
	if u.RequestedTools.IsProvided() {
		t.Errorf("requested tools of an older digest = %v, want not provided", u.RequestedTools.Counts())
	}
	if v, ok := u.Usage.Input.Value(); !ok || v != 5 {
		t.Errorf("input = (%d, %v), want 5: older digests still decode", v, ok)
	}
}

func TestRead_CurrentDigestEntryWithInvalidRequestedTools_ReadsAsNotProvided(t *testing.T) {
	cases := map[string]any{
		"zero count":  map[string]any{"glob": 0},
		"text count":  map[string]any{"glob": "1"},
		"empty name":  map[string]any{"": 1},
		"array":       []any{"glob"},
		"null":        nil,
		"fractional":  map[string]any{"glob": 1.5},
		"mixed valid": map[string]any{"glob": 1, "edit": -2},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			doc := validDoc("1.5.0")
			doc["runs"] = []any{map[string]any{
				"run_id": runA,
				"orchestrator": map[string]any{"usage": []any{map[string]any{
					"record_id": "r1", "model": "m", "requested_tools": value,
					"tokens": map[string]any{"input_tokens": 5},
				}}},
			}}

			got, _ := readDigest(t, writeRawDoc(t, doc))

			u := got.Aggregate.Runs[0].Orchestrator.Invocations[0]
			if u.RequestedTools.IsProvided() {
				t.Errorf("invalid value read as provided: %v", u.RequestedTools.Counts())
			}
		})
	}
}

func TestRoundTrip_RequestedToolsFiguresOfADigestLoadedRun_EqualThoseOfTheOriginalRun(t *testing.T) {
	agg := domain.Aggregate{Runs: []domain.RunAggregate{requestedToolsRun()}}
	loaded, _ := roundTrip(t, domain.Digest{Aggregate: agg})
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}
	table := domain.NewPricingTable(nil)

	original, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), src)
	restored, _ := analysis.Price(loaded.Aggregate, table, domain.DefaultAnalysisSettings(), src)

	got, want := restored.Runs[0].Context, original.Runs[0].Context
	sameCalls := func(what string, g, w []domain.ContextCall) {
		if len(g) != len(w) {
			t.Fatalf("%s: calls = %d, want %d", what, len(g), len(w))
		}
		for i := range w {
			if !g[i].RequestedTools.Equal(w[i].RequestedTools) || g[i].RequestedTools.IsProvided() != w[i].RequestedTools.IsProvided() {
				t.Errorf("%s call %d requested tools = %v (provided %v), want %v (provided %v)", what, i,
					g[i].RequestedTools.Counts(), g[i].RequestedTools.IsProvided(),
					w[i].RequestedTools.Counts(), w[i].RequestedTools.IsProvided())
			}
			if g[i].Context != w[i].Context || g[i].Cost != w[i].Cost {
				t.Errorf("%s call %d figures differ: got %+v want %+v", what, i, g[i], w[i])
			}
		}
	}
	sameCalls("orchestrator", got.Orchestrator.Calls, want.Orchestrator.Calls)
	wc, _ := want.Find("coder#1")
	gc, _ := got.Find("coder#1")
	sameCalls("coder#1", gc.Calls, wc.Calls)
	// Vacuous unless the original run exposes the field on its calls.
	o1 := want.Orchestrator.Calls[0]
	if o1.RequestedTools.Count("read_file") != 2 {
		t.Errorf("original orchestrator call requested tools = %v, want read_file 2", o1.RequestedTools.Counts())
	}
	coder, ok := got.Find("coder#1")
	if !ok || len(coder.Calls) != 2 || coder.Calls[0].RequestedTools.Count("edit") != 1 || coder.Calls[1].RequestedTools.IsProvided() {
		t.Errorf("digest-loaded invocation calls = %+v, want edit 1 then not provided", coder.Calls)
	}
}
