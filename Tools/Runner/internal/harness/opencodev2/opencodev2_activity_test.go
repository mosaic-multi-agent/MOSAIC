package opencodev2_test

// Tests that the adapter reports the live activity of Invoke and InvokeRaw to a
// domain.ActivityObserver while the process runs, announces the start and end
// of each invocation, never reports money cost, and leaves results and errors
// unchanged by the observer.

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/opencodev2"
	"mosaic-run/internal/testutil"
)

const (
	activityTextLine      = `{"type":"text","sessionID":"ses_h","part":{"type":"text","text":"Reading the file"}}`
	activityToolLine      = `{"type":"tool_use","sessionID":"ses_h","part":{"type":"tool","id":"c1","tool":"read","state":{"status":"completed","input":{"path":"a.txt"},"output":"hello"}}}`
	activitySubagentLine  = `{"type":"tool_use","sessionID":"ses_h","part":{"type":"tool","id":"c2","tool":"subagent","state":{"status":"completed","input":{"agent":"reviewer","description":"Review the change","prompt":"Please review"},"output":"started"}}}`
	activityErrorToolLine = `{"type":"tool_use","sessionID":"ses_h","part":{"type":"tool","id":"c3","tool":"write","state":{"status":"error","input":{"path":"b.txt"},"error":"Permission denied: write"}}}`
	activityFinishLine    = `{"type":"step_finish","sessionID":"ses_h","part":{"reason":"tool-calls","cost":0.25,"tokens":{"input":100,"output":20,"reasoning":5,"cache":{"read":50,"write":10}}}}`
)

func newActivityAdapter(t *testing.T, obs domain.ActivityObserver) *opencodev2.OpenCodeV2Adapter {
	t.Helper()
	return newAdapter(t, 30*time.Second).WithActivityObserver(obs)
}

func assertReportedActivity(t *testing.T, items []domain.ActivityItem) {
	t.Helper()
	if it, ok := testutil.FindItem(items, domain.ActivityToolCall, "read"); !ok || it.Summary == "" {
		t.Errorf("want a read tool call with a summary, got %+v", items)
	}
	if it, ok := testutil.FindItem(items, domain.ActivitySubagent, "reviewer"); !ok || it.Summary == "" {
		t.Errorf("want a reviewer subagent launch, got %+v", items)
	}
	if it, ok := testutil.FindItem(items, domain.ActivityToolCall, "write"); !ok || it.Summary == "" {
		t.Errorf("want the failed write call shown as a tool error, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityText, ""); !ok {
		t.Errorf("want assistant text, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityToolCall, "subagent"); ok {
		t.Errorf("a subagent launch must not also be reported as a tool call: %+v", items)
	}
}

func TestInvoke_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-pace")
	obs, gate := testutil.NewGatedObserver(t)

	_, _ = newActivityAdapter(t, obs).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "test-agent#1", Harness: "opencode-v2"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestInvokeRaw_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-pace")
	obs, gate := testutil.NewGatedObserver(t)

	_, _ = newActivityAdapter(t, obs).InvokeRaw(context.Background(), orchestratorRef(), []byte("consult"))

	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "orchestrator-agent", Harness: "opencode-v2"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestInvoke_ReportsTokensAndContextButNeverMoneyCost(t *testing.T) {
	setHelperEnv(t, "activity-pace")
	testutil.PresetOpenGate(t)
	obs := &testutil.Observer{}

	_, _ = newActivityAdapter(t, obs).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	_, items, _, _ := obs.Snapshot()
	var inTokens, outTokens int
	for _, it := range items {
		if it.Kind != domain.ActivityKindUsage {
			continue
		}
		inTokens += it.Usage.InputTokens
		outTokens += it.Usage.OutputTokens
		if it.Usage.HasCost || it.Usage.CostUSD != 0 {
			t.Errorf("usage item carries money cost: %+v", it.Usage)
		}
	}
	if inTokens != 101 || outTokens != 26 {
		t.Errorf("want 101 input / 26 output tokens over both steps, got %d / %d", inTokens, outTokens)
	}
	statuses := obs.Statuses()
	if len(statuses) == 0 {
		t.Fatal("want status updates")
	}
	for _, st := range statuses {
		if st.HasCost || st.CostUSD != 0 {
			t.Errorf("status carries money cost: %+v", st)
		}
	}
	if last := statuses[len(statuses)-1]; !last.HasContext || last.ContextTokens != 1 {
		t.Errorf("want the last step's context size 1 reported, got %+v", last)
	}
}

func TestInvoke_EachInvocationGetsItsOwnStartAndEnd(t *testing.T) {
	setHelperEnv(t, "activity-pace")
	testutil.PresetOpenGate(t)
	obs := &testutil.Observer{}
	adapter := newActivityAdapter(t, obs)

	_, _ = adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))
	_, _ = adapter.InvokeRaw(context.Background(), orchestratorRef(), []byte("consult"))

	starts, _, _, ends := obs.Snapshot()
	if len(starts) != 2 || ends != 2 {
		t.Errorf("want 2 starts and 2 ends, got %d and %d", len(starts), ends)
	}
}

func TestInvoke_LaunchFailure_StillAnnouncesStartAndEnd(t *testing.T) {
	obs := &testutil.Observer{}
	adapter := opencodev2.NewOpenCodeV2Adapter("/nonexistent/opencode-binary", 5*time.Second).WithActivityObserver(obs)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if !errors.Is(err, cliexec.ErrExecutableNotFound) {
		t.Fatalf("want ErrExecutableNotFound, got %v", err)
	}
	starts, _, events, ends := obs.Snapshot()
	if len(starts) != 1 {
		t.Errorf("want the invocation announced before the launch failed, got %+v", starts)
	}
	testutil.AssertLifecycle(t, events, ends)
}

func TestInvoke_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-pace", "success", "error-agent", "exit130"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref, req := ordinaryAgentRef(), minimalRequest("test-agent#1")
			baseResp, baseErr := newAdapter(t, 30*time.Second).Invoke(context.Background(), ref, req)

			resp, err := newActivityAdapter(t, &testutil.Observer{}).Invoke(context.Background(), ref, req)
			nilResp, nilErr := newActivityAdapter(t, nil).Invoke(context.Background(), ref, req)

			if baseErr == nil && scenario != "success" && scenario != "activity-pace" {
				t.Fatalf("scenario %s must fail", scenario)
			}
			if baseErr != nil && scenario != "error-agent" && scenario != "exit130" {
				t.Fatalf("scenario %s must succeed, got %v", scenario, baseErr)
			}
			if !reflect.DeepEqual(baseResp, resp) || !reflect.DeepEqual(baseResp, nilResp) {
				t.Errorf("response differs: base=%+v observed=%+v nil-observer=%+v", baseResp, resp, nilResp)
			}
			if !sameError(baseErr, err) || !sameError(baseErr, nilErr) {
				t.Errorf("error differs: base=%v observed=%v nil-observer=%v", baseErr, err, nilErr)
			}
		})
	}
}

func TestInvokeRaw_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"raw-text", "error-model"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			ref := orchestratorRef()
			baseOut, baseErr := newAdapter(t, 30*time.Second).InvokeRaw(context.Background(), ref, []byte("consult"))

			out, err := newActivityAdapter(t, &testutil.Observer{}).InvokeRaw(context.Background(), ref, []byte("consult"))

			if (scenario == "raw-text") != (baseErr == nil) {
				t.Fatalf("scenario %s: unexpected base result err=%v", scenario, baseErr)
			}
			if string(baseOut) != string(out) || !sameError(baseErr, err) {
				t.Errorf("result differs: base=%q/%v observed=%q/%v", baseOut, baseErr, out, err)
			}
		})
	}
}

func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error()
}
