package app_test

// rebuild_helpers_test.go holds the shared builders and assertions of the deployed-agent
// rebuild tests: real built-in harness modules fed from the tool-local frozen catalog, the
// tool-local deployed-agent fixtures, a catalog stub, and frontmatter readers.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentkey"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/claudecode"
	"mosaic-deploy/internal/harness/builtin/ghcpcli"
	"mosaic-deploy/internal/harness/builtin/opencode"
	"mosaic-deploy/internal/harness/builtin/vscodeghcp"
	"mosaic-deploy/internal/harness/registry"
)

// Harness ids of the built-in modules.
const (
	hClaude = "claude-code"
	hOpen   = "opencode"
	hCLI    = "ghcp-cli"
	hVSCode = "vscode-ghcp"
)

// rbModule builds a built-in harness module whose injection content comes from the frozen
// catalog under testdata/frozen-catalog.
func rbModule(t *testing.T, id string) domain.HarnessModule {
	t.Helper()
	catalogRoot := filepath.Join("..", "..", "testdata", "frozen-catalog", "Catalog")
	opts := registry.BuiltinOptions{ContentDir: func(dir string) (string, error) {
		return catalogpaths.HarnessInjectionsDir(catalogRoot, dir), nil
	}}
	var (
		m   domain.HarnessModule
		err error
	)
	switch id {
	case hClaude:
		m, err = claudecode.New(opts)
	case hOpen:
		m, err = opencode.New(opts)
	case hCLI:
		m, err = ghcpcli.New(opts)
	case hVSCode:
		m, err = vscodeghcp.New(opts)
	default:
		t.Fatalf("unknown harness id %q", id)
	}
	if err != nil {
		t.Fatalf("build %s module: %v", id, err)
	}
	return m
}

// rbFixture reads a deployed-agent fixture from testdata/rebuild.
func rbFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "rebuild", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// MockCatalogAgentLookup is a catalog stub keyed by numeric id and by agent key.
type MockCatalogAgentLookup struct {
	ByID  map[string]domain.Agent
	ByKey map[string]domain.Agent
}

func (m MockCatalogAgentLookup) AgentByNumericID(id string) (domain.Agent, bool) {
	a, ok := m.ByID[id]
	return a, ok
}

func (m MockCatalogAgentLookup) Agent(key string) (domain.Agent, bool) {
	a, ok := m.ByKey[key]
	return a, ok
}

// rbPair is one source-to-target harness pair.
type rbPair struct{ name, src, tgt string }

// rbPairs are the bounded pairs of the rebuild tests: every built-in harness is a source or
// a target and each frontmatter carry-over path is exercised.
var rbPairs = []rbPair{
	{"opencode-to-claude", hOpen, hClaude},
	{"claude-to-opencode", hClaude, hOpen},
	{"claude-to-vscode", hClaude, hVSCode},
	{"ghcpcli-to-claude", hCLI, hClaude},
}

// rbSourceFile names the deployed file of a source harness: its default agent key and the
// base name that harness would give it.
func rbSourceFile(src string, stampless bool) (key, fileName string) {
	key = "code-reviewer"
	if stampless {
		key = "notes-helper"
	}
	if src == hCLI {
		return key, key + ".agent.md"
	}
	return key, key + ".md"
}

// rbOpt adjusts a RebuildInput.
type rbOpt func(*testing.T, *app.RebuildInput)

func withFileName(name string) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) {
		in.FileName = name
		in.AgentKey = agentkey.FromFileName(name)
	}
}

func withCatalogLookup(c app.CatalogAgentLookup) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) { in.Catalog = c }
}

func withModelSelection(m domain.ModelSelection) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) { in.Model = m }
}

func withCustomToolAnswers(answers map[string]string) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) { in.CustomTools = answers }
}

func withSkippedTools(skipped map[string]bool) rbOpt {
	return func(_ *testing.T, in *app.RebuildInput) { in.SkippedTools = skipped }
}

// withSourceEdit rewrites the source text before the rebuild; an edit that changes nothing
// (for example a text that is absent from the fixture) fails the test.
func withSourceEdit(edit func(string) string) rbOpt {
	return func(t *testing.T, in *app.RebuildInput) {
		t.Helper()
		before := string(in.Source)
		after := edit(before)
		if after == before {
			t.Fatalf("source edit changed nothing; its target text is missing from the fixture")
		}
		in.Source = []byte(after)
	}
}

const rbModelID = "target-model-1"

// rbInput builds the input for a fixture of the source harness; stampless selects the
// stamp-less fixture.
func rbInput(t *testing.T, p rbPair, stampless bool, opts ...rbOpt) app.RebuildInput {
	t.Helper()
	suffix := "-full.md"
	if stampless {
		suffix = "-stampless.md"
	}
	_, fileName := rbSourceFile(p.src, stampless)
	in := app.RebuildInput{
		Source:       rbFixture(t, p.src+suffix),
		FileName:     fileName,
		AgentKey:     agentkey.FromFileName(fileName),
		SourceModule: rbModule(t, p.src),
		TargetModule: rbModule(t, p.tgt),
		Model:        domain.ModelSelection{ModelID: rbModelID, Origin: domain.OriginHarnessList},
		Timestamp:    "2026-10-31T12:00:00Z",
	}
	for _, o := range opts {
		o(t, &in)
	}
	return in
}

// rbRebuild runs RebuildDeployedAgent, failing the test on an error.
func rbRebuild(t *testing.T, in app.RebuildInput) (app.RebuildResult, string) {
	t.Helper()
	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}
	return res, string(res.Output)
}

// rbFrontmatter parses the output and returns its frontmatter.
func rbFrontmatter(t *testing.T, out string) *docformat.Frontmatter {
	t.Helper()
	doc, err := docformat.Parse([]byte(out))
	if err != nil {
		t.Fatalf("parse rebuilt output: %v\n%s", err, out)
	}
	return doc.Frontmatter()
}

// rbScalar returns a frontmatter scalar and whether the key exists.
func rbScalar(t *testing.T, out, key string) (string, bool) {
	t.Helper()
	v, ok := rbFrontmatter(t, out).Get(key)
	return v.Scalar, ok
}

// rbHasKey reports whether the output frontmatter holds the key.
func rbHasKey(t *testing.T, out, key string) bool {
	t.Helper()
	_, ok := rbFrontmatter(t, out).Get(key)
	return ok
}

// rbToolNames returns the tool names of a frontmatter key as a sorted set, whatever the
// shape: comma scalar, list, or permission mapping (allowed names only).
func rbToolNames(t *testing.T, out, key string) []string {
	t.Helper()
	v, ok := rbFrontmatter(t, out).Get(key)
	if !ok {
		return nil
	}
	var names []string
	switch v.Kind {
	case domain.KindScalar:
		for _, part := range strings.Split(v.Scalar, ",") {
			if s := strings.TrimSpace(part); s != "" {
				names = append(names, s)
			}
		}
	case domain.KindList:
		for _, item := range v.Items {
			names = append(names, item.Scalar)
		}
	case domain.KindMapping:
		for _, pair := range v.Pairs {
			if pair.Value.Scalar == string(domain.Allow) {
				names = append(names, pair.Key)
			}
		}
	}
	sort.Strings(names)
	return names
}

// sortedCopy returns a sorted copy of names.
func sortedCopy(names ...string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

// containsString reports whether list holds s.
func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// assertOnce asserts that text occurs exactly once in out.
func assertOnce(t *testing.T, out, text string) {
	t.Helper()
	if n := strings.Count(out, text); n != 1 {
		t.Errorf("%q must appear exactly once in the output, got %d", text, n)
	}
}

// removeLine deletes every line that starts with prefix.
func removeLine(prefix string) func(string) string {
	return func(s string) string {
		var kept []string
		for _, line := range strings.Split(s, "\n") {
			if !strings.HasPrefix(line, prefix) {
				kept = append(kept, line)
			}
		}
		return strings.Join(kept, "\n")
	}
}

// replaceText substitutes the first occurrence of old.
func replaceText(old, repl string) func(string) string {
	return func(s string) string { return strings.Replace(s, old, repl, 1) }
}

// equalStrings reports whether two string slices are equal element by element.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
