package runscan_test

// Test that an artifact the scanner cannot fully parse is reported as damaged,
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
	// The folder name must match the run_id the fixture records, or the
	// scanner rejects the run for an identity mismatch before parsing sections.
	folder := newTestRunFolder(t, rootDir, "20261030T131902Z-2d56")
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
	var de *domain.ArtifactDamageError
	if !errors.As(result.Candidates[0].ParseError, &de) {
		t.Fatalf("ParseError = %v, want a *domain.ArtifactDamageError", result.Candidates[0].ParseError)
	}
	if de.Resource != artifactPath {
		t.Errorf("Resource = %q, want %q", de.Resource, artifactPath)
	}
	found := false
	for _, p := range de.Problems {
		found = found || p.Section == "WorkflowNotes"
	}
	if !found {
		t.Errorf("Problems = %+v, want one naming the WorkflowNotes section", de.Problems)
	}
}
