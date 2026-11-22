package tui

// promote_batch_dialogs_test.go verifies the dialogs of the batch promote questions that use the
// plain selection (layer, category, existing agent: no skip row, existing agent defaults to skip)
// and the text prompt (unmapped tool, tier rationale, required skills), with Esc answering Cancelled.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Layer, category and existing agent: plain selection without a skip row
// ---------------------------------------------------------------------------

func plainChoiceMsg(id domain.QuestionID, opts ...domain.Option) questionMsg {
	q := buildSelectOneMsg(id, opts)
	q.choiceQ.AllowSkip = false
	return q
}

func existingAgentMsg() questionMsg {
	q := plainChoiceMsg(domain.QPromoteExistingAgent,
		domain.Option{ID: app.PromoteExistingSkip, Label: "Skip this agent"},
		domain.Option{ID: app.PromoteExistingReplace, Label: "Replace the existing agent in Team Layer (team)"})
	q.choiceQ.Subject = "beta-agent"
	q.choiceQ.Title = "Agent beta-agent already exists in Team Layer (team)"
	q.choiceQ.DefaultOptionID = app.PromoteExistingSkip
	return q
}

func twoOptionPlainQuestions() map[string]questionMsg {
	return map[string]questionMsg{
		"layer": plainChoiceMsg(domain.QPromoteTargetCatalog,
			domain.Option{ID: "team", Label: "Team Layer"}, domain.Option{ID: "other", Label: "Other Layer"}),
		"category": plainChoiceMsg(domain.QPromoteCategory,
			domain.Option{ID: "Dev", Label: "Dev"}, domain.Option{ID: "Ops", Label: "Ops"}),
		"existing agent": existingAgentMsg(),
	}
}

func TestBatchRouting_PlainSelections_ShowNoSkipRowAndUseTheInlineSelection(t *testing.T) {
	for name, q := range twoOptionPlainQuestions() {
		m := newBatchRoutingModel(100, 30)

		m.Update(q)

		if m.selectOverlay == nil || m.modelOverlay != nil {
			t.Errorf("%s: selectOverlay nil = %v, modelOverlay nil = %v; want the plain inline selection",
				name, m.selectOverlay == nil, m.modelOverlay == nil)
		}
		if strings.Contains(m.View(), "None") {
			t.Errorf("%s: the selection shows a None row although the question allows no skip:\n%s", name, m.View())
		}
	}
}

func TestBatchRouting_PlainSelections_EnterAnswersTheFirstOptionAndDownTheSecond(t *testing.T) {
	for name, q := range twoOptionPlainQuestions() {
		first, second := twoOptionPlainQuestions()[name], q
		m := newBatchRoutingModel(100, 30)
		m.Update(first)
		sendKeys(m, keyEnter)
		a := reply(t, first).choiceAns
		if a.Status != domain.Answered || a.OptionID != first.choiceQ.Options[0].ID {
			t.Errorf("%s: enter answered %+v; want the first option %q", name, a, first.choiceQ.Options[0].ID)
		}

		m.Update(second)
		sendKeys(m, keyDown, keyEnter)
		a = reply(t, second).choiceAns
		if a.Status != domain.Answered || a.OptionID != second.choiceQ.Options[1].ID {
			t.Errorf("%s: down+enter answered %+v; want the second option %q", name, a, second.choiceQ.Options[1].ID)
		}
	}
}

func TestBatchRouting_PlainSelections_EscAnswersCancelled(t *testing.T) {
	for name, q := range twoOptionPlainQuestions() {
		m := newBatchRoutingModel(100, 30)
		m.Update(q)

		sendKeys(m, keyEsc)

		if got := reply(t, q).choiceAns.Status; got != domain.Cancelled {
			t.Errorf("%s: status = %q after esc; want Cancelled", name, got)
		}
		if m.screen != screenRunning {
			t.Errorf("%s: screen = %v after esc; want screenRunning", name, m.screen)
		}
	}
}

// The cursor must start on the option named by DefaultOptionID, independent of row order.
func TestBatchRouting_ExistingAgent_CursorStartsOnTheDefaultWhenItIsNotTheFirstRow(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := plainChoiceMsg(domain.QPromoteExistingAgent,
		domain.Option{ID: app.PromoteExistingReplace, Label: "Replace the existing agent in Team Layer (team)"},
		domain.Option{ID: app.PromoteExistingSkip, Label: "Skip this agent"})
	q.choiceQ.Subject = "beta-agent"
	q.choiceQ.DefaultOptionID = app.PromoteExistingSkip
	m.Update(q)

	sendKeys(m, keyEnter)

	if a := reply(t, q).choiceAns; a.Status != domain.Answered || a.OptionID != app.PromoteExistingSkip {
		t.Errorf("answer = %+v; want Enter on the untouched dialog to choose the default %q", a, app.PromoteExistingSkip)
	}
}

// The None row appears when the question allows skipping (positive control for the no-skip test).
func TestBatchRouting_PlainSelection_ShowsNoneRowWhenSkipIsAllowed(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := plainChoiceMsg(domain.QPromoteCategory,
		domain.Option{ID: "Dev", Label: "Dev"}, domain.Option{ID: "Ops", Label: "Ops"})
	q.choiceQ.AllowSkip = true

	m.Update(q)

	if !strings.Contains(m.View(), "None") {
		t.Errorf("a skippable selection shows no None row:\n%s", m.View())
	}
}

// The service lists skip first; this test checks that order and the Replace offer.
func TestBatchRouting_ExistingAgent_DefaultsToSkipAndOffersReplace(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := existingAgentMsg()
	m.Update(q)

	view := m.View()
	if !strings.Contains(view, "Skip this agent") || !strings.Contains(view, "Replace the existing agent in Team Layer (team)") {
		t.Errorf("dialog does not offer skip and replace:\n%s", view)
	}
	sendKeys(m, keyEnter)

	if a := reply(t, q).choiceAns; a.Status != domain.Answered || a.OptionID != app.PromoteExistingSkip {
		t.Errorf("answer = %+v; want Enter on the untouched dialog to choose %q", a, app.PromoteExistingSkip)
	}
}

func TestBatchRouting_ExistingAgent_DownEnterChoosesReplace(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := existingAgentMsg()
	m.Update(q)

	sendKeys(m, keyDown, keyEnter)

	if a := reply(t, q).choiceAns; a.OptionID != app.PromoteExistingReplace {
		t.Errorf("answer = %+v; want %q", a, app.PromoteExistingReplace)
	}
}

// ---------------------------------------------------------------------------
// Text prompts: unmapped tool, tier rationale, required skills
// ---------------------------------------------------------------------------

func batchTextMsgs() map[string]questionMsg {
	out := map[string]questionMsg{}
	for name, id := range map[string]domain.QuestionID{
		"unmapped tool":   domain.QPromoteCustomTool,
		"tier rationale":  domain.QPromoteTierRationale,
		"required skills": domain.QPromoteRequiredSkills,
	} {
		q := buildTextMsg(id)
		q.textQ.AllowSkip = true
		q.textQ.AllowSkipAll = true
		out[name] = q
	}
	return out
}

func TestBatchRouting_TextPrompts_UseTheTextPromptDialog(t *testing.T) {
	for name, q := range batchTextMsgs() {
		m := newBatchRoutingModel(100, 30)

		m.Update(q)

		if m.textOverlay == nil || m.selectOverlay != nil || m.modelOverlay != nil {
			t.Errorf("%s: want the text prompt dialog only", name)
		}
		view := collapseSpaces(m.View())
		if !strings.Contains(view, "ctrl+k skip") || !strings.Contains(view, "ctrl+x skip all") {
			t.Errorf("%s: dialog does not name its skip keys:\n%s", name, view)
		}
	}
}

func TestBatchRouting_TextPrompts_CtrlKSkipsThisOneCtrlXSkipsAllEscCancels(t *testing.T) {
	cases := []struct {
		key  tea.KeyMsg
		want domain.AnswerStatus
	}{{keyCtrlK, domain.SkippedOne}, {keyCtrlX, domain.SkippedAll}, {keyEsc, domain.Cancelled}}
	for name := range batchTextMsgs() {
		for _, c := range cases {
			m := newBatchRoutingModel(100, 30)
			q := batchTextMsgs()[name]
			m.Update(q)

			sendKeys(m, c.key)

			if got := reply(t, q).textAns.Status; got != c.want {
				t.Errorf("%s: key %v answered %q; want %q", name, c.key.Type, got, c.want)
			}
			if m.textOverlay != nil || m.screen != screenRunning {
				t.Errorf("%s: key %v left the dialog open (textOverlay nil = %v, screen = %v)",
					name, c.key.Type, m.textOverlay == nil, m.screen)
			}
		}
	}
}

func TestBatchRouting_TextPrompts_TypedTextIsTheAnswer(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := batchTextMsgs()["required skills"]
	m.Update(q)

	typeRunes(m, "git, testing")
	sendKeys(m, keyEnter)

	if a := reply(t, q).textAns; a.Status != domain.Answered || a.Text != "git, testing" {
		t.Errorf("answer = %+v; want Answered with the typed text", a)
	}
}
