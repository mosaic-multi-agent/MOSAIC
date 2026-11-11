package app_test

// rerun_preserve_helpers_test.go holds the builders and assertions shared by the re-run
// preservation tests: a temporary copy of the tool-local parity catalog whose item versions
// can be bumped, a service over that copy with the four real built-in harness modules, a first
// deploy to a fresh workspace, planting of user state (project regions, custom regions, hand-set
// tools) into the deployed files, and readers for the files a later run wrote. Nothing here
// reads the repository Catalog.

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

const (
	// rpDeployedModel is the model the first deploy writes into every agent.
	rpDeployedModel = "deployed-model-1"
	// rpRunModel is a model a later run supplies explicitly for one agent.
	rpRunModel = "run-model-2"
	// rpTierModel is a model a later run supplies for a whole tier.
	rpTierModel = "tier-model-3"
)

// rpCatalogFile maps an agent key to its source file inside the catalog copy.
var rpCatalogFile = map[string]string{
	"parity-runner":       "Subagents/Execution/parity-runner.md",
	"orchestrator":        "Orchestrator/orchestrator.md",
	"orchestrator-script": "Orchestrator/orchestrator-script.md",
	"parity-helper":       "UtilityAgents/parity-helper.md",
}

// rpBumpedVersion is the catalog version each sample agent has after rpBumpAll.
var rpBumpedVersion = map[string]string{
	"parity-runner":       "1.5.0",
	"orchestrator":        "8.4.0",
	"orchestrator-script": "3.5.0",
	"parity-helper":       "1.2.0",
}

// rpProjectRegion names the project region each sample agent source carries; an agent absent
// from the map has none.
var rpProjectRegion = map[string]string{
	"parity-runner": "CodebaseContext",
	"orchestrator":  "ContextLimits",
}

// rpAllAgentKeys are the agents a first deploy gives a model, in a fixed order.
var rpAllAgentKeys = []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper", "parity-checker"}

// rpCopyCatalog copies the tool-local parity catalog into a temporary directory and returns
// the copy's root.
func rpCopyCatalog(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "parity", "catalog")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy parity catalog: %v", err)
	}
	return dst
}

var rpVersionLine = regexp.MustCompile(`(?m)^version: .*$`)

// rpBump sets the frontmatter version of a catalog source file; a file with no version line
// fails the test.
func rpBump(t *testing.T, catRoot, rel, version string) {
	t.Helper()
	path := filepath.Join(catRoot, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	loc := rpVersionLine.FindIndex(data)
	if loc == nil {
		t.Fatalf("%s has no version line", rel)
	}
	out := string(data[:loc[0]]) + "version: " + version + string(data[loc[1]:])
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// rpBumpAll raises the catalog version of every sample agent, so each deployed copy is stale.
func rpBumpAll(t *testing.T, catRoot string) {
	t.Helper()
	for key, rel := range rpCatalogFile {
		rpBump(t, catRoot, rel, rpBumpedVersion[key])
	}
}

// rpAddWorkflow adds a second workflow, a copy of the sample one under another id, to the
// catalog copy and to its index.
func rpAddWorkflow(t *testing.T, catRoot, id string) {
	t.Helper()
	dir := filepath.Join(catRoot, "Workflows")
	src, err := os.ReadFile(filepath.Join(dir, "Build", "parity-flow.md"))
	if err != nil {
		t.Fatalf("read sample workflow: %v", err)
	}
	body := strings.ReplaceAll(string(src), "parity-flow", id)
	if err := os.WriteFile(filepath.Join(dir, "Build", id+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write workflow %s: %v", id, err)
	}
	idx, err := os.ReadFile(filepath.Join(dir, "Index.md"))
	if err != nil {
		t.Fatalf("read workflow index: %v", err)
	}
	text := string(idx)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += "| " + id + " | Build | 1.0 | Second Parity Workflow | MOSAIC | `Build/" + id + ".md` |\n"
	if err := os.WriteFile(filepath.Join(dir, "Index.md"), []byte(text), 0o644); err != nil {
		t.Fatalf("write workflow index: %v", err)
	}
}

// rpService wires a service over the catalog copy. The catalog is read when the service is
// built, so build it after any version bump.
func rpService(t *testing.T, catRoot string) app.Service {
	t.Helper()
	abs, err := filepath.Abs(catRoot)
	if err != nil {
		t.Fatalf("resolve catalog copy: %v", err)
	}
	cat, err := catalog.Load(t.TempDir(), abs)
	if err != nil {
		t.Fatalf("load catalog copy: %v", err)
	}
	return parityServiceWith(t, func(d *app.Deps) { d.Catalog = cat })
}

// rpSkipModels skips the tier and agent model questions, so a model the run does not supply
// stays unresolved instead of being asked.
func rpSkipModels() map[domain.QuestionID]bool {
	return map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true}
}

// rpModels maps every key to one model id.
func rpModels(model string, keys ...string) map[string]string {
	out := map[string]string{}
	for _, k := range keys {
		out[k] = model
	}
	return out
}

// rpFirst selects what the first deploy puts into the workspace.
type rpFirst struct {
	utility []string // utility agents to deploy
	infra   []string // infrastructure agents to deploy
}

// rpFirstDeploy deploys the sample workflow, the orchestrators, the workflow's subagent and the
// selected extras to a fresh workspace, every agent with rpDeployedModel, and returns the
// workspace.
func rpFirstDeploy(t *testing.T, harness, catRoot string, f rpFirst) string {
	t.Helper()
	ws := t.TempDir()
	req := app.DeployRequest{
		HarnessID: harness, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs:            []string{"parity-flow"},
		UtilityAgentIDs:        append([]string{}, f.utility...),
		InfrastructureAgentIDs: append([]string{}, f.infra...),
		HookIDs:                []string{},
		AgentModels:            rpModels(rpDeployedModel, rpAllAgentKeys...),
		AutoConfirmPlan:        true,
	}
	if _, err := rpService(t, catRoot).DeployNew(context.Background(), req); err != nil {
		t.Fatalf("first deploy to %s: %v", harness, err)
	}
	return ws
}

// rpAgentWritePath returns the absolute path of an agent's deployed file.
func rpAgentWritePath(t *testing.T, ws, harness, key string) string {
	t.Helper()
	dir := rbModule(t, harness).Descriptor().Paths.Agents.Project
	ext := ".md"
	if harness == hCLI || harness == hVSCode {
		ext = ".agent.md"
	}
	return filepath.Join(ws, filepath.FromSlash(dir), key+ext)
}

// rpAgentExists reports whether an agent has a deployed file.
func rpAgentExists(t *testing.T, ws, harness, key string) bool {
	t.Helper()
	_, err := os.Stat(rpAgentWritePath(t, ws, harness, key))
	return err == nil
}

// rpSentinels name the user text planted into one agent.
func rpProjectSentinel(key string) string { return "PROJECT-SENTINEL-" + key }
func rpCustomSentinel(key string) string  { return "CUSTOM-SENTINEL-" + key }

// rpToolsLine matches the frontmatter tools line of the harnesses that keep tools on one line.
var rpToolsLine = regexp.MustCompile(`(?m)^tools: .*$`)

// rpHandSetTools returns doc with a custom tool added by hand. The tool is not in any
// harness's built-in tool universe, so an update keeps it while built-in tools follow the
// catalog.
func rpHandSetTools(t *testing.T, harness, doc string) string {
	t.Helper()
	switch harness {
	case hClaude:
		return rpToolsLine.ReplaceAllString(doc, "tools: hand-set-tool")
	case hCLI, hVSCode:
		return rpToolsLine.ReplaceAllString(doc, "tools: ['hand-set-tool']")
	case hOpen:
		if !strings.Contains(doc, "  webfetch: deny") {
			t.Fatalf("opencode file has no permission block to add a custom tool to")
		}
		return strings.Replace(doc, "  webfetch: deny", "  webfetch: deny\n  hand-set-tool: allow", 1)
	}
	t.Fatalf("unknown harness %q", harness)
	return doc
}

// rpHandSetToolsMarker matches the custom tool rpHandSetTools adds, wherever the tool sits
// among the built-in tools the catalog produces.
func rpHandSetToolsMarker(t *testing.T, harness string) *regexp.Regexp {
	t.Helper()
	switch harness {
	case hClaude:
		return regexp.MustCompile(`(?m)^tools: .*\bhand-set-tool\b.*$`)
	case hCLI, hVSCode:
		return regexp.MustCompile(`(?m)^tools: \[.*'hand-set-tool'.*\]\r?$`)
	case hOpen:
		return regexp.MustCompile(`(?m)^  hand-set-tool: allow\r?$`)
	}
	t.Fatalf("unknown harness %q", harness)
	return nil
}

// rpPlant writes user state into the deployed files of keys: text in the agent's project
// region (when its source has one), a custom region, and a hand-set tool choice.
func rpPlant(t *testing.T, ws, harness string, keys ...string) {
	t.Helper()
	for _, key := range keys {
		path := rpAgentWritePath(t, ws, harness, key)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read deployed %s: %v", key, err)
		}
		doc := string(data)
		if region, ok := rpProjectRegion[key]; ok {
			doc = rpFillProjectRegion(t, doc, region, rpProjectSentinel(key))
		}
		doc = strings.TrimRight(doc, "\r\n") + "\n\n<UserNotes type=\"custom\">\n" + rpCustomSentinel(key) + "\n</UserNotes>\n"
		doc = rpHandSetTools(t, harness, doc)
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatalf("write planted %s: %v", key, err)
		}
	}
}

// rpFillProjectRegion puts text on the line after the opening tag of a project region.
func rpFillProjectRegion(t *testing.T, doc, region, text string) string {
	t.Helper()
	open := "<" + region + ` type="project">`
	at := strings.Index(doc, open)
	if at < 0 {
		t.Fatalf("deployed file has no %s region", region)
	}
	eol := strings.Index(doc[at:], "\n")
	if eol < 0 {
		t.Fatalf("%s region has no line end", region)
	}
	cut := at + eol + 1
	return doc[:cut] + text + "\n" + doc[cut:]
}

// rpReadAgent returns the deployed file of an agent.
func rpReadAgent(t *testing.T, ws, harness, key string) string {
	t.Helper()
	return parityAgentFile(t, ws, harness, key)
}

// rpAssertKept checks that the planted project region text, custom region and hand-set tool
// choice of an agent are in got.
func rpAssertKept(t *testing.T, harness, key, got string) {
	t.Helper()
	if region, ok := rpProjectRegion[key]; ok {
		re := regexp.MustCompile(`<` + region + ` type="project">\s*` + rpProjectSentinel(key) + `\s*</` + region + `>`)
		if !re.MatchString(got) {
			t.Errorf("%s: project region %s lost its text", key, region)
		}
	}
	custom := regexp.MustCompile(`<UserNotes type="custom">\s*` + rpCustomSentinel(key) + `\s*</UserNotes>`)
	if !custom.MatchString(got) {
		t.Errorf("%s: custom region lost its text", key)
	}
	if marker := rpHandSetToolsMarker(t, harness); !marker.MatchString(got) {
		t.Errorf("%s: hand-set tool choice was replaced; pattern %s not found", key, marker)
	}
}

// rpAssertRewritten checks that the file carries the bumped catalog version, which proves the
// run rewrote it.
func rpAssertRewritten(t *testing.T, key, got string) {
	t.Helper()
	v, ok := rbScalar(t, got, "mosaic_version")
	if !ok || v != rpBumpedVersion[key] {
		t.Errorf("%s: mosaic_version = %q (present %v), want %q: the file was not rewritten", key, v, ok, rpBumpedVersion[key])
	}
}

// rpAssertModel checks the model key of a deployed file.
func rpAssertModel(t *testing.T, key, got, want string) {
	t.Helper()
	if m, ok := rbScalar(t, got, "model"); !ok || m != want {
		t.Errorf("%s: model = %q (present %v), want %q", key, m, ok, want)
	}
}

// rpManagedRegion returns the trimmed inner text of a managed region of a deployed file.
func rpManagedRegion(t *testing.T, doc, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<` + name + ` type="managed">(.*?)</` + name + `>`)
	m := re.FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("deployed file has no managed %s region", name)
	}
	return strings.TrimSpace(m[1])
}

// rpModelTodos returns the agent keys among keys that have a model entry in the run's TODO list.
func rpModelTodos(sum domain.RunSummary, keys ...string) []string {
	var out []string
	for _, item := range sum.Todos {
		if item.Category != domain.TodoModels {
			continue
		}
		for _, k := range keys {
			if item.Subject == k {
				out = append(out, k)
			}
		}
	}
	return out
}
