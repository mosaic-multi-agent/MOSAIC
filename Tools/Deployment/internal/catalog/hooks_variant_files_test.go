package catalog_test

import (
	"sort"
	"testing"
)

// TestHook_Variant_Files_TargetNames_MatchDeclaredSetExactly verifies that the catalog
// loader reports exactly the target file names a hook variant declares in hook.yaml:
// no undeclared extra file and no missing declared file. The fixture deliberately uses
// a source name that differs from its target name, so the loader must report targets.
func TestHook_Variant_Files_TargetNames_MatchDeclaredSetExactly(t *testing.T) {
	cat := loadFixtureCatalog(t, "hook-variant-files")

	h, ok := cat.Hook("files-bundle")
	if !ok {
		t.Fatal("Hook(\"files-bundle\"): not found in fixture catalog")
	}
	v, ok := h.Variants["test-harness"]
	if !ok {
		t.Fatal("files-bundle Variants[\"test-harness\"]: not found")
	}

	want := []string{"alpha.sh", "beta-target.sh", "settings.yaml"}
	got := make([]string, 0, len(v.Files))
	for _, f := range v.Files {
		got = append(got, f.TargetName)
	}
	sort.Strings(got)

	wantSet := make(map[string]bool, len(want))
	for _, name := range want {
		wantSet[name] = true
	}
	gotSet := make(map[string]bool, len(got))
	for _, name := range got {
		gotSet[name] = true
		if !wantSet[name] {
			t.Errorf("variant has unexpected target file %q", name)
		}
	}
	for _, name := range want {
		if !gotSet[name] {
			t.Errorf("variant is missing expected target file %q", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("variant has %d files %v, want %d files %v", len(got), got, len(want), want)
	}
}
