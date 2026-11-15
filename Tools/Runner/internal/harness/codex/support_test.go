package codex_test

// Fixtures shared by the adapter tests: a copy of the Codex agent fixture, a
// request builder and a recording debug logger.

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"mosaic-run/internal/domain"
)

// agentFixture is the Codex agent fixture under testdata.
const agentFixture = "agent_reviewer.toml"

// agentCopy copies the Codex agent fixture into a temp dir and returns a
// reference to the copy and its original bytes.
func agentCopy(t *testing.T) (domain.AgentReference, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", agentFixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), agentFixture)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return domain.AgentReference{Identifier: "test-agent", DefinitionPath: path}, data
}

// agentRef returns a reference to a fresh copy of the Codex agent fixture.
func agentRef(t *testing.T) domain.AgentReference {
	t.Helper()
	ref, _ := agentCopy(t)
	return ref
}

// agentWithContent writes content as the agent file and returns its reference.
func agentWithContent(t *testing.T, content string) domain.AgentReference {
	t.Helper()
	path := filepath.Join(t.TempDir(), "broken.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	return domain.AgentReference{Identifier: "test-agent", DefinitionPath: path}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// assertFileUnchanged fails unless the file at path still holds want.
func assertFileUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read agent file after invocation: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("agent file changed during the invocation:\n got %q\nwant %q", got, want)
	}
}

// request returns a minimal protocol request for the instance id.
func request(instanceID string) domain.ProtocolRequest {
	return domain.ProtocolRequest{
		AgentInstanceID: instanceID,
		TaskDescription: "review the change",
		InputArtifacts:  []string{},
		OutputArtifacts: []string{},
	}
}

var rawPayload = []byte(`{"action":"route","task_description":"consult the orchestrator"}`)

// recordingLogger is a thread-safe domain.DebugLogger that records event names.
type recordingLogger struct {
	mu     sync.Mutex
	events []string
}

// Log implements domain.DebugLogger.
func (r *recordingLogger) Log(event string, _ string, _ ...domain.DebugField) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingLogger) logged(event string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e == event {
			return true
		}
	}
	return false
}

func (r *recordingLogger) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}
