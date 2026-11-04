package plan_test

// unknown_hook_test.go verifies that a hook id no searched catalog provides is reported as an
// error naming the id and the searched catalogs, the same way unknown workflow and agent ids
// are, while a known hook is still resolved.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

func TestResolveArtifactsFrom_UnknownHook_ReturnsUnknownHookErrorNamingIdAndEverySearchedCatalog(t *testing.T) {
	cat := layeredCatalogWithOrchestrator(
		testLayer("team-extras", "/fake/team", false),
		testLayer("mosaic-default", "/fake/mosaic/Catalog", true),
	)

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{HookIDs: []string{"no-such-hook"}})

	if !errors.Is(err, plan.ErrUnknownHook) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownHook)", err)
	}
	var unknown *plan.UnknownHookError
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %T, want *plan.UnknownHookError", err)
	}
	if !strings.Contains(err.Error(), "no-such-hook") {
		t.Errorf("error %q does not name the hook id", err.Error())
	}
	for _, l := range cat.layers {
		if !strings.Contains(err.Error(), l.Identity.ID) {
			t.Errorf("error %q does not name the searched catalog %q", err.Error(), l.Identity.ID)
		}
	}
}

func TestResolveArtifactsFrom_UnknownHookAmongKnownOnes_StillFails(t *testing.T) {
	cat := &fakeCatalog{
		root:         "/fake/plain-catalogue",
		orchestrator: makeOrchestrator(),
		hooks:        []domain.HookBundle{{Key: "git-hooks", Version: "1.0"}},
	}

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{HookIDs: []string{"git-hooks", "no-such-hook"}})

	if !errors.Is(err, plan.ErrUnknownHook) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownHook)", err)
	}
	if !strings.Contains(err.Error(), "no-such-hook") || !strings.Contains(err.Error(), "/fake/plain-catalogue") {
		t.Errorf("error %q must name the unknown id and the catalog root", err.Error())
	}
}

func TestResolveArtifactsFrom_KnownHook_ResolvesWithoutError(t *testing.T) {
	cat := &fakeCatalog{
		root:         "/fake/plain-catalogue",
		orchestrator: makeOrchestrator(),
		hooks:        []domain.HookBundle{{Key: "git-hooks", Version: "1.0"}},
	}

	set, err := plan.ResolveArtifactsFrom(cat, plan.Selection{HookIDs: []string{"git-hooks"}})

	if err != nil {
		t.Fatalf("ResolveArtifactsFrom: %v", err)
	}
	if len(set.Hooks) != 1 || set.Hooks[0].Key != "git-hooks" {
		t.Errorf("hooks = %v, want exactly git-hooks", set.Hooks)
	}
}
