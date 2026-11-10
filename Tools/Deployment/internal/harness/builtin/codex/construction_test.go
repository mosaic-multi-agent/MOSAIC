package codex_test

// Tests for how the Codex module is constructed: its content directory comes only from
// the registry resolver, and its injection versions come from the injection files.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/harness/builtin/codex"
	"mosaic-deploy/internal/harness/registry"
)

// Frontmatter versions of the module-local injection fixtures under testdata/. They differ
// from each other and from any descriptor value so a mix-up between the two is detectable.
const (
	codexLocalSharedVersion       = "1.1.0"
	codexLocalOrchestratorVersion = "2.3.0"
)

// TestNew_ContentDirResolvedByRegistryResolver verifies that New asks the resolver for the
// Codex harness directory name, and loads the content from the directory it returns.
func TestNew_ContentDirResolvedByRegistryResolver(t *testing.T) {
	base := contentDirsUnder(localFixturesRoot(t))
	var asked []string
	resolver := func(harnessDirName string) (string, error) {
		asked = append(asked, harnessDirName)
		return base(harnessDirName)
	}

	mod, err := codex.New(registry.BuiltinOptions{ContentDir: resolver})
	if err != nil {
		t.Fatalf("codex.New: %v", err)
	}

	if len(asked) != 1 || asked[0] != catalogpaths.HarnessDirNameCodex {
		t.Errorf("resolver asked for %v; want exactly [%q]", asked, catalogpaths.HarnessDirNameCodex)
	}
	if got := mod.Descriptor().InjectionsVersion; got != codexLocalSharedVersion {
		t.Errorf("content was not loaded from the resolved directory: InjectionsVersion = %q, want %q",
			got, codexLocalSharedVersion)
	}
}

// TestNew_ContentDirIgnoresMosaicRoot verifies that MosaicRoot is not used to find content:
// a MosaicRoot without any Codex content does not matter when the resolver is present.
func TestNew_ContentDirIgnoresMosaicRoot(t *testing.T) {
	mod, err := codex.New(registry.BuiltinOptions{
		MosaicRoot: t.TempDir(),
		ContentDir: contentDirsUnder(localFixturesRoot(t)),
	})
	if err != nil {
		t.Fatalf("codex.New with an empty MosaicRoot and a resolver: %v", err)
	}
	if mod == nil {
		t.Fatal("codex.New returned a nil module without an error")
	}
}

// TestNew_MissingResolver_ReturnsError verifies that construction fails without a content
// directory resolver, even when MosaicRoot points at a tree that holds Codex content.
func TestNew_MissingResolver_ReturnsError(t *testing.T) {
	_, err := codex.New(registry.BuiltinOptions{MosaicRoot: localFixturesRoot(t)})
	if err == nil {
		t.Fatal("codex.New without ContentDir returned nil error; " +
			"content must be located only through the registry resolver")
	}
}

// TestNew_ResolverError_IsReturned verifies that a resolver failure fails construction and
// stays reachable through the returned error chain.
func TestNew_ResolverError_IsReturned(t *testing.T) {
	sentinel := errors.New("no such harness directory")
	resolver := func(string) (string, error) { return "", sentinel }

	_, err := codex.New(registry.BuiltinOptions{ContentDir: resolver})

	if !errors.Is(err, sentinel) {
		t.Fatalf("codex.New error = %v; want it to wrap the resolver error", err)
	}
}

// TestNew_UnloadableContent_ReturnsError verifies that a resolved directory without the
// injection files fails construction.
func TestNew_UnloadableContent_ReturnsError(t *testing.T) {
	_, err := codex.New(registry.BuiltinOptions{ContentDir: contentDirsUnder(t.TempDir())})
	if err == nil {
		t.Fatal("codex.New over an empty content directory returned nil error")
	}
}

// TestDescriptor_InjectionsVersions_ComeFromInjectionFiles verifies that the descriptor's
// shared and orchestrator injection versions equal the injection files' frontmatter versions.
// The shared half discriminates the new behavior (the descriptor yaml declares 1.0.0); the
// orchestrator half is regression coverage of behavior that already existed.
func TestDescriptor_InjectionsVersions_ComeFromInjectionFiles(t *testing.T) {
	mod := newModuleWithLocalFixtures(t)

	d := mod.Descriptor()

	if d.InjectionsVersion != codexLocalSharedVersion {
		t.Errorf("InjectionsVersion = %q, want %q (HarnessInjections.md frontmatter)",
			d.InjectionsVersion, codexLocalSharedVersion)
	}
	if d.OrchestratorInjectionsVersion != codexLocalOrchestratorVersion {
		t.Errorf("OrchestratorInjectionsVersion = %q, want %q (HarnessInjectionsOrchestrator.md frontmatter)",
			d.OrchestratorInjectionsVersion, codexLocalOrchestratorVersion)
	}
}
