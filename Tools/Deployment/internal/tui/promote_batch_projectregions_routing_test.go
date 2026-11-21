package tui

// promote_batch_projectregions_routing_test.go verifies that the batch question whether project
// region content is kept or emptied reaches the user as a selectable overlay with Keep
// highlighted, and that the chosen option is answered back to the batch.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func projectRegionsMsg() questionMsg {
	// Empty is listed first so only DefaultOptionID can make Keep the highlighted row.
	q := plainChoiceMsg(domain.QPromoteProjectRegions,
		domain.Option{ID: string(app.ProjectRegionsEmpty), Label: "Empty every project region"},
		domain.Option{ID: string(app.ProjectRegionsKeep), Label: "Keep project region content (it becomes the deploy-once default)"})
	q.choiceQ.Title = "Keep the content of project regions in the promoted agents?"
	q.choiceQ.DefaultOptionID = string(app.ProjectRegionsKeep)
	return q
}

func TestBatchRouting_ProjectRegions_ShowsASelectableOverlayWithBothOptions(t *testing.T) {
	m := newBatchRoutingModel(100, 30)

	m.Update(projectRegionsMsg())

	if m.selectOverlay == nil || m.modelOverlay != nil {
		t.Fatalf("selectOverlay nil = %v, modelOverlay nil = %v; want the inline selection", m.selectOverlay == nil, m.modelOverlay == nil)
	}
	view := m.View()
	if !strings.Contains(view, "Keep project region content") || !strings.Contains(view, "Empty every project region") {
		t.Errorf("overlay does not offer both options:\n%s", view)
	}
}

func TestBatchRouting_ProjectRegions_EnterOnTheUntouchedOverlayAnswersKeep(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := projectRegionsMsg()
	m.Update(q)

	sendKeys(m, keyEnter)

	if a := reply(t, q).choiceAns; a.Status != domain.Answered || a.OptionID != string(app.ProjectRegionsKeep) {
		t.Errorf("answer = %+v, want Answered with %q", a, app.ProjectRegionsKeep)
	}
}

func TestBatchRouting_ProjectRegions_UpEnterAnswersEmpty(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := projectRegionsMsg()
	m.Update(q)

	sendKeys(m, tea.KeyMsg{Type: tea.KeyUp}, keyEnter)

	if a := reply(t, q).choiceAns; a.Status != domain.Answered || a.OptionID != string(app.ProjectRegionsEmpty) {
		t.Errorf("answer = %+v, want Answered with %q", a, app.ProjectRegionsEmpty)
	}
}

func TestBatchRouting_ProjectRegions_EscAnswersCancelled(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := projectRegionsMsg()
	m.Update(q)

	sendKeys(m, keyEsc)

	if got := reply(t, q).choiceAns.Status; got != domain.Cancelled {
		t.Errorf("status = %q after esc, want Cancelled", got)
	}
}
