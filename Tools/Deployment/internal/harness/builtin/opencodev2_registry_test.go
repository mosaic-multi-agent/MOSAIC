package builtin_test

// opencodev2_registry_test.go specifies how the registry treats opencode-v2 next to the other
// built-in harnesses: it is discovered and resolvable from the frozen test catalog, offers the
// shared catalog's model ids and format hint through the ModelCatalog hook, keeps its own
// descriptor models on a hook miss, and is the only built-in OpenCode target marked
// experimental. Every test uses the frozen test catalog, never the live Catalog/.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

func TestRegistry_ListsSixBuiltinHarnessesFromTheFrozenCatalog(t *testing.T) {
	reg := discoverBuiltinsWithModelHookFromFrozen(t)

	listed := map[string]domain.HarnessRef{}
	for _, r := range reg.List() {
		listed[r.ID] = r
	}

	for _, id := range []string{"claude-code", "opencode", "opencode-v2", "ghcp-cli", "vscode-ghcp", "codex"} {
		r, ok := listed[id]
		if !ok {
			t.Errorf("registry does not list built-in harness %q", id)
			continue
		}
		if !r.Usable {
			t.Errorf("%s is listed as unusable (%q), want usable", id, r.UnusableReason)
		}
	}
}

func TestRegistry_OpenCodeV2IsExperimentalAndV1OpenCodeIsNot(t *testing.T) {
	reg := discoverBuiltinsWithModelHookFromFrozen(t)

	v2, err := reg.Resolve("opencode-v2")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "opencode-v2", err)
	}
	v1, err := reg.Resolve("opencode")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "opencode", err)
	}

	if !v2.Ref().IsExperimental() || v2.Ref().ExperimentalReason == "" {
		t.Errorf("opencode-v2 ref = %+v, want experimental with a reason", v2.Ref())
	}
	if v1.Ref().IsExperimental() {
		t.Errorf("opencode ref IsExperimental() = true (%q), want V1 output and picker entry unchanged", v1.Ref().ExperimentalReason)
	}
}

func TestModelSource_OpenCodeV2_OffersSharedCatalogIDsAndFormatHint(t *testing.T) {
	reg := discoverBuiltinsWithModelHookFromFrozen(t)

	m, err := reg.Resolve("opencode-v2")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "opencode-v2", err)
	}

	want := testSharedModelCatalog["opencode-v2"]
	got := m.Descriptor().Models
	if !modelSlicesEqual(got.IDs, want.IDs) {
		t.Errorf("opencode-v2: Models.IDs = %v; want %v (from the shared catalog hook)", got.IDs, want.IDs)
	}
	if got.FormatHint != want.FormatHint {
		t.Errorf("opencode-v2: Models.FormatHint = %q; want %q (from the shared catalog hook)", got.FormatHint, want.FormatHint)
	}
}

func TestModelSource_OpenCodeV2_HookMissKeepsDescriptorModels(t *testing.T) {
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: frozenCatalogRoot(t),
		ModelCatalog: func(string, domain.ModelCatalog) domain.ModelCatalog {
			return domain.ModelCatalog{} // miss for every harness
		},
	})
	if err != nil {
		t.Fatalf("Discover with an always-miss ModelCatalog hook: %v", err)
	}

	m, err := reg.Resolve("opencode-v2")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "opencode-v2", err)
	}

	got := m.Descriptor().Models
	if len(got.IDs) == 0 || got.FormatHint == "" {
		t.Errorf("opencode-v2: Models = %+v after a hook miss, want the descriptor's own ids and format hint", got)
	}
}
