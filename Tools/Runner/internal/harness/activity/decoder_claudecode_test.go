package activity_test

// Decoder tests for the Claude Code --output-format stream-json lines.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func claudeToolLine(name, inputJSON string) []byte {
	return []byte(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"tool_use","id":"tu","name":"` +
		name + `","input":` + inputJSON + `}]}}`)
}

func TestClaudeCodeDecoder_Transcript_ReportsTextToolCallAndSubagentInOrder(t *testing.T) {
	items := decodeTranscript(t, activity.NewClaudeCodeDecoder(), "claude_transcript.jsonl")

	assertItems(t, withoutUsage(items), []wantItem{
		{domain.ActivityText, "", "I will read the file first."},
		{domain.ActivityToolCall, "Read", "src/main.go"},
		{domain.ActivitySubagent, "reviewer", "Review the change"},
		{domain.ActivityText, "", "Done."},
	})
}

func TestClaudeCodeDecoder_Transcript_UsageIsReportedOncePerMessage(t *testing.T) {
	items := decodeTranscript(t, activity.NewClaudeCodeDecoder(), "claude_transcript.jsonl")

	sum := sumUsage(items)

	// msg_1 spans two lines carrying the same usage; it counts once.
	if sum.InputTokens != 320 || sum.OutputTokens != 55 {
		t.Errorf("want 320 input / 55 output tokens summed, got %d / %d", sum.InputTokens, sum.OutputTokens)
	}
}

func TestClaudeCodeDecoder_SubagentToolNames_YieldSubagentNotToolCall(t *testing.T) {
	for _, tool := range []string{"Task", "Agent"} {
		t.Run(tool, func(t *testing.T) {
			dec := activity.NewClaudeCodeDecoder()

			items := dec.Decode(claudeToolLine(tool, `{"subagent_type":"planner","description":"Plan it","prompt":"p"}`))

			assertItems(t, items, []wantItem{{domain.ActivitySubagent, "planner", "Plan it"}})
		})
	}
}

func TestClaudeCodeDecoder_MultilineText_OneItemPerNonEmptyLine(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	line := []byte(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"first\n\nsecond\nthird"}]}}`)

	items := dec.Decode(line)

	assertItems(t, items, []wantItem{
		{domain.ActivityText, "", "first"},
		{domain.ActivityText, "", "second"},
		{domain.ActivityText, "", "third"},
	})
}

func TestClaudeCodeDecoder_MessageWithTextThenToolUse_KeepsBlockOrder(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	line := []byte(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[` +
		`{"type":"text","text":"checking"},{"type":"tool_use","id":"a","name":"Grep","input":{"pattern":"foo"}}]}}`)

	items := dec.Decode(line)

	assertItems(t, withoutUsage(items), []wantItem{
		{domain.ActivityText, "", "checking"},
		{domain.ActivityToolCall, "Grep", "foo"},
	})
}

func TestClaudeCodeDecoder_LinesWithoutActivity_YieldNothing(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()
	lines := []string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"out"}]}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"final","total_cost_usd":0.5}`,
	}
	for _, l := range lines {
		if items := dec.Decode([]byte(l)); len(items) != 0 {
			t.Errorf("line %q: want no items, got %+v", l, items)
		}
	}
}

func TestClaudeCodeDecoder_SubagentWithoutType_StillYieldsSingleSubagentItem(t *testing.T) {
	dec := activity.NewClaudeCodeDecoder()

	items := dec.Decode(claudeToolLine("Task", `{"description":"Plan it","prompt":"p"}`))

	if len(items) != 1 || items[0].Kind != domain.ActivitySubagent || items[0].Summary == "" {
		t.Errorf("want exactly one subagent item with a summary and no tool call, got %+v", items)
	}
}
