package stack_test

// layer_no_id_error_test.go covers how a layer error names a layer whose catalog id is not
// known: by its root only, never with a placeholder id.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
)

// assertNamesRootOnly fails unless msg names root and carries no placeholder id.
func assertNamesRootOnly(t *testing.T, msg, root string) {
	t.Helper()
	assertContains(t, msg, root)
	for _, bad := range []string{"layer ?", "catalog layer  ("} {
		if strings.Contains(msg, bad) {
			t.Errorf("error %q must not contain %q", msg, bad)
		}
	}
}

func TestNew_LayerWithoutKnownID_NamesRootOnly(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "does-not-exist")
	fileRoot := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file root: %v", err)
	}
	invalidIdentity := t.TempDir()
	mustWrite(t, invalidIdentity, "catalog-identity.yaml", "id: \"Not Valid\"\nname: X\nversion: \"1\"\n")

	cases := map[string]string{
		"no identity file":    t.TempDir(),
		"invalid identity":    invalidIdentity,
		"missing root":        missingRoot,
		"root is a file":      fileRoot,
		"relative layer root": "relative/layer",
	}
	for name, root := range cases {
		t.Run(name, func(t *testing.T) {
			mosaic := newTestMosaicRoot(t)

			_, err := stack.New(mosaic, []stack.Layer{{Root: root}})

			if err == nil {
				t.Fatal("New succeeded, want a layer error")
			}
			assertNamesRootOnly(t, err.Error(), root)
		})
	}
}

func TestNew_LayerWithKnownID_StillNamesID(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	broken := newTestCatalogRoot(t, "broken-cat")
	mustWrite(t, broken, filepath.Join("Subagents", "Cat", "x.md"), "No frontmatter.\n")

	_, err := stack.New(mosaic, []stack.Layer{{Root: broken}})

	if err == nil {
		t.Fatal("New succeeded, want a layer error")
	}
	assertContains(t, err.Error(), "catalog layer broken-cat ("+broken+")")
}
