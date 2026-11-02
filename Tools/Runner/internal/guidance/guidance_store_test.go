package guidance_test

// Tests for the file-backed guidance store in a real run folder.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/guidance"
)

func TestFileStore_Load_NoFile_ReportsNotFoundWithoutError(t *testing.T) {
	store := guidance.NewFileStore()

	g, found, err := store.Load(t.TempDir())

	if err != nil {
		t.Fatalf("want nil error when the file does not exist, got %v", err)
	}
	if found || g != (domain.Guidance{}) {
		t.Errorf("want found=false and zero guidance, got found=%v %+v", found, g)
	}
}

func TestFileStore_SaveThenLoad_ReturnsBothFieldsIntact(t *testing.T) {
	dir := t.TempDir()
	store := guidance.NewFileStore()
	want := domain.Guidance{TaskDescription: "line1\n\nline3", Constraints: "[constraints]\nx"}

	if err := store.Save(dir, want); err != nil {
		t.Fatalf("want Save to succeed, got %v", err)
	}
	got, found, err := store.Load(dir)

	if err != nil || !found {
		t.Fatalf("want the saved file found, got found=%v err=%v", found, err)
	}
	if got != want.Normalized() {
		t.Errorf("want %+v, got %+v", want.Normalized(), got)
	}
}

func TestFileStore_Save_EmptyGuidance_IsFoundOnLoad(t *testing.T) {
	dir := t.TempDir()
	store := guidance.NewFileStore()

	if err := store.Save(dir, domain.Guidance{}); err != nil {
		t.Fatalf("want Save to succeed, got %v", err)
	}
	g, found, err := store.Load(dir)

	if err != nil || !found {
		t.Fatalf("want an empty result saved as an existing file, got found=%v err=%v", found, err)
	}
	if g != (domain.Guidance{}) {
		t.Errorf("want zero guidance, got %+v", g)
	}
}

func TestFileStore_Save_WritesRunnerGuidanceFileInRunFolder(t *testing.T) {
	dir := t.TempDir()

	if err := guidance.NewFileStore().Save(dir, domain.Guidance{TaskDescription: "x"}); err != nil {
		t.Fatalf("want Save to succeed, got %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "RunnerGuidance.txt"))
	if err != nil {
		t.Fatalf("want RunnerGuidance.txt in the run folder, got %v", err)
	}
	if guidance.FileName != "RunnerGuidance.txt" {
		t.Errorf("want FileName RunnerGuidance.txt, got %q", guidance.FileName)
	}
	if !strings.Contains(string(data), "x") {
		t.Errorf("want the text in the file, got %q", data)
	}
}

func TestFileStore_Save_ReplacesPreviousContentAndLeavesNoOtherFile(t *testing.T) {
	dir := t.TempDir()
	store := guidance.NewFileStore()
	_ = store.Save(dir, domain.Guidance{TaskDescription: "first"})

	if err := store.Save(dir, domain.Guidance{Constraints: "second"}); err != nil {
		t.Fatalf("want Save to succeed, got %v", err)
	}

	got, _, _ := store.Load(dir)
	if got.TaskDescription != "" || got.Constraints != "second" {
		t.Errorf("want the second save to replace the first, got %+v", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("want only the guidance file in the run folder after saves, got %d entries", len(entries))
	}
}

func TestFileStore_Load_HandEditedFile_IsUsed(t *testing.T) {
	dir := t.TempDir()
	content := "# hand edited\n[constraints]\nno network\n\n[task_description]\nprefer small steps\n"
	if err := os.WriteFile(filepath.Join(dir, guidance.FileName), []byte(content), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	g, found, err := guidance.NewFileStore().Load(dir)

	if err != nil || !found {
		t.Fatalf("want the hand-edited file found, got found=%v err=%v", found, err)
	}
	if g.TaskDescription != "prefer small steps" || g.Constraints != "no network" {
		t.Errorf("want the hand-edited text, got %+v", g)
	}
}

func TestFileStore_Load_MalformedFile_ErrorNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, guidance.FileName), []byte("stray\n[task_description]\nx\n"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, _, err := guidance.NewFileStore().Load(dir)

	if err == nil {
		t.Fatal("want an error for a malformed guidance file, got nil")
	}
	if !strings.Contains(err.Error(), guidance.FileName) {
		t.Errorf("want the error to name %s, got %v", guidance.FileName, err)
	}
}

func TestFileStore_Save_MissingRunFolder_ErrorNamesTheFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no", "such", "folder")

	err := guidance.NewFileStore().Save(missing, domain.Guidance{TaskDescription: "x"})

	if err == nil {
		t.Fatal("want an error when the run folder does not exist, got nil")
	}
	if !strings.Contains(err.Error(), guidance.FileName) {
		t.Errorf("want the error to name %s, got %v", guidance.FileName, err)
	}
}
