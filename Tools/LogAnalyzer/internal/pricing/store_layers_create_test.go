package pricing_test

// Tests for EnsureGlobalFile: the global pricing file is written from the
// comment-only template when absent, never overwritten, and prices nothing.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/pricing"
)

func TestEnsureGlobalFile_Absent_WritesTemplateWithComments(t *testing.T) {
	l := newLayered(t)

	created, err := l.store.EnsureGlobalFile()

	if err != nil || !created {
		t.Fatalf("EnsureGlobalFile = (%v, %v), want (true, nil)", created, err)
	}
	content := readFile(t, l.globalFile)
	if !contains(content, "#") || !contains(content, "pricing") {
		t.Errorf("global file should hold the documented template, got:\n%s", content)
	}
}

func TestEnsureGlobalFile_TemplateHasNoActiveEntryAndPricesNothing(t *testing.T) {
	l := newLayered(t)
	if _, err := l.store.EnsureGlobalFile(); err != nil {
		t.Fatalf("EnsureGlobalFile: %v", err)
	}

	active := activeLines(readFile(t, l.globalFile))
	table, findings := loadLayered(t, l)

	if len(active) != 0 {
		t.Errorf("template must hold comments only, active lines: %v", active)
	}
	if table.Len() != 0 || len(findings) != 0 {
		t.Errorf("fresh file: %d entries, findings %v; want none", table.Len(), findings)
	}
}

func TestEnsureGlobalFile_Present_IsNeverOverwritten(t *testing.T) {
	l := newLayered(t)
	custom := modelYAML("my-model", "1.00")
	writeFileAt(t, l.globalFile, custom)

	created, err := l.store.EnsureGlobalFile()

	if err != nil || created {
		t.Fatalf("EnsureGlobalFile = (%v, %v), want (false, nil)", created, err)
	}
	if got := readFile(t, l.globalFile); got != custom {
		t.Errorf("existing global file was changed:\n%s", got)
	}
}

func TestEnsureGlobalFile_CalledTwice_SecondCallCreatesNothing(t *testing.T) {
	l := newLayered(t)

	first, _ := l.store.EnsureGlobalFile()
	second, err := l.store.EnsureGlobalFile()

	if !first || second || err != nil {
		t.Errorf("calls = (%v, %v, %v), want first true, second false, no error", first, second, err)
	}
}

func TestEnsureGlobalFile_CreatesNoProjectFile(t *testing.T) {
	l := newLayered(t)

	if _, err := l.store.EnsureGlobalFile(); err != nil {
		t.Fatalf("EnsureGlobalFile: %v", err)
	}
	loadLayered(t, l)

	if exists(l.projFile) {
		t.Error("the project file is created only by a save")
	}
}

func TestEnsureGlobalFile_NoGlobalFolder_WritesNothingAndReportsNoError(t *testing.T) {
	store := pricing.NewStore("", t.TempDir())

	created, err := store.EnsureGlobalFile()

	if created || err != nil {
		t.Errorf("EnsureGlobalFile = (%v, %v), want (false, nil)", created, err)
	}
	if _, _, err := store.Load(context.Background()); err != nil {
		t.Errorf("Load after unavailable global level: %v", err)
	}
}
