package activity_test

// Status tests for the OpenCode decoder.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

func openCodeStepFinish(cost string, in, cacheRead, cacheWrite string) string {
	return `{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"stop","cost":` + cost +
		`,"tokens":{"input":` + in + `,"output":9,"reasoning":0,"cache":{"read":` + cacheRead + `,"write":` + cacheWrite + `}}}}`
}

func TestOpenCodeDecoder_Status_Transcript_TracksEachFactAfterEveryLine(t *testing.T) {
	got := statusAfterEachLine(t, activity.NewOpenCodeDecoder(), "opencode_status_transcript.jsonl")

	assertStatuses(t, got, []domain.ActivityStatus{
		{},
		{}, // completed tool: never a running tool
		{ContextTokens: 325, HasContext: true, CostUSD: 0.004, HasCost: true},
		{ContextTokens: 325, HasContext: true, CostUSD: 0.004, HasCost: true},
		{ContextTokens: 325, HasContext: true, CostUSD: 0.004, HasCost: true},
		{ContextTokens: 480, HasContext: true, CostUSD: 0.006, HasCost: true}, // latest step, summed cost
	})
}

func TestOpenCodeDecoder_Status_ContextSumsInputAndBothCacheCounters(t *testing.T) {
	got := statusAfter(activity.NewOpenCodeDecoder(), openCodeStepFinish("0.01", "300", "20", "5"))

	if !got.HasContext || got.ContextTokens != 325 {
		t.Errorf("want context 325, got %+v", got)
	}
}

func TestOpenCodeDecoder_Status_ContextIsTheLatestStepNotTheSum(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	statusAfter(dec, openCodeStepFinish("0.01", "300", "20", "5"))

	got := statusAfter(dec, openCodeStepFinish("0.01", "10", "0", "0"))

	if got.ContextTokens != 10 {
		t.Errorf("want the newest step's 10 tokens, got %d", got.ContextTokens)
	}
}

func TestOpenCodeDecoder_Status_CostIsTheSumOfStepFinishCosts(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	statusAfter(dec, openCodeStepFinish("0.004", "1", "0", "0"))
	statusAfter(dec, openCodeStepFinish("0.002", "1", "0", "0"))

	got := statusAfter(dec, openCodeStepFinish("0.5", "1", "0", "0"))

	if !got.HasCost || !approxEqual(got.CostUSD, 0.506) {
		t.Errorf("want summed cost 0.506, got %+v", got)
	}
}

func TestOpenCodeDecoder_Status_StepFinishWithoutCost_HasNoCostUntilOneIsReported(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()

	got := statusAfter(dec, `{"type":"step_finish","part":{"type":"step-finish","tokens":{"input":5,"output":1,"cache":{"read":0,"write":0}}}}`)
	if got.HasCost {
		t.Fatalf("no cost reported yet, got %+v", got)
	}
	if !got.HasContext || got.ContextTokens != 5 {
		t.Errorf("context must still be reported, got %+v", got)
	}
}

func TestOpenCodeDecoder_Status_ToolsAreReportedOnlyOnCompletion_NeverRunning(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	for _, status := range []string{"pending", "running", "completed"} {
		got := statusAfter(dec, `{"type":"tool_use","part":{"type":"tool","callID":"c1","tool":"bash","state":{"status":"`+
			status+`","input":{"command":"ls"}}}}`)
		if got.RunningTool != "" || got.RunningToolID != "" {
			t.Errorf("state %q: OpenCode never has a running tool, got %+v", status, got)
		}
	}
}

func TestOpenCodeDecoder_Status_ModelIsEmptyWhenTheStreamCarriesNone(t *testing.T) {
	got := statusAfter(activity.NewOpenCodeDecoder(),
		`{"type":"step_start","sessionID":"s","part":{"type":"step-start"}}`,
		openCodeStepFinish("0.01", "1", "0", "0"))

	if got.Model != "" {
		t.Errorf("the stream reports no model; got %q", got.Model)
	}
}

func TestOpenCodeDecoder_Status_MalformedAndUnknownLines_KeepThePreviousStatus(t *testing.T) {
	dec := activity.NewOpenCodeDecoder()
	want := statusAfter(dec, openCodeStepFinish("0.01", "10", "0", "0"))
	if !want.HasContext || want.ContextTokens != 10 || !want.HasCost || !approxEqual(want.CostUSD, 0.01) {
		t.Fatalf("baseline must hold every reported fact (context 10, cost 0.01), got %+v", want)
	}

	for _, l := range []string{``, `nope`, `{"type":`, `{"type":"step_start","part":{}}`} {
		if got := statusAfter(dec, l); got != want {
			t.Errorf("line %q changed the status: want %+v, got %+v", l, want, got)
		}
	}
}
