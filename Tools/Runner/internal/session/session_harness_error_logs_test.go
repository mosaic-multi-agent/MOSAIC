package session_test

// Tests of how the dispatch log and the Runner (debug) log identify an
// invocation that ended without a usable subagent response as HARNESS_ERROR,
// while a subagent-returned BLOCKED keeps being logged as a response.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// logsWorkflow: agent-a has no On Findings column, so any non-SUCCESS status
// deviates; no routing consultant is wired.
const logsWorkflow = `<Workflow type="core" name="deviate-log" version="1.0">
## Deviation Log Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | FALSE | COMPLETE | - | plan.md | result.md |
</Workflow>
`

// harnessErrorCall is one LogHarnessError call.
type harnessErrorCall struct {
	id      string
	failure domain.HarnessFailure
	errText string
}

// kindRecordingDispatchLogger records which dispatch-log method each
// invocation's outcome was written with.
type kindRecordingDispatchLogger struct {
	mu            sync.Mutex
	harnessErrors []harnessErrorCall
	errorIDs      []string
	responses     []domain.ProtocolResponse
}

func (l *kindRecordingDispatchLogger) LogRequest(domain.ProtocolRequest) {}
func (l *kindRecordingDispatchLogger) LogResponse(r domain.ProtocolResponse) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.responses = append(l.responses, r)
}
func (l *kindRecordingDispatchLogger) LogError(id, _ string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorIDs = append(l.errorIDs, id)
}
func (l *kindRecordingDispatchLogger) LogHarnessError(id string, f domain.HarnessFailure, errText string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.harnessErrors = append(l.harnessErrors, harnessErrorCall{id: id, failure: f, errText: errText})
}
func (l *kindRecordingDispatchLogger) SetRunID(string) {}
func (l *kindRecordingDispatchLogger) Close()          {}
func (l *kindRecordingDispatchLogger) Path() string    { return "" }

// startFirstAgentWith runs the deviation workflow in auto mode with the given
// scripted outcome for agent-a and both loggers wired.
func startFirstAgentWith(t *testing.T, entry harness.ScriptedEntry) (*kindRecordingDispatchLogger, *sessionRecordingLogger) {
	t.Helper()
	debug := &sessionRecordingLogger{}
	dlog := &kindRecordingDispatchLogger{}
	dir := t.TempDir()
	orchPath := filepath.Join(dir, "deviate-log-orch.md")
	if err := os.WriteFile(orchPath, []byte(logsWorkflow), 0o600); err != nil {
		t.Fatalf("write orchestrator fixture: %v", err)
	}
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f := harness.NewMockAdapter()
	ses := session.New(session.Deps{
		Harness: f, Store: &memStore{}, Clock: fixedClock{t: epoch},
		Interact: &noopInteraction{}, Debug: debug, DispatchLog: dlog,
	})
	// A non-SUCCESS outcome may be retried as agent-a#2, #3, ...; queue enough
	// copies that a retry never runs into an empty script (a genuine harness error).
	for i := 0; i < 5; i++ {
		f.Queue("agent-a", entry)
	}

	ses.Start(context.Background(), domain.RunConfig{ //nolint:errcheck
		OrchestratorFilePath: orchPath,
		WorkflowID:           "deviate-log",
		Task:                 "task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
	})
	return dlog, debug
}

func TestSession_HarnessErrorInvocation_DispatchLogEntryCarriesHarnessErrorKind(t *testing.T) {
	dlog, _ := startFirstAgentWith(t, harness.ScriptedEntry{Err: fmt.Errorf("agent stalled: %w", commonharness.ErrTimeout)})

	if len(dlog.harnessErrors) == 0 {
		t.Fatalf("want the failed invocation logged with LogHarnessError, got none (LogError ids: %v)", dlog.errorIDs)
	}
	call := dlog.harnessErrors[0]
	if call.id != "agent-a#1" {
		t.Errorf("harness-error entry id = %q, want agent-a#1", call.id)
	}
	if call.failure.Kind != domain.HarnessFailureTimeout {
		t.Errorf("failure kind = %q, want %q", call.failure.Kind, domain.HarnessFailureTimeout)
	}
	if !strings.Contains(call.errText, "agent stalled") {
		t.Errorf("error text = %q, want the harness error text", call.errText)
	}
}

func TestSession_HarnessErrorInvocation_IsNotAlsoLoggedAsPlainError(t *testing.T) {
	dlog, _ := startFirstAgentWith(t, harness.ScriptedEntry{Err: fmt.Errorf("exit status 1: %w", commonharness.ErrNonZeroExit)})

	for _, id := range dlog.errorIDs {
		if id == "agent-a#1" {
			t.Errorf("invocation agent-a#1 logged with LogError; want LogHarnessError only")
		}
	}
	if len(dlog.harnessErrors) == 0 || dlog.harnessErrors[0].failure.Kind != domain.HarnessFailureProcess {
		t.Errorf("want a process-failure harness-error entry first, got %+v", dlog.harnessErrors)
	}
}

func TestSession_SubagentReturnedBlocked_IsLoggedAsResponseNotHarnessError(t *testing.T) {
	dlog, _ := startFirstAgentWith(t, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-a#1", StatusCode: domain.StatusBLOCKED,
		ErrorCode: domain.ErrorTOOL_UNAVAILABLE, StatusMessage: "tool missing",
	}})

	if len(dlog.harnessErrors) != 0 {
		t.Errorf("subagent-returned BLOCKED logged as harness error: %+v", dlog.harnessErrors)
	}
	if len(dlog.responses) == 0 || dlog.responses[0].StatusCode != domain.StatusBLOCKED {
		t.Errorf("want the BLOCKED response logged via LogResponse, got %+v", dlog.responses)
	}
}

func TestSession_HarnessErrorInvocation_RunnerLogEventIdentifiesHarnessError(t *testing.T) {
	_, debug := startFirstAgentWith(t, harness.ScriptedEntry{Err: fmt.Errorf("agent stalled: %w", commonharness.ErrTimeout)})

	for key, want := range map[string]string{"agent": "agent-a#1", "status": "HARNESS_ERROR", "kind": "timeout"} {
		got, ok := debug.fieldValue(domain.EventSessionHarnessError, key)
		if !ok || got != want {
			t.Errorf("%s field %q = %q (present %v), want %q", domain.EventSessionHarnessError, key, got, ok, want)
		}
	}
}

func TestSession_HarnessErrorInvocation_RunnerLogNeverReportsItAsBlocked(t *testing.T) {
	_, debug := startFirstAgentWith(t, harness.ScriptedEntry{Err: fmt.Errorf("agent stalled: %w", commonharness.ErrTimeout)})

	debug.mu.Lock()
	defer debug.mu.Unlock()
	for _, e := range debug.entries {
		for _, f := range e.Fields {
			if f.Key == "status" && f.Value == string(domain.StatusBLOCKED) {
				t.Errorf("event %s reports the harness-error invocation as BLOCKED: %+v", e.Event, e.Fields)
			}
			if f.Key == "error_code" && f.Value != "" && e.Event == domain.EventSessionStepDone {
				t.Errorf("step.done of a harness-error row carries error_code %q", f.Value)
			}
		}
	}
}
