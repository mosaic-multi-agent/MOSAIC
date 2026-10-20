package main

// tui_reload_stack_test.go verifies the TUI stack reload: a confirmed stack description rebuilds
// the service from exactly that stack (a user folder alone has no default-catalogue layer, the
// default description gives the default layer alone), and a stack that fails validation returns
// the stack error with no service. The fixture's reload takes a folder for brevity and maps it
// to the description the TUI would pass.
//
// The layers of the reloaded service are observed through CheckWorkflowIndex, which reports
// one entry per stack layer. A catalogue merged through catalog.Load reports a single entry
// with no catalogue identity, so the identity checks below separate a stack from a merge.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// reloadFixture is a MOSAIC root with the default catalogue plus the reload function built
// from deps that were launched on the default catalogue.
type reloadFixture struct {
	mosaicRoot string
	defaultDir string
	reload     func(folder string) (app.Service, error)
}

func newReloadFixture(t *testing.T) reloadFixture {
	t.Helper()
	mosaicRoot := newStackMosaicRoot(t)
	defaultDir := filepath.Join(mosaicRoot, "Catalog")
	deps := buildTestDeps(t, mosaicRoot, defaultDir)
	deps.Interaction = interactiontest.NewBuilder().Build() // auto-confirms the dry-run plan review
	reload := buildReloadFunc(mosaicRoot, deps)
	return reloadFixture{mosaicRoot: mosaicRoot, defaultDir: defaultDir, reload: func(folder string) (app.Service, error) {
		if filepath.Clean(folder) == filepath.Clean(defaultDir) {
			return reload(stackconfig.Description{IncludeDefault: true}, stackconfig.OriginDefault)
		}
		return reload(stackconfig.Description{UserLayers: []string{absOf(t, folder)}}, stackconfig.OriginStackInput)
	}}
}

// indexLayers returns the per-catalogue entries the reloaded service reports.
func indexLayers(t *testing.T, svc app.Service) []app.IndexCheckCatalog {
	t.Helper()
	res, err := svc.CheckWorkflowIndex(context.Background())
	if err != nil {
		t.Fatalf("CheckWorkflowIndex: %v", err)
	}
	return res.Catalogs
}

func TestReloadCatalog_DifferentFolder_ServiceHoldsThatFolderAlone(t *testing.T) {
	fx := newReloadFixture(t)
	folder := newStackUserCatalog(t, "reload-user", "2.0.0")

	svc, err := fx.reload(folder)

	if err != nil {
		t.Fatalf("reload(%q): %v", folder, err)
	}
	layers := indexLayers(t, svc)
	if len(layers) != 1 {
		t.Fatalf("reloaded service has %d catalogue layers (%+v), want exactly the chosen folder", len(layers), layers)
	}
	if layers[0].Catalog.ID != "reload-user" {
		t.Errorf("layer catalogue id = %q, want %q (a stack layer carrying the folder's identity)",
			layers[0].Catalog.ID, "reload-user")
	}
	if filepath.Clean(layers[0].CatalogRoot) != filepath.Clean(absOf(t, folder)) {
		t.Errorf("layer root = %q, want %q", layers[0].CatalogRoot, absOf(t, folder))
	}
}

// agentResolves reports whether the service can render the catalogue agent key (dry run).
func agentResolves(t *testing.T, svc app.Service, key string) bool {
	t.Helper()
	_, err := svc.RenderAgent(context.Background(), app.RenderAgentRequest{
		SourceAgentKey:  key,
		TargetHarnessID: "claude-code",
		DestinationPath: filepath.Join(t.TempDir(), key+".md"),
		DryRun:          true,
	})
	return err == nil
}

// hookResolves reports whether a dry-run hook deployment of the catalogue hook id plans an
// action for that hook. An unknown hook id is skipped silently by planning (no error), so the
// run summary's actions are the oracle, not the error.
func hookResolves(t *testing.T, svc app.Service, id string) bool {
	t.Helper()
	sum, err := svc.DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID:       "claude-code",
		WorkspacePath:   t.TempDir(),
		HookIDs:         []string{id},
		AutoConfirmPlan: true,
		DryRun:          true,
	})
	if err != nil {
		return false
	}
	for _, a := range sum.Actions {
		if strings.Contains(a.Ref.String(), id) {
			return true
		}
	}
	return false
}

func TestReloadCatalog_DifferentFolder_HoldsOnlyThatFoldersContent(t *testing.T) {
	fx := newReloadFixture(t)
	folder := newStackUserCatalog(t, "reload-user", "2.0.0")

	svc, err := fx.reload(folder)

	if err != nil {
		t.Fatalf("reload(%q): %v", folder, err)
	}
	if !agentResolves(t, svc, "reload-user-agent") {
		t.Error("the chosen folder's own agent does not resolve in the reloaded service")
	}
	if agentResolves(t, svc, "default-only-agent") {
		t.Error("default-only-agent resolves although the default layer must not be part of the reloaded service")
	}
	if hookResolves(t, svc, "default-hook") {
		t.Error("default-hook resolves; default hooks must not be merged into the chosen folder's catalogue")
	}
}

func TestReloadCatalog_UnchangedDefaultFolder_HoldsTheDefaultContent(t *testing.T) {
	fx := newReloadFixture(t)

	svc, err := fx.reload(fx.defaultDir)

	if err != nil {
		t.Fatalf("reload(%q): %v", fx.defaultDir, err)
	}
	if !agentResolves(t, svc, "default-only-agent") {
		t.Error("default-only-agent does not resolve in the reloaded default catalogue")
	}
	if !hookResolves(t, svc, "default-hook") {
		t.Error("default-hook does not resolve in the reloaded default catalogue")
	}
}

func TestReloadCatalog_UnchangedDefaultFolder_GivesTheDefaultLayerAlone(t *testing.T) {
	fx := newReloadFixture(t)

	svc, err := fx.reload(fx.defaultDir)

	if err != nil {
		t.Fatalf("reload(%q): %v", fx.defaultDir, err)
	}
	layers := indexLayers(t, svc)
	if len(layers) != 1 {
		t.Fatalf("reloaded service has %d catalogue layers (%+v), want the default layer alone", len(layers), layers)
	}
	if layers[0].Catalog.ID != "stack-default" {
		t.Errorf("layer catalogue id = %q, want the default catalogue identity %q", layers[0].Catalog.ID, "stack-default")
	}
}

func TestReloadCatalog_StackErrors_ReturnTheStackErrorAndNoService(t *testing.T) {
	cases := []struct {
		name   string
		folder func(t *testing.T) string
		want   error
	}{
		{"folder without identity", func(t *testing.T) string { return t.TempDir() }, stackerr.ErrLayerIdentity},
		{"folder that does not exist", func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "no-such-catalog")
		}, stackerr.ErrLayerLoad},
		{"folder with half an orchestrator pair", func(t *testing.T) string {
			half := newIdentifiedCatalogRoot(t, "half-pair")
			writeStackOrchestratorFile(t, half, "orchestrator", "2.0.0")
			return half
		}, stackerr.ErrOrchestratorPairIncomplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newReloadFixture(t)
			folder := tc.folder(t)

			svc, err := fx.reload(folder)

			if !errors.Is(err, tc.want) {
				t.Errorf("reload(%q) error = %v, want a stack error matching %v", folder, err, tc.want)
			}
			if svc != nil {
				t.Errorf("reload(%q) returned a service alongside the error; want nil", folder)
			}
		})
	}
}

func TestReloadCatalog_TwoLayerStackErrors_ReturnTheStackErrorAndNoService(t *testing.T) {
	cases := []struct {
		name  string
		layer func(t *testing.T) (a, b string)
		want  error
	}{
		{"duplicate catalogue id", func(t *testing.T) (string, string) {
			return newStackUserCatalog(t, "same-id", "2.0.0"), newStackUserCatalog(t, "same-id", "3.0.0")
		}, stackerr.ErrDuplicateCatalogID},
		{"numeric agent id used by two agents", func(t *testing.T) (string, string) {
			a := newIdentifiedCatalogRoot(t, "owner-a")
			b := newIdentifiedCatalogRoot(t, "owner-b")
			writeStackAgent(t, a, "agent-one", "clash-001", "1.0.0")
			writeStackAgent(t, b, "agent-two", "clash-001", "1.0.0")
			return a, b
		}, stackerr.ErrNumericIDCollision},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newOriginFixture(t)
			a, b := tc.layer(t)

			svc, err := fx.reload(stackconfig.Description{UserLayers: []string{absOf(t, a), absOf(t, b)}}, stackconfig.OriginStackInput)

			if !errors.Is(err, tc.want) {
				t.Errorf("reload error = %v, want a stack error matching %v", err, tc.want)
			}
			if svc != nil {
				t.Error("reload returned a service alongside the error; want nil")
			}
		})
	}
}
