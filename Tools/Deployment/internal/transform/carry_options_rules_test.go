package transform_test

// carry_options_rules_test.go specifies the transform request options for a rules-shape
// target: AppendTools adds one whole-tool allow rule per permission action just before the
// trailing rules, DenyTools writes nothing (the leading deny-all already denies the tool, and a
// rule denying a shared action would remove its sibling tools), and a rewrite of a deployed
// file keeps the user's custom rules.

import (
	"bytes"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const rulesTargetDescriptorYAML = `schema_version: "1"
id: "rules-target-harness"
display_name: "Rules Target Harness"
tools:
  shape: rules
  rules:
    leading:
      - {action: "*", resource: "*", effect: deny}
    trailing:
      - {action: external_directory, resource: "*", effect: ask}
  mcp:
    server_form: "{server}_*"
    tool_form: "{server}_{tool}"
  universe:
    - name: "read"
      unused: deny
      by_convention: false
    - name: "write"
      action: "edit"
      unused: deny
      by_convention: false
    - name: "patch"
      action: "edit"
      unused: deny
      by_convention: false
    - name: "shell"
      unused: deny
      by_convention: false
  mappings:
    - generic: "generic_a"
      destinations:
        - to: main
          names: ["read"]
    - generic: "generic_b"
      destinations:
        - to: main
          names: ["shell"]
frontmatter:
  tools_key: "permissions"
`

const rulesTargetSource = `---
id: 200
version: 1.0.0
description: Rules target test agent
tools: [generic_a]
---
Body.
`

func rulesRequest(t *testing.T) transform.Request {
	t.Helper()
	return transform.Request{
		Source: []byte(rulesTargetSource),
		Kind:   domain.ArtifactAgent,
		Key:    "rules-agent",
		Module: newDescriptorModule(t, rulesTargetDescriptorYAML, "inline:rules-target"),
		Model:  domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:  domain.ScopeProject,
	}
}

func outputRules(t *testing.T, doc *docformat.Document, out string) []rl {
	t.Helper()
	v, ok := doc.Frontmatter().Get("permissions")
	if !ok {
		t.Fatalf("permissions field absent:\n%s", out)
	}
	return rlRead(v)
}

func TestApply_RulesTarget_WritesTheRenderedRuleList(t *testing.T) {
	doc, out := applyAndParse(t, rulesRequest(t))

	assertRules(t, "permissions", outputRules(t, doc, out), []rl{rlDenyAll, rlAllow("read"), rlOutside})
}

func TestAppendTools_Rules_AddsAnAllowRuleJustBeforeTheTrailingRule(t *testing.T) {
	req := rulesRequest(t)
	req.AppendTools = []string{"shell", "hitl_*"}

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("shell"), rlAllow("hitl_*"), rlOutside})
}

func TestAppendTools_Rules_NameIsMappedToItsPermissionAction(t *testing.T) {
	req := rulesRequest(t)
	req.AppendTools = []string{"write"}

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("edit"), rlOutside})
}

func TestAppendTools_Rules_EachActionIsAddedAtMostOnce(t *testing.T) {
	req := rulesRequest(t)
	req.AppendTools = []string{"write", "patch", "write", "shell", "shell"}

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("edit"), rlAllow("shell"), rlOutside})
}

func TestAppendTools_Rules_ActionAlreadyAllowedAddsNothing(t *testing.T) {
	req := rulesRequest(t)
	req.AppendTools = []string{"read"}

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out), []rl{rlDenyAll, rlAllow("read"), rlOutside})
}

func TestDenyTools_Rules_WritesNothing(t *testing.T) {
	plain := rulesRequest(t)
	denied := rulesRequest(t)
	denied.DenyTools = []string{"shell", "write", "srv_*"}

	plainRes, err := transform.Apply(plain)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	deniedRes, err := transform.Apply(denied)
	if err != nil {
		t.Fatalf("Apply with DenyTools: %v", err)
	}

	if !bytes.Equal(plainRes.Output, deniedRes.Output) {
		t.Errorf("DenyTools must write no rule for a rules-shape target; the leading deny-all already denies it.\nwithout:\n%s\nwith:\n%s",
			plainRes.Output, deniedRes.Output)
	}
}

func TestDenyTools_Rules_SharedActionOfAnAllowedSiblingStaysAllowed(t *testing.T) {
	req := rulesRequest(t)
	req.AppendTools = []string{"write"}
	req.DenyTools = []string{"patch"}

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("edit"), rlOutside})
}

// rulesDeployed is a deployed file of the rules target: a stale built-in allow, a custom
// server the user added, and a custom rule with a narrower resource.
const rulesDeployed = `---
mosaic_id: 200
version: 1.0.0
transform_version: 1.0.0
permissions:
  - action: "*"
    resource: "*"
    effect: deny
  - action: shell
    resource: "*"
    effect: allow
  - action: hitl_*
    resource: "*"
    effect: allow
  - action: srv_lookup
    resource: "src/*"
    effect: ask
  - action: external_directory
    resource: "*"
    effect: allow
---
Body.
`

func TestApply_RulesTarget_RewriteKeepsDeployedCustomRulesAndFollowsTheCatalogForBuiltIns(t *testing.T) {
	req := rulesRequest(t)
	req.Deployed = []byte(rulesDeployed)

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out), []rl{
		rlDenyAll, rlAllow("read"), rlAllow("hitl_*"), {"srv_lookup", "src/*", "ask"}, rlOutside,
	})
}

func TestApply_RulesTarget_RewriteIsIdempotent(t *testing.T) {
	req := rulesRequest(t)
	req.Deployed = []byte(rulesDeployed)
	first, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	req.Deployed = first.Output
	second, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	if !bytes.Equal(first.Output, second.Output) {
		t.Errorf("rewriting its own output changed it:\n%s\n---\n%s", first.Output, second.Output)
	}
}
