package todo_test

// unknown_harness_tool_test.go verifies that a GapUnknownHarnessTool gap becomes a Tool
// mappings item, appears in the generated TODO file with the tool name, and stays distinct per
// owning agent.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

func unknownToolGap(owner, name string) domain.Gap {
	return domain.Gap{
		Kind:    domain.GapUnknownHarnessTool,
		Subject: name,
		Owner:   owner,
		Detail:  `tool "` + name + `" is neither a known GitHub Copilot CLI tool nor an MCP name; it ignores it`,
	}
}

func TestCollector_AddGap_UnknownHarnessTool_MapsToToolMappingsCategory(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(unknownToolGap("agent-a", "bogus-tool"))

	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d; want 1", len(items))
	}
	if items[0].Category != domain.TodoToolMappings {
		t.Errorf("Category = %q; want %q", items[0].Category, domain.TodoToolMappings)
	}
	if items[0].Subject != "bogus-tool" || items[0].Owner != "agent-a" {
		t.Errorf("item = %+v; want subject bogus-tool owned by agent-a", items[0])
	}
}

func TestRenderMarkdown_UnknownHarnessToolAppearsWithNameAndDetail(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(unknownToolGap("agent-a", "bogus-tool"))

	out := string(todo.RenderMarkdown(c.Groups(), sampleMeta()))

	for _, want := range []string{string(domain.TodoToolMappings), "bogus-tool", "neither a known"} {
		if !strings.Contains(out, want) {
			t.Errorf("TODO markdown must contain %q; got:\n%s", want, out)
		}
	}
}

func TestCollector_AddGap_UnknownHarnessToolSameNameForTwoAgents_KeepsBoth(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(unknownToolGap("agent-a", "bogus-tool"))
	c.AddGap(unknownToolGap("agent-b", "bogus-tool"))

	if n := len(c.Items()); n != 2 {
		t.Errorf("Items() len = %d; want 2 (one per owning agent)", n)
	}
}
