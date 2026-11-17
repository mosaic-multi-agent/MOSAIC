package builtindesc_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mosaic-common/harness/builtindesc"
)

var wantIDs = []string{"claude-code", "opencode", "ghcp-cli", "vscode-ghcp", "codex"}

func TestIDs_ListsEveryBuiltinInBuiltinOrder(t *testing.T) {
	got := builtindesc.IDs()

	if !reflect.DeepEqual(got, wantIDs) {
		t.Errorf("want %v, got %v", wantIDs, got)
	}
}

func TestIDs_ReturnsFreshSliceOnEveryCall(t *testing.T) {
	first := builtindesc.IDs()
	if len(first) == 0 {
		t.Fatal("IDs() returned no ids")
	}
	first[0] = "mutated"

	second := builtindesc.IDs()

	if second[0] != wantIDs[0] {
		t.Errorf("mutating a returned slice leaked into the next call: got %q", second[0])
	}
}

func TestRaw_EachBuiltinYieldsNonEmptyDescriptorYAML(t *testing.T) {
	for _, id := range wantIDs {
		t.Run(id, func(t *testing.T) {
			src, ok := builtindesc.Raw(id)

			if !ok {
				t.Fatalf("Raw(%q): ok = false, want true", id)
			}
			if len(src) == 0 {
				t.Fatalf("Raw(%q): empty descriptor", id)
			}
			text := "\n" + string(src)
			if !strings.Contains(text, "\nid: "+id) && !strings.Contains(text, "\nid: \""+id+"\"") {
				t.Errorf("Raw(%q): descriptor does not declare its own id", id)
			}
		})
	}
}

func TestRaw_MatchesFileStoredInPackageDirectory(t *testing.T) {
	for _, id := range wantIDs {
		t.Run(id, func(t *testing.T) {
			want, err := os.ReadFile(id + ".yaml")
			if err != nil {
				t.Fatalf("read %s.yaml: %v", id, err)
			}

			got, ok := builtindesc.Raw(id)

			if !ok || string(got) != string(want) {
				t.Errorf("Raw(%q) differs from %s.yaml (ok=%v)", id, id, ok)
			}
		})
	}
}

func TestRaw_ReturnsFreshSliceOnEveryCall(t *testing.T) {
	first, ok := builtindesc.Raw("codex")
	if !ok || len(first) == 0 {
		t.Fatal("Raw(codex) returned nothing")
	}
	want := string(first)
	first[0] ^= 0xff

	second, _ := builtindesc.Raw("codex")

	if string(second) != want {
		t.Error("mutating a returned slice leaked into the next call")
	}
}

func TestRaw_UnknownIDYieldsNothing(t *testing.T) {
	for _, id := range []string{"nope", "", "Claude-Code", "claude-code.yaml", "../harness/catalog"} {
		t.Run("id="+id, func(t *testing.T) {
			src, ok := builtindesc.Raw(id)

			if ok || src != nil {
				t.Errorf("Raw(%q) = (%d bytes, %v), want (nil, false)", id, len(src), ok)
			}
		})
	}
}

func TestIDs_EqualsTheYAMLFilesInThePackageDirectory(t *testing.T) {
	files, err := filepath.Glob("*.yaml")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var onDisk []string
	for _, f := range files {
		onDisk = append(onDisk, strings.TrimSuffix(f, ".yaml"))
	}
	got := builtindesc.IDs()
	sort.Strings(onDisk)
	sort.Strings(got)

	if len(onDisk) == 0 {
		t.Fatal("no descriptor YAML files found in the package directory")
	}
	if !reflect.DeepEqual(onDisk, got) {
		t.Errorf("YAML files on disk %v do not match IDs() %v", onDisk, got)
	}
}
