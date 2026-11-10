package main

// tui_reload_injections_test.go verifies that the TUI catalogue reload rebuilds the harness
// modules from the stack it builds: after a reload the injection content and the orchestrator
// injections version rendered for a harness are those of the reloaded stack's supplying layer,
// not those the launch service was constructed with, and a reloaded stack that supplies no
// injections for the harness stops the render with an error naming the injection item.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack/injections"
)

// newInjectionReloadFixture returns a reload fixture whose default catalog carries default
// marker injections, plus a user folder with its own marker injections; both carry the regions.
func newInjectionReloadFixture(t *testing.T) (reloadFixture, string) {
	t.Helper()
	fx := newReloadFixture(t)
	writeClaudeMarkerInjections(t, fx.defaultDir, defaultInjMarker, defaultOrchVer)
	addInjectionRegions(t, fx.defaultDir, "default-only-agent")
	folder := newStackUserCatalog(t, "reload-user", "2.0.0")
	writeClaudeMarkerInjections(t, folder, userInjMarker, userOrchVer)
	addInjectionRegions(t, folder, "reload-user-agent")
	return fx, folder
}

// renderClaude renders the catalogue agent for claude-code into a temp file and returns the
// file text. Orchestrators are rendered with no workflows.
func renderClaude(t *testing.T, svc app.Service, key string) (string, error) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), key+".md")
	_, err := svc.RenderAgent(context.Background(), app.RenderAgentRequest{
		SourceAgentKey:  key,
		TargetHarnessID: "claude-code",
		DestinationPath: dest,
		Workflows:       []string{},
	})
	if err != nil {
		return "", err
	}
	b, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("read rendered %q: %v", dest, readErr)
	}
	return string(b), nil
}

func mustRender(t *testing.T, svc app.Service, key string) string {
	t.Helper()
	text, err := renderClaude(t, svc, key)
	if err != nil {
		t.Fatalf("render %s: %v", key, err)
	}
	return text
}

func TestReloadCatalog_DifferentFolder_RendersThatFoldersInjections(t *testing.T) {
	fx, folder := newInjectionReloadFixture(t)

	svc, err := fx.reload(folder)

	if err != nil {
		t.Fatalf("reload(%q): %v", folder, err)
	}
	worker := mustRender(t, svc, "reload-user-agent")
	assertContains(t, "reload-user-agent", worker, userInjMarker+"-SHARED")
	assertOmits(t, "reload-user-agent", worker, defaultInjMarker)
}

func TestReloadCatalog_DifferentFolder_OrchestratorInjectionsFollowTheResolvedFiles(t *testing.T) {
	fx, folder := newInjectionReloadFixture(t)

	svc, err := fx.reload(folder)

	if err != nil {
		t.Fatalf("reload(%q): %v", folder, err)
	}
	orch := mustRender(t, svc, "orchestrator")
	assertContains(t, "orchestrator", orch, userInjMarker+"-SHARED", userInjMarker+"-ORCH", userOrchVer)
	assertOmits(t, "orchestrator", orch, defaultInjMarker, defaultOrchVer)
}

func TestReloadCatalog_BackToTheDefaultFolder_RendersTheDefaultInjectionsAgain(t *testing.T) {
	fx, folder := newInjectionReloadFixture(t)
	if _, err := fx.reload(folder); err != nil {
		t.Fatalf("reload(%q): %v", folder, err)
	}

	svc, err := fx.reload(fx.defaultDir)

	if err != nil {
		t.Fatalf("reload(%q): %v", fx.defaultDir, err)
	}
	orch := mustRender(t, svc, "orchestrator")
	assertContains(t, "orchestrator", orch, defaultInjMarker+"-SHARED", defaultInjMarker+"-ORCH", defaultOrchVer)
	assertOmits(t, "orchestrator", orch, userInjMarker, userOrchVer)
}

func TestReloadCatalog_FolderWithoutInjections_RenderStopsNamingTheInjectionItem(t *testing.T) {
	fx, folder := newInjectionReloadFixture(t)
	removeHarnessInjections(t, folder)
	item := catalogpaths.RelHarnessInjectionsDir + "/" + catalogpaths.HarnessDirNameClaudeCode

	svc, err := fx.reload(folder)

	if err != nil {
		t.Fatalf("reload(%q) must succeed so the folder stays selectable: %v", folder, err)
	}
	_, err = renderClaude(t, svc, "reload-user-agent")
	var notSupplied *injections.NotSuppliedError
	if !errors.As(err, &notSupplied) {
		t.Fatalf("render error = %v; want *injections.NotSuppliedError (no fallback to the launch stack or MOSAIC root)", err)
	}
	if !strings.Contains(err.Error(), item) {
		t.Errorf("error %q does not name %s", err.Error(), item)
	}
}
