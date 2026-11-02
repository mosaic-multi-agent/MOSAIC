package session_test

// Helpers of the request wire form tests: the mock harness records requests as
// structs and never serializes, so these helpers serialize every captured
// request through harness.MarshalRequest, the function the production adapters
// use, and assert on the resulting JSON.

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// wireWorkflow is the workflow of the request-wire-linear fixture.
const wireWorkflow = "request-wire"

// requireWireArtifactArrays serializes req through harness.MarshalRequest and
// asserts that both artifact fields are present as JSON arrays, never null.
func requireWireArtifactArrays(t *testing.T, label string, req domain.ProtocolRequest) {
	t.Helper()
	data, err := harness.MarshalRequest(req)
	if err != nil {
		t.Fatalf("%s: MarshalRequest: %v", label, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("%s: serialized request is not a JSON object: %v: %s", label, err, data)
	}
	for _, key := range []string{"input_artifacts", "output_artifacts"} {
		raw, ok := fields[key]
		if !ok {
			t.Errorf("%s: want %s present in %s", label, key, data)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
			t.Errorf("%s: want %s to be a JSON array, got %s", label, key, raw)
		}
	}
}

// requireWireArraysOfAgent asserts the serialized form of every request sent
// to agent, and returns how many there were. It fails when there were none.
func requireWireArraysOfAgent(t *testing.T, invs []harness.Invocation, agent string) int {
	t.Helper()
	n := 0
	for i, inv := range invs {
		if inv.Agent.Identifier != agent {
			continue
		}
		n++
		requireWireArtifactArrays(t, agent+" dispatch "+strconv.Itoa(n)+" (invocation "+strconv.Itoa(i+1)+")", inv.Request)
	}
	if n == 0 {
		t.Fatalf("want at least one dispatch of %s, got none", agent)
	}
	return n
}

// requireWireArraysOfAll asserts the serialized form of every captured request
// and fails when there were none.
func requireWireArraysOfAll(t *testing.T, invs []harness.Invocation) {
	t.Helper()
	if len(invs) == 0 {
		t.Fatal("want at least one dispatch, got none")
	}
	for i, inv := range invs {
		requireWireArtifactArrays(t, inv.Agent.Identifier+" invocation "+strconv.Itoa(i+1), inv.Request)
	}
}

// newWireSession builds a session over the request-wire-linear fixture, whose
// three rows declare empty input and output cells. routing is the routing
// consultant; nil wires none.
func newWireSession(t *testing.T, routing domain.RoutingConsultant) (session.Session, *harness.MockAdapter, string) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "request-wire-linear-orch.md")
	for _, agent := range []string{"agent-a", "agent-b", "agent-c"} {
		writeAgentFile(t, dir, agent)
	}
	f := harness.NewMockAdapter()
	ses := newWireSessionOn(f, routing)
	return ses, f, orchPath
}

// newWireSessionOn builds a session on harness f with an in-memory store.
func newWireSessionOn(f *harness.MockAdapter, routing domain.RoutingConsultant) session.Session {
	return session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Routing:  routing,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
}

// wireConfig is the new-run request of the request-wire-linear workflow.
func wireConfig(orchPath string, mode domain.ExecutionMode) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           wireWorkflow,
		Task:                 "test task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: mode},
	}
}

// queueWireSuccess queues one SUCCESS dispatch for each agent.
func queueWireSuccess(f *harness.MockAdapter, agents ...string) {
	for _, agent := range agents {
		f.Queue(agent, statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))
	}
}
