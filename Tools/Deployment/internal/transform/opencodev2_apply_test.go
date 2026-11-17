package transform_test

// opencodev2_apply_test.go runs the real embedded opencode-v2 descriptor through
// transform.Apply: an agent declaring generic tools, the orchestrator with the
// {tool-permissions} placeholder, a rewrite of a deployed file that keeps the user's custom
// rules, and the tool deny and append options. The descriptor-level rendering has its own
// tests; these pin that the whole transform produces the same complete rule set.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-common/harness/builtindesc"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
	"mosaic-deploy/internal/transform"
)

// openCodeV2Module returns a harness module backed by the embedded opencode-v2 descriptor.
func openCodeV2Module(t *testing.T) domain.HarnessModule {
	t.Helper()
	src, ok := builtindesc.Raw("opencode-v2")
	if !ok || len(src) == 0 {
		t.Fatal("builtin descriptor opencode-v2 missing from Common")
	}
	d, err := descriptor.Parse(src, "builtin:opencode-v2")
	if err != nil {
		t.Fatalf("parse builtin opencode-v2: %v", err)
	}
	return &descriptorModule{desc: d}
}

const v2AgentSource = `---
id: 300
version: 1.0.0
description: V2 test agent
tools: [file_read, file_write, terminal]
---
Body.
`

const v2OrchestratorSource = `---
version: 6.0.0
name: orchestrator
description: V2 test orchestrator
tools: {tool-permissions}
---
Body.
`

func v2Request(t *testing.T, source, key string) transform.Request {
	t.Helper()
	return transform.Request{
		Source: []byte(source),
		Kind:   domain.ArtifactAgent,
		Key:    key,
		Module: openCodeV2Module(t),
		Model:  domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:  domain.ScopeProject,
	}
}

func TestApply_OpenCodeV2_AgentGetsDenyByDefaultWithExactlyTheDeclaredToolsAllowed(t *testing.T) {
	doc, out := applyAndParse(t, v2Request(t, v2AgentSource, "v2-agent"))

	assertRules(t, "permissions", outputRules(t, doc, out),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("edit"), rlAllow("shell"), rlOutside})
}

func TestApply_OpenCodeV2_OrchestratorPlaceholderYieldsTheCompleteRuleSet(t *testing.T) {
	doc, out := applyAndParse(t, v2Request(t, v2OrchestratorSource, "orchestrator"))

	assertRules(t, "permissions", outputRules(t, doc, out), []rl{
		rlDenyAll,
		rlAllow("read"), rlAllow("edit"), rlAllow("glob"), rlAllow("grep"),
		rlAllow("shell"), rlAllow("question"), rlAllow("subagent"),
		rlOutside,
	})
	if strings.Contains(out, "{tool-permissions}") {
		t.Errorf("the placeholder must not survive into the output:\n%s", out)
	}
}

// v2Deployed is a deployed V2 file: a stale built-in allow, custom MCP rules and a rule the
// user narrowed to a path.
const v2Deployed = `---
mosaic_id: 300
version: 1.0.0
transform_version: 1.0.0
permissions:
  - action: "*"
    resource: "*"
    effect: deny
  - action: websearch
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

func TestApply_OpenCodeV2_RewriteKeepsCustomRulesAndFollowsTheCatalogForBuiltIns(t *testing.T) {
	req := v2Request(t, v2AgentSource, "v2-agent")
	req.Deployed = []byte(v2Deployed)

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out), []rl{
		rlDenyAll,
		rlAllow("read"), rlAllow("edit"), rlAllow("shell"),
		rlAllow("hitl_*"), {"srv_lookup", "src/*", "ask"},
		rlOutside,
	})
}

func TestApply_OpenCodeV2_RewriteIsIdempotent(t *testing.T) {
	req := v2Request(t, v2AgentSource, "v2-agent")
	req.Deployed = []byte(v2Deployed)
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

func TestDenyTools_OpenCodeV2_McpAndBuiltInDenialsWriteNoRule(t *testing.T) {
	plain := v2Request(t, v2AgentSource, "v2-agent")
	denied := v2Request(t, v2AgentSource, "v2-agent")
	denied.DenyTools = []string{"srv_*", "srv_lookup", "shell", "patch"}

	plainRes, err := transform.Apply(plain)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	deniedRes, err := transform.Apply(denied)
	if err != nil {
		t.Fatalf("Apply with DenyTools: %v", err)
	}

	if !bytes.Equal(plainRes.Output, deniedRes.Output) {
		t.Errorf("DenyTools must add no rule for the V2 target (the leading deny-all covers it, and a deny of the shared edit action would remove file writing).\nwithout:\n%s\nwith:\n%s",
			plainRes.Output, deniedRes.Output)
	}
}

func TestAppendTools_OpenCodeV2_McpNameIsAllowedJustBeforeTheOutsideRule(t *testing.T) {
	req := v2Request(t, v2AgentSource, "v2-agent")
	req.AppendTools = []string{"hitl_*"}

	doc, out := applyAndParse(t, req)

	assertRules(t, "permissions", outputRules(t, doc, out), []rl{
		rlDenyAll,
		rlAllow("read"), rlAllow("edit"), rlAllow("shell"),
		rlAllow("hitl_*"),
		rlOutside,
	})
}
