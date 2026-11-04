package todo_test

// collector_dedup_test.go verifies gap identity in the collector: a gap with the same
// identity as one already added since the last Reset is not stored again, the first one wins,
// and gaps that differ in any identity field all stay.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

func countItems(c todo.Collector, cat domain.TodoCategory) int {
	n := 0
	for _, it := range c.Items() {
		if it.Category == cat {
			n++
		}
	}
	return n
}

// A missing model reported by the planner and again by the transform, with different
// details, is one entry; the first (planner) wording is kept.
func TestCollector_NoModelSameAgent_StoredOnceFirstWins(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a", Detail: "plan wording"})
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a", Detail: "transform wording"})
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a", Detail: "plan wording"})

	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d, want 1", len(items))
	}
	if items[0].Detail != "plan wording" {
		t.Errorf("Detail = %q, want the first gap's detail", items[0].Detail)
	}
}

func TestCollector_NoModelDifferentAgents_AllKept(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a"})
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-b"})

	if n := countItems(c, domain.TodoModels); n != 2 {
		t.Errorf("model entries = %d, want 2", n)
	}
}

// The transform reports an unmapped tool with a different detail than the planner and no
// fragment difference; the same tool on the same agent is one entry.
func TestCollector_UnmappedToolSameAgentAndTool_StoredOnce(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a", Detail: "plan wording"})
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a", Detail: "unmapped tool: terminal"})

	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d, want 1", len(items))
	}
	if items[0].Detail != "plan wording" {
		t.Errorf("Detail = %q, want the first gap's detail", items[0].Detail)
	}
}

func TestCollector_UnmappedToolSameToolDifferentAgents_AllKept(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a"})
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-b"})

	if n := countItems(c, domain.TodoToolMappings); n != 2 {
		t.Errorf("tool entries = %d, want 2", n)
	}
}

func TestCollector_UnmappedToolSameAgentDifferentTools_AllKept(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a"})
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "file_search", Owner: "agent-a"})

	if n := countItems(c, domain.TodoToolMappings); n != 2 {
		t.Errorf("tool entries = %d, want 2", n)
	}
}

// Identity of a missing model is the agent alone: a differing fragment or detail does not
// make a second entry.
func TestCollector_NoModelDifferingFragment_StoredOnce(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a", Fragment: "frag-1"})
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a", Fragment: "frag-2"})

	if n := countItems(c, domain.TodoModels); n != 1 {
		t.Errorf("model entries = %d, want 1: the agent is the whole identity", n)
	}
}

// Identity of an unmapped tool is the agent and the tool: a differing fragment does not make a
// second entry, while another tool on the same agent does (covered above).
func TestCollector_UnmappedToolDifferingFragment_StoredOnce(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a", Fragment: "frag-1"})
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a", Fragment: "frag-2"})

	if n := countItems(c, domain.TodoToolMappings); n != 1 {
		t.Errorf("tool entries = %d, want 1: agent and tool are the whole identity", n)
	}
}

// A skipped custom tool is reported only by the transform; it still yields its entry.
func TestCollector_UnmappedToolReportedOnlyOnce_StillListed(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "terminal", Owner: "agent-a", Detail: "unmapped tool: terminal"})

	if n := countItems(c, domain.TodoToolMappings); n != 1 {
		t.Errorf("tool entries = %d, want 1", n)
	}
}

// For every other kind only gaps identical in all five fields collapse.
func TestCollector_OtherKindIdenticalGaps_Collapse(t *testing.T) {
	g := domain.Gap{Kind: domain.GapSkippedFile, Subject: "skills/x/a.md", Owner: "x", Detail: "skipped", Fragment: "frag"}
	c := todo.NewCollector()
	c.AddGap(g)
	c.AddGap(g)

	if len(c.Items()) != 1 {
		t.Errorf("Items() len = %d, want 1 for identical gaps", len(c.Items()))
	}
}

func TestCollector_OtherKindGapsDifferingInAnyField_AllKept(t *testing.T) {
	base := domain.Gap{Kind: domain.GapSkippedFile, Subject: "s", Owner: "o", Detail: "d", Fragment: "f"}
	variants := []domain.Gap{base}
	for _, mutate := range []func(g *domain.Gap){
		func(g *domain.Gap) { g.Subject = "s2" },
		func(g *domain.Gap) { g.Owner = "o2" },
		func(g *domain.Gap) { g.Detail = "d2" },
		func(g *domain.Gap) { g.Fragment = "f2" },
	} {
		v := base
		mutate(&v)
		variants = append(variants, v)
	}

	c := todo.NewCollector()
	for _, v := range variants {
		c.AddGap(v)
	}

	if got := len(c.Items()); got != len(variants) {
		t.Errorf("Items() len = %d, want %d: gaps that differ in any field must all stay", got, len(variants))
	}
}

// Repeated model gaps with different fragments are not collapsed away for other kinds, and a
// no-model gap does not swallow a gap of another kind with the same subject.
func TestCollector_DifferentKindsSameSubject_AllKept(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a"})
	c.AddGap(domain.Gap{Kind: domain.GapUnmappedTool, Subject: "agent-a", Owner: "agent-a"})
	c.AddGap(domain.Gap{Kind: domain.GapManualStep, Subject: "agent-a"})

	if got := len(c.Items()); got != 3 {
		t.Errorf("Items() len = %d, want 3", got)
	}
}

// After Reset the same gap is accepted again.
func TestCollector_ResetForgetsGapIdentities(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a"})
	c.Reset()
	c.AddGap(domain.Gap{Kind: domain.GapNoModel, Subject: "agent-a"})

	if got := len(c.Items()); got != 1 {
		t.Errorf("Items() len = %d after Reset and re-add, want 1", got)
	}
}

// Direct items are never de-duplicated.
func TestCollector_AddDirectItems_NotDeduplicated(t *testing.T) {
	c := todo.NewCollector()
	it := domain.TodoItem{Category: domain.TodoManual, Subject: "step"}
	c.Add(it)
	c.Add(it)

	if got := len(c.Items()); got != 2 {
		t.Errorf("Items() len = %d, want 2: Add stores items verbatim", got)
	}
}
