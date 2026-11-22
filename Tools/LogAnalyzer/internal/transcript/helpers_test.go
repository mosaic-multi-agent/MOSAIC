package transcript_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const (
	runA = "20261031T100000Z-a001"
	runB = "20261031T100000Z-a002"

	claudeFormat   = string(domain.TranscriptClaudeCodeJSONL)
	openCodeFormat = string(domain.TranscriptOpenCodeMessages)
)

// fixtureBytes returns the content of a synthetic transcript fixture.
func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "transcripts", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func sidecarJSON(format string) []byte {
	return []byte(`{"native_format":"` + format + `"}`)
}

// newRun creates a run folder under root and returns its (empty) entry.
func newRun(t *testing.T, root, id string) domain.RunEntry {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	return domain.RunEntry{Run: domain.NamedRun(id), Dir: dir}
}

// withOrchestrator writes the run-root orchestrator transcript and its sidecar.
func withOrchestrator(t *testing.T, run domain.RunEntry, content []byte, format string) domain.RunEntry {
	t.Helper()
	writeFile(t, filepath.Join(run.Dir, "00_orchestrator_session.raw"), content)
	writeFile(t, filepath.Join(run.Dir, "00_orchestrator_session.meta.json"), sidecarJSON(format))
	return run
}

// withAgent adds an agent folder holding a transcript and its sidecar. The
// folder name is the instance id with '#' replaced by '_'.
func withAgent(t *testing.T, run domain.RunEntry, hint string, content []byte, format string) domain.RunEntry {
	t.Helper()
	dir := agentDir(run, hint)
	writeFile(t, filepath.Join(dir, "04_session.raw"), content)
	writeFile(t, filepath.Join(dir, "04_session.meta.json"), sidecarJSON(format))
	run.Agents = append(run.Agents, domain.AgentEntry{Dir: dir, InstanceHint: domain.AgentInstanceID(hint)})
	return run
}

func agentDir(run domain.RunEntry, hint string) string {
	return filepath.Join(run.Dir, strings.ReplaceAll(hint, "#", "_"))
}

func logsRoot(root string, runs ...domain.RunEntry) domain.Inventory {
	return domain.Inventory{
		Source: domain.Source{Kind: domain.SourceLogsRoot, Path: root},
		Runs:   runs,
	}
}

// findAgent returns the facts of the agent with the given instance hint.
func findAgent(t *testing.T, facts []domain.TranscriptFacts, hint string) domain.TranscriptFacts {
	t.Helper()
	for _, f := range facts {
		if f.Owner == domain.StreamAgentInstance && f.InstanceHint == domain.AgentInstanceID(hint) {
			return f
		}
	}
	t.Fatalf("no agent facts for %q in %+v", hint, facts)
	return domain.TranscriptFacts{}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
