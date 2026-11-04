package hookbundle_test

// Tests that Load returns every file entry a multi-entry variant declares, in
// declared order, without loss or duplication, including entries whose source
// escapes the variant directory into a sibling folder or to the bundle manifest.
//
// Fixture: Tools/Common/testdata/hookbundle/load-multi-file/hook.yaml.

import (
	"path/filepath"
	"testing"

	"mosaic-common/hookbundle"
)

func TestLoad_MultiEntryVariant_ReturnsEveryDeclaredFileInOrder(t *testing.T) {
	bundleDir := filepath.Join(testdataDir, "load-multi-file")

	m, err := hookbundle.Load(bundleDir)
	if err != nil {
		t.Fatalf("Load(%q): unexpected error: %v", bundleDir, err)
	}
	v, ok := m.Variants["multi"]
	if !ok {
		t.Fatal(`Variants["multi"] not found`)
	}

	want := []hookbundle.File{
		{Source: "main.py", Target: "main.py"},
		{Source: "../shared/zeta.py", Target: "shared_zeta.py"},
		{Source: "../shared/alpha.py", Target: "shared_alpha.py"},
		{Source: "helper.py", Target: "helper.py"},
		{Source: "../shared/mid.py", Target: "shared_mid.py"},
		{Source: "../hook.yaml", Target: "hook.yaml"},
	}
	if len(v.Files) != len(want) {
		t.Fatalf("Files has %d entries %v, want %d entries %v", len(v.Files), v.Files, len(want), want)
	}
	for i, w := range want {
		if v.Files[i] != w {
			t.Errorf("Files[%d] = %+v, want %+v", i, v.Files[i], w)
		}
	}
}
