package activity_test

// Decoder tests for the OpenCode V2 run --format json lines: text, tool calls
// (completed and failed), subagent launches, token usage, and the rule that no
// money cost is ever reported.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func v2ToolLine(tool, status, inputJSON, extra string) []byte {
	return []byte(`{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","id":"call_1","tool":"` + tool +
		`","state":{"status":"` + status + `","input":` + inputJSON + extra + `}}}`)
}

func v2StepFinish(reason, cost, in, out, reasoning, cacheRead, cacheWrite string) string {
	return `{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"` + reason +
		`","cost":` + cost + `,"tokens":{"input":` + in + `,"output":` + out + `,"reasoning":` + reasoning +
		`,"cache":{"read":` + cacheRead + `,"write":` + cacheWrite + `}}}}`
}

func TestOpenCodeV2Decoder_ParallelTranscript_ReportsSubagentLaunchesAndTextInOrder(t *testing.T) {
	items := decodeTranscript(t, activity.NewOpenCodeV2Decoder(), "opencodev2_parallel.jsonl")

	assertItems(t, withoutUsage(items), []wantItem{
		{domain.ActivitySubagent, "spike-child", "ALPHA spike task"},
		{domain.ActivitySubagent, "spike-child", "BETA spike task"},
		{domain.ActivitySubagent, "spike-child", "Await ALPHA result"},
		{domain.ActivitySubagent, "spike-child", "Await BETA result"},
		{domain.ActivityText, "", "Both subagents are running"},
		{domain.ActivitySubagent, "spike-child", "Await BETA completion"},
		{domain.ActivitySubagent, "spike-child", "Wait for BETA result"},
		{domain.ActivityText, "", "ALPHA:"},
		{domain.ActivityText, "", "BETA is still running."},
		{domain.ActivityText, "", `"child":"ALPHA"`},
		{domain.ActivityText, "", `"child":"BETA"`},
		{domain.ActivityText, "", "ALL-DONE"},
	})
}

func TestOpenCodeV2Decoder_ParallelTranscript_UsageSumsInputAndOutputPlusReasoning(t *testing.T) {
	items := decodeTranscript(t, activity.NewOpenCodeV2Decoder(), "opencodev2_parallel.jsonl")

	sum := sumUsage(items)

	if sum.InputTokens != 17102 || sum.OutputTokens != 611 {
		t.Errorf("want 17102 input / 611 output (output + reasoning) tokens, got %d / %d", sum.InputTokens, sum.OutputTokens)
	}
}

func TestOpenCodeV2Decoder_DeniedToolTranscript_ShowsToolErrorThenLaterCallsAndText(t *testing.T) {
	items := decodeTranscript(t, activity.NewOpenCodeV2Decoder(), "opencodev2_denied_tool.jsonl")

	assertItems(t, withoutUsage(items), []wantItem{
		{domain.ActivityToolCall, "shell", "error: Permission denied: shell"},
		{domain.ActivityToolCall, "shell", "echo AFTER"},
		{domain.ActivityText, "", "permission.rejected"},
		{domain.ActivityText, "", "AFTER"},
	})
}

func TestOpenCodeV2Decoder_PatchTranscript_SummaryNamesTheFileNotThePatchBody(t *testing.T) {
	items := decodeTranscript(t, activity.NewOpenCodeV2Decoder(), "opencodev2_patch_read.jsonl")

	var calls []domain.ActivityItem
	for _, it := range items {
		if it.Kind == domain.ActivityToolCall {
			calls = append(calls, it)
		}
	}
	if len(calls) != 2 || calls[0].Name != "patch" || calls[1].Name != "read" {
		t.Fatalf("want a patch call then a read call, got %+v", calls)
	}
	if !strings.Contains(calls[0].Summary, "note.txt") {
		t.Errorf("want the patch summary to name note.txt, got %q", calls[0].Summary)
	}
	for _, body := range []string{"Begin Patch", "hello", "Add File"} {
		if strings.Contains(calls[0].Summary, body) {
			t.Errorf("patch summary must not show the patch body (%q), got %q", body, calls[0].Summary)
		}
	}
	if !strings.Contains(calls[1].Summary, "README.md") {
		t.Errorf("want the read summary to name README.md, got %q", calls[1].Summary)
	}
}

func TestOpenCodeV2Decoder_PatchWithSeveralFiles_SummaryNamesEachFile(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	input := `{"patchText":"*** Begin Patch\n*** Add File: a.txt\n+one\n*** Update File: dir/b.go\n@@\n-x\n+y\n*** End Patch\n"}`

	items := dec.Decode(v2ToolLine("patch", "completed", input, `,"output":"ok"`))

	if len(items) != 1 || items[0].Kind != domain.ActivityToolCall || items[0].Name != "patch" {
		t.Fatalf("want one patch tool call, got %+v", items)
	}
	for _, want := range []string{"a.txt", "dir/b.go"} {
		if !strings.Contains(items[0].Summary, want) {
			t.Errorf("want the summary to name %q, got %q", want, items[0].Summary)
		}
	}
	assertSingleLine(t, items[0].Summary)
}

func TestOpenCodeV2Decoder_SubagentLaunch_YieldsSubagentItemWithAgentAndDescription(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode(v2ToolLine("subagent", "completed",
		`{"agent":"planner","description":"Plan it","prompt":"long prompt text"}`, `,"output":"started"`))

	assertItems(t, items, []wantItem{{domain.ActivitySubagent, "planner", "Plan it"}})
}

func TestOpenCodeV2Decoder_SubagentWithoutDescription_FallsBackToPrompt(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode(v2ToolLine("subagent", "completed",
		`{"agent":"planner","description":"","prompt":"Draft the outline"}`, ``))

	assertItems(t, items, []wantItem{{domain.ActivitySubagent, "planner", "Draft the outline"}})
}

func TestOpenCodeV2Decoder_SubagentRecallWithSessionID_StillOneSubagentItem(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode(v2ToolLine("subagent", "completed",
		`{"agent":"planner","description":"Await result","prompt":"","sessionID":"ses_abc"}`, ``))

	assertItems(t, items, []wantItem{{domain.ActivitySubagent, "planner", "Await result"}})
}

func TestOpenCodeV2Decoder_TaskTool_IsAnOrdinaryToolCallNotASubagent(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode(v2ToolLine("task", "completed", `{"description":"Plan it","prompt":"p"}`, ``))

	if len(items) != 1 || items[0].Kind != domain.ActivityToolCall || items[0].Name != "task" {
		t.Errorf("want an ordinary task tool call (only subagent is a launch on V2), got %+v", items)
	}
}

func TestOpenCodeV2Decoder_ToolError_IsShownAsAnErrorToolCall(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode(v2ToolLine("write", "error", `{"path":"a.txt","content":"x"}`, `,"error":"Permission denied: write"`))

	assertItems(t, items, []wantItem{{domain.ActivityToolCall, "write", "error: Permission denied: write"}})
}

func TestOpenCodeV2Decoder_ToolError_MessageIsShortenedToOneLine(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	longErr := "first line of the failure\\nsecond line " + strings.Repeat("x", 300)

	items := dec.Decode(v2ToolLine("bash", "error", `{"command":"ls"}`, `,"error":"`+longErr+`"`))

	if len(items) != 1 || items[0].Kind != domain.ActivityToolCall || items[0].Name != "bash" {
		t.Fatalf("want one bash tool call, got %+v", items)
	}
	if !strings.HasPrefix(items[0].Summary, "error: first line of the failure") {
		t.Errorf("want the summary to start with the error text, got %q", items[0].Summary)
	}
	assertSingleLine(t, items[0].Summary)
}

func TestOpenCodeV2Decoder_SubagentToolError_IsAToolErrorNotASubagentLaunch(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode(v2ToolLine("subagent", "error", `{"agent":"planner","description":"Plan it"}`, `,"error":"Agent not found"`))

	assertItems(t, items, []wantItem{{domain.ActivityToolCall, "subagent", "error: Agent not found"}})
}

func TestOpenCodeV2Decoder_ToolNotFinished_YieldsNothing(t *testing.T) {
	for _, status := range []string{"pending", "running"} {
		t.Run(status, func(t *testing.T) {
			dec := activity.NewOpenCodeV2Decoder()

			if items := dec.Decode(v2ToolLine("bash", status, `{"command":"go test"}`, ``)); len(items) != 0 {
				t.Errorf("a tool in state %q is not reported yet, got %+v", status, items)
			}
		})
	}
}

func TestOpenCodeV2Decoder_MultilineText_OneItemPerNonEmptyLine(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	line := []byte(`{"type":"text","sessionID":"s","part":{"type":"text","text":"one\ntwo\n\nthree"}}`)

	items := dec.Decode(line)

	assertItems(t, items, []wantItem{
		{domain.ActivityText, "", "one"},
		{domain.ActivityText, "", "two"},
		{domain.ActivityText, "", "three"},
	})
}

func TestOpenCodeV2Decoder_StepFinishUsage_InputIsPlainInputAndOutputIncludesReasoning(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode([]byte(v2StepFinish("stop", "0", "300", "40", "15", "20", "5")))

	if len(items) != 1 || items[0].Kind != domain.ActivityKindUsage {
		t.Fatalf("want one usage item, got %+v", items)
	}
	if u := items[0].Usage; u.InputTokens != 300 || u.OutputTokens != 55 {
		t.Errorf("want 300 input / 55 output (40 + 15 reasoning), got %d / %d", u.InputTokens, u.OutputTokens)
	}
}

func TestOpenCodeV2Decoder_StepFinishWithNoTokens_YieldsNoUsageItem(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode([]byte(v2StepFinish("tool-calls", "0", "0", "0", "0", "900", "0")))

	if len(items) != 0 {
		t.Errorf("want no usage item when input and output are both 0, got %+v", items)
	}
}

func TestOpenCodeV2Decoder_StepFinishCost_IsNeverReportedInItems(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()

	items := dec.Decode([]byte(v2StepFinish("stop", "0.5", "10", "5", "0", "0", "0")))

	sum := sumUsage(items)
	if sum.InputTokens != 10 || sum.OutputTokens != 5 {
		t.Fatalf("want 10/5 tokens reported, got %+v", sum)
	}
	if sum.HasCost || sum.CostUSD != 0 {
		t.Errorf("a V2 usage item must never carry money cost, got %+v", sum)
	}
}

func TestOpenCodeV2Decoder_LinesWithoutActivity_YieldNothing(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	lines := []string{
		`{"type":"step_start","sessionID":"s","part":{"type":"step-start"}}`,
		`{"type":"background_wait","sessionID":"s"}`,
		`{"type":"retry","sessionID":"s"}`,
		`{"type":"error","error":{"type":"unknown","message":"x"}}`,
		`{"type":"surprise","part":{"text":"ignored"}}`,
	}
	for _, l := range lines {
		if items := dec.Decode([]byte(l)); len(items) != 0 {
			t.Errorf("line %q: want no items, got %+v", l, items)
		}
	}
}

func TestOpenCodeV2Decoder_MalformedLines_AreIgnored(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	for _, l := range []string{``, `nope`, `{"type":`, `[1,2]`, `{"type":"text","part":"not-an-object"}`} {
		if items := dec.Decode([]byte(l)); len(items) != 0 {
			t.Errorf("line %q: want no items, got %+v", l, items)
		}
	}
}
