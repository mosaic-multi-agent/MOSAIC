package todo_test

// gap_workflow_kept_test.go specifies that a kept-workflow gap has its own TODO category and
// reaches the TODO file with its text, instead of falling back to the manual-steps category.

import (
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

const keptGapDetail = "custom-x is not in the catalog stack; kept as deployed, not updated or validated."

func TestCollector_WorkflowKept_IsNotMappedToManual(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapWorkflowKept, Subject: "custom-x", Detail: keptGapDetail})

	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d; want 1", len(items))
	}
	if items[0].Category == domain.TodoManual {
		t.Errorf("Category = %q; GapWorkflowKept fell through to the default category - it must be registered in gapToCategory", items[0].Category)
	}
	if items[0].Kind != domain.GapWorkflowKept {
		t.Errorf("Kind = %q; want %q", items[0].Kind, domain.GapWorkflowKept)
	}
}

func TestRenderMarkdown_WorkflowKept_ShowsTheWarningText(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapWorkflowKept, Subject: "custom-x", Detail: keptGapDetail})

	out := string(todo.RenderMarkdown(c.Groups(), todo.Meta{GeneratedAt: time.Unix(0, 0).UTC()}))

	if strings.Count(out, keptGapDetail) != 1 {
		t.Errorf("TODO file must show the warning text exactly once; got:\n%s", out)
	}
}
