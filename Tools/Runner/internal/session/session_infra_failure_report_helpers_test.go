package session_test

// Helpers of the infrastructure failure reporting tests: sessions over the
// checkpoint fixtures with a recording debug logger, and readers of the
// Runner-log entries one event name produced.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const (
	infraCheckpointAgent = "checkpoint-manager-git"

	// infraContinueFixture declares the checkpoint agent with on_failure=continue;
	// infraHaltFixture with on_failure=halt. Both fire INVOCATION_INTERVAL:1.
	infraContinueFixture = "continue-infra-orch.md"
	infraHaltFixture     = "interval-agent-orch.md"
)

// infraReportRig is a session over a checkpoint fixture whose debug log is
// captured.
type infraReportRig struct {
	ses      session.Session
	adapter  *harness.MockAdapter
	store    *memStore
	logger   *sessionRecordingLogger
	orchPath string
}

// newInfraReportRig builds the rig. withDefinition false leaves the checkpoint
// agent's definition file unwritten, so the dispatch cannot resolve it.
func newInfraReportRig(t *testing.T, fixture string, withDefinition bool, stop func() bool) *infraReportRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, fixture)
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	if withDefinition {
		writeAgentFile(t, dir, infraCheckpointAgent)
	}
	rig := &infraReportRig{
		adapter:  harness.NewMockAdapter(),
		store:    &memStore{},
		logger:   &sessionRecordingLogger{},
		orchPath: orchPath,
	}
	rig.ses = session.New(session.Deps{
		Harness:       rig.adapter,
		Store:         rig.store,
		Clock:         fixedClock{t: epoch},
		Interact:      &noopInteraction{},
		Debug:         rig.logger,
		StopRequested: stop,
	})
	return rig
}

// queueWorkflow queues successful replies of both workflow agents.
func (r *infraReportRig) queueWorkflow() {
	for _, agent := range []string{"agent-a", "agent-b"} {
		r.adapter.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#x", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
	}
}

// queueCheckpoint queues one reply of the checkpoint agent.
func (r *infraReportRig) queueCheckpoint(entry harness.ScriptedEntry) {
	r.adapter.Queue(infraCheckpointAgent, entry)
}

// checkpointReply is a checkpoint agent response with the given status.
func checkpointReply(status domain.StatusCode, code domain.ErrorCode, message, reason string) harness.ScriptedEntry {
	return harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: infraCheckpointAgent + "#x",
		StatusCode:      status,
		StatusMessage:   message,
		ErrorCode:       code,
		ErrorReason:     reason,
	}}
}

// run starts a new checkpoint-enabled run.
func (r *infraReportRig) run(t *testing.T) domain.RunOutcome {
	t.Helper()
	cfg := baseLinearConfig(r.orchPath)
	cfg.Checkpoints = true
	got, err := r.ses.Start(t.Context(), cfg)
	if err != nil {
		t.Fatalf("want nil error from Start, got %v", err)
	}
	return got
}

// entriesOf returns, in order, the captured log entries of event.
func (r *sessionRecordingLogger) entriesOf(event string) []sessionLogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []sessionLogEntry
	for _, e := range r.entries {
		if e.Event == event {
			out = append(out, e)
		}
	}
	return out
}

// field returns the value of key on the entry; ok is false when absent.
func (e sessionLogEntry) field(key string) (string, bool) {
	for _, f := range e.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}

// stepDoneOf returns the step-done entries whose agent field equals agent.
func (r *sessionRecordingLogger) stepDoneOf(agent string) []sessionLogEntry {
	var out []sessionLogEntry
	for _, e := range r.entriesOf(domain.EventSessionStepDone) {
		if v, _ := e.field("agent"); v == agent {
			out = append(out, e)
		}
	}
	return out
}

// requireFields asserts every key of want is on e with that value.
func requireFields(t *testing.T, e sessionLogEntry, want map[string]string) {
	t.Helper()
	for key, wantVal := range want {
		got, ok := e.field(key)
		if !ok {
			t.Errorf("event %s has no %q field, want %q (fields: %+v)", e.Event, key, wantVal, e.Fields)
			continue
		}
		if got != wantVal {
			t.Errorf("event %s field %q = %q, want %q", e.Event, key, got, wantVal)
		}
	}
}
