package builtin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// builtinModuleDirs are the Deploy builtin harness module directories. The
// descriptors live once in the shared Common package, so none of these
// directories may carry a descriptor YAML of its own.
var builtinModuleDirs = []string{"claudecode", "opencode", "ghcpcli", "vscodeghcp", "codex"}

func TestBuiltinModuleDirs_HoldNoDescriptorYAML(t *testing.T) {
	for _, dir := range builtinModuleDirs {
		t.Run(dir, func(t *testing.T) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read module directory %s: %v", dir, err)
			}

			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					t.Errorf("%s still holds its own descriptor copy %s; it must live only in Common",
						dir, filepath.Join(dir, e.Name()))
				}
			}
		})
	}
}
