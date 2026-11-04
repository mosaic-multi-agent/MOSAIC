package runscan_test

// Test that an artifact the scanner cannot parse is reported with a refusal
// naming the artifact file and the section that failed.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/runscan"
)

func TestScan_ArtifactWithUnreadableWorkflowNotes_ParseErrorNamesFileAndSection(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "artifact", "execlog_close_tag_after_content.md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	doc := strings.ReplaceAll(string(data), "\r\n", "\n")
	doc = strings.Replace(doc, "</WorkflowNotes>", "", 1)
	rootDir := t.TempDir()
	folder := newTestRunFolder(t, rootDir, runID1)
	artifactPath := filepath.Join(folder, "Orchestration.md")
	if err := os.WriteFile(artifactPath, []byte(doc), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	result, err := runscan.NewDirScanner().Scan(rootDir)

	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want 1", len(result.Candidates))
	}
	var re *domain.RefusalError
	if !errors.As(result.Candidates[0].ParseError, &re) {
		t.Fatalf("ParseError = %v, want a *domain.RefusalError", result.Candidates[0].ParseError)
	}
	if re.Resource != artifactPath {
		t.Errorf("Resource = %q, want %q", re.Resource, artifactPath)
	}
	if re.Section != "WorkflowNotes" {
		t.Errorf("Section = %q, want WorkflowNotes", re.Section)
	}
}
