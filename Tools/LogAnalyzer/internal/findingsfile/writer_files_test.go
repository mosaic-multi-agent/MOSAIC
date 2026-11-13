package findingsfile_test

// File handling of the findings writer: overwrite protection, parent
// directories, no partial files, cancellation.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/findingsfile"
)

const previousContent = "previous content\n"

func existingFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(path, []byte(previousContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteFindings_ExistingFileWithoutOverwriteIsRefusedAndUntouched(t *testing.T) {
	path := existingFile(t)

	err := findingsfile.NewWriter().WriteFindings(context.Background(), path, mixedFindings(), false)

	if !errors.Is(err, domain.ErrFindingsFileExists) {
		t.Fatalf("err = %v, want ErrFindingsFileExists", err)
	}
	if got, _ := os.ReadFile(path); string(got) != previousContent {
		t.Errorf("the refused write changed the file: %q", got)
	}
}

func TestWriteFindings_ExistingFileWithOverwriteIsReplaced(t *testing.T) {
	path := existingFile(t)

	err := findingsfile.NewWriter().WriteFindings(context.Background(), path, mixedFindings(), true)

	if err != nil {
		t.Fatalf("WriteFindings: %v", err)
	}
	doc := readDoc(t, path)
	if len(doc.Findings) != len(mixedFindings()) {
		t.Errorf("%d entries after overwrite, want %d", len(doc.Findings), len(mixedFindings()))
	}
}

func TestWriteFindings_CreatesMissingParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", "deeper", "findings.json")

	err := findingsfile.NewWriter().WriteFindings(context.Background(), path, mixedFindings(), false)

	if err != nil {
		t.Fatalf("WriteFindings: %v", err)
	}
	if len(readDoc(t, path).Findings) == 0 {
		t.Error("the file in the created directory is empty")
	}
}

func TestWriteFindings_LeavesNoOtherFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "findings.json")
	w := findingsfile.NewWriter()

	_ = w.WriteFindings(context.Background(), path, mixedFindings(), false)
	_ = w.WriteFindings(context.Background(), path, mixedFindings(), false) // refused

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "findings.json" {
		t.Errorf("directory holds %d entries, want only findings.json", len(entries))
	}
	if len(readDoc(t, path).Findings) == 0 {
		t.Error("the findings file was not written")
	}
}

func TestWriteFindings_CancelledContextWritesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "findings.json")

	err := findingsfile.NewWriter().WriteFindings(ctx, path, mixedFindings(), false)

	if err == nil {
		t.Error("a cancelled context must return an error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("a cancelled write left a file behind (stat err = %v)", statErr)
	}
}

func TestWriteFindings_UnwritableTargetReturnsAnErrorOtherThanExists(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := findingsfile.NewWriter().WriteFindings(context.Background(), filepath.Join(blocker, "findings.json"), mixedFindings(), false)

	if err == nil {
		t.Fatal("a path below a regular file cannot be written; want an error")
	}
	if errors.Is(err, domain.ErrFindingsFileExists) {
		t.Errorf("err = %v; an I/O failure is not the exists error", err)
	}
}
