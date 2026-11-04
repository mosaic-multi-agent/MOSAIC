package opencode_test

// custom_tools_deploy_test.go: a deploy to OpenCode (transform.Apply with the OpenCode
// module) of an agent with a custom tool writes that custom tool into the permission
// mapping of the agent file; agents without custom tools are unaffected.

import (
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const customToolAgentSource = `---
id: 901
version: 1.0.0
description: Agent that uses an MCP server.
tools: [file_read, web_search]
---

You are a test agent.
`

// applyCustomToolAgent deploys customToolAgentSource to OpenCode and returns the parsed
// permission pairs of the output.
func applyCustomToolAgent(t *testing.T, custom map[string]string) []domain.FieldPair {
	t.Helper()
	result, err := transform.Apply(transform.Request{
		Source:      []byte(customToolAgentSource),
		Kind:        domain.ArtifactAgent,
		Key:         "mcp-user",
		Module:      newModule(t),
		Model:       testModel,
		Scope:       domain.ScopeProject,
		Role:        domain.RoleSubagent,
		CustomTools: custom,
	})
	if err != nil {
		t.Fatalf("transform.Apply: %v", err)
	}
	doc, err := docformat.Parse(result.Output)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	v, ok := doc.Frontmatter().Get("permission")
	if !ok {
		t.Fatalf("permission field absent from output:\n%s", result.Output)
	}
	return v.Pairs
}

func permissionValue(pairs []domain.FieldPair, key string) (string, int) {
	count, value := 0, ""
	for _, p := range pairs {
		if p.Key == key {
			count++
			value = p.Value.Scalar
		}
	}
	return value, count
}

func TestDeploy_CustomToolServerIsWrittenAsAllowInPermission(t *testing.T) {
	pairs := applyCustomToolAgent(t, map[string]string{"web_search": "my-server"})

	value, count := permissionValue(pairs, "my-server")
	if count != 1 || value != "allow" {
		t.Errorf("my-server: want exactly one pair with value allow, got %d pair(s) value %q; pairs: %v",
			count, value, pairs)
	}
}

func TestDeploy_CustomToolKeepsBuiltInPermissionPairsAndPutsServerLast(t *testing.T) {
	withCustom := applyCustomToolAgent(t, map[string]string{"web_search": "my-server"})
	without := applyCustomToolAgent(t, nil)

	if len(withCustom) != len(without)+1 {
		t.Fatalf("want %d pairs (built-ins + custom), got %d: %v", len(without)+1, len(withCustom), withCustom)
	}
	for i, p := range without {
		if withCustom[i].Key != p.Key {
			t.Errorf("pair %d key changed: want %q, got %q", i, p.Key, withCustom[i].Key)
		}
	}
	if last := withCustom[len(withCustom)-1]; last.Key != "my-server" || last.Value.Scalar != "allow" {
		t.Errorf("last pair: want my-server: allow, got %s: %s", last.Key, last.Value.Scalar)
	}
	if v, _ := permissionValue(withCustom, "read"); v != "allow" {
		t.Errorf("read: want allow, got %q", v)
	}
}

func TestDeploy_NoCustomToolsWritesOnlyUniversePairs(t *testing.T) {
	pairs := applyCustomToolAgent(t, nil)

	if len(pairs) != 15 {
		t.Errorf("want exactly the 15 universe pairs, got %d: %v", len(pairs), pairs)
	}
}
