package main

// usageingest_helpers_test.go holds the shared fixtures of the GHCP CLI usage
// ingest wiring tests: a scripted harness, a recording ingest seam, a capturing
// debug logger and writers for the MOSAIC log files a harness hook leaves behind.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mosaic-common/otelingest"

	"mosaic-run/internal/domain"
)

// uiHarness is a scripted ghcpCLIHarness. OnInvoke and OnInvokeRaw simulate the
// harness process (and its hook) running before the call returns.
type uiHarness struct {
	RunIDValue string
	ExecPath   string

	OnInvoke    func(request domain.ProtocolRequest)
	Response    domain.ProtocolResponse
	Err         error
	OnInvokeRaw func()
	RawReply    []byte
	RawErr      error
}

func (h *uiHarness) Invoke(_ context.Context, _ domain.AgentReference, request domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	if h.OnInvoke != nil {
		h.OnInvoke(request)
	}
	return h.Response, h.Err
}

func (h *uiHarness) InvokeRaw(_ context.Context, _ domain.AgentReference, _ []byte) ([]byte, error) {
	if h.OnInvokeRaw != nil {
		h.OnInvokeRaw()
	}
	return h.RawReply, h.RawErr
}

func (h *uiHarness) ExecutablePath() string { return h.ExecPath }
func (h *uiHarness) RunID() string          { return h.RunIDValue }

// uiRecorder is the ingest seam: it records every request and answers with Result.
type uiRecorder struct {
	mu     sync.Mutex
	Reqs   []otelingest.Request
	Result otelingest.Result
}

func (r *uiRecorder) ingest(req otelingest.Request) otelingest.Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Reqs = append(r.Reqs, req)
	return r.Result
}

func (r *uiRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Reqs)
}

// uiLogger captures debug entries as single searchable strings.
type uiLogger struct {
	mu      sync.Mutex
	Entries []string
}

func (l *uiLogger) Log(event string, message string, fields ...domain.DebugField) {
	l.mu.Lock()
	defer l.mu.Unlock()
	parts := []string{event, message}
	for _, f := range fields {
		parts = append(parts, f.Key, f.Value)
	}
	l.Entries = append(l.Entries, strings.ToLower(strings.Join(parts, " ")))
}

// naming counts the entries that contain any of the needles (case-insensitive).
func (l *uiLogger) naming(needles ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.Entries {
		for _, needle := range needles {
			if strings.Contains(e, strings.ToLower(needle)) {
				n++
				break
			}
		}
	}
	return n
}

// uiRig is one wired GHCP CLI harness over a fresh workspace.
type uiRig struct {
	Harness   *uiHarness
	Adapter   domain.HarnessAdapter // what withGHCPCLIUsageIngest returned
	Cfg       *usageIngestConfig
	Rec       *uiRecorder
	Log       *uiLogger
	Workspace string
	RunFolder string
	RunDir    string // <workspace>/OrchestrationLogs/<run id>
	OTelPath  string
}

// uiSetEnv sets the Runner's OTel path variable; "" leaves it unset.
func uiSetEnv(t *testing.T, value string) {
	t.Helper()
	t.Setenv(otelingest.EnvVar, value)
	if value == "" {
		if err := os.Unsetenv(otelingest.EnvVar); err != nil {
			t.Fatal(err)
		}
	}
}

// uiNewRig wires a GHCP CLI harness for run mlRunID in a fresh workspace with the
// OTel variable set to otelPath ("" = unset). The ingest seam is replaced by a recorder.
func uiNewRig(t *testing.T, otelPath string) *uiRig {
	t.Helper()
	runFolder, ws := mlRunFolder(t)
	return uiWire(t, ws, runFolder, otelPath, &uiLogger{})
}

// uiWire wires a harness for an existing workspace/run folder (used to wire one run twice).
func uiWire(t *testing.T, ws, runFolder, otelPath string, logger *uiLogger) *uiRig {
	t.Helper()
	uiSetEnv(t, otelPath)
	h := &uiHarness{RunIDValue: mlRunID, ExecPath: "copilot", Response: mlSuccessResponse(), RawReply: []byte("reply")}
	adapter := withGHCPCLIUsageIngest(h, runFolder, logger)
	cfg := usageIngestOf(adapter)
	if cfg == nil {
		t.Fatalf("withGHCPCLIUsageIngest returned %T without a usage ingest config", adapter)
	}
	rec := &uiRecorder{}
	cfg.Ingest = rec.ingest
	return &uiRig{
		Harness: h, Adapter: adapter, Cfg: cfg, Rec: rec, Log: logger, Workspace: ws,
		RunFolder: runFolder, RunDir: filepath.Join(ws, "OrchestrationLogs", mlRunID), OTelPath: otelPath,
	}
}

func (r *uiRig) invoke(t *testing.T, instance string) (domain.ProtocolResponse, error) {
	t.Helper()
	return r.Adapter.Invoke(context.Background(), domain.AgentReference{Identifier: "Research"}, domain.ProtocolRequest{AgentInstanceID: instance, RunID: mlRunID, TaskDescription: "do it"})
}

func (r *uiRig) invokeRaw(t *testing.T) ([]byte, error) {
	t.Helper()
	raw, ok := r.Adapter.(domain.RawInvoker)
	if !ok {
		t.Fatalf("%T does not expose domain.RawInvoker", r.Adapter)
	}
	return raw.InvokeRaw(context.Background(), domain.AgentReference{}, []byte("{}"))
}

// uiWriteEvents appends one envelope line per sessionID to the stream file at path.
func uiWriteEvents(t *testing.T, path string, sessionIDs ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, s := range sessionIDs {
		line := `{"event":"tool_call_start","harness":"ghcp-cli","run_id":"` + mlRunID + `","session_id":"` + s + `"}` + "\n"
		if _, err := f.WriteString(line); err != nil {
			t.Fatal(err)
		}
	}
}

// uiInvocationEvents is the 03_events.jsonl path of an invocation folder.
func (r *uiRig) invocationEvents(instance string) string {
	return filepath.Join(r.RunDir, instance, "03_events.jsonl")
}

// orchestratorEvents is the run's orchestrator stream path.
func (r *uiRig) orchestratorEvents() string {
	return filepath.Join(r.RunDir, "00_orchestrator_events.jsonl")
}

// uiRunProcess simulates one harness process of the rig whose hook recorded a session.
func uiRunProcess(t *testing.T, rig *uiRig) {
	t.Helper()
	rig.Harness.OnInvoke = func(domain.ProtocolRequest) {
		uiWriteEvents(t, rig.invocationEvents(mlAgentInstance), "sess-research")
	}
	if _, err := rig.invoke(t, mlAgentInstance); err != nil {
		t.Fatal(err)
	}
}

// uiWriteOTelFile creates the OTel file with n bytes.
func uiWriteOTelFile(t *testing.T, n int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "otel.jsonl")
	if err := os.WriteFile(p, []byte(strings.Repeat("x", n)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
