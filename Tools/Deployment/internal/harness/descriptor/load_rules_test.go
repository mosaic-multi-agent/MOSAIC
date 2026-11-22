package descriptor_test

// load_rules_test.go specifies how the descriptor loader reads the rules tool shape: the fixed
// leading and trailing rules, the per-tool permission action, and the validation of both.
//
// Every rejection test first proves that the same document parses when the offending line is
// valid, so a rejection can only come from the rule under test.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// rulesDescriptorTemplate is a rules-shape descriptor. The markers are replaced per test.
const rulesDescriptorTemplate = `schema_version: "1"
id: "rules-test-harness"
display_name: "Rules Test Harness"
tools:
  shape: @SHAPE@
  rules:
    leading:
      - {action: "*", resource: "*", effect: deny}
    trailing:
      - {action: @TRAILING_ACTION@, resource: @TRAILING_RESOURCE@, effect: @TRAILING_EFFECT@}
  universe:
    - name: "read"
      unused: deny
      by_convention: false
    - name: "write"
      action: @UNIVERSE_ACTION@
      unused: deny
      by_convention: false
    - name: "patch"
      action: "edit"
      unused: deny
      by_convention: false
  mappings:
    - generic: "file_read"
      destinations:
        - to: main
          names: ["read"]
frontmatter:
  tools_key: @TOOLS_KEY@
`

type rulesDoc struct {
	shape, trailingAction, trailingResource, trailingEffect, universeAction, toolsKey string
}

func validRulesDoc() rulesDoc {
	return rulesDoc{
		shape: "rules", trailingAction: "external_directory", trailingResource: `"*"`,
		trailingEffect: "ask", universeAction: `"edit"`, toolsKey: `"permissions"`,
	}
}

func (r rulesDoc) yaml() []byte {
	return []byte(strings.NewReplacer(
		"@SHAPE@", r.shape,
		"@TRAILING_ACTION@", r.trailingAction,
		"@TRAILING_RESOURCE@", r.trailingResource,
		"@TRAILING_EFFECT@", r.trailingEffect,
		"@UNIVERSE_ACTION@", r.universeAction,
		"@TOOLS_KEY@", r.toolsKey,
	).Replace(rulesDescriptorTemplate))
}

// fixtureSource names the parsed document. Loader errors are prefixed with it, so it must not
// contain any failure token the tests look for (rules, action, resource, tools_key).
const fixtureSource = "inline:fixture"

func parseRulesDoc(r rulesDoc) (*domain.HarnessDescriptor, error) {
	return descriptor.Parse(r.yaml(), fixtureSource)
}

// assertRejectedBecause checks the valid document parses and the broken one fails with an
// error that mentions token.
func assertRejectedBecause(t *testing.T, broken rulesDoc, token string) {
	t.Helper()
	if _, err := parseRulesDoc(validRulesDoc()); err != nil {
		t.Fatalf("the valid rules descriptor must parse: %v", err)
	}
	_, err := parseRulesDoc(broken)
	if err == nil {
		t.Fatalf("want a load error mentioning %q, got none", token)
	}
	if !strings.Contains(err.Error(), token) {
		t.Errorf("load error should mention %q, got: %v", token, err)
	}
}

func TestParse_RulesShape_ReadsShapeFixedRulesAndUniverseAction(t *testing.T) {
	d, err := parseRulesDoc(validRulesDoc())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if d.Tools.Shape != domain.ShapeRules {
		t.Errorf("Shape = %q, want %q", d.Tools.Shape, domain.ShapeRules)
	}
	wantLeading := []domain.PermissionRule{{Action: "*", Resource: "*", Effect: domain.EffectDeny}}
	wantTrailing := []domain.PermissionRule{{Action: "external_directory", Resource: "*", Effect: domain.EffectAsk}}
	if !reflect.DeepEqual(d.Tools.Rules.Leading, wantLeading) {
		t.Errorf("Leading = %v, want %v", d.Tools.Rules.Leading, wantLeading)
	}
	if !reflect.DeepEqual(d.Tools.Rules.Trailing, wantTrailing) {
		t.Errorf("Trailing = %v, want %v", d.Tools.Rules.Trailing, wantTrailing)
	}
	actions := map[string]string{}
	for _, u := range d.Tools.Universe {
		actions[u.Name] = u.Action
	}
	if actions["write"] != "edit" || actions["patch"] != "edit" {
		t.Errorf("write and patch must carry action edit, got %v", actions)
	}
	if actions["read"] != "" {
		t.Errorf("read declares no action, got %q", actions["read"])
	}
}

func TestParse_RulesShape_KeepsDeclaredRuleOrder(t *testing.T) {
	src := strings.Replace(string(validRulesDoc().yaml()),
		`      - {action: "*", resource: "*", effect: deny}`,
		`      - {action: "*", resource: "*", effect: deny}
      - {action: "execute", resource: "*", effect: deny}`, 1)

	d, err := descriptor.Parse([]byte(src), fixtureSource)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var got []string
	for _, r := range d.Tools.Rules.Leading {
		got = append(got, r.Action)
	}
	if !reflect.DeepEqual(got, []string{"*", "execute"}) {
		t.Errorf("Leading actions = %v, want [* execute] in declared order", got)
	}
}

func TestParse_RulesBlockIsRejectedWhenTheShapeIsNotRules(t *testing.T) {
	for _, shape := range []string{"permission", "list"} {
		doc := validRulesDoc()
		doc.shape = shape
		doc.universeAction = `""`
		assertRejectedBecause(t, doc, "rules")
	}
}

// permissionDescriptorTemplate is a permission-shape descriptor with no rules block; @ACTION@
// is the line (or nothing) placed under the "write" universe entry.
const permissionDescriptorTemplate = `schema_version: "1"
id: "perm-test-harness"
display_name: "Perm Test Harness"
tools:
  shape: permission
  universe:
    - name: "read"
      unused: deny
      by_convention: false
    - name: "write"
@ACTION@      unused: deny
      by_convention: false
  mappings:
    - generic: "file_read"
      destinations:
        - to: main
          names: ["read"]
frontmatter:
  tools_key: "permission"
`

func TestParse_UniverseActionIsRejectedWhenTheShapeIsNotRules(t *testing.T) {
	valid := strings.Replace(permissionDescriptorTemplate, "@ACTION@", "", 1)
	broken := strings.Replace(permissionDescriptorTemplate, "@ACTION@", "      action: \"edit\"\n", 1)
	if _, err := descriptor.Parse([]byte(valid), "inline:valid"); err != nil {
		t.Fatalf("a permission descriptor without actions must parse: %v", err)
	}

	_, err := descriptor.Parse([]byte(broken), "inline:action-on-permission")

	if err == nil || !strings.Contains(err.Error(), "action") || !strings.Contains(err.Error(), "shape") {
		t.Errorf("universe action on a permission-shape descriptor must be rejected for its shape, naming action, got %v", err)
	}
}

func TestParse_RulesShapeRequiresAToolsKey(t *testing.T) {
	doc := validRulesDoc()
	doc.toolsKey = `""`

	assertRejectedBecause(t, doc, "tools_key")
}

func TestParse_FixedRuleNeedsActionResourceAndAKnownEffect(t *testing.T) {
	cases := map[string]struct {
		mutate func(*rulesDoc)
		token  string
	}{
		"empty action":    {func(r *rulesDoc) { r.trailingAction = `""` }, "action"},
		"empty resource":  {func(r *rulesDoc) { r.trailingResource = `""` }, "resource"},
		"unknown effect":  {func(r *rulesDoc) { r.trailingEffect = "maybe" }, "effect"},
		"capitalised one": {func(r *rulesDoc) { r.trailingEffect = "Allow" }, "effect"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			doc := validRulesDoc()
			c.mutate(&doc)
			assertRejectedBecause(t, doc, c.token)
		})
	}
}

func TestParse_FixedRuleResourceMayNotBeAnAbsolutePath(t *testing.T) {
	for _, res := range []string{`"/etc/*"`, `"\\share\\x"`, `"~/code"`, `"$HOME/code"`, `"C:\\code"`, `"c:/code"`} {
		t.Run(res, func(t *testing.T) {
			doc := validRulesDoc()
			doc.trailingResource = res
			assertRejectedBecause(t, doc, "resource")
		})
	}
}

func TestParse_FixedRuleResourceRelativeAndWildcardFormsAreAccepted(t *testing.T) {
	for _, res := range []string{`"*"`, `"src/*"`, `"**/*.env"`, `"api:v1/*"`} {
		doc := validRulesDoc()
		doc.trailingResource = res
		if _, err := parseRulesDoc(doc); err != nil {
			t.Errorf("resource %s must be accepted: %v", res, err)
		}
	}
}

func TestParse_UniverseActionStarIsRejected(t *testing.T) {
	doc := validRulesDoc()
	doc.universeAction = `"*"`

	assertRejectedBecause(t, doc, "action")
}

func TestParse_UnknownKeyInsideARuleIsRejected(t *testing.T) {
	valid := string(validRulesDoc().yaml())
	broken := strings.Replace(valid, "effect: ask}", "effect: ask, note: x}", 1)
	if _, err := descriptor.Parse([]byte(valid), "inline:valid"); err != nil {
		t.Fatalf("valid document must parse: %v", err)
	}

	_, err := descriptor.Parse([]byte(broken), "inline:broken")

	if err == nil || !strings.Contains(err.Error(), "note") {
		t.Errorf("an unknown key inside a rule must be rejected naming the key, got %v", err)
	}
}
