package screens_test

// workflows_layout_test.go verifies the terminal-height contract of WorkflowBrowserScreen: the
// whole view fits the terminal however many workflows are selected (also on a 40 column
// terminal), the title stays the first line, the list scrolls with the cursor row rendered, and
// the content block is no taller than the rows need.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func workflowsLayout() layoutScreen {
	return layoutScreen{
		name:       "workflows",
		title:      "Select Workflows",
		hasSummary: true,
		topMarks:   []string{layoutRowName(0)},
		build: func(w, h, n int, withProv bool) layoutDriver {
			wfs := make([]domain.Workflow, n)
			for i := range wfs {
				wfs[i] = domain.Workflow{ID: layoutRowKey(i), Name: layoutRowName(i), Description: layoutDescription(), Hint: "hint"}
			}
			cats := []domain.WorkflowCategory{{Name: "Build", Workflows: wfs}}
			s := screens.NewWorkflowBrowserScreen(cats, w, h, plainStyles(), "")
			if withProv {
				s.SetProvenance(layoutLabels(n))
			}
			s.Update(tea.KeyMsg{Type: tea.KeyRight}) // open the folder that lists the workflows
			return s
		},
	}
}

func TestWorkflowBrowserLayout_FitsWhateverIsSelected(t *testing.T) {
	layoutRunFitsWhateverSelected(t, workflowsLayout())
}

func TestWorkflowBrowserLayout_CursorStaysVisibleWhileScrolling(t *testing.T) {
	layoutRunCursorStaysVisible(t, workflowsLayout())
}

func TestWorkflowBrowserLayout_ResizeKeepsFit(t *testing.T) {
	layoutRunResizeKeepsFit(t, workflowsLayout())
}

func TestWorkflowBrowserLayout_ProvenanceLabelsKeepFit(t *testing.T) {
	layoutRunProvenanceKeepsFit(t, workflowsLayout())
}

func TestWorkflowBrowserLayout_ShortListIsNotPadded(t *testing.T) {
	layoutRunShortListIsNotPadded(t, workflowsLayout())
}

func TestWorkflowBrowserLayout_ValidationMessageKeepsFit(t *testing.T) {
	layoutRunValidationFits(t, workflowsLayout())
}

// TestWorkflowBrowserLayout_CategoryLevelShortListIsNotPadded covers the category level, where
// the rows are folders: two categories must not leave a tall empty area either.
func TestWorkflowBrowserLayout_CategoryLevelShortListIsNotPadded(t *testing.T) {
	s := screens.NewWorkflowBrowserScreen(wfAllCategories, 80, 40, plainStyles(), "")

	view := s.View()

	if got := contentHeight(view); got < 0 || got > 3 {
		t.Errorf("category level with 2 categories: content block is %d lines tall; want at most 3\n%s", got, view)
	}
}
