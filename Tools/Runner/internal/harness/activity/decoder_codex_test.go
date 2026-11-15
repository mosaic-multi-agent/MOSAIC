package activity_test

// Tests for the Codex decoder: `codex exec --json` lines become tool-call,
// text and usage items; unknown or partial lines are ignored.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

// decodeOne decodes a single line with a fresh decoder.
func decodeOne(line string) []domain.ActivityItem {
	return activity.NewCodexDecoder().Decode([]byte(line))
}

func requireOneToolCall(t *testing.T, items []domain.ActivityItem, name string) domain.ActivityItem {
	t.Helper()
	if len(items) != 1 || items[0].Kind != domain.ActivityToolCall || items[0].Name != name {
		t.Fatalf("want one tool call named %q, got %+v", name, items)
	}
	return items[0]
}

func TestCodexDecoder_CommandExecution_StartIsOneToolCallWithTheCommand(t *testing.T) {
	items := decodeOne(`{"type":"item.started","item":{"id":"i1","type":"command_execution","command":"go test ./...","aggregated_output":"","exit_code":null,"status":"in_progress"}}`)

	it := requireOneToolCall(t, items, "command")

	if it.Summary != "go test ./..." {
		t.Errorf("Summary = %q, want the command text", it.Summary)
	}
}

func TestCodexDecoder_McpToolCall_NamesServerAndTool(t *testing.T) {
	items := decodeOne(`{"type":"item.started","item":{"id":"i2","type":"mcp_tool_call","server":"docs","tool":"search","arguments":{"query":"golang channels"},"status":"in_progress"}}`)

	it := requireOneToolCall(t, items, "docs.search")

	if !strings.Contains(it.Summary, "golang channels") {
		t.Errorf("Summary = %q, want the tool arguments", it.Summary)
	}
}

func TestCodexDecoder_WebSearch_CarriesTheQuery(t *testing.T) {
	items := decodeOne(`{"type":"item.started","item":{"id":"i3","type":"web_search","query":"go generics tutorial"}}`)

	it := requireOneToolCall(t, items, "web_search")

	if it.Summary != "go generics tutorial" {
		t.Errorf("Summary = %q, want the query", it.Summary)
	}
}

func TestCodexDecoder_FileChange_ListsTheChangedPaths(t *testing.T) {
	items := decodeOne(`{"type":"item.started","item":{"id":"i4","type":"file_change","changes":[{"path":"src/a.go","kind":"update"},{"path":"src/b.go","kind":"add"}],"status":"in_progress"}}`)

	it := requireOneToolCall(t, items, "file_change")

	if !strings.Contains(it.Summary, "src/a.go") || !strings.Contains(it.Summary, "src/b.go") {
		t.Errorf("Summary = %q, want both changed paths", it.Summary)
	}
}

func TestCodexDecoder_CollabToolCall_NamesTheCollabToolAndCarriesThePrompt(t *testing.T) {
	items := decodeOne(`{"type":"item.started","item":{"id":"i5","type":"collab_tool_call","tool":"spawn_agent","sender_thread_id":"t1","receiver_thread_ids":[],"prompt":"Review the parser change","agents_states":{},"status":"in_progress"}}`)

	it := requireOneToolCall(t, items, "spawn_agent")

	if !strings.Contains(it.Summary, "Review the parser change") {
		t.Errorf("Summary = %q, want the prompt", it.Summary)
	}
}

func TestCodexDecoder_ToolItem_IsReportedOncePerItemId(t *testing.T) {
	dec := activity.NewCodexDecoder()
	started := `{"type":"item.started","item":{"id":"i1","type":"command_execution","command":"ls","aggregated_output":"","exit_code":null,"status":"in_progress"}}`
	completed := `{"type":"item.completed","item":{"id":"i1","type":"command_execution","command":"ls","aggregated_output":"a","exit_code":0,"status":"completed"}}`

	first := dec.Decode([]byte(started))
	second := dec.Decode([]byte(completed))

	requireOneToolCall(t, first, "command")
	if len(second) != 0 {
		t.Errorf("completing an already reported item must add nothing, got %+v", second)
	}
}

func TestCodexDecoder_ToolItem_CompletedWithoutStart_IsReportedOnCompletion(t *testing.T) {
	dec := activity.NewCodexDecoder()
	completed := `{"type":"item.completed","item":{"id":"i9","type":"command_execution","command":"ls","aggregated_output":"a","exit_code":0,"status":"completed"}}`

	items := dec.Decode([]byte(completed))
	again := dec.Decode([]byte(completed))

	requireOneToolCall(t, items, "command")
	if len(again) != 0 {
		t.Errorf("a repeated completion must add nothing, got %+v", again)
	}
}

func TestCodexDecoder_AgentMessage_CompletedIsTextOthersAreNot(t *testing.T) {
	dec := activity.NewCodexDecoder()

	updated := dec.Decode([]byte(`{"type":"item.updated","item":{"id":"m1","type":"agent_message","text":"partial"}}`))
	completed := dec.Decode([]byte(`{"type":"item.completed","item":{"id":"m1","type":"agent_message","text":"Reading the file"}}`))

	if len(updated) != 0 {
		t.Errorf("an update is not a final message, got %+v", updated)
	}
	if len(completed) != 1 || completed[0].Kind != domain.ActivityText || !strings.Contains(completed[0].Summary, "Reading the file") {
		t.Errorf("want one text item with the message, got %+v", completed)
	}
}

func TestCodexDecoder_TurnCompleted_IsUsageWithoutCost(t *testing.T) {
	items := decodeOne(`{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":5}}`)

	if len(items) != 1 || items[0].Kind != domain.ActivityKindUsage {
		t.Fatalf("want one usage item, got %+v", items)
	}
	u := items[0].Usage
	if u.InputTokens != 100 || u.OutputTokens != 20 {
		t.Errorf("usage = %+v, want input 100 and output 20 (cached and reasoning tokens are subsets, not added)", u)
	}
	if u.HasCost || u.CostUSD != 0 {
		t.Errorf("Codex reports no cost, got %+v", u)
	}
}

func TestCodexDecoder_ToolSummary_IsSingleLineAndBounded(t *testing.T) {
	long := strings.Repeat("word ", 100)
	items := decodeOne(`{"type":"item.started","item":{"id":"i1","type":"command_execution","command":"echo one\necho two ` + long + `","aggregated_output":"","exit_code":null,"status":"in_progress"}}`)

	it := requireOneToolCall(t, items, "command")

	if strings.ContainsAny(it.Summary, "\r\n") {
		t.Errorf("Summary must be one line, got %q", it.Summary)
	}
	if n := len([]rune(it.Summary)); n > domain.ActivitySummaryMaxRunes {
		t.Errorf("Summary has %d runes, want at most %d", n, domain.ActivitySummaryMaxRunes)
	}
}

// Guard (not RED evidence): a decoder that decodes nothing satisfies this; it
// protects against decoding noise once the positive tests are green.
func TestCodexDecoder_UnknownPartialAndNonJSONLines_AreIgnored(t *testing.T) {
	lines := []string{
		"",
		"not json at all",
		`{"type":"item.started","item":`,
		`{"type":"thread.started","thread_id":"t1"}`,
		`{"type":"turn.started"}`,
		`{"type":"future.event","x":1}`,
		`{"type":"item.completed","item":{"id":"r1","type":"reasoning","text":"thinking"}}`,
		`{"type":"item.started","item":{"id":"z1","type":"brand_new_item"}}`,
		`{"type":"turn.completed"}`,
	}
	for _, line := range lines {
		dec := activity.NewCodexDecoder()

		items := dec.Decode([]byte(line))

		if len(items) != 0 {
			t.Errorf("line %q: want no items, got %+v", line, items)
		}
		if dec.Status() != (domain.ActivityStatus{}) {
			t.Errorf("line %q: want the status unchanged, got %+v", line, dec.Status())
		}
	}
}

func TestCodexDecoder_Status_Transcript_TracksRunningToolAfterEveryLine(t *testing.T) {
	got := statusAfterEachLine(t, activity.NewCodexDecoder(), "codex_status_transcript.jsonl")

	assertStatuses(t, got, []domain.ActivityStatus{
		{},
		{},
		{RunningTool: "command", RunningToolID: "item_1"},
		{RunningTool: "web_search", RunningToolID: "item_2"}, // latest open tool wins
		{RunningTool: "command", RunningToolID: "item_1"},    // the older one is reported again
		{},
		{},
		{}, // usage: no model, no context, no cost
	})
}

// Guard (not RED evidence): protects against setting context or cost from usage.
func TestCodexDecoder_Status_NeverReportsModelContextOrCost(t *testing.T) {
	dec := activity.NewCodexDecoder()

	got := statusAfter(dec,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"m1","type":"agent_message","text":"hi"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":5000,"cached_input_tokens":4000,"output_tokens":300,"reasoning_output_tokens":100}}`,
	)

	if got.Model != "" || got.HasContext || got.ContextTokens != 0 || got.HasCost || got.CostUSD != 0 {
		t.Errorf("model, context and cost must stay unset, got %+v", got)
	}
}
