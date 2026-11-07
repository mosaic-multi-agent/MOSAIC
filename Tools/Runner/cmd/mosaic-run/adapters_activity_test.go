package main

// Tests that buildAdapter hands the activity observer to every CLI adapter
// (the OpenCode adapter behind its safety-net wrapper included), that a nil
// observer is safe, that the mock adapter ignores it, and that the interactive
// composition seam, which wires no observer yet, still runs invocations.
//
// The test binary doubles as the streaming fake CLI: init() prints the
// transcript file named by MOSAIC_TEST_STREAM_FILE and exits.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

func init() {
	if p := os.Getenv("MOSAIC_TEST_STREAM_FILE"); p != "" {
		data, _ := os.ReadFile(p)
		os.Stdout.Write(data) //nolint:errcheck
		os.Exit(0)
	}
}

// streamTranscripts holds a short scripted stream per harness: some text, one
// tool call and one subagent spawn.
var streamTranscripts = map[string]struct {
	lines    []string
	toolName string
}{
	commonharness.HarnessIDClaudeCode: {
		toolName: "Read",
		lines: []string{
			`{"type":"system","subtype":"init","session_id":"s1"}`,
			`{"type":"assistant","message":{"id":"m1","role":"assistant","content":[{"type":"text","text":"Reading the file"}]}}`,
			`{"type":"assistant","message":{"id":"m2","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"a.txt"}}]}}`,
			`{"type":"assistant","message":{"id":"m3","role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Task","input":{"subagent_type":"reviewer","description":"Review it","prompt":"p"}}]}}`,
			`{"type":"result","subtype":"success","is_error":false,"result":"done"}`,
		},
	},
	commonharness.HarnessIDOpenCode: {
		toolName: "read",
		lines: []string{
			`{"type":"step_start","sessionID":"s1","part":{"type":"step-start"}}`,
			`{"type":"text","sessionID":"s1","part":{"type":"text","text":"Reading the file"}}`,
			`{"type":"tool_use","sessionID":"s1","part":{"type":"tool","callID":"c1","tool":"read","state":{"status":"completed","input":{"filePath":"a.txt"},"output":"x"}}}`,
			`{"type":"tool_use","sessionID":"s1","part":{"type":"tool","callID":"c2","tool":"task","state":{"status":"completed","input":{"subagent_type":"reviewer","description":"Review it","prompt":"p"},"output":""}}}`,
			`{"type":"step_finish","sessionID":"s1","part":{"type":"step-finish","reason":"stop"}}`,
		},
	},
	commonharness.HarnessIDGHCPCLI: {
		toolName: "view",
		lines: []string{
			`{"type":"assistant.message","data":{"messageId":"m1","content":"Reading the file"}}`,
			`{"type":"tool.execution_start","data":{"toolCallId":"t1","toolName":"view","arguments":{"path":"a.txt"}}}`,
			`{"type":"subagent.started","data":{"toolCallId":"t2","agentName":"reviewer","agentDescription":"Review it"}}`,
			`{"type":"result","exitCode":0}`,
		},
	},
}

type wiringObserver struct {
	mu     sync.Mutex
	starts []domain.ActivityInvocation
	items  []domain.ActivityItem
	ends   int
}

func (o *wiringObserver) InvocationStarted(inv domain.ActivityInvocation) domain.ActivityFeed {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.starts = append(o.starts, inv)
	return &wiringFeed{o: o}
}

type wiringFeed struct{ o *wiringObserver }

func (f *wiringFeed) Activity(item domain.ActivityItem) {
	f.o.mu.Lock()
	defer f.o.mu.Unlock()
	f.o.items = append(f.o.items, item)
}

func (f *wiringFeed) Ended() {
	f.o.mu.Lock()
	defer f.o.mu.Unlock()
	f.o.ends++
}

func (o *wiringObserver) has(kind domain.ActivityKind, name string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, it := range o.items {
		if it.Kind == kind && it.Name == name {
			return true
		}
	}
	return false
}

// useStreamTranscript writes the transcript of harnessID and points the fake
// CLI at it.
func useStreamTranscript(t *testing.T, harnessID string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stream.jsonl")
	content := ""
	for _, l := range streamTranscripts[harnessID].lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	t.Setenv("MOSAIC_TEST_STREAM_FILE", path)
}

func assertWiredActivity(t *testing.T, obs *wiringObserver, harnessID string, source domain.ActivitySource, agent string) {
	t.Helper()
	obs.mu.Lock()
	starts, ends := append([]domain.ActivityInvocation(nil), obs.starts...), obs.ends
	obs.mu.Unlock()
	want := domain.ActivityInvocation{Source: source, Agent: agent, Harness: harnessID}
	if len(starts) != 1 || starts[0] != want || ends != 1 {
		t.Errorf("want one start %+v and one end, got starts=%+v ends=%d", want, starts, ends)
	}
	if !obs.has(domain.ActivityToolCall, streamTranscripts[harnessID].toolName) {
		t.Errorf("tool call %q not reported", streamTranscripts[harnessID].toolName)
	}
	if !obs.has(domain.ActivitySubagent, "reviewer") {
		t.Error("subagent spawn not reported")
	}
	if !obs.has(domain.ActivityText, "") {
		t.Error("assistant text not reported")
	}
}

func TestBuildAdapter_Invoke_ReportsActivityToObserver(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			useStreamTranscript(t, id)
			obs := &wiringObserver{}
			h := buildAdapter("", id, os.Args[0], "blanket", nonHITLTimeouts(guardTimeout), obs)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			req := domain.ProtocolRequest{AgentInstanceID: "test-agent#1", TaskDescription: "x"}

			_, _ = h.Invoke(ctx, timeoutAgentRef(t, domain.InvocationOrdinary), req)

			assertWiredActivity(t, obs, id, domain.ActivitySourceDispatch, "test-agent#1")
		})
	}
}

func TestBuildAdapter_InvokeRaw_ReportsActivityToObserver(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			useStreamTranscript(t, id)
			obs := &wiringObserver{}
			h := buildAdapter("", id, os.Args[0], "blanket", nonHITLTimeouts(guardTimeout), obs)
			raw, ok := h.(domain.RawInvoker)
			if !ok {
				t.Fatalf("%T does not implement domain.RawInvoker", h)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			_, _ = raw.InvokeRaw(ctx, timeoutAgentRef(t, domain.InvocationOrchestrator), []byte(`{}`))

			assertWiredActivity(t, obs, id, domain.ActivitySourceConsultation, "test-agent")
		})
	}
}

func TestBuildAdapter_NilObserver_InvocationsStillRun(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			useStreamTranscript(t, id)
			h := buildAdapter("", id, os.Args[0], "blanket", nonHITLTimeouts(guardTimeout), nil)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			_, err := h.Invoke(ctx, timeoutAgentRef(t, domain.InvocationOrdinary),
				domain.ProtocolRequest{AgentInstanceID: "test-agent#1", TaskDescription: "x"})

			if ctx.Err() != nil {
				t.Fatalf("invocation did not finish with a nil observer: %v", err)
			}
		})
	}
}

func TestBuildAdapter_MockAdapter_IgnoresObserver(t *testing.T) {
	obs := &wiringObserver{}

	h := buildAdapter("", harness.FakeHarnessID, "", "", nonHITLTimeouts(time.Minute), obs)

	if _, ok := h.(*harness.MockAdapter); !ok {
		t.Fatalf("want *harness.MockAdapter, got %T", h)
	}
	if len(obs.starts) != 0 {
		t.Errorf("mock adapter must not report activity, got %+v", obs.starts)
	}
}

func TestInteractiveWiring_WithoutObserver_InvocationsStillRun(t *testing.T) {
	useStreamTranscript(t, commonharness.HarnessIDClaudeCode)
	w := buildInteractiveWiring(testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t)))
	cfg := runconfig.ConfigSelection{
		Harness:        commonharness.HarnessIDClaudeCode,
		ExecutablePath: os.Args[0],
		Timeout:        guardTimeout,
		HITLTimeout:    guardTimeout,
	}
	deps := w.NewDeps(testRunFolder(t, "20260101T000000Z-0007"), true, "", cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, err := deps.Harness.Invoke(ctx, timeoutAgentRef(t, domain.InvocationOrdinary),
		domain.ProtocolRequest{AgentInstanceID: "test-agent#1", TaskDescription: "x"})

	if ctx.Err() != nil {
		t.Fatalf("invocation through the interactive seam did not finish: %v", err)
	}
}
