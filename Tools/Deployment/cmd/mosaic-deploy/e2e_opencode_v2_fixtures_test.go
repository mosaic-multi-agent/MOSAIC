package main

// e2e_opencode_v2_fixtures_test.go holds the fixtures of the OpenCode V2 binary tests: a
// MOSAIC root whose default catalog carries a worker with declared tools and a required skill,
// an orchestrator with the tool-permissions placeholder, a hook bundle with an opencode-v2
// variant and the empty OpenCode V2 injection files, plus readers for the deployed workspace.
// Nothing in this file is a test itself. Everything is built in temporary folders; nothing
// reads the shipped catalog.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

const (
	v2HarnessID   = "opencode-v2"
	v2WorkerKey   = "default-only-agent"
	v2HookKey     = "v2-plugin"
	v2PluginRel   = ".opencode/plugins/plugin.ts"
	v2HelperRel   = ".opencode/plugins/lib/helper.ts"
	v2PluginSrc   = "export default { name: 'fixture' }\n"
	v2HelperSrc   = "export const helper = 'fixture'\n"
	v2UserConfig  = "{\"theme\": \"user-owned\"}\n"
	v2AgentsDir   = ".opencode/agents"
	v2SkillRel    = ".opencode/skills/default-skill/SKILL.md"
	v2EmptyInject = "---\nversion: \"1.0.0\"\nharness: opencode-v2\n---\n\n# Injections\n\n## Design Rationale\n\nNone yet.\n"
)

// v2WorkerDoc is a subagent that declares two generic tools and requires one skill.
const v2WorkerDoc = "---\nid: \"default-only-001\"\nversion: \"1.0.0\"\nname: default-only-agent\n" +
	"description: Fixture worker.\nrole: subagent\nrequired_skills: [default-skill]\n" +
	"tools: [file_read, file_search]\n---\nContent.\n"

// v2OrchestratorDoc is an orchestrator that declares the tool-permissions placeholder.
const v2OrchestratorDoc = "---\nversion: \"1.0.0\"\nname: orchestrator\ndescription: Orchestrator.\n" +
	"role: orchestrator\ntools: {tool-permissions}\n---\nBody.\n"

// newV2MosaicRoot returns a MOSAIC root that can deploy the opencode-v2 harness.
func newV2MosaicRoot(t *testing.T) string {
	t.Helper()
	root := newStackMosaicRoot(t)
	def := filepath.Join(root, "Catalog")
	injections := catalogpaths.HarnessInjectionsDir(def, catalogpaths.HarnessDirNameOpenCodeV2)
	writeStackFile(t, injections, "HarnessInjections.md", []byte(v2EmptyInject))
	writeStackFile(t, injections, "HarnessInjectionsOrchestrator.md", []byte(v2EmptyInject))
	writeStackFile(t, def, filepath.Join("Subagents", "Test", v2WorkerKey+".md"), []byte(v2WorkerDoc))
	writeStackFile(t, def, filepath.Join("Orchestrator", "orchestrator.md"), []byte(v2OrchestratorDoc))
	bundle := "schema_version: \"1\"\nid: " + v2HookKey + "\nversion: 1.0.0\ndescription: Plugin.\nplaceholder: false\n" +
		"variants:\n  opencode-v2:\n    supported: true\n    files:\n" +
		"      - source: plugin.ts\n        target: plugin.ts\n" +
		"      - source: lib/helper.ts\n        target: lib/helper.ts\n    registration: []\n"
	writeStackFile(t, def, filepath.Join("Hooks", v2HookKey, "hook.yaml"), []byte(bundle))
	writeStackFile(t, def, filepath.Join("Hooks", v2HookKey, "opencode-v2", "plugin.ts"), []byte(v2PluginSrc))
	writeStackFile(t, def, filepath.Join("Hooks", v2HookKey, "opencode-v2", "lib", "helper.ts"), []byte(v2HelperSrc))
	return root
}

// v2Selections writes a selections file that deploys the default workflow and the hook bundle.
func v2Selections(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "selections.yaml")
	if err := os.WriteFile(p, []byte("workflows:\n  - default-wf\nhooks:\n  - "+v2HookKey+"\n"), 0o644); err != nil {
		t.Fatalf("write selections: %v", err)
	}
	return p
}

// runV2 runs a subcommand against ws for the opencode-v2 harness; the caller appends the flags.
func runV2(t *testing.T, root, ws string, args ...string) binaryRun {
	t.Helper()
	full := append(append([]string{}, args...), "--mosaic-root", root, "--harness", v2HarnessID, "--workspace", ws)
	return runDeployBinary(t, full...)
}

// deployV2 deploys the representative catalog into ws, failing the test unless it succeeds.
func deployV2(t *testing.T, root, ws string) {
	t.Helper()
	run := runV2(t, root, ws, "deploy", "--selections", v2Selections(t), "--auto-confirm", "--output", "json")
	if run.ExitCode != 0 && run.ExitCode != 2 {
		t.Fatalf("deploy --harness %s: exit %d\nstdout: %s\nstderr: %s", v2HarnessID, run.ExitCode, run.Stdout, run.Stderr)
	}
}

// workspaceFiles returns the slash-form relative paths of every file under ws.
func workspaceFiles(t *testing.T, ws string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(ws, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(ws, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", ws, err)
	}
	sort.Strings(out)
	return out
}

var harnessConfigName = regexp.MustCompile(`(?i)\.jsonc?$`)

// requireNoHarnessConfigOrTools fails when the workspace holds any JSON or JSONC file outside
// .mosaic (OpenCode reads its settings from such files) other than the named user-owned ones,
// or any file under .opencode/tools.
func requireNoHarnessConfigOrTools(t *testing.T, ws string, userOwned ...string) {
	t.Helper()
	owned := map[string]bool{}
	for _, p := range userOwned {
		owned[p] = true
	}
	for _, rel := range workspaceFiles(t, ws) {
		switch {
		case strings.HasPrefix(rel, ".opencode/tools/"):
			t.Errorf("deploy wrote %q; V2 does not load .opencode/tools and nothing is shipped there", rel)
		case strings.HasPrefix(rel, ".mosaic/") || owned[rel]:
		case harnessConfigName.MatchString(rel):
			t.Errorf("deploy created the harness config file %q; deploy never creates or edits harness settings", rel)
		}
	}
}

// readWorkspaceFile returns the text of a workspace file.
func readWorkspaceFile(t *testing.T, ws, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// v2Rule is one permission rule of a deployed V2 agent.
type v2Rule struct{ Action, Resource, Effect string }

// v2Agent is the frontmatter of a deployed or rendered V2 agent.
type v2Agent struct {
	Keys  map[string]any
	Mode  string
	Rules []v2Rule
}

// parseV2Agent reads the frontmatter of an agent file's text.
func parseV2Agent(t *testing.T, text string) v2Agent {
	t.Helper()
	parts := strings.SplitN(text, "---", 3)
	if len(parts) < 3 {
		t.Fatalf("agent text has no frontmatter:\n%s", text)
	}
	var keys map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &keys); err != nil {
		t.Fatalf("parse frontmatter: %v\n%s", err, parts[1])
	}
	agent := v2Agent{Keys: keys}
	agent.Mode, _ = keys["mode"].(string)
	items, _ := keys["permissions"].([]any)
	for _, item := range items {
		m, _ := item.(map[string]any)
		str := func(k string) string { s, _ := m[k].(string); return s }
		agent.Rules = append(agent.Rules, v2Rule{str("action"), str("resource"), str("effect")})
	}
	return agent
}

// effectFor evaluates the rules as V2 does: the last rule whose action and resource match
// decides and no match asks.
func (a v2Agent) effectFor(action, resource string) string {
	effect := "ask"
	for _, r := range a.Rules {
		if v2Wildcard(r.Action, action) && v2Wildcard(r.Resource, resource) {
			effect = r.Effect
		}
	}
	return effect
}

func v2Wildcard(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.MustCompile("^" + strings.Join(parts, ".*") + "$").MatchString(s)
}

// requireV2Shape fails unless the agent has the V2 frontmatter shape: the wanted mode, a
// rules list under permissions, and neither the V1 permission key nor the generic tools key.
func requireV2Shape(t *testing.T, a v2Agent, wantMode string) {
	t.Helper()
	if a.Mode != wantMode {
		t.Errorf("mode = %q, want %q", a.Mode, wantMode)
	}
	if len(a.Rules) == 0 {
		t.Error("no permission rules under the permissions key")
	}
	for _, gone := range []string{"permission", "tools", "name"} {
		if _, ok := a.Keys[gone]; ok {
			t.Errorf("agent still carries the key %q", gone)
		}
	}
}

// decodeJSONMap decodes one JSON document into a map.
func decodeJSONMap(t *testing.T, doc string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, doc)
	}
	return m
}
