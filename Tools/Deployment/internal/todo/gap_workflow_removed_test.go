package todo_test

// gap_workflow_removed_test.go specifies that a removed-workflow gap has its own TODO category
// and reaches the TODO file with its text, instead of falling back to the manual-steps category.

import (
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

const removedGapDetail = "custom-x was not selected and was removed from the orchestrator."

func TestCollector_WorkflowRemoved_IsNotMappedToManual(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapWorkflowRemoved, Subject: "custom-x", Detail: removedGapDetail})

	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d; want 1", len(items))
	}
	if items[0].Category == domain.TodoManual {
		t.Errorf("Category = %q; GapWorkflowRemoved fell through to the default category - it must be registered in gapToCategory", items[0].Category)
	}
	if items[0].Kind != domain.GapWorkflowRemoved {
		t.Errorf("Kind = %q; want %q", items[0].Kind, domain.GapWorkflowRemoved)
	}
}

func TestRenderMarkdown_WorkflowRemoved_ShowsTheNoticeText(t *testing.T) {
	c := todo.NewCollector()
	c.AddGap(domain.Gap{Kind: domain.GapWorkflowRemoved, Subject: "custom-x", Detail: removedGapDetail})

	out := string(todo.RenderMarkdown(c.Groups(), todo.Meta{GeneratedAt: time.Unix(0, 0).UTC()}))

	if strings.Count(out, removedGapDetail) != 1 {
		t.Errorf("TODO file must show the removal notice exactly once; got:\n%s", out)
	}
}
