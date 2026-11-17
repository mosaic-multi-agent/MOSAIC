package builtin_test

// opencodev2_rules_helpers_test.go holds the fixtures shared by the opencode-v2 permission
// rule tests: a reader for the rendered rule list that does not depend on the domain's own
// rule parser, and an evaluator that applies V2's "last matching rule wins, no match asks".

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// ruleT is one rendered rule as plain strings.
type ruleT struct{ action, resource, effect string }

// readRules reads a rendered permissions value without the domain parser. It fails the test
// when the value is not a block list of three-pair mappings in the order action, resource,
// effect.
func readRules(t *testing.T, v domain.FieldValue) []ruleT {
	t.Helper()
	if v.Kind != domain.KindList {
		t.Fatalf("permissions value must be a list of rules, got kind %q", v.Kind)
	}
	if v.List != domain.ListBlock {
		t.Errorf("permissions list must use the block style, got %q", v.List)
	}
	var out []ruleT
	for i, item := range v.Items {
		if item.Kind != domain.KindMapping || len(item.Pairs) != 3 {
			t.Fatalf("rule %d must be a mapping of three pairs, got %+v", i, item)
		}
		keys := []string{item.Pairs[0].Key, item.Pairs[1].Key, item.Pairs[2].Key}
		if keys[0] != "action" || keys[1] != "resource" || keys[2] != "effect" {
			t.Fatalf("rule %d keys must be action, resource, effect in that order, got %v", i, keys)
		}
		out = append(out, ruleT{item.Pairs[0].Value.Scalar, item.Pairs[1].Value.Scalar, item.Pairs[2].Value.Scalar})
	}
	return out
}

// renderV2 maps a request through the embedded opencode-v2 descriptor and returns the rendered
// permission rules.
func renderV2(t *testing.T, req domain.ToolRequest) []ruleT {
	t.Helper()
	d := loadV2(t)
	res, err := descriptor.MapTools(d, req)
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	if len(res.Fields) == 0 || res.Fields[0].Key != "permissions" {
		t.Fatalf("first field must be %q, got %+v", "permissions", res.Fields)
	}
	return readRules(t, res.Fields[0].Value)
}

// renderGenerics renders the given generic tools with no custom names.
func renderGenerics(t *testing.T, generics ...string) []ruleT {
	t.Helper()
	return renderV2(t, domain.ToolRequest{AgentKey: "a", Generic: generics})
}

// wildcardMatch reports whether pattern, in which * matches any run of characters, matches s.
func wildcardMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.MustCompile("^" + strings.Join(parts, ".*") + "$").MatchString(s)
}

// effectFor evaluates rules as V2 does for a whole-tool call of action: the last rule whose
// action and resource match decides, and no match asks.
func effectFor(rules []ruleT, action string) string {
	effect := "ask"
	for _, r := range rules {
		if wildcardMatch(r.action, action) && wildcardMatch(r.resource, "src/file.go") {
			effect = r.effect
		}
	}
	return effect
}

// everyKnownAction lists the V2 actions the deny-by-default rule must cover: the universe,
// the default-on tools that have no generic mapping, an MCP tool of an undeclared server and
// an action V2 might add later.
var everyKnownAction = []string{
	"read", "edit", "glob", "grep", "shell", "webfetch", "websearch", "question", "skill",
	"subagent", "execute", "experimental_image_generate", "browser",
	"opencode_list_mcp_resources", "opencode_read_mcp_resource",
	"undeclared_lookup", "some_future_action",
}

// allowedActions returns the actions of everyKnownAction that rules allow.
func allowedActions(rules []ruleT) []string {
	var out []string
	for _, a := range everyKnownAction {
		if effectFor(rules, a) == "allow" {
			out = append(out, a)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	return slicesEqual(sortedCopy(a), sortedCopy(b))
}

// effects maps each action to its effect for readable failure output.
func effects(rules []ruleT, actions []string) map[string]string {
	out := map[string]string{}
	for _, a := range actions {
		out[a] = effectFor(rules, a)
	}
	return out
}
