package catalog_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
)

func TestLoadSingleRoot_DoesNotMergeDefaultCatalogSkillsAndHooks(t *testing.T) {
	mosaicRoot := t.TempDir()
	writeSkillToDefaultCatalogue(t, mosaicRoot, "default-skill")
	writeHookBundleToDefaultCatalogue(t, mosaicRoot, "default-hook")
	custom := makeDistinctCatalogRoot(t)
	writeSkillToCustomCatalogueRoot(t, custom, "custom-skill")
	writeHookBundleToCustomCatalogueRoot(t, custom, "custom-hook")

	cat, err := catalog.LoadSingleRoot(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("LoadSingleRoot: %v", err)
	}

	if _, ok := cat.Skill("default-skill"); ok {
		t.Error("default catalog skill was merged into a single-root load")
	}
	if _, ok := cat.Hook("default-hook"); ok {
		t.Error("default catalog hook was merged into a single-root load")
	}
	if _, ok := cat.Skill("custom-skill"); !ok {
		t.Error("custom skill missing")
	}
	if _, ok := cat.Hook("custom-hook"); !ok {
		t.Error("custom hook missing")
	}
	if cat.CatalogRoot() != custom {
		t.Errorf("CatalogRoot() = %q, want %q", cat.CatalogRoot(), custom)
	}
}

func TestLoadSingleRoot_EmptyCatalogRoot_ReturnsErrorWithoutDefaulting(t *testing.T) {
	mosaicRoot := t.TempDir()
	writeSkillToDefaultCatalogue(t, mosaicRoot, "default-skill")

	cat, err := catalog.LoadSingleRoot(mosaicRoot, "")

	if cat != nil || err == nil {
		t.Fatalf("LoadSingleRoot = %v, %v; want nil and an error", cat, err)
	}
	if !strings.Contains(err.Error(), "catalog root is empty") {
		t.Errorf("error %q does not contain %q", err, "catalog root is empty")
	}
}

func TestLoadSingleRoot_UninterpretableSource_ReturnsErrCatalogSourceInvalid(t *testing.T) {
	mosaicRoot := t.TempDir()
	custom := makeDistinctCatalogRoot(t)
	mustMkdir(t, custom, "Subagents", "Cat")
	mustWriteFile(t, custom, filepath.Join("Subagents", "Cat", "broken.md"), []byte("No frontmatter.\n"))

	cat, err := catalog.LoadSingleRoot(mosaicRoot, custom)

	if cat != nil || !errors.Is(err, catalog.ErrCatalogSourceInvalid) {
		t.Fatalf("LoadSingleRoot = %v, %v; want nil and ErrCatalogSourceInvalid", cat, err)
	}
	if !strings.Contains(err.Error(), "broken.md") {
		t.Errorf("error %q does not name the offending file", err)
	}
}
