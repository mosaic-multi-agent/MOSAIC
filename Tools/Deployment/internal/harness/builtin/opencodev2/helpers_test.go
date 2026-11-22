package opencodev2_test

// helpers_test.go holds the fixtures shared by the OpenCode V2 module tests: the frozen
// catalog content directories, a constructor over them, and a reader for the permission
// rules a module renders. Nothing in this file is a test itself.

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/opencodev2"
	"mosaic-deploy/internal/harness/registry"
)

// deploymentTestdata returns the absolute path inside Tools/Deployment/testdata.
func deploymentTestdata(t *testing.T, parts ...string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(append([]string{"..", "..", "..", "..", "testdata"}, parts...)...))
	if err != nil {
		t.Fatalf("resolve testdata path: %v", err)
	}
	return abs
}

// frozenContentDirs resolves a harness's injection content directory inside the frozen
// fixture catalog (testdata/frozen-catalog/Catalog), without a stack or identity file.
func frozenContentDirs(t *testing.T) func(string) (string, error) {
	t.Helper()
	catalogRoot := filepath.Join(deploymentTestdata(t, "frozen-catalog"), "Catalog")
	return func(harnessDirName string) (string, error) {
		return catalogpaths.HarnessInjectionsDir(catalogRoot, harnessDirName), nil
	}
}

// contentDirsUnder resolves a harness's injection content directory below a temp
// MOSAIC-style root (<root>/Catalog/HarnessInjections/<harness dir>).
func contentDirsUnder(root string) func(string) (string, error) {
	return func(harnessDirName string) (string, error) {
		return filepath.Join(root, catalogpaths.MosaicRelHarnessInjectionsDir, harnessDirName), nil
	}
}

// newModule constructs the OpenCode V2 module over the frozen catalog's injection files.
func newModule(t *testing.T) domain.HarnessModule {
	t.Helper()
	mod, err := opencodev2.New(registry.BuiltinOptions{ContentDir: frozenContentDirs(t)})
	if err != nil {
		t.Fatalf("opencodev2.New over the frozen catalog: %v", err)
	}
	return mod
}

// loadFixtureBundle loads one hook bundle of the tool-local fixture catalog that carries an
// opencode-v2 variant (testdata/catalog/hook-plugin-v2).
func loadFixtureBundle(t *testing.T, id string) domain.HookBundle {
	t.Helper()
	root := deploymentTestdata(t, "catalog", "hook-plugin-v2")
	cat, err := catalog.LoadSingleRoot(root, root)
	if err != nil {
		t.Fatalf("load fixture catalog %s: %v", root, err)
	}
	bundle, ok := cat.Hook(id)
	if !ok {
		t.Fatalf("fixture catalog has no hook bundle %q", id)
	}
	return bundle
}

// ruleT is one rendered permission rule as plain strings.
type ruleT struct{ action, resource, effect string }

// readRules reads a rendered permissions value: a list of mappings with the pairs action,
// resource and effect. It fails the test when the value has any other shape.
func readRules(t *testing.T, v domain.FieldValue) []ruleT {
	t.Helper()
	if v.Kind != domain.KindList {
		t.Fatalf("permissions value must be a list of rules, got kind %q", v.Kind)
	}
	var out []ruleT
	for i, item := range v.Items {
		if item.Kind != domain.KindMapping || len(item.Pairs) != 3 {
			t.Fatalf("rule %d must be a mapping of three pairs, got %+v", i, item)
		}
		out = append(out, ruleT{item.Pairs[0].Value.Scalar, item.Pairs[1].Value.Scalar, item.Pairs[2].Value.Scalar})
	}
	return out
}

// rulesFor renders the module's permission rules for the tool request.
func rulesFor(t *testing.T, mod domain.HarnessModule, req domain.ToolRequest) []ruleT {
	t.Helper()
	res, err := mod.Tools(req)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(res.Fields) == 0 || res.Fields[0].Key != "permissions" {
		t.Fatalf("first tool field must be %q, got %+v", "permissions", res.Fields)
	}
	return readRules(t, res.Fields[0].Value)
}

// wildcardMatch reports whether pattern, in which * matches any run of characters, matches s.
func wildcardMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.MustCompile("^" + strings.Join(parts, ".*") + "$").MatchString(s)
}

// effectFor evaluates rules as OpenCode V2 does for a call of action on resource: the last
// rule whose action and resource patterns match decides, and no match asks.
func effectFor(rules []ruleT, action, resource string) string {
	effect := "ask"
	for _, r := range rules {
		if wildcardMatch(r.action, action) && wildcardMatch(r.resource, resource) {
			effect = r.effect
		}
	}
	return effect
}
