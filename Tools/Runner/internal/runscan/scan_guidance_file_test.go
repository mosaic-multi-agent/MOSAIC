package runscan_test

// A run folder that also holds the Runner-owned guidance file is classified and
// described exactly as a folder without it.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/runscan"
)

const guidanceFileContent = "# MOSAIC Runner standing guidance for this run.\n[task_description]\nuse small steps\n\n[constraints]\nno new dependencies\n"

func addGuidanceFile(t *testing.T, folder string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(folder, "RunnerGuidance.txt"), []byte(guidanceFileContent), 0o600); err != nil {
		t.Fatalf("setup: write guidance file: %v", err)
	}
}

func TestScan_FolderWithGuidanceFile_StillResumableWithSameMetadata(t *testing.T) {
	rootDir := t.TempDir()
	plain := newTestRunFolder(t, rootDir, runID1)
	withFile := newTestRunFolder(t, rootDir, runID2)
	writeArtifactWithMeta(t, plain, "PLANNING", "linear", "the task", t1)
	writeArtifactWithMeta(t, withFile, "PLANNING", "linear", "the task", t1)
	addGuidanceFile(t, withFile)

	result, err := runscan.NewDirScanner().Scan(rootDir)

	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}
	if len(result.Candidates) != 2 || len(result.Unresumable) != 0 {
		t.Fatalf("want 2 candidates and no unresumable run, got %d and %d", len(result.Candidates), len(result.Unresumable))
	}
	for _, c := range result.Candidates {
		if c.ParseError != nil || c.Workflow != "linear" || c.Task != "the task" || c.Phase != "PLANNING" {
			t.Errorf("want identical metadata for %s, got %+v", c.RunID, c)
		}
	}
}

func TestScan_CompletedRunWithGuidanceFile_StillUnresumableAsCompleted(t *testing.T) {
	rootDir := t.TempDir()
	folder := newTestRunFolder(t, rootDir, runID1)
	writeArtifact(t, folder, "COMPLETED", t1)
	addGuidanceFile(t, folder)

	result, err := runscan.NewDirScanner().Scan(rootDir)

	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}
	if len(result.Candidates) != 0 || result.CompletedCount() != 1 {
		t.Errorf("want no candidate and one completed run, got %d candidates, %d completed", len(result.Candidates), result.CompletedCount())
	}
	if len(result.Unresumable) != 1 || result.Unresumable[0].Reason != runscan.ReasonCompleted {
		t.Errorf("want one unresumable run with reason completed, got %+v", result.Unresumable)
	}
}

func TestScan_FolderWithGuidanceFileButNoArtifact_StillTreatedAsMissingArtifact(t *testing.T) {
	rootDir := t.TempDir()
	folder := newTestRunFolder(t, rootDir, runID1)
	addGuidanceFile(t, folder)

	result, err := runscan.NewDirScanner().Scan(rootDir)

	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].ParseError == nil {
		t.Errorf("want one resumable candidate with a ParseError, got %+v", result.Candidates)
	}
	if len(result.Unresumable) != 0 {
		t.Errorf("want no unresumable run, got %d", len(result.Unresumable))
	}
}
