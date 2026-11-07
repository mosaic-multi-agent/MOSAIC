package activity_test

// Decoder tests for the OpenCode run --format json lines.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func openCodeToolLine(tool, inputJSON string) []byte {
	return []byte(`{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","callID":"c","tool":"` + tool +
		`","state":{"status":"completed","input":` + inputJSON + `,"output":"o"}}}`)
}

func TestOpenCodeDecoder_Transcript_ReportsTextToolCallAndSubagentInOrder(t *testing.T) {
	items := decodeTranscript(t, activity.NewOpenCodeDecoder(), "opencode_transcript.jsonl")

	assertItems(t, withoutUsage(items), []wantItem{
		{domain.ActivityText, "", "Running the tests."},
		{domain.ActivityToolCall, "bash", "go test"},
		{domain.ActivitySubagent, "reviewer", "Review the change"},
		{domain.ActivityText, "", "All done."},
	})
}

func TestOpenCodeDecoder_Transcript_StepFinishUsageIsSummedWithCost(t *testing.T) {
	items := decodeTranscript(t, activity.NewOpenCodeDecoder(), "opencode_transcript.jsonl")

	sum := sumUsage(items)

	if sum.InputTokens != 380 || sum.OutputTokens != 60 {
		t.Errorf("want 380 input / 60 output tokens, got %d / %d", sum.InputTokens, sum.OutputTokens)
	}
	if !sum.HasCost || !approxEqual(sum.CostUSD, 0.006) {
		t.Errorf("want cost 0.006 reported, got HasCost=%v cost=%v", sum.HasCost, sum.CostUSD)
	}
}

func TestOpenCodeDecoder_StepFinishWithoutCost_ReportsNoCost(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	line := []byte(`{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"stop","tokens":{"input":10,"output":5}}}`)

	items := dec.Decode(line)

	sum := sumUsage(items)
	if sum.InputTokens != 10 || sum.OutputTokens != 5 || sum.HasCost {
		t.Errorf("want 10/5 tokens without cost, got %+v", sum)
	}
}

func TestOpenCodeDecoder_TaskTool_YieldsSubagentNotToolCall(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()

	items := dec.Decode(openCodeToolLine("task", `{"subagent_type":"planner","description":"Plan it","prompt":"p"}`))

	assertItems(t, items, []wantItem{{domain.ActivitySubagent, "planner", "Plan it"}})
}

func TestOpenCodeDecoder_MultilineText_OneItemPerNonEmptyLine(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	line := []byte(`{"type":"text","sessionID":"s","part":{"type":"text","text":"one\ntwo\n\nthree"}}`)

	items := dec.Decode(line)

	assertItems(t, items, []wantItem{
		{domain.ActivityText, "", "one"},
		{domain.ActivityText, "", "two"},
		{domain.ActivityText, "", "three"},
	})
}

func TestOpenCodeDecoder_LinesWithoutActivity_YieldNothing(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	lines := []string{
		`{"type":"step_start","sessionID":"s","part":{"type":"step-start"}}`,
		`{"type":"error","error":{"name":"Boom","data":{"message":"x"}}}`,
	}
	for _, l := range lines {
		if items := dec.Decode([]byte(l)); len(items) != 0 {
			t.Errorf("line %q: want no items, got %+v", l, items)
		}
	}
}

func TestOpenCodeDecoder_SubagentWithoutType_StillYieldsSingleSubagentItem(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()

	items := dec.Decode(openCodeToolLine("task", `{"description":"Plan it","prompt":"p"}`))

	if len(items) != 1 || items[0].Kind != domain.ActivitySubagent || items[0].Summary == "" {
		t.Errorf("want exactly one subagent item with a summary and no tool call, got %+v", items)
	}
}

func TestOpenCodeDecoder_ToolNotCompleted_YieldsNothing(t *testing.T) {
	for _, status := range []string{"pending", "running", "error"} {
		t.Run(status, func(t *testing.T) {
			dec := activity.NewOpenCodeDecoder()
			line := `{"type":"tool_use","sessionID":"s","part":{"type":"tool","callID":"c","tool":"bash","state":{"status":"` +
				status + `","input":{"command":"go test"}}}}`

			if items := dec.Decode([]byte(line)); len(items) != 0 {
				t.Errorf("a tool in state %q is not reported yet, got %+v", status, items)
			}
		})
	}
}
