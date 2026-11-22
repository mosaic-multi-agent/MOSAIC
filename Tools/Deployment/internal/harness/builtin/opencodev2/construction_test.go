package opencodev2_test

// Tests for how the OpenCode V2 module is constructed: from the embedded descriptor, with
// its injection content taken only from the directory the registry resolver returns, and
// with its injection versions taken from the injection files.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/harness/builtin/opencodev2"
	"mosaic-deploy/internal/harness/registry"
)

// writeInjectionFiles writes both injection files with the given frontmatter versions into
// <root>/Catalog/HarnessInjections/OpenCode V2.
func writeInjectionFiles(t *testing.T, root, sharedVersion, orchVersion string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(catalogpaths.MosaicRelHarnessInjectionsDir), catalogpaths.HarnessDirNameOpenCodeV2)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := func(version string) []byte {
		return []byte("---\nversion: \"" + version + "\"\nharness: opencode-v2\n---\n\n# Injections\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "HarnessInjections.md"), doc(sharedVersion), 0o644); err != nil {
		t.Fatalf("write shared file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "HarnessInjectionsOrchestrator.md"), doc(orchVersion), 0o644); err != nil {
		t.Fatalf("write orchestrator file: %v", err)
	}
}

func TestNew_BuildsTheV2ModuleFromTheEmbeddedDescriptor(t *testing.T) {
	// Arrange / Act
	mod := newModule(t)

	// Assert
	d := mod.Descriptor()
	if d == nil {
		t.Fatal("Descriptor() returned nil")
	}
	if d.ID != "opencode-v2" {
		t.Errorf("descriptor ID = %q, want %q", d.ID, "opencode-v2")
	}
	if got := mod.Ref().ID; got != "opencode-v2" {
		t.Errorf("Ref().ID = %q, want %q", got, "opencode-v2")
	}
	if got := mod.Ref().DisplayName; got != "OpenCode V2" {
		t.Errorf("Ref().DisplayName = %q, want %q", got, "OpenCode V2")
	}
}

func TestHarnessDirName_IsTheOpenCodeV2DisplayName(t *testing.T) {
	if catalogpaths.HarnessDirNameOpenCodeV2 != "OpenCode V2" {
		t.Errorf("HarnessDirNameOpenCodeV2 = %q, want %q", catalogpaths.HarnessDirNameOpenCodeV2, "OpenCode V2")
	}
	if want := catalogpaths.MosaicRelHarnessInjectionsDir + "/OpenCode V2"; opencodev2.RepoContentDir != want {
		t.Errorf("RepoContentDir = %q, want %q", opencodev2.RepoContentDir, want)
	}
}

func TestNew_AsksTheResolverForTheOpenCodeV2DirectoryAndLoadsItsVersions(t *testing.T) {
	root := t.TempDir()
	writeInjectionFiles(t, root, "3.1.0", "4.2.0")
	base := contentDirsUnder(root)
	var asked []string
	resolver := func(harnessDirName string) (string, error) {
		asked = append(asked, harnessDirName)
		return base(harnessDirName)
	}

	mod, err := opencodev2.New(registry.BuiltinOptions{ContentDir: resolver})
	if err != nil {
		t.Fatalf("opencodev2.New: %v", err)
	}

	if len(asked) != 1 || asked[0] != catalogpaths.HarnessDirNameOpenCodeV2 {
		t.Errorf("resolver asked for %v; want exactly [%q]", asked, catalogpaths.HarnessDirNameOpenCodeV2)
	}
	d := mod.Descriptor()
	if d.InjectionsVersion != "3.1.0" {
		t.Errorf("InjectionsVersion = %q, want %q (HarnessInjections.md frontmatter)", d.InjectionsVersion, "3.1.0")
	}
	if d.OrchestratorInjectionsVersion != "4.2.0" {
		t.Errorf("OrchestratorInjectionsVersion = %q, want %q (HarnessInjectionsOrchestrator.md frontmatter)",
			d.OrchestratorInjectionsVersion, "4.2.0")
	}
}

func TestNew_ContentDirIgnoresMosaicRoot(t *testing.T) {
	root := t.TempDir()
	writeInjectionFiles(t, root, "1.0.0", "1.0.0")

	mod, err := opencodev2.New(registry.BuiltinOptions{MosaicRoot: t.TempDir(), ContentDir: contentDirsUnder(root)})

	if err != nil || mod == nil {
		t.Fatalf("opencodev2.New with an empty MosaicRoot and a resolver = (%v, %v); want a module", mod, err)
	}
}

func TestNew_MissingResolver_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeInjectionFiles(t, root, "1.0.0", "1.0.0")

	if _, err := opencodev2.New(registry.BuiltinOptions{MosaicRoot: root}); err == nil {
		t.Fatal("opencodev2.New without ContentDir returned nil error; content must come only through the resolver")
	}
}

func TestNew_ResolverError_IsReturned(t *testing.T) {
	sentinel := errors.New("no such harness directory")

	_, err := opencodev2.New(registry.BuiltinOptions{ContentDir: func(string) (string, error) { return "", sentinel }})

	if !errors.Is(err, sentinel) {
		t.Fatalf("opencodev2.New error = %v; want it to wrap the resolver error", err)
	}
}

func TestNew_ResolvedDirectoryWithoutInjectionFiles_ReturnsError(t *testing.T) {
	_, err := opencodev2.New(registry.BuiltinOptions{ContentDir: contentDirsUnder(t.TempDir())})

	if err == nil {
		t.Fatal("opencodev2.New over an empty content directory returned nil error")
	}
}
