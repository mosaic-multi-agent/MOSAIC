package activity_test

// Status tests for the Claude Code decoder: model, context size of the latest
// main-thread call, running tool, final cost.

import (
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

const claudeInit = `{"type":"system","subtype":"init","model":"claude-sonnet-4-5","session_id":"s1"}`

// claudeAssistant builds an assistant line; parent is "null" or a quoted id.
func claudeAssistant(msgID, parent, blockJSON, usageJSON string) string {
	return `{"type":"assistant","parent_tool_use_id":` + parent + `,"message":{"id":"` + msgID +
		`","role":"assistant","content":[` + blockJSON + `],"usage":` + usageJSON + `}}`
}

func claudeUsage(in, cacheRead, cacheWrite int) string {
	return `{"input_tokens":` + strconv.Itoa(in) + `,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(cacheWrite) + `,"output_tokens":7}`
}

const claudeTextBlock = `{"type":"text","text":"x"}`

func claudeToolBlock(id, name string) string {
	return `{"type":"tool_use","id":"` + id + `","name":"` + name + `","input":{"file_path":"a.go"}}`
}

func claudeToolResult(id, parent string) string {
	return `{"type":"user","parent_tool_use_id":` + parent + `,"message":{"role":"user","content":[` +
		`{"type":"tool_result","tool_use_id":"` + id + `","content":"ok"}]}}`
}

func TestClaudeCodeDecoder_Status_StartsAsTheZeroValue(t *testing.T) {
	if got := activity.NewClaudeCodeDecoder().Status(); got != (domain.ActivityStatus{}) {
		t.Errorf("want zero status before any line, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_Transcript_TracksEachFactAfterEveryLine(t *testing.T) {
	const m = "claude-sonnet-4-5"
	got := statusAfterEachLine(t, activity.NewClaudeCodeDecoder(), "claude_status_transcript.jsonl")

	assertStatuses(t, got, []domain.ActivityStatus{
		{Model: m},
		{Model: m, ContextTokens: 115, HasContext: true},
		{Model: m, ContextTokens: 115, HasContext: true, RunningTool: "Read", RunningToolID: "toolu_1"},
		{Model: m, ContextTokens: 115, HasContext: true},
		{Model: m, ContextTokens: 115, HasContext: true}, // sub-agent call excluded
		{Model: m, ContextTokens: 204, HasContext: true}, // latest call, not summed
		{Model: m, ContextTokens: 204, HasContext: true, CostUSD: 0.25, HasCost: true},
	})
}

func TestClaudeCodeDecoder_Status_ModelComesFromTheInitLine(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	if got := statusAfter(dec, claudeAssistant("m1", "null", claudeTextBlock, claudeUsage(1, 0, 0))); got.Model != "" {
		t.Errorf("model must stay empty until reported, got %q", got.Model)
	}

	got := statusAfter(dec, claudeInit)

	if got.Model != "claude-sonnet-4-5" {
		t.Errorf("want model from system/init, got %q", got.Model)
	}
}

func TestClaudeCodeDecoder_Status_ContextSumsInputAndBothCacheCounters(t *testing.T) {
	got := statusAfter(activity.NewClaudeCodeDecoder(),
		claudeAssistant("m1", "null", claudeTextBlock, claudeUsage(10, 100, 5)))

	if !got.HasContext || got.ContextTokens != 115 {
		t.Errorf("want context 115, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_MissingCacheFieldsCountAsZero(t *testing.T) {
	got := statusAfter(activity.NewClaudeCodeDecoder(),
		claudeAssistant("m1", "null", claudeTextBlock, `{"input_tokens":42}`))

	if !got.HasContext || got.ContextTokens != 42 {
		t.Errorf("want context 42, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_NoUsage_HasNoContext(t *testing.T) {
	got := statusAfter(activity.NewClaudeCodeDecoder(),
		`{"type":"assistant","parent_tool_use_id":null,"message":{"id":"m1","content":[{"type":"text","text":"x"}]}}`)

	if got.HasContext || got.ContextTokens != 0 {
		t.Errorf("a message without usage must not produce a context size, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_BlocksOfOneMessage_CountOnce(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	u := claudeUsage(10, 100, 5)

	statusAfter(dec, claudeAssistant("m1", "null", claudeTextBlock, u))
	statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("a", "Read"), u))
	got := statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("b", "Grep"), u))

	if got.ContextTokens != 115 {
		t.Errorf("three blocks of one call must count once (115), got %d", got.ContextTokens)
	}
}

func TestClaudeCodeDecoder_Status_NewMessage_ReplacesTheContextNotAddsToIt(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	statusAfter(dec, claudeAssistant("m1", "null", claudeTextBlock, claudeUsage(10, 100, 5)))

	got := statusAfter(dec, claudeAssistant("m2", "null", claudeTextBlock, claudeUsage(3, 50, 0)))

	if got.ContextTokens != 53 {
		t.Errorf("want the newest call's 53 tokens, got %d", got.ContextTokens)
	}
}

func TestClaudeCodeDecoder_Status_SubagentLines_AreExcluded(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()

	only := statusAfter(dec, claudeAssistant("s1", `"toolu_task"`, claudeToolBlock("x", "Read"), claudeUsage(9000, 9000, 9000)))
	if only.HasContext || only.ContextTokens != 0 || only.RunningToolID != "" {
		t.Fatalf("a sub-agent line alone must leave the status empty, got %+v", only)
	}

	statusAfter(dec, claudeAssistant("m1", "null", claudeTextBlock, claudeUsage(10, 0, 0)))
	got := statusAfter(dec, claudeAssistant("s2", `"toolu_task"`, claudeTextBlock, claudeUsage(5000, 0, 0)))
	if got.ContextTokens != 10 {
		t.Errorf("a later sub-agent call must not replace the main-thread context, got %d", got.ContextTokens)
	}
}

func TestClaudeCodeDecoder_Status_ToolStartAndFinish(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()

	started := statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("tu1", "Read"), claudeUsage(1, 0, 0)))
	if started.RunningTool != "Read" || started.RunningToolID != "tu1" {
		t.Fatalf("want Read/tu1 running, got %+v", started)
	}

	finished := statusAfter(dec, claudeToolResult("tu1", "null"))
	if finished.RunningTool != "" || finished.RunningToolID != "" {
		t.Errorf("want no running tool after the result, got %+v", finished)
	}
}

func TestClaudeCodeDecoder_Status_OverlappingTools_ReportTheNewestOpenOne(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("a", "Read"), claudeUsage(1, 0, 0)))
	got := statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("b", "Grep"), claudeUsage(1, 0, 0)))
	if got.RunningToolID != "b" || got.RunningTool != "Grep" {
		t.Fatalf("want the newest tool b/Grep, got %+v", got)
	}

	got = statusAfter(dec, claudeToolResult("b", "null"))
	if got.RunningToolID != "a" || got.RunningTool != "Read" {
		t.Errorf("after b finished the older open tool a/Read must show, got %+v", got)
	}

	got = statusAfter(dec, claudeToolResult("a", "null"))
	if got.RunningToolID != "" {
		t.Errorf("want no running tool once all finished, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_ResultForUnknownToolID_ChangesNothing(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("a", "Read"), claudeUsage(1, 0, 0)))

	got := statusAfter(dec, claudeToolResult("never-started", "null"))

	if got.RunningToolID != "a" {
		t.Errorf("an unrelated result must not close tool a, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_TaskAndAgentToolUse_AreRunningTools(t *testing.T) {
	for _, name := range []string{"Task", "Agent"} {
		dec := activity.NewClaudeCodeDecoder()

		started := statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("tk1", name), claudeUsage(1, 0, 0)))
		if started.RunningTool != name || started.RunningToolID != "tk1" {
			t.Fatalf("%s: want running %s/tk1, got %+v", name, name, started)
		}

		finished := statusAfter(dec, claudeToolResult("tk1", "null"))
		if finished.RunningTool != "" || finished.RunningToolID != "" {
			t.Errorf("%s: want no running tool after the result, got %+v", name, finished)
		}
	}
}

func TestClaudeCodeDecoder_Status_ResultOnASubagentLine_DoesNotCloseAMainThreadTool(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	statusAfter(dec, claudeAssistant("m1", "null", claudeToolBlock("a", "Read"), claudeUsage(1, 0, 0)))

	got := statusAfter(dec, claudeToolResult("a", `"toolu_task"`))

	if got.RunningToolID != "a" || got.RunningTool != "Read" {
		t.Errorf("only main-thread user lines finish a tool, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_SubagentToolUse_IsNotARunningTool(t *testing.T) {
	got := statusAfter(activity.NewClaudeCodeDecoder(),
		claudeAssistant("s1", `"toolu_task"`, claudeToolBlock("inner", "Bash"), claudeUsage(1, 0, 0)))

	if got.RunningToolID != "" {
		t.Errorf("tools run inside a sub-agent are not the agent's own, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_CostComesFromTheResultLine(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	if got := statusAfter(dec, claudeInit); got.HasCost {
		t.Fatalf("no cost before the result line, got %+v", got)
	}

	got := statusAfter(dec, `{"type":"result","subtype":"success","is_error":false,"result":"x","total_cost_usd":1.5}`)

	if !got.HasCost || !approxEqual(got.CostUSD, 1.5) {
		t.Errorf("want cost 1.5, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_ResultWithoutCost_HasNoCost(t *testing.T) {
	got := statusAfter(activity.NewClaudeCodeDecoder(),
		`{"type":"result","subtype":"error_max_turns","is_error":true,"errors":["x"]}`)

	if got.HasCost {
		t.Errorf("no total_cost_usd means no cost, got %+v", got)
	}
}

func TestClaudeCodeDecoder_Status_MalformedAndUnknownLines_KeepThePreviousStatus(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	want := statusAfter(dec, claudeInit,
		claudeAssistant("m1", "null", claudeToolBlock("a", "Read"), claudeUsage(10, 0, 0)))
	explicit := domain.ActivityStatus{Model: "claude-sonnet-4-5", ContextTokens: 10, HasContext: true,
		RunningTool: "Read", RunningToolID: "a"}
	if want != explicit {
		t.Fatalf("baseline must hold every reported fact: want %+v, got %+v", explicit, want)
	}

	for _, l := range []string{``, `not json`, `{"type":`, `{"type":"rate_limit_event"}`, `[]`} {
		if got := statusAfter(dec, l); got != want {
			t.Errorf("line %q changed the status: want %+v, got %+v", l, want, got)
		}
	}
}
