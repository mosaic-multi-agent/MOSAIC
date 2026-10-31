package app

// transform_plan_helpers_test.go holds the shared builders of the transform planning tests:
// harness modules backed by the built-in descriptors, a catalog that only knows hook bundles,
// workspace builders that copy tool-local fixtures into a temporary directory, and plan
// lookups. Nothing here reads the repository Catalog.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

const (
	tpClaude = "claudecode/claude-code.yaml"
	tpOpen   = "opencode/opencode.yaml"
	tpCLI    = "ghcpcli/ghcp-cli.yaml"
	tpVSCode = "vscodeghcp/vscode-ghcp.yaml"

	tpHookKey = "mosaic-logger"
)

// tpModule is a harness module backed by a built-in descriptor. Agent target paths follow
// the descriptor (<agents dir>/<key><agent extension>); hook plans are fixed per module.
type tpModule struct {
	domain.HarnessModule // nil: only the methods below are used
	desc                 *domain.HarnessDescriptor
	hookPlan             domain.HookPlan
}

func (m *tpModule) Ref() domain.HarnessRef {
	return domain.HarnessRef{ID: m.desc.ID, DisplayName: m.desc.DisplayName, Usable: true}
}
func (m *tpModule) Descriptor() *domain.HarnessDescriptor { return m.desc }
func (m *tpModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	switch req.Kind {
	case domain.ArtifactAgent:
		return m.desc.Paths.Agents.Project + "/" + req.Key + m.desc.Extensions[domain.ArtifactAgent], nil
	case domain.ArtifactSkill:
		return m.desc.Paths.Skills.Project + "/" + req.Key, nil
	}
	return "", domain.ErrArtifactUnsupported
}
func (m *tpModule) HookPlan(domain.HookPlanRequest) (domain.HookPlan, error) {
	return m.hookPlan, nil
}

// tpHarness loads a built-in descriptor and wraps it as a module whose hook plan has one
// file "logger.sh" in the descriptor's hooks folder.
func tpHarness(t *testing.T, rel string) *tpModule {
	t.Helper()
	d, err := descriptor.Load("../harness/builtin/" + rel)
	if err != nil {
		t.Fatalf("load %s: %v", rel, err)
	}
	return &tpModule{desc: d, hookPlan: tpHookPlan(d.Paths.Hooks.Project, "logger.sh")}
}

// tpHookPlan returns a supported hook plan with the given target files in dir.
func tpHookPlan(dir string, names ...string) domain.HookPlan {
	hp := domain.HookPlan{Supported: true, TargetDir: dir}
	for _, n := range names {
		hp.Files = append(hp.Files, domain.HookFile{SourcePath: "unused/" + n, TargetName: n})
	}
	return hp
}

// tpCatalog is a catalog that knows only the logger hook bundle.
type tpCatalog struct{ catalog.Catalog }

func (tpCatalog) Hooks() []domain.HookBundle {
	return []domain.HookBundle{{Key: tpHookKey, Version: "1.5.1"}}
}

// tpService returns a service whose catalog holds the logger hook bundle.
func tpService() *service {
	return &service{deps: Deps{Catalog: tpCatalog{}}}
}

// tpWriteFile writes content below root, creating folders.
func tpWriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// tpFixture reads a file below testdata/transform.
func tpFixture(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "transform", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(data)
}

// tpPlaceAgent copies the agents fixture fixtureName into dir (workspace-relative) as fileName.
func tpPlaceAgent(t *testing.T, ws, dir, fileName, fixtureName string) {
	t.Helper()
	tpWriteFile(t, ws, dir+"/"+fileName, tpFixture(t, "agents/"+fixtureName))
}

// tpPlaceAllAgents copies every recognised agent fixture into the module's agents folder
// under the module's own extension.
func tpPlaceAllAgents(t *testing.T, ws string, m *tpModule) {
	t.Helper()
	ext := m.desc.Extensions[domain.ArtifactAgent]
	dir := m.desc.Paths.Agents.Project
	for _, k := range []string{"subagent", "orchestrator", "orchestrator-script", "utility", "standalone", "harness-only"} {
		tpPlaceAgent(t, ws, dir, k+ext, k+".md")
	}
}

// tpPlan runs planTransform and fails the test on an error.
func tpPlan(t *testing.T, ws string, src, tgt *tpModule, overwrite bool) TransformPlan {
	t.Helper()
	p, err := tpService().planTransform(transformPlanInput{Workspace: ws, Source: src, Target: tgt, Overwrite: overwrite})
	if err != nil {
		t.Fatalf("planTransform: %v", err)
	}
	return p
}

// tpAgent returns the planned agent for a source file name.
func tpAgent(t *testing.T, p TransformPlan, fileName string) PlannedAgent {
	t.Helper()
	for _, a := range p.Agents {
		if a.FileName == fileName {
			return a
		}
	}
	t.Fatalf("no planned agent for file %q; plan agents: %v", fileName, tpAgentNames(p))
	return PlannedAgent{}
}

func tpAgentNames(p TransformPlan) []string {
	var names []string
	for _, a := range p.Agents {
		names = append(names, a.FileName)
	}
	return names
}

// tpNotes returns the plan notes of one kind.
func tpNotes(p TransformPlan, kind TransformNoteKind) []TransformNote {
	var out []TransformNote
	for _, n := range p.Notes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// tpNoteMentions reports whether the note's message or paths contain every needle.
func tpNoteMentions(n TransformNote, needles ...string) bool {
	hay := filepath.ToSlash(n.Message + " " + strings.Join(n.Paths, " "))
	for _, s := range needles {
		if !strings.Contains(hay, s) {
			return false
		}
	}
	return true
}

// tpExists reports whether a workspace-relative file exists.
func tpExists(ws, rel string) bool {
	_, err := os.Stat(filepath.Join(ws, filepath.FromSlash(rel)))
	return err == nil
}

// tpSnapshot returns every file below root (relative, slash form) mapped to its content.
func tpSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// tpDest returns the slash form of a planned destination.
func tpDest(a PlannedAgent) string { return filepath.ToSlash(a.DestinationPath) }
