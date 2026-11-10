package catalogpaths_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

func TestRelHarnessInjectionsDir_Name(t *testing.T) {
	if catalogpaths.RelHarnessInjectionsDir != "HarnessInjections" {
		t.Fatalf("RelHarnessInjectionsDir = %q", catalogpaths.RelHarnessInjectionsDir)
	}
}

func TestHarnessInjectionsDir_JoinsRootInjectionsDirAndHarnessDir(t *testing.T) {
	root := filepath.Join("some", "catalog")

	got := catalogpaths.HarnessInjectionsDir(root, "Claude Code")

	want := filepath.Join(root, "HarnessInjections", "Claude Code")
	if got != want {
		t.Fatalf("HarnessInjectionsDir = %q, want %q", got, want)
	}
}

func TestHarnessInjectionsDir_CoversEveryBuiltInHarnessDirName(t *testing.T) {
	root := filepath.Join("some", "catalog")
	names := []string{
		catalogpaths.HarnessDirNameClaudeCode,
		catalogpaths.HarnessDirNameGhcpCLI,
		catalogpaths.HarnessDirNameOpenCode,
		catalogpaths.HarnessDirNameVSCodeGHCP,
	}

	for _, name := range names {
		got := catalogpaths.HarnessInjectionsDir(root, name)

		want := filepath.Join(root, "HarnessInjections", name)
		if got != want {
			t.Errorf("HarnessInjectionsDir(%q) = %q, want %q", name, got, want)
		}
	}
}

// The catalog-root-relative layout is the MOSAIC-root-relative layout minus the Catalog/ prefix,
// so the default catalog resolves to the directories the built-in modules read today.
func TestHarnessInjectionsDir_OfDefaultCatalogMatchesMosaicRootContentDirs(t *testing.T) {
	mosaic := filepath.Join("some", "mosaic")
	defaultCatalog := filepath.Join(mosaic, catalogpaths.CatalogDirName)

	got := catalogpaths.HarnessInjectionsDir(defaultCatalog, catalogpaths.HarnessDirNameOpenCode)

	want := filepath.Join(mosaic, filepath.FromSlash(catalogpaths.HarnessContentDirOpenCode))
	if got != want {
		t.Fatalf("HarnessInjectionsDir(default catalog) = %q, want %q", got, want)
	}
}
