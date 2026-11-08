package activity_test

// Status tests for the GHCP CLI decoder.

import (
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func ghcpUsage(model string, in, cacheRead, cacheWrite int, parent string) string {
	s := `{"type":"assistant.usage","data":{"model":"` + model + `","outputTokens":3`
	if in >= 0 {
		s += `,"inputTokens":` + strconv.Itoa(in)
	}
	s += `,"cacheReadTokens":` + strconv.Itoa(cacheRead) + `,"cacheWriteTokens":` + strconv.Itoa(cacheWrite)
	if parent != "" {
		s += `,"parentToolCallId":"` + parent + `"`
	}
	return s + `}}`
}

func TestGHCPCLIDecoder_Status_Transcript_TracksEachFactAfterEveryLine(t *testing.T) {
	const m = "gpt-5"
	got := statusAfterEachLine(t, activity.NewGHCPCLIDecoder(), "ghcpcli_status_transcript.jsonl")

	assertStatuses(t, got, []domain.ActivityStatus{
		{},
		{Model: m, ContextTokens: 160, HasContext: true},
		{Model: m, ContextTokens: 160, HasContext: true, RunningTool: "view", RunningToolID: "t1"},
		{Model: m, ContextTokens: 160, HasContext: true, RunningTool: "view", RunningToolID: "t1"}, // sub-agent tool ignored
		{Model: m, ContextTokens: 160, HasContext: true, RunningTool: "view", RunningToolID: "t1"}, // sub-agent call ignored
		{Model: m, ContextTokens: 160, HasContext: true},
		{Model: m, ContextTokens: 340, HasContext: true}, // latest call, not summed
		{Model: m, ContextTokens: 340, HasContext: true}, // no USD cost for GHCP
	})
}

func TestGHCPCLIDecoder_Status_ContextSumsInputAndBothCacheCounters(t *testing.T) {
	got := statusAfter(activity.NewGHCPCLIDecoder(), ghcpUsage("gpt-5", 100, 50, 10, ""))

	if !got.HasContext || got.ContextTokens != 160 || got.Model != "gpt-5" {
		t.Errorf("want gpt-5 with context 160, got %+v", got)
	}
}

func TestGHCPCLIDecoder_Status_MissingTokenFieldsCountAsZero(t *testing.T) {
	got := statusAfter(activity.NewGHCPCLIDecoder(),
		`{"type":"assistant.usage","data":{"model":"gpt-5","cacheReadTokens":25}}`)

	if !got.HasContext || got.ContextTokens != 25 {
		t.Errorf("want context 25, got %+v", got)
	}
}

func TestGHCPCLIDecoder_Status_SubagentUsage_IsExcludedFromModelAndContext(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()

	alone := statusAfter(dec, ghcpUsage("sub-model", 9000, 0, 0, "tsub"))
	if alone.HasContext || alone.Model != "" {
		t.Fatalf("a sub-agent call alone must leave the status empty, got %+v", alone)
	}

	statusAfter(dec, ghcpUsage("gpt-5", 10, 0, 0, ""))
	got := statusAfter(dec, ghcpUsage("sub-model", 9000, 0, 0, "tsub"))
	if got.ContextTokens != 10 || got.Model != "gpt-5" {
		t.Errorf("sub-agent call must not replace main-thread facts, got %+v", got)
	}
}

func TestGHCPCLIDecoder_Status_SubagentStarted_IsNotTheModelSource(t *testing.T) {
	got := statusAfter(activity.NewGHCPCLIDecoder(),
		`{"type":"subagent.started","data":{"toolCallId":"t2","agentName":"reviewer","model":"other-model"}}`)

	if got.Model != "" {
		t.Errorf("a spawned sub-agent is not the agent's own model, got %q", got.Model)
	}
}

func TestGHCPCLIDecoder_Status_ToolStartAndComplete(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()

	started := statusAfter(dec,
		`{"type":"tool.execution_start","data":{"toolCallId":"t1","toolName":"view","arguments":{"path":"a"}}}`)
	if started.RunningTool != "view" || started.RunningToolID != "t1" {
		t.Fatalf("want view/t1 running, got %+v", started)
	}

	done := statusAfter(dec, `{"type":"tool.execution_complete","data":{"toolCallId":"t1","success":true}}`)
	if done.RunningToolID != "" || done.RunningTool != "" {
		t.Errorf("want no running tool after completion, got %+v", done)
	}
}

func TestGHCPCLIDecoder_Status_CompleteForUnknownToolID_ChangesNothing(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	statusAfter(dec, `{"type":"tool.execution_start","data":{"toolCallId":"t1","toolName":"view","arguments":{}}}`)

	got := statusAfter(dec, `{"type":"tool.execution_complete","data":{"toolCallId":"other","success":true}}`)

	if got.RunningToolID != "t1" || got.RunningTool != "view" {
		t.Errorf("an unrelated completion must not close t1, got %+v", got)
	}
}

func TestGHCPCLIDecoder_Status_OverlappingTools_ReportTheNewestOpenOne(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	start := func(id, name string) string {
		return `{"type":"tool.execution_start","data":{"toolCallId":"` + id + `","toolName":"` + name + `","arguments":{}}}`
	}
	done := func(id string) string {
		return `{"type":"tool.execution_complete","data":{"toolCallId":"` + id + `","success":true}}`
	}
	statusAfter(dec, start("a", "view"))
	if got := statusAfter(dec, start("b", "grep")); got.RunningToolID != "b" {
		t.Fatalf("want b running, got %+v", got)
	}

	if got := statusAfter(dec, done("b")); got.RunningToolID != "a" || got.RunningTool != "view" {
		t.Errorf("after b completed, a/view must show, got %+v", got)
	}
	if got := statusAfter(dec, done("a")); got.RunningToolID != "" {
		t.Errorf("want none running, got %+v", got)
	}
}

func TestGHCPCLIDecoder_Status_NeverReportsAUSDCost(t *testing.T) {
	got := statusAfter(activity.NewGHCPCLIDecoder(),
		`{"type":"assistant.usage","data":{"model":"gpt-5","inputTokens":1,"cost":2.5}}`,
		`{"type":"result","exitCode":0,"usage":{"premiumRequests":3,"totalCostUSD":9.9}}`)

	if got.HasCost || got.CostUSD != 0 {
		t.Errorf("GHCP reports premium requests, not USD; got %+v", got)
	}
}

func TestGHCPCLIDecoder_Status_MalformedAndUnknownLines_KeepThePreviousStatus(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	want := statusAfter(dec, ghcpUsage("gpt-5", 10, 0, 0, ""))
	explicit := domain.ActivityStatus{Model: "gpt-5", ContextTokens: 10, HasContext: true}
	if want != explicit {
		t.Fatalf("baseline must hold every reported fact: want %+v, got %+v", explicit, want)
	}

	for _, l := range []string{``, `garbage`, `{"type":`, `{"type":"session.usage_info","data":{}}`} {
		if got := statusAfter(dec, l); got != want {
			t.Errorf("line %q changed the status: want %+v, got %+v", l, want, got)
		}
	}
}
