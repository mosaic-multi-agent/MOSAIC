package activity_test

// Status tests for the OpenCode V2 decoder: context size, no money cost, and
// the facts the stream does not carry.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func TestOpenCodeV2Decoder_Status_DeniedToolTranscript_TracksContextAfterEveryLine(t *testing.T) {
	got := statusAfterEachLine(t, activity.NewOpenCodeV2Decoder(), "opencodev2_denied_tool.jsonl")

	assertStatuses(t, got, []domain.ActivityStatus{
		{},
		{}, // failed tool call: never a running tool
		{ContextTokens: 7120, HasContext: true},
		{ContextTokens: 7120, HasContext: true},
		{ContextTokens: 7120, HasContext: true},
		{ContextTokens: 7175, HasContext: true},
		{ContextTokens: 7175, HasContext: true},
		{ContextTokens: 7175, HasContext: true},
		{ContextTokens: 7206, HasContext: true},
	})
}

func TestOpenCodeV2Decoder_Status_ContextSumsInputAndBothCacheCounters(t *testing.T) {
	got := statusAfter(activity.NewOpenCodeV2Decoder(), v2StepFinish("stop", "0", "300", "9", "0", "20", "5"))

	if !got.HasContext || got.ContextTokens != 325 {
		t.Errorf("want context 325, got %+v", got)
	}
}

func TestOpenCodeV2Decoder_Status_ContextIsTheLatestStepNotTheSum(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	statusAfter(dec, v2StepFinish("tool-calls", "0", "300", "9", "0", "20", "5"))

	got := statusAfter(dec, v2StepFinish("stop", "0", "10", "9", "0", "0", "0"))

	if got.ContextTokens != 10 {
		t.Errorf("want the newest step's 10 tokens, got %d", got.ContextTokens)
	}
}

func TestOpenCodeV2Decoder_Status_ContextIsReportedEvenWhenNoUsageItemIsProduced(t *testing.T) {
	got := statusAfter(activity.NewOpenCodeV2Decoder(), v2StepFinish("tool-calls", "0", "0", "0", "0", "900", "0"))

	if !got.HasContext || got.ContextTokens != 900 {
		t.Errorf("want context 900, got %+v", got)
	}
}

func TestOpenCodeV2Decoder_Status_CostInTheStreamIsNeverAccumulated(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	statusAfter(dec, v2StepFinish("tool-calls", "0.004", "1", "1", "0", "0", "0"))

	got := statusAfter(dec, v2StepFinish("stop", "0.5", "1", "1", "0", "0", "0"))

	if got.HasCost || got.CostUSD != 0 {
		t.Errorf("money cost must never be shown for this harness, got %+v", got)
	}
	if !got.HasContext {
		t.Errorf("context must still be reported, got %+v", got)
	}
}

func TestOpenCodeV2Decoder_Status_ToolsAreNeverRunningTools(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	for _, status := range []string{"pending", "running", "completed", "error"} {
		got := statusAfter(dec, string(v2ToolLine("bash", status, `{"command":"ls"}`, ``)))
		if got.RunningTool != "" || got.RunningToolID != "" {
			t.Errorf("state %q: the V2 stream has no running tool, got %+v", status, got)
		}
	}
}

func TestOpenCodeV2Decoder_Status_ModelIsEmptyWhenTheStreamCarriesNone(t *testing.T) {
	got := statusAfter(activity.NewOpenCodeV2Decoder(),
		`{"type":"step_start","sessionID":"s","part":{"type":"step-start"}}`,
		v2StepFinish("stop", "0", "1", "1", "0", "0", "0"))

	if got.Model != "" {
		t.Errorf("the stream reports no model; got %q", got.Model)
	}
}

func TestOpenCodeV2Decoder_Status_MalformedAndUnknownLines_KeepThePreviousStatus(t *testing.T) {
	dec := activity.NewOpenCodeV2Decoder()
	want := statusAfter(dec, v2StepFinish("stop", "0", "10", "1", "0", "0", "0"))
	if !want.HasContext || want.ContextTokens != 10 {
		t.Fatalf("baseline must hold the reported context 10, got %+v", want)
	}

	for _, l := range []string{``, `nope`, `{"type":`, `{"type":"step_start","part":{}}`, `{"type":"background_wait"}`} {
		if got := statusAfter(dec, l); got != want {
			t.Errorf("line %q changed the status: want %+v, got %+v", l, want, got)
		}
	}
}
