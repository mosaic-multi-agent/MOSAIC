package builtin_test

// content_dir_construction_test.go specifies that every built-in harness module takes its
// injection content, and its orchestrator injections version, only from the content
// directory its resolver returns, never from the MOSAIC root.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/claudecode"
	"mosaic-deploy/internal/harness/builtin/ghcpcli"
	"mosaic-deploy/internal/harness/builtin/opencode"
	"mosaic-deploy/internal/harness/builtin/vscodeghcp"
	"mosaic-deploy/internal/harness/registry"
)

type contentDirCase struct {
	name    string
	dirName string
	build   func(registry.BuiltinOptions) (domain.HarnessModule, error)
	// repoContentDir is the MOSAIC-root-relative default location of this harness's content.
	repoContentDir string
}

func contentDirCases() []contentDirCase {
	return []contentDirCase{
		{"claude-code", catalogpaths.HarnessDirNameClaudeCode, claudecode.New, claudecode.RepoContentDir},
		{"ghcp-cli", catalogpaths.HarnessDirNameGhcpCLI, ghcpcli.New, ghcpcli.RepoContentDir},
		{"opencode", catalogpaths.HarnessDirNameOpenCode, opencode.New, opencode.RepoContentDir},
		{"vscode-ghcp", catalogpaths.HarnessDirNameVSCodeGHCP, vscodeghcp.New, vscodeghcp.RepoContentDir},
	}
}

// writeInjectionContent writes both injection files into dir. tag makes the content of one
// directory distinguishable from another's.
func writeInjectionContent(t *testing.T, dir, tag, orchVersion string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	shared := "<HarnessConstraints type=\"managed\">\nSHARED_" + tag + "\n</HarnessConstraints>\n"
	orch := "---\nversion: \"" + orchVersion + "\"\n---\n<HarnessConstraints type=\"managed\">\nORCH_" + tag + "\n</HarnessConstraints>\n"
	for name, src := range map[string]string{"HarnessInjections.md": shared, "HarnessInjectionsOrchestrator.md": orch} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func injectionFor(m domain.HarnessModule, role domain.AgentRole) (string, bool) {
	return m.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "agent", Role: role})
}

func TestBuiltinConstruction_UsesGivenContentDir_NotMosaicRoot(t *testing.T) {
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			given := t.TempDir() // the directory the resolver returns
			writeInjectionContent(t, given, "GIVEN", "9.4.2")

			// A MOSAIC root holding different content at the default location.
			root := t.TempDir()
			writeInjectionContent(t, filepath.Join(root, tc.repoContentDir), "ROOT", "1.1.1")

			var asked []string
			mod, err := tc.build(registry.BuiltinOptions{
				MosaicRoot: root,
				ContentDir: func(dirName string) (string, error) {
					asked = append(asked, dirName)
					return given, nil
				},
			})
			if err != nil {
				t.Fatalf("construct from given content dir: %v", err)
			}

			shared, ok := injectionFor(mod, domain.AgentRole(""))
			if !ok || !strings.Contains(shared, "SHARED_GIVEN") || strings.Contains(shared, "_ROOT") {
				t.Errorf("shared injection = %q (ok=%v); want content of the given directory only", shared, ok)
			}
			orch, ok := injectionFor(mod, domain.RoleOrchestrator)
			if !ok || !strings.Contains(orch, "SHARED_GIVEN") || !strings.Contains(orch, "ORCH_GIVEN") || strings.Contains(orch, "_ROOT") {
				t.Errorf("orchestrator injection = %q (ok=%v); want shared and orchestrator content of the given directory only", orch, ok)
			}
			if got := mod.Descriptor().OrchestratorInjectionsVersion; got != "9.4.2" {
				t.Errorf("OrchestratorInjectionsVersion = %q, want %q from the given directory", got, "9.4.2")
			}
			if len(asked) != 1 || asked[0] != tc.dirName {
				t.Errorf("resolver asked for %v, want exactly [%q]", asked, tc.dirName)
			}
		})
	}
}

func TestBuiltinConstruction_EmptyMosaicRoot_StillLoadsGivenContentDir(t *testing.T) {
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			given := t.TempDir()
			writeInjectionContent(t, given, "GIVEN", "3.0.0")
			mod, err := tc.build(registry.BuiltinOptions{
				ContentDir: func(string) (string, error) { return given, nil },
			})
			if err != nil {
				t.Fatalf("construct without MosaicRoot: %v", err)
			}
			if got, ok := injectionFor(mod, domain.RoleOrchestrator); !ok || !strings.Contains(got, "ORCH_GIVEN") {
				t.Errorf("orchestrator injection = %q (ok=%v), want given directory content", got, ok)
			}
		})
	}
}

func TestBuiltinConstruction_ResolverError_FailsConstructionAndIsPreserved(t *testing.T) {
	sentinel := errors.New("no supplier for this harness")
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			// Content at the MOSAIC-root default must not rescue a failed resolver.
			root := t.TempDir()
			writeInjectionContent(t, filepath.Join(root, tc.repoContentDir), "ROOT", "1.1.1")
			mod, err := tc.build(registry.BuiltinOptions{
				MosaicRoot: root,
				ContentDir: func(string) (string, error) { return "", sentinel },
			})
			if err == nil {
				t.Fatalf("expected construction error, got module %v", mod)
			}
			if !errors.Is(err, sentinel) {
				t.Errorf("construction error %v does not wrap the resolver error", err)
			}
		})
	}
}

func TestBuiltinConstruction_NilResolver_IsConstructionError(t *testing.T) {
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeInjectionContent(t, filepath.Join(root, tc.repoContentDir), "ROOT", "1.1.1")
			if _, err := tc.build(registry.BuiltinOptions{MosaicRoot: root}); err == nil {
				t.Error("expected error for nil ContentDir resolver, got nil")
			}
		})
	}
}

// TestMosaicRootContentDirs_MatchesRepoContentDir verifies that the MOSAIC-root resolver
// returns each built-in's documented MOSAIC-root-relative default location.
func TestMosaicRootContentDirs_MatchesRepoContentDir(t *testing.T) {
	root := t.TempDir()
	resolve := registry.MosaicRootContentDirs(root)
	for _, tc := range contentDirCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolve(tc.dirName)
			if err != nil {
				t.Fatalf("resolve(%q): %v", tc.dirName, err)
			}
			want := filepath.Join(root, filepath.FromSlash(tc.repoContentDir))
			if filepath.Clean(got) != filepath.Clean(want) {
				t.Errorf("MosaicRootContentDirs(root)(%q) = %q, want %q", tc.dirName, got, want)
			}
		})
	}
}
