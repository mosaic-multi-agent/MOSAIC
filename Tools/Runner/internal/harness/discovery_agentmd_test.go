package harness_test

// Tests for DiscoverOrchestrator accepting both orchestrator-script.md and
// orchestrator-script.agent.md in the harness agents directory.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/harness"
)

const (
	orchPlainName = "orchestrator-script.md"
	orchAgentName = "orchestrator-script.agent.md"
)

// writeAgentsFiles creates the claude-code agents directory under a fresh
// temp workDir and writes each named file into it. Returns the workDir.
func writeAgentsFiles(t *testing.T, names ...string) string {
	t.Helper()
	workDir := t.TempDir()
	dir := filepath.Join(workDir, ".claude", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %v: %v", dir, err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# orchestrator\n"), 0o644); err != nil {
			t.Fatalf("write %v: %v", name, err)
		}
	}
	return workDir
}

func TestDiscoverOrchestrator_PlainMdOnly_Found(t *testing.T) {
	workDir := writeAgentsFiles(t, orchPlainName)

	got, err := harness.DiscoverOrchestrator(workDir, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(workDir, ".claude", "agents", orchPlainName)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverOrchestrator_AgentMdOnly_Found(t *testing.T) {
	workDir := writeAgentsFiles(t, orchAgentName)

	got, err := harness.DiscoverOrchestrator(workDir, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error for workspace with only %s: %v", orchAgentName, err)
	}
	want := filepath.Join(workDir, ".claude", "agents", orchAgentName)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverOrchestrator_Neither_RefusalNamesBothFiles(t *testing.T) {
	workDir := writeAgentsFiles(t)

	got, err := harness.DiscoverOrchestrator(workDir, "claude-code")
	if err == nil {
		t.Fatal("expected refusal when neither file exists, got nil")
	}
	if got != "" {
		t.Errorf("got path %q, want empty on refusal", got)
	}
	re := asRefusalError(t, err)
	for _, name := range []string{orchPlainName, orchAgentName} {
		if !strings.Contains(re.Reason, name) {
			t.Errorf("Reason %q does not name %s", re.Reason, name)
		}
	}
	if !strings.Contains(re.Reason, "claude-code") {
		t.Errorf("Reason %q does not name the harness", re.Reason)
	}
}

func TestDiscoverOrchestrator_BothPresent_RefusesNamingBothAndReturnsNoPath(t *testing.T) {
	workDir := writeAgentsFiles(t, orchPlainName, orchAgentName)

	got, err := harness.DiscoverOrchestrator(workDir, "claude-code")
	if err == nil {
		t.Fatalf("expected refusal when both files exist, got path %q", got)
	}
	if got != "" {
		t.Errorf("got path %q, want empty so neither file is preferred", got)
	}
	re := asRefusalError(t, err)
	if re.Component != "harness" {
		t.Errorf("Component = %q, want %q", re.Component, "harness")
	}
	for _, name := range []string{orchPlainName, orchAgentName} {
		if !strings.Contains(re.Reason, name) {
			t.Errorf("Reason %q does not name %s", re.Reason, name)
		}
	}
}

func TestDiscoverOrchestrator_UnknownHarness_StillRefusedWithAgentMdPresent(t *testing.T) {
	workDir := writeAgentsFiles(t, orchAgentName)

	got, err := harness.DiscoverOrchestrator(workDir, "unknown-harness")
	if err == nil {
		t.Fatalf("expected refusal for unknown harness, got path %q", got)
	}
	asRefusalError(t, err)
}
