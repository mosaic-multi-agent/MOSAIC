package todo_test

// tool_capability_withheld_test.go verifies that a GapToolCapabilityWithheld gap becomes a
// Tool mappings item that names the tool and its owning agent.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

func withheldGap(owner string) domain.Gap {
	return domain.Gap{
		Kind:    domain.GapToolCapabilityWithheld,
		Subject: "web_fetch",
		Owner:   owner,
		Detail:  "agent " + owner + " gets no network access because it is read-only",
	}
}

func TestCollector_AddGap_ToolCapabilityWithheld_MapsToToolMappingsCategory(t *testing.T) {
	// Arrange
	c := todo.NewCollector()

	// Act
	c.AddGap(withheldGap("agent-a"))

	// Assert
	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d; want 1", len(items))
	}
	if items[0].Category != domain.TodoToolMappings {
		t.Errorf("Category = %q; want %q", items[0].Category, domain.TodoToolMappings)
	}
	if items[0].Subject != "web_fetch" || items[0].Owner != "agent-a" {
		t.Errorf("item = %+v; want subject web_fetch owned by agent-a", items[0])
	}
}

func TestCollector_AddGap_ToolCapabilityWithheldForTwoAgents_KeepsBoth(t *testing.T) {
	// Arrange
	c := todo.NewCollector()

	// Act
	c.AddGap(withheldGap("agent-a"))
	c.AddGap(withheldGap("agent-b"))

	// Assert
	if n := len(c.Items()); n != 2 {
		t.Errorf("Items() len = %d; want 2 (one per owning agent)", n)
	}
}

func TestRenderMarkdown_ToolCapabilityWithheldAppearsWithToolAndAgent(t *testing.T) {
	// Arrange
	c := todo.NewCollector()
	c.AddGap(withheldGap("agent-a"))

	// Act
	out := string(todo.RenderMarkdown(c.Groups(), sampleMeta()))

	// Assert
	for _, want := range []string{"web_fetch", "agent-a"} {
		if !strings.Contains(out, want) {
			t.Errorf("TODO markdown must contain %q; got:\n%s", want, out)
		}
	}
}
