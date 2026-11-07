package claudecode_test

// Tests that ClaudeCodeAdapter reports the live activity of Invoke and
// InvokeRaw to a domain.ActivityObserver while the process runs, announces the
// start and end of each invocation, and leaves results and errors unchanged.

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/claudecode"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/testutil"
)

const streamSubagentLine = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_2","name":"Task","input":{"subagent_type":"reviewer","description":"Review the change","prompt":"Please review"}}]},"session_id":"s1"}`

func init() {
	helperScenarios["activity-paced"] = testutil.PacedScenario(
		[]string{streamInitLine, streamAssistantTextLine("Reading the file"), streamToolUseLine, streamSubagentLine},
		[]string{streamToolResultLine, streamResultLine(false, "success", streamProtocolOK)},
	)
}

func newActivityAdapter(t *testing.T, obs domain.ActivityObserver) *claudecode.ClaudeCodeAdapter {
	t.Helper()
	return claudecode.NewClaudeCodeAdapter(helperExe(t), 30*time.Second).WithActivityObserver(obs)
}

func assertReportedActivity(t *testing.T, items []domain.ActivityItem) {
	t.Helper()
	if it, ok := testutil.FindItem(items, domain.ActivityToolCall, "Read"); !ok || it.Summary == "" {
		t.Errorf("want a Read tool call with a summary, got %+v", items)
	}
	if it, ok := testutil.FindItem(items, domain.ActivitySubagent, "reviewer"); !ok || it.Summary == "" {
		t.Errorf("want a reviewer subagent spawn, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityText, ""); !ok {
		t.Errorf("want assistant text, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityToolCall, "Task"); ok {
		t.Errorf("a subagent spawn must not also be reported as a tool call: %+v", items)
	}
}

func TestClaudeCodeAdapter_Invoke_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, err := newActivityAdapter(t, obs).Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))

	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "test-agent#1", Harness: "claude-code"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestClaudeCodeAdapter_InvokeRaw_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, err := newActivityAdapter(t, obs).InvokeRaw(context.Background(), orchestratorAgentRefCC(t), []byte("consult"))

	if err != nil {
		t.Fatalf("InvokeRaw: %v", err)
	}
	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "orchestrator-agent", Harness: "claude-code"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestClaudeCodeAdapter_Invoke_EachInvocationGetsItsOwnStartAndEnd(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	testutil.PresetOpenGate(t)
	obs := &testutil.Observer{}
	adapter := newActivityAdapter(t, obs)

	_, _ = adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	_, _ = adapter.InvokeRaw(context.Background(), orchestratorAgentRefCC(t), []byte("consult"))

	starts, _, _, ends := obs.Snapshot()
	if len(starts) != 2 || ends != 2 {
		t.Errorf("want 2 starts and 2 ends, got %d and %d", len(starts), ends)
	}
}

func TestClaudeCodeAdapter_Invoke_RejectedBeforeSpawn_StillAnnouncesStartAndEnd(t *testing.T) {
	cases := map[string]func(obs domain.ActivityObserver) error{
		"executable not found": func(obs domain.ActivityObserver) error {
			a := claudecode.NewClaudeCodeAdapter("/nonexistent/claude-binary", 5*time.Second).WithActivityObserver(obs)
			_, err := a.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
			return err
		},
		"tool extraction failure": func(obs domain.ActivityObserver) error {
			a := newActivityAdapter(t, obs)
			_, err := a.Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			setHelperEnv(t, "activity-paced")
			obs := &testutil.Observer{}

			err := run(obs)

			if err == nil {
				t.Fatal("want the invocation to be rejected")
			}
			_, _, events, ends := obs.Snapshot()
			testutil.AssertLifecycle(t, events, ends)
		})
	}
}

func TestClaudeCodeAdapter_Invoke_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-paced", "exit1"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref, req := ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1")
			baseResp, baseErr := claudecode.NewClaudeCodeAdapter(helperExe(t), 30*time.Second).Invoke(context.Background(), ref, req)

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

func TestClaudeCodeAdapter_InvokeRaw_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-paced", "exit1"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref := orchestratorAgentRefCC(t)
			baseOut, baseErr := claudecode.NewClaudeCodeAdapter(helperExe(t), 30*time.Second).InvokeRaw(context.Background(), ref, []byte("consult"))

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
