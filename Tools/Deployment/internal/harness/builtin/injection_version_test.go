package builtin_test

// injection_version_test.go specifies that every built-in harness reports, as its subagent
// injection version, the frontmatter version of the HarnessInjections.md in the content directory
// it loaded; that editing the file changes the reported version with no rebuild; that a content
// file without a frontmatter version fails construction naming the file; and that the built-in
// harness YAML files no longer declare an injections version of their own.

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/harness/injectionfile"
	"mosaic-deploy/internal/harness/registry"
)

// builtinYAMLPaths maps each built-in harness name to its embedded descriptor file.
var builtinYAMLPaths = map[string]string{
	"claude-code": filepath.Join("claudecode", "claude-code.yaml"),
	"ghcp-cli":    filepath.Join("ghcpcli", "ghcp-cli.yaml"),
	"opencode":    filepath.Join("opencode", "opencode.yaml"),
	"vscode-ghcp": filepath.Join("vscodeghcp", "vscode-ghcp.yaml"),
}

func contentFile(dir, name string) string { return filepath.Join(dir, name) }

func writeContentFile(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(contentFile(dir, name), []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func versionedInjectionFile(version string) string {
	return "---\nversion: \"" + version + "\"\n---\n<HarnessConstraints type=\"managed\">\nx\n</HarnessConstraints>\n"
}

func fixedContentDir(dir string) registry.BuiltinOptions {
	return registry.BuiltinOptions{ContentDir: func(string) (string, error) { return dir, nil }}
}

// TestBuiltinInjectionsVersion_EqualsCatalogFileVersion is the drift check: a built-in must
// never report an injection version different from the HarnessInjections.md it loaded.
func TestBuiltinInjectionsVersion_EqualsCatalogFileVersion(t *testing.T) {
	contentDirs := frozenContentDirs(t)
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := contentDirs(tc.dirName)
			if err != nil {
				t.Fatalf("resolve content dir: %v", err)
			}
			loaded, err := injectionfile.LoadDir(dir)
			if err != nil {
				t.Fatalf("LoadDir(%s): %v", dir, err)
			}
			mod, err := tc.build(registry.BuiltinOptions{ContentDir: contentDirs})
			if err != nil {
				t.Fatalf("construct: %v", err)
			}

			got := mod.Descriptor().InjectionsVersion

			if got == "" || got != loaded.SharedVersion {
				t.Errorf("InjectionsVersion = %q, want the HarnessInjections.md version %q", got, loaded.SharedVersion)
			}
		})
	}
}

func TestBuiltinInjectionsVersion_ChangingTheFileChangesTheVersionWithoutRebuild(t *testing.T) {
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeInjectionContent(t, dir, "A", "1.0.0")
			opts := fixedContentDir(dir)

			first, err := tc.build(opts)
			if err != nil {
				t.Fatalf("first construct: %v", err)
			}
			writeContentFile(t, dir, injectionfile.SharedFileName, versionedInjectionFile("8.8.8"))
			second, err := tc.build(opts)
			if err != nil {
				t.Fatalf("second construct: %v", err)
			}

			if got := first.Descriptor().InjectionsVersion; got != sharedFixtureVersion {
				t.Errorf("first InjectionsVersion = %q, want %q", got, sharedFixtureVersion)
			}
			if got := second.Descriptor().InjectionsVersion; got != "8.8.8" {
				t.Errorf("second InjectionsVersion = %q, want %q after editing the file", got, "8.8.8")
			}
		})
	}
}

func TestBuiltinConstruction_ContentFileWithoutVersion_FailsNamingTheFile(t *testing.T) {
	const noVersion = "---\nharness: x\n---\n<HarnessConstraints type=\"managed\">\nx\n</HarnessConstraints>\n"
	for _, tc := range contentDirCases() {
		for _, file := range []string{injectionfile.SharedFileName, injectionfile.OrchestratorFileName} {
			t.Run(tc.name+"/"+file, func(t *testing.T) {
				dir := t.TempDir()
				writeInjectionContent(t, dir, "A", "1.0.0")
				writeContentFile(t, dir, file, noVersion)

				_, err := tc.build(fixedContentDir(dir))

				var loadErr *injectionfile.LoadError
				if !errors.As(err, &loadErr) || loadErr.Kind != injectionfile.LoadErrorMissingVersion {
					t.Fatalf("construct error = %v, want a missing-version *injectionfile.LoadError", err)
				}
				if !strings.Contains(err.Error(), contentFile(dir, file)) {
					t.Errorf("error %q does not name the file path %q", err.Error(), contentFile(dir, file))
				}
			})
		}
	}
}

func TestBuiltinYAML_DoesNotDeclareInjectionsVersion_KeepsLegacyDropEntry(t *testing.T) {
	topLevel := regexp.MustCompile(`(?m)^injections_version\s*:`)
	for name, path := range builtinYAMLPaths {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if topLevel.Match(src) {
				t.Errorf("%s still declares injections_version", path)
			}
			if !strings.Contains(string(src), `"mosaic_injections_version"`) {
				t.Errorf("%s lost its mosaic_injections_version drop entry", path)
			}
		})
	}
}
