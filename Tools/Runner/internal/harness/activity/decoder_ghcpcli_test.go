package activity_test

// Decoder tests for the GHCP CLI --output-format json lines.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func TestGHCPCLIDecoder_Transcript_ReportsTextToolCallAndSubagentInOrder(t *testing.T) {
	items := decodeTranscript(t, activity.NewGHCPCLIDecoder(), "ghcpcli_transcript.jsonl")

	assertItems(t, withoutUsage(items), []wantItem{
		{domain.ActivityText, "", "Looking at the file."},
		{domain.ActivityToolCall, "view", "src/main.go"},
		{domain.ActivitySubagent, "reviewer", "Review the change"},
		{domain.ActivityText, "", "All done."},
	})
}

func TestGHCPCLIDecoder_Transcript_AssistantUsageReportsTokens(t *testing.T) {
	items := decodeTranscript(t, activity.NewGHCPCLIDecoder(), "ghcpcli_transcript.jsonl")

	sum := sumUsage(items)

	if sum.InputTokens != 150 || sum.OutputTokens != 25 {
		t.Errorf("want 150 input / 25 output tokens, got %d / %d", sum.InputTokens, sum.OutputTokens)
	}
}

func TestGHCPCLIDecoder_MultilineMessage_OneItemPerNonEmptyLine(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	line := []byte(`{"type":"assistant.message","data":{"messageId":"m","content":"a\nb\n\nc"}}`)

	items := dec.Decode(line)

	assertItems(t, items, []wantItem{
		{domain.ActivityText, "", "a"},
		{domain.ActivityText, "", "b"},
		{domain.ActivityText, "", "c"},
	})
}

func TestGHCPCLIDecoder_MessageWithoutContent_YieldsNothing(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	line := []byte(`{"type":"assistant.message","data":{"messageId":"m","content":"","toolRequests":[{"toolCallId":"t"}]}}`)

	if items := dec.Decode(line); len(items) != 0 {
		t.Errorf("want no items for an empty message, got %+v", items)
	}
}

func TestGHCPCLIDecoder_LinesWithoutActivity_YieldNothing(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	lines := []string{
		`{"type":"tool.execution_complete","data":{"toolCallId":"t","success":true}}`,
		`{"type":"subagent.completed","data":{"toolCallId":"t2","agentName":"reviewer"}}`,
		`{"type":"session.error","data":{"errorType":"quota","message":"x"}}`,
		`{"type":"result","exitCode":0}`,
	}
	for _, l := range lines {
		if items := dec.Decode([]byte(l)); len(items) != 0 {
			t.Errorf("line %q: want no items, got %+v", l, items)
		}
	}
}

func TestGHCPCLIDecoder_SubagentStartedWithoutName_StillYieldsSingleSubagentItem(t *testing.T) {
	dec := activity.NewGHCPCLIDecoder()
	line := `{"type":"subagent.started","data":{"toolCallId":"t2","agentDescription":"Review the change"}}`

	items := dec.Decode([]byte(line))

	if len(items) != 1 || items[0].Kind != domain.ActivitySubagent || items[0].Summary == "" {
		t.Errorf("want exactly one subagent item with a summary, got %+v", items)
	}
}
