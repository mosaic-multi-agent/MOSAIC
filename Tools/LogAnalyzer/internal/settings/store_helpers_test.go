package settings_test

// Shared helpers for the settings store tests. Every test works in a temp
// directory standing in for the MOSAIC runtime root.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/settings"
)

// newTestStore returns a store rooted at a fresh temp directory and that root.
func newTestStore(t *testing.T) (*settings.Store, string) {
	t.Helper()
	root := t.TempDir()
	return settings.NewStore(t.TempDir(), root), root
}

// writeSettingsFile writes content to the store's settings file, creating parents.
func writeSettingsFile(t *testing.T, s *settings.Store, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(s.Path()), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(s.Path(), []byte(content), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}
}

// readSettingsFile returns the settings file's content.
func readSettingsFile(t *testing.T, s *settings.Store) string {
	t.Helper()
	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	return string(data)
}

// mustLoad loads settings and fails the test on any error.
func mustLoad(t *testing.T, s *settings.Store) (domain.AnalysisSettings, []domain.Finding) {
	t.Helper()
	got, findings, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return got, findings
}

// assertBuiltInDefaults checks concrete default values, so a store that returns
// empty settings cannot pass.
func assertBuiltInDefaults(t *testing.T, got domain.AnalysisSettings) {
	t.Helper()
	if got.CompactionDropPercent != 30 {
		t.Errorf("CompactionDropPercent = %d, want 30", got.CompactionDropPercent)
	}
	writes := map[domain.Harness]string{
		domain.HarnessClaudeCode: "Write",
		domain.HarnessGHCPCLI:    "create",
		domain.HarnessOpenCode:   "apply_patch",
		domain.HarnessVSCodeGHCP: "edit/createFile",
	}
	for h, tool := range writes {
		if !got.IsWriteTool(h, tool) {
			t.Errorf("default write tool %q missing for %s", tool, h)
		}
	}
	asks := map[domain.Harness]string{
		domain.HarnessClaudeCode: "AskUserQuestion",
		domain.HarnessGHCPCLI:    "ask_user",
		domain.HarnessOpenCode:   "question",
		domain.HarnessVSCodeGHCP: "vscode/askQuestions",
	}
	for h, tool := range asks {
		if !got.IsUserInteractionTool(h, tool) {
			t.Errorf("default user-interaction tool %q missing for %s", tool, h)
		}
	}
}

// countKind returns how many findings have the given kind.
func countKind(findings []domain.Finding, kind domain.FindingKind) int {
	n := 0
	for _, f := range findings {
		if f.Kind == kind {
			n++
		}
	}
	return n
}

// commentLines returns the file's lines that start with '#'.
func commentLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			out = append(out, line)
		}
	}
	return out
}
