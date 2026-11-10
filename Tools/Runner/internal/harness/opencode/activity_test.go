package opencode_test

// Tests that OpenCodeAdapter reports the live activity of Invoke and
// InvokeRaw to a domain.ActivityObserver while the process runs, announces the
// start and end of each invocation, and leaves results and errors unchanged.

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/opencode"
	"mosaic-run/internal/testutil"
)

const (
	ocStepStartLine = `{"type":"step_start","sessionID":"ses_1","part":{"type":"step-start"}}`
	ocTextLine      = `{"type":"text","sessionID":"ses_1","part":{"type":"text","text":"Reading the file"}}`
	ocToolLine      = `{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","callID":"c1","tool":"read","state":{"status":"completed","input":{"filePath":"a.txt"},"output":"hello"}}}`
	ocSubagentLine  = `{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","callID":"c2","tool":"task","state":{"status":"completed","input":{"subagent_type":"reviewer","description":"Review the change","prompt":"Please review"},"output":""}}}`
	ocFinishLine    = `{"type":"step_finish","sessionID":"ses_1","part":{"type":"step-finish","reason":"stop"}}`
)

func init() {
	helperScenarios["activity-paced"] = testutil.PacedScenario(
		[]string{ocStepStartLine, ocTextLine, ocToolLine, ocSubagentLine},
		[]string{ocFinishLine},
	)
}

func newActivityAdapter(t *testing.T, obs domain.ActivityObserver) *opencode.OpenCodeAdapter {
	t.Helper()
	return opencode.NewOpenCodeAdapter(helperExe(t), 30*time.Second).WithActivityObserver(obs)
}

func assertReportedActivity(t *testing.T, items []domain.ActivityItem) {
	t.Helper()
	if it, ok := testutil.FindItem(items, domain.ActivityToolCall, "read"); !ok || it.Summary == "" {
		t.Errorf("want a read tool call with a summary, got %+v", items)
	}
	if it, ok := testutil.FindItem(items, domain.ActivitySubagent, "reviewer"); !ok || it.Summary == "" {
		t.Errorf("want a reviewer subagent spawn, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityText, ""); !ok {
		t.Errorf("want assistant text, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityToolCall, "task"); ok {
		t.Errorf("a subagent spawn must not also be reported as a tool call: %+v", items)
	}
}

func TestOpenCodeAdapter_Invoke_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, _ = newActivityAdapter(t, obs).Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))

	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "test-agent#1", Harness: "opencode"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestOpenCodeAdapter_InvokeRaw_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, _ = newActivityAdapter(t, obs).InvokeRaw(context.Background(), orchestratorAgentRef(), []byte("consult"))

	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "orchestrator-agent", Harness: "opencode"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestOpenCodeAdapter_Invoke_EachInvocationGetsItsOwnStartAndEnd(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	testutil.PresetOpenGate(t)
	obs := &testutil.Observer{}
	adapter := newActivityAdapter(t, obs)

	_, _ = adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))
	_, _ = adapter.InvokeRaw(context.Background(), orchestratorAgentRef(), []byte("consult"))

	starts, _, _, ends := obs.Snapshot()
	if len(starts) != 2 || ends != 2 {
		t.Errorf("want 2 starts and 2 ends, got %d and %d", len(starts), ends)
	}
}

func TestOpenCodeAdapter_Invoke_RejectedBeforeSpawn_StillAnnouncesStartAndEnd(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs := &testutil.Observer{}
	adapter := opencode.NewOpenCodeAdapter("/nonexistent/opencode-binary", 5*time.Second).WithActivityObserver(obs)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))

	if !errors.Is(err, cliexec.ErrExecutableNotFound) {
		t.Fatalf("want ErrExecutableNotFound, got %v", err)
	}
	_, _, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
}

func TestOpenCodeAdapter_Invoke_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-paced", "exit1", "opencode-success"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref, req := ordinaryAgentRef(), minimalClaudeRequest("test-agent#1")
			baseResp, baseErr := opencode.NewOpenCodeAdapter(helperExe(t), 30*time.Second).Invoke(context.Background(), ref, req)

			resp, err := newActivityAdapter(t, &testutil.Observer{}).Invoke(context.Background(), ref, req)
			nilResp, nilErr := newActivityAdapter(t, nil).Invoke(context.Background(), ref, req)

			if !reflect.DeepEqual(baseResp, resp) || !reflect.DeepEqual(baseResp, nilResp) {
				t.Errorf("response differs: base=%+v observed=%+v nil-observer=%+v", baseResp, resp, nilResp)
			}
			if !sameError(baseErr, err) || !sameError(baseErr, nilErr) {
				t.Errorf("error differs: base=%v observed=%v nil-observer=%v", baseErr, err, nilErr)
			}
		})
	}
}

func TestOpenCodeAdapter_InvokeRaw_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-paced", "exit1"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref := orchestratorAgentRef()
			baseOut, baseErr := opencode.NewOpenCodeAdapter(helperExe(t), 30*time.Second).InvokeRaw(context.Background(), ref, []byte("consult"))

			out, err := newActivityAdapter(t, &testutil.Observer{}).InvokeRaw(context.Background(), ref, []byte("consult"))
			nilOut, nilErr := newActivityAdapter(t, nil).InvokeRaw(context.Background(), ref, []byte("consult"))

			if string(baseOut) != string(out) || string(baseOut) != string(nilOut) {
				t.Errorf("output differs: base=%q observed=%q nil-observer=%q", baseOut, out, nilOut)
			}
			if !sameError(baseErr, err) || !sameError(baseErr, nilErr) {
				t.Errorf("error differs: base=%v observed=%v nil-observer=%v", baseErr, err, nilErr)
			}
		})
	}
}

// sameError compares two errors by class and text.
func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error() &&
		errors.Is(a, cliexec.ErrNonZeroExit) == errors.Is(b, cliexec.ErrNonZeroExit)
}
