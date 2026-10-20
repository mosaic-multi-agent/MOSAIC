package builtin_test

// model_source_test.go tests that every builtin harness known to the shared catalog (the
// CLI-backed harnesses and vscode-ghcp), when discovered through the registry with the
// ModelCatalog hook, offers the shared catalog's model ids and format hint.
//
// vscode-ghcp no longer declares a models: block in its descriptor, so with a hook miss it
// offers no models at all. The test also verifies that a descriptor-only harness unknown to
// the shared catalog keeps its own descriptor-sourced model catalog.
//
// The ModelCatalog hook is the seam that lets the registry overlay the shared catalog's data
// onto a module's Descriptor().Models without any package importing builtin harness data
// directly. The hook receives "no shared entry" (empty IDs) for any harness the shared
// catalog does not know, so such modules fall back to their own descriptor-sourced catalog.

import (
	"os"
	"path/filepath"
	"testing"

	// Side-effect imports ensure the four builtin harness init() functions run, registering
	// their factories with the package-level registry. Discover then constructs and hooks them.
	_ "mosaic-deploy/internal/harness/builtin/claudecode"
	_ "mosaic-deploy/internal/harness/builtin/ghcpcli"
	_ "mosaic-deploy/internal/harness/builtin/opencode"
	_ "mosaic-deploy/internal/harness/builtin/vscodeghcp"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// ---------------------------------------------------------------------------
// Shared test catalog — a per-test substitute for the shared mosaic-common/harness
// catalog. Using concrete test IDs (rather than importing the real common module)
// keeps this module's go.mod unchanged and tests the hook mechanism independently
// of the common module's own content.
// ---------------------------------------------------------------------------

// testSharedModelCatalog is the model data the hook installs for the three CLI-backed
// harnesses and vscode-ghcp. It uses deliberately invented IDs (prefixed "shared-") that
// appear in no real YAML descriptor, so the presence of any of them in a module's
// Models.IDs proves the hook was applied — not that the YAML happened to contain the same IDs.
var testSharedModelCatalog = map[string]domain.ModelCatalog{
	"claude-code": {
		IDs:        []string{"shared-claude-model-1", "shared-claude-model-2"},
		FormatHint: "shared-claude-<model>-<version>",
	},
	"opencode": {
		IDs:        []string{"shared-opencode-provider/shared-model"},
		FormatHint: "shared-provider/model-id",
	},
	"ghcp-cli": {
		IDs:        []string{"shared-ghcp-model-a", "shared-ghcp-model-b"},
		FormatHint: "shared-ghcp-<model>-<version>",
	},
	"vscode-ghcp": {
		IDs:        []string{"Shared VsCode Model One", "Shared VsCode Model Two"},
		FormatHint: "shared-vscode-display-name",
	},
}

// testModelCatalogHook returns a ModelCatalog hook function backed by testSharedModelCatalog.
// It returns the catalog for every harness the shared catalog knows (CLI-backed harnesses
// and vscode-ghcp) and an empty catalog for all others, mirroring what the composition
// root does with mosaic-common/harness.LookupModelCatalog.
func testModelCatalogHook() func(string, domain.ModelCatalog) domain.ModelCatalog {
	return func(harnessID string, declared domain.ModelCatalog) domain.ModelCatalog {
		if cat, ok := testSharedModelCatalog[harnessID]; ok {
			return cat
		}
		return domain.ModelCatalog{} // empty IDs → keep descriptor's own catalog
	}
}

// discoverBuiltinsWithModelHook calls registry.Discover with the frozen fixture root (so builtin
// factories load their content files from its Catalog/HarnessInjections/) and the test
// ModelCatalog hook. The four builtin harnesses must have registered themselves via init()
// through the side-effect imports above before this function is called.
func discoverBuiltinsWithModelHook(t *testing.T) registry.Registry {
	t.Helper()
	root := deploymentTestdata(t, "frozen-catalog")
	reg, err := registry.Discover(registry.Options{
		MosaicRoot:   root,
		ModelCatalog: testModelCatalogHook(), // field does not exist → compile error (TDD RED)
	})
	if err != nil {
		t.Fatalf("Discover with ModelCatalog hook: %v", err)
	}
	return reg
}

// modelSlicesEqual reports whether a and b contain exactly the same elements in the same
// order. It is used to verify that model ID lists match the hook's expected output exactly.
func modelSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// T9.1 — CLI-backed harnesses offer shared catalog's model ids and format hint
// ---------------------------------------------------------------------------

// TestModelSource_ClaudeCode_OffersSharedCatalogIDs verifies that the claude-code module,
// when discovered with the ModelCatalog hook, offers exactly the shared catalog's model ids
// rather than whatever ids are declared in its own YAML models: block.
func TestModelSource_ClaudeCode_OffersSharedCatalogIDs(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("claude-code")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "claude-code", err)
	}

	want := testSharedModelCatalog["claude-code"]
	got := m.Descriptor().Models

	if !modelSlicesEqual(got.IDs, want.IDs) {
		t.Errorf("claude-code: Descriptor().Models.IDs = %v; want %v (from shared catalog hook); "+
			"the ModelCatalog hook result must be installed on the module's Descriptor().Models",
			got.IDs, want.IDs)
	}
}

// TestModelSource_ClaudeCode_OffersSharedCatalogFormatHint verifies that the claude-code
// module, when discovered with the ModelCatalog hook, offers the shared catalog's format hint.
func TestModelSource_ClaudeCode_OffersSharedCatalogFormatHint(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("claude-code")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "claude-code", err)
	}

	want := testSharedModelCatalog["claude-code"].FormatHint
	got := m.Descriptor().Models.FormatHint

	if got != want {
		t.Errorf("claude-code: Descriptor().Models.FormatHint = %q; want %q (from shared catalog hook)",
			got, want)
	}
}

// TestModelSource_OpenCode_OffersSharedCatalogIDs verifies that the opencode module, when
// discovered with the ModelCatalog hook, offers exactly the shared catalog's model ids.
func TestModelSource_OpenCode_OffersSharedCatalogIDs(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("opencode")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "opencode", err)
	}

	want := testSharedModelCatalog["opencode"]
	got := m.Descriptor().Models

	if !modelSlicesEqual(got.IDs, want.IDs) {
		t.Errorf("opencode: Descriptor().Models.IDs = %v; want %v (from shared catalog hook)",
			got.IDs, want.IDs)
	}
}

// TestModelSource_OpenCode_OffersSharedCatalogFormatHint verifies that the opencode module,
// when discovered with the ModelCatalog hook, offers the shared catalog's format hint.
func TestModelSource_OpenCode_OffersSharedCatalogFormatHint(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("opencode")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "opencode", err)
	}

	want := testSharedModelCatalog["opencode"].FormatHint
	got := m.Descriptor().Models.FormatHint

	if got != want {
		t.Errorf("opencode: Descriptor().Models.FormatHint = %q; want %q (from shared catalog hook)",
			got, want)
	}
}

// TestModelSource_GHCPCLI_OffersSharedCatalogIDs verifies that the ghcp-cli module, when
// discovered with the ModelCatalog hook, offers exactly the shared catalog's model ids.
func TestModelSource_GHCPCLI_OffersSharedCatalogIDs(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("ghcp-cli")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "ghcp-cli", err)
	}

	want := testSharedModelCatalog["ghcp-cli"]
	got := m.Descriptor().Models

	if !modelSlicesEqual(got.IDs, want.IDs) {
		t.Errorf("ghcp-cli: Descriptor().Models.IDs = %v; want %v (from shared catalog hook)",
			got.IDs, want.IDs)
	}
}

// TestModelSource_GHCPCLI_OffersSharedCatalogFormatHint verifies that the ghcp-cli module,
// when discovered with the ModelCatalog hook, offers the shared catalog's format hint.
func TestModelSource_GHCPCLI_OffersSharedCatalogFormatHint(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("ghcp-cli")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "ghcp-cli", err)
	}

	want := testSharedModelCatalog["ghcp-cli"].FormatHint
	got := m.Descriptor().Models.FormatHint

	if got != want {
		t.Errorf("ghcp-cli: Descriptor().Models.FormatHint = %q; want %q (from shared catalog hook)",
			got, want)
	}
}

// ---------------------------------------------------------------------------
// vscodeghcp is hook-sourced like the CLI-backed harnesses
// ---------------------------------------------------------------------------

// TestModelSource_VsCodeGHCP_OffersSharedCatalogIDs verifies that the vscode-ghcp module,
// when discovered with the ModelCatalog hook, offers exactly the shared catalog's model ids.
func TestModelSource_VsCodeGHCP_OffersSharedCatalogIDs(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("vscode-ghcp")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "vscode-ghcp", err)
	}

	want := testSharedModelCatalog["vscode-ghcp"]
	got := m.Descriptor().Models

	if !modelSlicesEqual(got.IDs, want.IDs) {
		t.Errorf("vscode-ghcp: Descriptor().Models.IDs = %v; want %v (from shared catalog hook)",
			got.IDs, want.IDs)
	}
}

// TestModelSource_VsCodeGHCP_OffersSharedCatalogFormatHint verifies that the vscode-ghcp
// module, when discovered with the ModelCatalog hook, offers the shared catalog's format hint.
func TestModelSource_VsCodeGHCP_OffersSharedCatalogFormatHint(t *testing.T) {
	reg := discoverBuiltinsWithModelHook(t)

	m, err := reg.Resolve("vscode-ghcp")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "vscode-ghcp", err)
	}

	want := testSharedModelCatalog["vscode-ghcp"].FormatHint
	got := m.Descriptor().Models.FormatHint

	if got != want {
		t.Errorf("vscode-ghcp: Descriptor().Models.FormatHint = %q; want %q (from shared catalog hook)",
			got, want)
	}
}

// TestModelSource_VsCodeGHCP_HookMissOffersNoDescriptorFallback verifies that when the hook
// returns a miss (empty IDs) for vscode-ghcp, the module offers no models: its embedded
// descriptor no longer declares a models: block, so there is no fallback list to keep.
func TestModelSource_VsCodeGHCP_HookMissOffersNoDescriptorFallback(t *testing.T) {
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: deploymentTestdata(t, "frozen-catalog"),
		ModelCatalog: func(harnessID string, declared domain.ModelCatalog) domain.ModelCatalog {
			return domain.ModelCatalog{} // miss for every harness
		},
	})
	if err != nil {
		t.Fatalf("Discover with always-miss ModelCatalog hook: %v", err)
	}

	m, err := reg.Resolve("vscode-ghcp")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "vscode-ghcp", err)
	}

	got := m.Descriptor().Models
	if len(got.IDs) != 0 {
		t.Errorf("vscode-ghcp: Descriptor().Models.IDs = %v after a hook miss; want empty; "+
			"model data comes from the shared catalog only, the descriptor must not declare a models: block",
			got.IDs)
	}
	if got.FormatHint != "" {
		t.Errorf("vscode-ghcp: Descriptor().Models.FormatHint = %q after a hook miss; want empty",
			got.FormatHint)
	}
}

// ---------------------------------------------------------------------------
// T9.1 — Unknown harness falls back to descriptor-sourced catalog (AC9.5)
// ---------------------------------------------------------------------------

// TestModelSource_DescriptorOnlyHarness_KeepsDescriptorCatalog verifies that a
// descriptor-only on-disk harness with its own models: block keeps that catalog when the
// hook returns empty IDs (the "unknown to shared catalog" case). This satisfies AC9.5: a
// harness the shared catalog does not know must still resolve its model data from its own
// descriptor rather than producing an empty list.
func TestModelSource_DescriptorOnlyHarness_KeepsDescriptorCatalog(t *testing.T) {
	// Use a fresh temp root so we have full control over on-disk harnesses.
	tempRoot := t.TempDir()
	harnessDir := filepath.Join(tempRoot, "MosaicDeploy", "harnesses", "unknown-to-shared-catalog")
	if err := os.MkdirAll(harnessDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", harnessDir, err)
	}
	yaml := "schema_version: \"1\"\nid: \"unknown-to-shared-catalog\"\ndisplay_name: \"Unknown Harness\"\nmodels:\n  ids:\n    - \"descriptor-model-alpha\"\n    - \"descriptor-model-beta\"\n  format_hint: \"descriptor-format-hint\"\n"
	if err := os.WriteFile(filepath.Join(harnessDir, "harness.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("WriteFile harness.yaml: %v", err)
	}

	// The hook returns empty IDs for "unknown-to-shared-catalog" (it is not in the shared catalog).
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: tempRoot,
		// field does not exist → compile error (TDD RED)
		ModelCatalog: func(harnessID string, declared domain.ModelCatalog) domain.ModelCatalog {
			if cat, ok := testSharedModelCatalog[harnessID]; ok {
				return cat
			}
			return domain.ModelCatalog{} // empty IDs → keep descriptor's own catalog
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	m, err := reg.Resolve("unknown-to-shared-catalog")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", "unknown-to-shared-catalog", err)
	}

	got := m.Descriptor().Models
	if !modelSlicesEqual(got.IDs, []string{"descriptor-model-alpha", "descriptor-model-beta"}) {
		t.Errorf("unknown-to-shared-catalog: Descriptor().Models.IDs = %v; "+
			"want [descriptor-model-alpha descriptor-model-beta] from the descriptor; "+
			"a harness unknown to the shared catalog must fall back to its own descriptor-sourced catalog (AC9.5)",
			got.IDs)
	}
	if got.FormatHint != "descriptor-format-hint" {
		t.Errorf("unknown-to-shared-catalog: Descriptor().Models.FormatHint = %q; "+
			"want %q from the descriptor",
			got.FormatHint, "descriptor-format-hint")
	}
}
