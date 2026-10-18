package main

// tui_reload_injection_error_test.go verifies that after a TUI catalogue reload whose winning
// layer supplies an incomplete Claude Code injection directory, the stack-screen check and a
// render both fail with an error naming the layer (id and root), the harness directory and the
// file and saying injections are not merged across layers. The reload itself succeeds so the
// folder stays selectable; the TUI shows err.Error() of these calls.

import (
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/harness/injectionfile"
)

// newIncompleteInjectionFolder returns a user catalog folder whose Claude Code injection
// directory exists but holds no files.
func newIncompleteInjectionFolder(t *testing.T) string {
	t.Helper()
	folder := newStackUserCatalog(t, "reload-user", "2.0.0")
	removeHarnessInjectionDir(t, folder, catalogpaths.HarnessDirNameClaudeCode)
	dir := catalogpaths.HarnessInjectionsDir(folder, catalogpaths.HarnessDirNameClaudeCode)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	return folder
}

func assertAttributedReloadError(t *testing.T, what string, err error, folder string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s returned nil; an incomplete winning injection directory must fail", what)
	}
	text := err.Error()
	for label, want := range map[string]string{
		"layer id":          "reload-user",
		"layer root":        absOf(t, folder),
		"harness directory": catalogpaths.HarnessDirNameClaudeCode,
		"file":              injectionfile.SharedFileName,
		"not-merged note":   "not merged",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%s error %q does not contain the %s %q", what, text, label, want)
		}
	}
}

func TestReloadCatalog_WinningLayerInjectionsIncomplete_StackScreenCheckNamesLayerDirectoryAndFile(t *testing.T) {
	fx := newReloadFixture(t)
	folder := newIncompleteInjectionFolder(t)
	svc, err := fx.reload(folder)
	if err != nil {
		t.Fatalf("reload(%q) must succeed so the folder stays selectable: %v", folder, err)
	}

	checkErr := svc.CheckHarnessInjections("claude-code")

	assertAttributedReloadError(t, "CheckHarnessInjections", checkErr, folder)
}

func TestReloadCatalog_WinningLayerInjectionsIncomplete_RenderStopsWithTheSameContent(t *testing.T) {
	fx := newReloadFixture(t)
	folder := newIncompleteInjectionFolder(t)
	svc, err := fx.reload(folder)
	if err != nil {
		t.Fatalf("reload(%q) must succeed so the folder stays selectable: %v", folder, err)
	}

	_, renderErr := renderClaude(t, svc, "orchestrator")

	assertAttributedReloadError(t, "render", renderErr, folder)
}
