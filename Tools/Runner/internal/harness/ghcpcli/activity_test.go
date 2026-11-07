package ghcpcli_test

// Tests that GHCPCLIAdapter reports the live activity of Invoke and
// InvokeRaw to a domain.ActivityObserver while the process runs, announces the
// start and end of each invocation, and leaves results and errors unchanged.

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/ghcpcli"
	"mosaic-run/internal/testutil"
)

const (
	ghTextLine     = `{"type":"assistant.message","data":{"messageId":"m1","content":"Reading the file"}}`
	ghToolLine     = `{"type":"tool.execution_start","data":{"toolCallId":"t1","toolName":"view","arguments":{"path":"a.txt"}}}`
	ghSubagentLine = `{"type":"subagent.started","data":{"toolCallId":"t2","agentName":"reviewer","agentDisplayName":"Reviewer","agentDescription":"Review the change"}}`
	ghDoneLine     = `{"type":"tool.execution_complete","data":{"toolCallId":"t1","success":true,"result":{"content":"hello"}}}`
)

func init() {
	helperScenarios["activity-paced"] = testutil.PacedScenario(
		[]string{ghTextLine, ghToolLine, ghSubagentLine},
		[]string{ghDoneLine, strings.TrimSpace(ghcpProtocolReplyLine()), `{"type":"result","exitCode":0}`},
	)
}

func newActivityAdapter(t *testing.T, obs domain.ActivityObserver) *ghcpcli.GHCPCLIAdapter {
	t.Helper()
	return ghcpcli.NewGHCPCLIAdapter(helperExe(t), 30*time.Second).WithActivityObserver(obs)
}

func assertReportedActivity(t *testing.T, items []domain.ActivityItem) {
	t.Helper()
	if it, ok := testutil.FindItem(items, domain.ActivityToolCall, "view"); !ok || it.Summary == "" {
		t.Errorf("want a view tool call with a summary, got %+v", items)
	}
	if it, ok := testutil.FindItem(items, domain.ActivitySubagent, "reviewer"); !ok || it.Summary == "" {
		t.Errorf("want a reviewer subagent spawn, got %+v", items)
	}
	if _, ok := testutil.FindItem(items, domain.ActivityText, ""); !ok {
		t.Errorf("want assistant text, got %+v", items)
	}
}

func TestGHCPCLIAdapter_Invoke_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, err := newActivityAdapter(t, obs).Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))

	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "test-agent#1", Harness: "ghcp-cli"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestGHCPCLIAdapter_InvokeRaw_ReportsActivityWhileProcessRuns(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs, gate := testutil.NewGatedObserver(t)

	_, err := newActivityAdapter(t, obs).InvokeRaw(context.Background(), orchestratorAgentRef(), []byte("consult"))

	if err != nil {
		t.Fatalf("InvokeRaw: %v", err)
	}
	testutil.AssertGateWasOpenedWhileRunning(t, gate)
	starts, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	want := domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "orchestrator-agent", Harness: "ghcp-cli"}
	if len(starts) != 1 || starts[0] != want {
		t.Errorf("want one start %+v, got %+v", want, starts)
	}
	assertReportedActivity(t, items)
}

func TestGHCPCLIAdapter_Invoke_EachInvocationGetsItsOwnStartAndEnd(t *testing.T) {
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

func TestGHCPCLIAdapter_Invoke_RejectedBeforeSpawn_StillAnnouncesStartAndEnd(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	obs := &testutil.Observer{}
	adapter := ghcpcli.NewGHCPCLIAdapter("/nonexistent/copilot-binary", 5*time.Second).WithActivityObserver(obs)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))

	if !errors.Is(err, cliexec.ErrExecutableNotFound) {
		t.Fatalf("want ErrExecutableNotFound, got %v", err)
	}
	_, _, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
}

func TestGHCPCLIAdapter_Invoke_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-paced", "exit1"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref, req := ordinaryAgentRef(), minimalClaudeRequest("test-agent#1")
			baseResp, baseErr := ghcpcli.NewGHCPCLIAdapter(helperExe(t), 30*time.Second).Invoke(context.Background(), ref, req)

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

func TestGHCPCLIAdapter_InvokeRaw_ResultIdenticalWithAndWithoutObserver(t *testing.T) {
	for _, scenario := range []string{"activity-paced", "exit1"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			testutil.PresetOpenGate(t)
			ref := orchestratorAgentRef()
			baseOut, baseErr := ghcpcli.NewGHCPCLIAdapter(helperExe(t), 30*time.Second).InvokeRaw(context.Background(), ref, []byte("consult"))

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
