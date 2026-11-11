package plan_test

// two_version_tier_acceptance_test.go specifies, for each harness tier (built-in,
// descriptor-only, external), that editing the version of HarnessInjections.md and
// reconstructing the registry (no rebuild) marks subagents and orchestrators for update, and
// that editing only HarnessInjectionsOrchestrator.md marks only orchestrators. Injection files
// live in temporary directories; deployed agents carry the tags written for versions 1.0.0 and
// 2.0.0.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"mosaic-deploy/internal/domain"
	_ "mosaic-deploy/internal/harness/builtin/opencode"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/plan"
)

const (
	tierSharedDoc = "---\nversion: %q\n---\n<HarnessConstraints type=\"managed\">\nshared text\n</HarnessConstraints>\n"
	tierOrchDoc   = "---\nversion: %q\n---\n<OrchestratorHarnessConstraints type=\"managed\">\norchestrator text\n</OrchestratorHarnessConstraints>\n"
)

// tierModule gives the resolved module flat agent paths so the fixtures stay tier-independent.
type tierModule struct{ domain.HarnessModule }

func (m tierModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	if req.Kind == domain.ArtifactAgent {
		return "agents/" + req.Key + ".md", nil
	}
	return m.HarnessModule.TargetPath(req)
}

// tierFixture is one harness tier: resolve reads the injection files of the harness as they are
// on disk now, by discovering the registry again.
type tierFixture struct {
	name    string
	resolve func(t *testing.T) domain.HarnessModule
	write   func(t *testing.T, shared, orch string) // rewrites both injection files
}

func tierWriteDoc(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func tierWriteBoth(t *testing.T, dir, shared, orch string) {
	t.Helper()
	tierWriteDoc(t, dir, "HarnessInjections.md", fmt.Sprintf(tierSharedDoc, shared))
	tierWriteDoc(t, dir, "HarnessInjectionsOrchestrator.md", fmt.Sprintf(tierOrchDoc, orch))
}

func tierResolve(t *testing.T, opts registry.Options, id string) domain.HarnessModule {
	t.Helper()
	reg, err := registry.Discover(opts)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	mod, err := reg.Resolve(id)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", id, err)
	}
	t.Cleanup(func() { _ = mod.Close() })
	return mod
}

func tierYAMLDoc(id string) string {
	return "schema_version: \"1\"\nid: \"" + id + "\"\ndisplay_name: \"" + id + "\"\n"
}

func builtinTier(t *testing.T) tierFixture {
	root := t.TempDir()
	contentDir := t.TempDir()
	tierWriteDoc(t, filepath.Join(root, "MosaicDeploy", "harnesses"), ".keep", "")
	return tierFixture{
		name: "built-in",
		resolve: func(t *testing.T) domain.HarnessModule {
			return tierResolve(t, registry.Options{
				MosaicRoot:        root,
				BuiltinContentDir: func(string) (string, error) { return contentDir, nil },
			}, "opencode")
		},
		write: func(t *testing.T, shared, orch string) { tierWriteBoth(t, contentDir, shared, orch) },
	}
}

func descriptorTier(t *testing.T) tierFixture {
	root := t.TempDir()
	const id = "tier-descriptor"
	dir := filepath.Join(root, "MosaicDeploy", "harnesses", id)
	tierWriteDoc(t, dir, "harness.yaml", tierYAMLDoc(id))
	return tierFixture{
		name:    "descriptor-only",
		resolve: func(t *testing.T) domain.HarnessModule { return tierResolve(t, registry.Options{MosaicRoot: root}, id) },
		write:   func(t *testing.T, shared, orch string) { tierWriteBoth(t, dir, shared, orch) },
	}
}

func externalTier(t *testing.T) tierFixture {
	root := t.TempDir()
	const id = "tier-external"
	// Incidental copy: the versions the tests vary are written into the harness dir; the frozen
	// catalog only supplies the reference module's content. The module is built with go build
	// (the Go toolchain must be on PATH).
	src := filepath.Join("..", "..", "testdata", "frozen-catalog", "Catalog", "HarnessInjections", "OpenCode")
	for _, name := range []string{"HarnessInjections.md", "HarnessInjectionsOrchestrator.md"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		tierWriteDoc(t, filepath.Join(root, "Catalog", "HarnessInjections", "OpenCode"), name, string(data))
	}
	dir := filepath.Join(root, "MosaicDeploy", "harnesses", id)
	tierWriteDoc(t, dir, "harness.yaml", tierYAMLDoc(id))
	bin := tierBuildReferenceModule(t)
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("read module binary: %v", err)
	}
	execName := "harness-exec"
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, execName), data, 0o755); err != nil {
		t.Fatalf("write module binary: %v", err)
	}
	return tierFixture{
		name: "external",
		resolve: func(t *testing.T) domain.HarnessModule {
			return tierResolve(t, registry.Options{MosaicRoot: root, AllowExternal: true}, id)
		},
		write: func(t *testing.T, shared, orch string) { tierWriteBoth(t, dir, shared, orch) },
	}
}

func tierBuildReferenceModule(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "harness-exec")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "mosaic-deploy/cmd/harness-opencode-module")
	cmd.Dir = filepath.Join("..", "..")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build reference module: %v\n%s", err, stderr.String())
	}
	return out
}

// tierPlanFor plans the update of agents deployed for versions tvShared/tvOrch against the
// harness as currently on disk.
func tierPlanFor(t *testing.T, fx tierFixture) domain.Plan {
	t.Helper()
	mod := fx.resolve(t)
	states := tvStates(tvShared, tvOrch)
	for path, s := range states {
		s.HarnessVersion = mod.Descriptor().TransformVersion
		states[path] = s
	}
	in := buildInfraInput(plan.InfrastructureDeclarationIntent{}, states)
	in.Module = tierModule{mod}
	// The fixture entries belong to the harness of the run's module.
	for i := range in.Manifest.Manifest.Entries {
		in.Manifest.Manifest.Entries[i].Harness = mod.Descriptor().ID
	}
	return buildInfraPlan(t, in)
}

func TestTiers_InjectionFileBumps_MarkTheRightAgents(t *testing.T) {
	for _, mk := range []func(*testing.T) tierFixture{builtinTier, descriptorTier, externalTier} {
		fx := mk(t)
		t.Run(fx.name, func(t *testing.T) {
			t.Run("baseline", func(t *testing.T) {
				fx.write(t, tvShared, tvOrch)
				tvAssertUnchanged(t, tierPlanFor(t, fx), "orchestrator", "orchestrator-script", "test-worker")
			})

			t.Run("shared file bump marks subagent and orchestrators", func(t *testing.T) {
				fx.write(t, "1.1.0", tvOrch)
				p := tierPlanFor(t, fx)
				for _, key := range []string{"orchestrator", "orchestrator-script", "test-worker"} {
					item := mustFindItem(t, p, key)
					if _, ok := tvDelta(item, "injections_version"); !ok || item.Action != domain.ActionUpdate {
						t.Errorf("%s: Action %q, deltas %v; want an update with an injections_version delta", key, item.Action, item.Stale)
					}
					if _, ok := tvDelta(item, "orchestrator_injections_version"); ok {
						t.Errorf("%s: orchestrator_injections_version delta present after a shared-file bump", key)
					}
				}
			})

			t.Run("orchestrator file bump marks only orchestrators", func(t *testing.T) {
				fx.write(t, tvShared, "2.1.0")
				p := tierPlanFor(t, fx)
				for _, key := range []string{"orchestrator", "orchestrator-script"} {
					item := mustFindItem(t, p, key)
					fields := tvInjectionFields(item)
					if item.Action != domain.ActionUpdate || len(fields) != 1 || fields[0] != "orchestrator_injections_version" {
						t.Errorf("%s: Action %q, injection deltas %v; want an update through orchestrator_injections_version only", key, item.Action, fields)
					}
				}
				tvAssertUnchanged(t, p, "test-worker")
			})
		})
	}
}
