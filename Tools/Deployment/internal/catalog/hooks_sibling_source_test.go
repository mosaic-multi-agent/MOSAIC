package catalog_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestHook_Variant_Files_SiblingFolderSource_LoadedUnderTargetName verifies that a variant
// file whose source lives in a sibling folder of the variant directory ("../shared/x.py")
// is resolved to that real file and planned for deployment under its flat target name.
func TestHook_Variant_Files_SiblingFolderSource_LoadedUnderTargetName(t *testing.T) {
	cat := loadFixtureCatalog(t, "hook-sibling-source")

	h, ok := cat.Hook("sibling-bundle")
	if !ok {
		t.Fatal("Hook(\"sibling-bundle\"): not found in fixture catalog")
	}
	v, ok := h.Variants["test-harness"]
	if !ok {
		t.Fatal("sibling-bundle Variants[\"test-harness\"]: not found")
	}

	targets := make([]string, 0, len(v.Files))
	for _, f := range v.Files {
		targets = append(targets, f.TargetName)
		if f.TargetName != "shared_x.py" {
			continue
		}
		if filepath.Base(filepath.Dir(f.SourcePath)) != "shared" {
			t.Errorf("shared_x.py SourcePath = %q, want a file inside the sibling \"shared\" folder", f.SourcePath)
		}
		if filepath.Base(f.SourcePath) != "x.py" {
			t.Errorf("shared_x.py SourcePath = %q, want base name x.py", f.SourcePath)
		}
		if _, err := os.Stat(f.SourcePath); err != nil {
			t.Errorf("shared_x.py SourcePath %q does not resolve to an existing file: %v", f.SourcePath, err)
		}
	}
	sort.Strings(targets)
	want := []string{"main.sh", "shared_x.py"}
	if len(targets) != len(want) || targets[0] != want[0] || targets[1] != want[1] {
		t.Errorf("variant target names = %v, want %v", targets, want)
	}
}
