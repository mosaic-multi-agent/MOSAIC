package main

// codex_e2e_test.go drives the real CLI composition with the Codex harness
// against a Codex-deployed fixture workspace (TOML agents and TOML orchestrator
// script under testdata/integration/codex) and a fake Codex CLI: agent
// resolution, subagent invocations, the run-start orchestrator consultation and
// response extraction all run through the Codex adapter.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
)

const codexFixtureAgents = "../../testdata/integration/codex/agents"

// codexWorkspace is a temp workspace holding the Codex-deployed fixture agents.
type codexWorkspace struct {
	root      string
	agentsDir string
	before    map[string][]byte
	callsLog  string
}

// newCodexWorkspace copies the fixture agents into <tmp>/.codex/agents, makes the
// workspace the working directory and points the fake CLI's call log at it.
func newCodexWorkspace(t *testing.T) *codexWorkspace {
	t.Helper()
	ws := &codexWorkspace{root: t.TempDir(), before: map[string][]byte{}}
	ws.agentsDir = filepath.Join(ws.root, ".codex", "agents")
	if err := os.MkdirAll(ws.agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(codexFixtureAgents)
	if err != nil || len(entries) == 0 {
		t.Fatalf("read fixture agents: %v (%d entries)", err, len(entries))
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(codexFixtureAgents, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws.agentsDir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
		ws.before[e.Name()] = data
	}
	ws.callsLog = filepath.Join(t.TempDir(), "codex-calls.jsonl")
	t.Chdir(ws.root)
	t.Setenv(fakeCodexEnv, "1")
	t.Setenv(fakeCodexLogEnv, ws.callsLog)
	return ws
}

// run executes the linear workflow through runCLIMode in auto mode on the fake
// Codex CLI and returns the exit code.
func (ws *codexWorkspace) run(t *testing.T) int {
	t.Helper()
	args := []string{
		"run", "--harness", "codex", "--executable-path", os.Args[0],
		"--new-run", "--mode", "auto", "--workflow", "linear", "--task", "codex end to end",
		"--review-loop-limit", "none", "--checkpoints", "disabled", "--commits", "disabled",
	}
	return runCLIMode(args, args)
}

// calls returns the recorded fake Codex invocations in order.
func (ws *codexWorkspace) calls(t *testing.T) []fakeCodexCall {
	t.Helper()
	data, err := os.ReadFile(ws.callsLog)
	if err != nil {
		t.Fatalf("read fake codex call log: %v", err)
	}
	var out []fakeCodexCall
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var c fakeCodexCall
		if err := json.Unmarshal(line, &c); err != nil {
			t.Fatalf("decode call log line %q: %v", line, err)
		}
		out = append(out, c)
	}
	return out
}

func (ws *codexWorkspace) runFolderFile(t *testing.T, name string) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(ws.root, "Orchestration-*", name))
	if len(matches) != 1 {
		t.Fatalf("want exactly one Orchestration-*/%s, got %v", name, matches)
	}
	return matches[0]
}

func TestCodexEndToEnd_RunCompletesThroughSubagentsAndConsultation(t *testing.T) {
	ws := newCodexWorkspace(t)

	if code := ws.run(t); code != 0 {
		t.Fatalf("runCLIMode exit code = %d, want 0 (run completed)", code)
	}

	data, err := os.ReadFile(ws.runFolderFile(t, "Orchestration.md"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := artifact.Parse(data)
	if err != nil {
		t.Fatalf("parse produced artifact: %v", err)
	}
	var agents []string
	for _, e := range state.ExecutionLog {
		agents = append(agents, e.Agent)
	}
	if got := strings.Join(agents, ","); got != "agent-a#1,agent-b#2" {
		t.Errorf("execution log agents = %q, want %q", got, "agent-a#1,agent-b#2")
	}
}

func TestCodexEndToEnd_InvocationsCarryAgentInstructionsAndTheConsultation(t *testing.T) {
	ws := newCodexWorkspace(t)
	if code := ws.run(t); code != 0 {
		t.Fatalf("runCLIMode exit code = %d, want 0", code)
	}

	calls := ws.calls(t)
	if len(calls) != 3 {
		t.Fatalf("fake codex invocations = %d, want 3 (run-start consultation, agent-a, agent-b)", len(calls))
	}
	wantInstructions := []string{"Linear Workflow", "INSTRUCTIONS-OF-AGENT-A", "INSTRUCTIONS-OF-AGENT-B"}
	for i, c := range calls {
		if len(c.Args) == 0 || c.Args[0] != "exec" {
			t.Errorf("call %d args = %v, want a codex exec invocation", i, c.Args)
		}
		if !strings.Contains(c.Stdin, wantInstructions[i]) {
			t.Errorf("call %d stdin lacks %q: the agent's own instructions must reach Codex", i, wantInstructions[i])
		}
	}
	if !strings.Contains(calls[0].Stdin, "pre_consultation") {
		t.Errorf("first call must be the run-start orchestrator consultation, stdin = %q", calls[0].Stdin)
	}
}

func TestCodexEndToEnd_AgentFilesAndWorkspaceAreUntouched(t *testing.T) {
	ws := newCodexWorkspace(t)
	if code := ws.run(t); code != 0 {
		t.Fatalf("runCLIMode exit code = %d, want 0", code)
	}

	for name, want := range ws.before {
		got, err := os.ReadFile(filepath.Join(ws.agentsDir, name))
		if err != nil {
			t.Fatalf("read %s after the run: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed during the run", name)
		}
	}
	entries, err := os.ReadDir(filepath.Join(ws.root, ".codex"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "agents" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf(".codex holds %v after the run, want only agents (no snapshot, no backup)", names)
	}
}

func TestCodexEndToEnd_MosaicLogHoldsRunLifecycleOnly(t *testing.T) {
	ws := newCodexWorkspace(t)
	if code := ws.run(t); code != 0 {
		t.Fatalf("runCLIMode exit code = %d, want 0", code)
	}

	matches, _ := filepath.Glob(filepath.Join(ws.root, "OrchestrationLogs", "*", "00_orchestrator_events.jsonl"))
	if len(matches) != 1 {
		t.Fatalf("run logs = %v, want exactly one", matches)
	}
	events := mlReadEvents(t, matches[0])
	mlAssertPairs(t, events, 1)
	for _, ev := range events {
		if ev["harness"] != "codex" {
			t.Errorf("event %v: harness = %v, want codex", ev["event"], ev["harness"])
		}
	}
	if events[1]["outcome"] != "completed" {
		t.Errorf("run_end outcome = %v, want completed", events[1]["outcome"])
	}
	invocationLogs, _ := filepath.Glob(filepath.Join(ws.root, "OrchestrationLogs", "*", "*", "03_events.jsonl"))
	if len(invocationLogs) != 0 {
		t.Errorf("invocation event files = %v, want none for Codex", invocationLogs)
	}
}
