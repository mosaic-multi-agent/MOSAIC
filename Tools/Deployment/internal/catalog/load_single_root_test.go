package catalog_test

// load_single_root_test.go covers catalog.Load with a custom catalogue root: the root is the
// only source of skills and hooks, and the default catalogue under the MOSAIC root
// contributes nothing. Loading the default root itself is unchanged.

import (
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/catalog"
)

func TestLoad_CustomRoot_DefaultCatalogSkillsAbsent(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeSkillToDefaultCatalogue(t, mosaicRoot, "default-skill")
	writeSkillToCustomCatalogueRoot(t, custom, "custom-skill")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cat.Skill("default-skill"); ok {
		t.Error("default catalogue skill appeared in a custom-root load")
	}
	if _, ok := cat.Skill("custom-skill"); !ok {
		t.Error("custom root skill missing")
	}
	if len(cat.Skills()) != 1 {
		t.Errorf("Skills() has %d entries, want exactly the custom root's 1", len(cat.Skills()))
	}
}

func TestLoad_CustomRoot_DefaultCatalogHooksAbsent(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeHookBundleToDefaultCatalogue(t, mosaicRoot, "default-hook")
	writeHookBundleToCustomCatalogueRoot(t, custom, "custom-hook")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cat.Hook("default-hook"); ok {
		t.Error("default catalogue hook appeared in a custom-root load")
	}
	if _, ok := cat.Hook("custom-hook"); !ok {
		t.Error("custom root hook missing")
	}
	if len(cat.Hooks()) != 1 {
		t.Errorf("Hooks() has %d entries, want exactly the custom root's 1", len(cat.Hooks()))
	}
}

func TestLoad_CustomRoot_HookKeepsPerHarnessVariants(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeHookBundleToCustomCatalogueRoot(t, custom, "custom-hook")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	h, ok := cat.Hook("custom-hook")
	if !ok {
		t.Fatal("custom-hook missing")
	}
	v, ok := h.Variants["test-harness"]
	if !ok || !v.Supported || len(v.Files) != 1 {
		t.Errorf("Variants[test-harness] = %+v (present %v), want one supported variant with 1 file", v, ok)
	}
}

func TestLoad_CustomRoot_SameKeyAsDefault_OnlyCustomEntryLoaded(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeSkillToDefaultCatalogue(t, mosaicRoot, "shared-skill")
	writeSkillToCustomCatalogueRoot(t, custom, "shared-skill")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	s, ok := cat.Skill("shared-skill")
	if !ok || s.Name != "shared-skill custom" {
		t.Errorf("Skill(shared-skill) = %+v (present %v), want the custom root's entry", s, ok)
	}
}

func TestLoad_CustomRoot_TamperedHookBundle_ReportsHashMismatch(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeTamperedHookBundleToCustomCatalogueRoot(t, custom, "tampered-custom-bundle")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, found := findIssue(cat.Issues(), "hook-hash-mismatch"); !found {
		t.Error("no hook-hash-mismatch issue for a tampered bundle in the custom root")
	}
}

func TestLoad_CustomRoot_EquivalentToLoadSingleRoot(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeSkillToDefaultCatalogue(t, mosaicRoot, "default-skill")
	writeHookBundleToDefaultCatalogue(t, mosaicRoot, "default-hook")
	writeSkillToCustomCatalogueRoot(t, custom, "custom-skill")
	writeHookBundleToCustomCatalogueRoot(t, custom, "custom-hook")

	viaLoad, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	viaSingle, err := catalog.LoadSingleRoot(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("LoadSingleRoot: %v", err)
	}

	if got, want := skillKeys(viaLoad), skillKeys(viaSingle); !reflect.DeepEqual(got, want) {
		t.Errorf("Load skills = %v, LoadSingleRoot skills = %v", got, want)
	}
	if got, want := hookKeys(viaLoad), hookKeys(viaSingle); !reflect.DeepEqual(got, want) {
		t.Errorf("Load hooks = %v, LoadSingleRoot hooks = %v", got, want)
	}
	if got, want := issueCodes(viaLoad), issueCodes(viaSingle); !reflect.DeepEqual(got, want) {
		t.Errorf("Load issue codes = %v, LoadSingleRoot issue codes = %v", got, want)
	}
}

func skillKeys(cat catalog.Catalog) []string {
	var keys []string
	for _, s := range cat.Skills() {
		keys = append(keys, s.Key)
	}
	sort.Strings(keys)
	return keys
}

func hookKeys(cat catalog.Catalog) []string {
	var keys []string
	for _, h := range cat.Hooks() {
		keys = append(keys, h.Key)
	}
	sort.Strings(keys)
	return keys
}

func issueCodes(cat catalog.Catalog) []string {
	var codes []string
	for _, i := range cat.Issues() {
		codes = append(codes, i.Code)
	}
	sort.Strings(codes)
	return codes
}

func TestLoad_DefaultRoot_LoadsItsOwnSkillsAndHooksOnce(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	writeSkillToDefaultCatalogue(t, mosaicRoot, "my-skill")
	writeHookBundleToDefaultCatalogue(t, mosaicRoot, "my-bundle")

	for _, root := range []string{"", catalog.DefaultCatalogRoot(mosaicRoot)} {
		cat, err := catalog.Load(mosaicRoot, root)
		if err != nil {
			t.Fatalf("Load(%q): %v", root, err)
		}
		if len(cat.Skills()) != 1 || len(cat.Hooks()) != 1 {
			t.Errorf("Load(%q): %d skills / %d hooks, want 1 / 1", root, len(cat.Skills()), len(cat.Hooks()))
		}
	}
}
