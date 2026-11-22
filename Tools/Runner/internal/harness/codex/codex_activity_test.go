package codex_test

// Tests that the Codex adapter reports the live activity of Invoke and
// InvokeRaw to a domain.ActivityObserver while the process runs, announces the
// start and end of each invocation, and leaves results and errors unchanged.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/codex"
	"mosaic-run/internal/testutil"
)

var (
	activityFirstLines = []string{
		threadStartedLine,
		turnStartedLine,
		`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"ls -la","aggregated_output":"","exit_code":null,"status":"in_progress"}}`,
		`{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Reading the directory"}}`,
	}
	activityRestLines = []string{
		`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls -la","aggregated_output":"a","exit_code":0,"status":"completed"}}`,
		`{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"{\"agent_instance_id\":\"test-agent#1\",\"status_code\":\"SUCCESS\",\"status_message\":\"ok\"}"}}`,
		usageLine,
	}
)

func newObservedAdapter(t *testing.T, obs domain.ActivityObserver) *codex.CodexAdapter {
	t.Helper()
	return codex.NewCodexAdapter(helperExe(t), adapterTimeout).WithActivityObserver(obs)
}

func assertReportedActivity(t *testing.T, items []domain.ActivityItem) {
	t.Helper()
	if it, ok := testutil.FindItem(items, domain.ActivityToolCall, "command"); !ok || it.Summary == "" {
		t.Errorf("want a command tool call with a summary, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityText, ""); !ok {
		t.Errorf("want assistant text, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityKindUsage, ""); !ok {
		t.Errorf("want token usage, got %+v", items)
	}
}

func TestCodexAdapter_Invoke_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "codex-activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, _ = newObservedAdapter(t, obs).Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "test-agent#1", Harness: "codex"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestCodexAdapter_InvokeRaw_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "codex-activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, _ = newObservedAdapter(t, obs).InvokeRaw(context.Background(), agentRef(t), rawPayload)

	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "test-agent", Harness: "codex"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestCodexAdapter_Invoke_RejectedBeforeSpawn_StillAnnouncesStartAndEnd(t *testing.T) {
	setHelperEnv(t, "codex-success")
	obs := &testutil.Observer{}
	adapter := codex.NewCodexAdapter("/nonexistent/codex-binary", adapterTimeout).WithActivityObserver(obs)

	_, err := adapter.Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	if !errors.Is(err, commonharness.ErrExecutableNotFound) {
		t.Fatalf("want ErrExecutableNotFound, got %v", err)
	}
	_, _, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
}

func TestCodexAdapter_Invoke_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"codex-activity-paced", "codex-exit1", "codex-success"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref, req := agentRef(t), request("test-agent#1")
			baseResp, baseErr := codex.NewCodexAdapter(helperExe(t), adapterTimeout).Invoke(context.Background(), ref, req)

			resp, err := newObservedAdapter(t, &testutil.Observer{}).Invoke(context.Background(), ref, req)
			nilResp, nilErr := newObservedAdapter(t, nil).Invoke(context.Background(), ref, req)

			if !reflect.DeepEqual(baseResp, resp) || !reflect.DeepEqual(baseResp, nilResp) {
				t.Errorf("response differs: base=%+v observed=%+v nil=%+v", baseResp, resp, nilResp)
			}
			if !sameError(baseErr, err) || !sameError(baseErr, nilErr) {
				t.Errorf("error differs: base=%v observed=%v nil=%v", baseErr, err, nilErr)
			}
		})
	}
}

// sameError compares two errors by text and non-zero-exit class.
func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error() &&
		errors.Is(a, commonharness.ErrNonZeroExit) == errors.Is(b, commonharness.ErrNonZeroExit)
}
