package app_test

// parity_helpers_test.go holds the builders of the deploy-versus-transform parity tests: a
// service wired with the four real built-in harness modules, the real planner, executor,
// manifest store and a catalog loaded from the tool-local sample under testdata/parity/catalog.
// Nothing here reads the repository Catalog.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
	"mosaic-deploy/internal/todo"
)

// parityModel is the one model every deploy and transform of the parity tests resolves to.
const parityModel = "parity-model"

// parityHarnesses are the four built-in harnesses, in a fixed order.
var parityHarnesses = []string{hClaude, hOpen, hCLI, hVSCode}

// parityAgent names one sample agent: the deployed key and what kind of agent it is.
type parityAgent struct {
	key     string
	utility bool // a utility agent whose catalog entry does not list the skill tool
}

// parityAgents are the four sample agents: one subagent, the orchestrator, the script
// orchestrator and one utility agent.
var parityAgents = []parityAgent{
	{key: "parity-runner"},
	{key: "orchestrator"},
	{key: "orchestrator-script"},
	{key: "parity-helper", utility: true},
}

// parityService wires a service over the sample catalog for one workspace.
func parityService(t *testing.T) app.Service {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "parity", "catalog"))
	if err != nil {
		t.Fatalf("resolve parity catalog root: %v", err)
	}
	cat, err := catalog.Load(t.TempDir(), root)
	if err != nil {
		t.Fatalf("load parity catalog: %v", err)
	}
	script := interactiontest.NewBuilder().Build()
	deps, _ := newBaseDeps(t, script)
	reg := &stubRegistry{modules: map[string]domain.HarnessModule{}}
	for _, id := range parityHarnesses {
		m := rbModule(t, id)
		reg.list = append(reg.list, m.Ref())
		reg.modules[id] = m
	}
	deps.Catalog = cat
	deps.Registry = reg
	deps.Planner = plan.New()
	deps.Manifest = manifest.NewStore()
	deps.Todo = todo.NewCollector()
	deps.Executor = deploy.NewExecutor(deps.Manifest, deps.Logger, deps.Todo)
	deps.ProtocolLoader = catalog.FileProtocolLoader{}
	deps.BundleLoader = catalog.FileBundleLoader{}
	return app.New(deps)
}

// parityDeploy deploys the sample catalog to a fresh workspace for one harness and returns
// the workspace root.
func parityDeploy(t *testing.T, harnessID string) string {
	t.Helper()
	ws := t.TempDir()
	models := map[string]string{}
	for _, a := range parityAgents {
		models[a.key] = parityModel
	}
	req := app.DeployRequest{
		HarnessID:       harnessID,
		WorkspacePath:   ws,
		Scope:           domain.ScopeProject,
		WorkflowIDs:     []string{"parity-flow"},
		UtilityAgentIDs: []string{"parity-helper"},
		AgentModels:     models,
		AutoConfirmPlan: true,
	}
	if _, err := parityService(t).DeployNew(context.Background(), req); err != nil {
		t.Fatalf("deploy to %s: %v", harnessID, err)
	}
	return ws
}

// parityTransform transforms the deployment in ws from src to tgt with the parity model.
func parityTransform(t *testing.T, ws, src, tgt string) app.TransformHarnessResult {
	t.Helper()
	res, err := parityService(t).TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: src,
		TargetHarnessID: tgt,
		Workspace:       ws,
		TargetModel:     parityModel,
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("transform %s -> %s: %v", src, tgt, err)
	}
	return res
}

// parityAgentFile returns the content of the deployed file of an agent key in a harness's
// agents folder. Both file spellings are tried, because deploy and transform name GHCP CLI
// files differently (X.md versus X.agent.md), which is an allowed difference.
func parityAgentFile(t *testing.T, ws, harnessID, key string) string {
	t.Helper()
	dir := rbModule(t, harnessID).Descriptor().Paths.Agents.Project
	for _, ext := range []string{".md", ".agent.md"} {
		b, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(dir), key+ext))
		if err == nil {
			return string(b)
		}
	}
	t.Fatalf("no deployed file for agent %q in %s of %s", key, dir, harnessID)
	return ""
}

// parityReadRel returns a workspace-relative file, failing the test when it is missing.
func parityReadRel(t *testing.T, ws, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// paritySkillEntryRemoved returns doc with the frontmatter entry that grants skill access on
// the harness taken back to what a deploy writes for an agent that does not list the skill
// tool: a list or comma-separated element is removed, a permission pair is set to deny. A
// document without such an entry, or a harness that writes no skill access, is returned
// unchanged. It is the one normalisation the parity comparison allows, and only for a utility
// agent whose catalog entry does not list the skill tool.
func paritySkillEntryRemoved(t *testing.T, harnessID, doc string) string {
	t.Helper()
	names := paritySkillNames(t, harnessID)
	if len(names) == 0 {
		return doc
	}
	lines := strings.Split(doc, "\n")
	out := make([]string, 0, len(lines))
	inFront, dashes := false, 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "---" {
			dashes++
			inFront = dashes == 1
			out = append(out, l)
			continue
		}
		if inFront {
			l2, drop := paritySkillLine(l, names)
			if drop {
				continue
			}
			l = l2
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// paritySkillNames returns the harness-side names of the skill tool mapping.
func paritySkillNames(t *testing.T, harnessID string) []string {
	t.Helper()
	var names []string
	for _, m := range rbModule(t, harnessID).Descriptor().Tools.Mappings {
		if m.Generic != "skill" {
			continue
		}
		for _, d := range m.Destinations {
			names = append(names, d.Names...)
		}
	}
	return names
}

// paritySkillLine undoes the skill entry on one frontmatter line. It returns the line to keep
// and whether the line is dropped altogether (a block-list item that is the skill entry).
func paritySkillLine(line string, names []string) (string, bool) {
	s := strings.TrimSpace(line)
	item := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(s, "- "), ","), "\"'")
	for _, n := range names {
		if strings.HasPrefix(s, "- ") && item == n {
			return line, true
		}
		if s == n+": allow" {
			return strings.Replace(line, n+": allow", n+": deny", 1), false
		}
		if strings.HasPrefix(s, "tools:") {
			for _, q := range []string{"'", "\"", ""} {
				tok := q + n + q
				for _, pat := range []string{", " + tok, tok + ", ", "[" + tok + "]"} {
					repl := ""
					if pat == "["+tok+"]" {
						repl = "[]"
					}
					if strings.Contains(line, pat) {
						return strings.Replace(line, pat, repl, 1), false
					}
				}
			}
		}
	}
	return line, false
}
