package registry_test

// rebuild_test.go verifies Rebuilder.Rebuild: the registry it returns is built from the same
// options with the built-in modules constructed again, so the ToolMappings hook runs again and
// reflects configuration that changed since the first registry was built, while the first
// registry keeps what it had.

import (
	"testing"

	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

func rebuildHasMapping(mod domain.HarnessModule, generic string) bool {
	for _, m := range mod.Descriptor().Tools.Mappings {
		if m.Generic == generic {
			return true
		}
	}
	return false
}

func TestRebuild_ToolMappingsHookRunsAgain_NewRegistrySeesTheChangedMappings(t *testing.T) {
	id := uniqueID(t, "rebuild")
	registry.Register(id, func(registry.BuiltinOptions) (domain.HarnessModule, error) {
		return stubModule(id), nil
	})
	var current []domain.ToolMapping
	reg, err := registry.DiscoverForStack(registry.Options{
		MosaicRoot: makeRoot(t),
		ToolMappings: func(harnessID string, _ []domain.ToolMapping) []domain.ToolMapping {
			if harnessID != id {
				return nil
			}
			return current
		},
	}, injections.Result{})
	if err != nil {
		t.Fatalf("DiscoverForStack: %v", err)
	}
	current = []domain.ToolMapping{{
		Generic:      "bash",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"mcp__shell__*"}}},
	}}

	rebuilt, err := reg.(registry.Rebuilder).Rebuild()

	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	newMod, err := rebuilt.Resolve(id)
	if err != nil {
		t.Fatalf("Resolve on the rebuilt registry: %v", err)
	}
	if !rebuildHasMapping(newMod, "bash") {
		t.Error("the rebuilt registry's module does not carry the mapping the hook returns now")
	}
	oldMod, err := reg.Resolve(id)
	if err != nil {
		t.Fatalf("Resolve on the first registry: %v", err)
	}
	if rebuildHasMapping(oldMod, "bash") {
		t.Error("the first registry changed; a registry holder must keep its own mappings")
	}
}

func TestRebuild_RebuiltRegistryCanBeRebuiltAgain(t *testing.T) {
	id := uniqueID(t, "twice")
	registry.Register(id, func(registry.BuiltinOptions) (domain.HarnessModule, error) {
		return stubModule(id), nil
	})
	reg, err := registry.DiscoverForStack(registry.Options{MosaicRoot: makeRoot(t)}, injections.Result{})
	if err != nil {
		t.Fatalf("DiscoverForStack: %v", err)
	}
	first, err := reg.(registry.Rebuilder).Rebuild()
	if err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}

	second, err := first.(registry.Rebuilder).Rebuild()

	if err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	if _, err := second.Resolve(id); err != nil {
		t.Errorf("Resolve on the twice rebuilt registry: %v", err)
	}
}

func TestRebuild_ExternalTierEntriesAreCarriedOver(t *testing.T) {
	root := makeRoot(t)
	id := uniqueID(t, "ext-rebuild")
	addExternal(t, root, id, "External Rebuild")
	reg, err := registry.Discover(registry.Options{MosaicRoot: root, AllowExternal: true})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	rebuilt, err := reg.(registry.Rebuilder).Rebuild()

	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	found := false
	for _, ref := range rebuilt.List() {
		if ref.ID == id && ref.Tier == domain.TierExternal {
			found = true
		}
	}
	if !found {
		t.Errorf("the rebuilt registry does not list the external harness %q: %+v", id, rebuilt.List())
	}
}
