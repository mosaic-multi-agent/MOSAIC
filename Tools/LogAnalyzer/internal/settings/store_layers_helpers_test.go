package settings_test

// Helpers for the two-level (global + project) settings store tests. Every test
// works in temporary folders; none touches the real user config folder.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/settings"
)

// layered bundles a store with the two folders it was built from.
type layered struct {
	store      *settings.Store
	globalFile string
	projFile   string
}

// newLayered returns a store whose global folder and project root are fresh temp
// folders. The global folder is nested so the store must create it itself.
func newLayered(t *testing.T) layered {
	t.Helper()
	globalDir := filepath.Join(t.TempDir(), "MOSAIC", "LogAnalyzer")
	root := t.TempDir()
	return layered{
		store:      settings.NewStore(globalDir, root),
		globalFile: filepath.Join(globalDir, "analysis.yaml"),
		projFile:   filepath.Join(root, "MosaicLogAnalyzer", "config", "analysis.yaml"),
	}
}

// writeAt writes content to path, creating parent folders.
func writeAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readAt returns the content of path.
func readAt(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadLayered loads the merged settings and fails the test on any error.
func loadLayered(t *testing.T, l layered) (domain.AnalysisSettings, []domain.Finding) {
	t.Helper()
	got, findings, err := l.store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return got, findings
}

// findingsAt returns the findings whose Path is path.
func findingsAt(findings []domain.Finding, path string) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Path == path {
			out = append(out, f)
		}
	}
	return out
}

// nonCommentLines returns the lines of content that are neither blank nor comments.
func nonCommentLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// hasLine reports whether any line of content contains sub.
func hasLine(content, sub string) bool { return strings.Contains(content, sub) }
