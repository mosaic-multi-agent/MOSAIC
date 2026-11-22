package opencodev2_test

// Fixtures shared by the adapter tests: agent references, requests, the env
// recording helpers and a recording debug logger.

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/opencodev2"
)

// rawPayload is a consultation payload that is not a protocol message.
var rawPayload = []byte(`{"action":"route","agent":"agent-b","task_description":"do the thing"}`)

func ordinaryAgentRef() domain.AgentReference {
	return domain.AgentReference{Identifier: "test-agent", DefinitionPath: "/agents/test-agent.md"}
}

func orchestratorRef() domain.AgentReference {
	return domain.AgentReference{Identifier: "orchestrator-agent", DefinitionPath: "/agents/orchestrator-agent.md"}
}

func minimalRequest(instanceID string) domain.ProtocolRequest {
	return domain.ProtocolRequest{
		AgentInstanceID: instanceID,
		TaskDescription: "do the thing",
		InputArtifacts:  []string{},
		OutputArtifacts: []string{},
	}
}

// readEnv returns the environment recorded by the helper as a map.
func readEnv(t *testing.T, envFile string) map[string]string {
	t.Helper()
	var entries []string
	if err := json.Unmarshal(readFileOrFail(t, envFile), &entries); err != nil {
		t.Fatalf("could not decode recorded environment: %v", err)
	}
	env := make(map[string]string, len(entries))
	for _, e := range entries {
		if i := strings.Index(e, "="); i > 0 {
			env[e[:i]] = e[i+1:]
		}
	}
	return env
}

// newAdapter builds the adapter under test against the helper executable.
func newAdapter(t *testing.T, timeout time.Duration) *opencodev2.OpenCodeV2Adapter {
	t.Helper()
	return opencodev2.NewOpenCodeV2Adapter(helperExe(t), timeout)
}

// recordingLogger is a thread-safe domain.DebugLogger recording every call.
type recordingLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	Event   string
	Message string
}

func (r *recordingLogger) Log(event string, message string, _ ...domain.DebugField) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, logEntry{Event: event, Message: message})
}

func (r *recordingLogger) count(event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.entries {
		if e.Event == event {
			n++
		}
	}
	return n
}

func (r *recordingLogger) message(event string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.Event == event {
			return e.Message
		}
	}
	return ""
}
