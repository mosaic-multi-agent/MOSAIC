package tui

// promote_batch_routing_test.go verifies how the root model routes the questions of a batch
// promote to dialogs and keys:
//
//   - the agent selection opens the multi-select agent list, which fits the terminal and keeps
//     the same behaviour for one selected agent as for many
//   - the recommended tier opens the model selection dialog (skip, skip all, custom entry)
//   - the layer, category and existing-agent questions use the plain selection without a skip row
//   - the unmapped tool, tier rationale and required skills questions use the text prompt
//   - Esc on every one of them answers Cancelled, which aborts the batch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newBatchRoutingModel(width, height int) *rootModel {
	m := newRoutingModel()
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m
}

func sendKeys(m *rootModel, keys ...tea.KeyMsg) {
	for _, k := range keys {
		m.Update(k)
	}
}

func typeRunes(m *rootModel, text string) {
	for _, r := range text {
		m.Update(runeKey(r))
	}
}

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
	keySpace = runeKey(' ')
	keyCtrlK = tea.KeyMsg{Type: tea.KeyCtrlK}
	keyCtrlX = tea.KeyMsg{Type: tea.KeyCtrlX}
)

// reply returns the answer the root model sent to the question, failing when none was sent.
func reply(t *testing.T, q questionMsg) answerMsg {
	t.Helper()
	select {
	case a := <-q.reply:
		return a
	default:
		t.Fatal("no answer was sent to the reply channel")
		return answerMsg{}
	}
}

func requireNoReply(t *testing.T, q questionMsg, why string) {
	t.Helper()
	select {
	case <-q.reply:
		t.Errorf("an answer was sent: %s", why)
	default:
	}
}

func promoteAgentOptions() []domain.Option {
	return []domain.Option{
		{ID: "alpha.md", Label: "alpha-agent", Description: "alpha.md"},
		{ID: "beta.md", Label: "beta-agent", Description: "beta.md", Hint: "already in Team Layer (team)"},
		{ID: "gamma.md", Label: "gamma-agent", Description: "gamma.md"},
	}
}

func agentSelectionMsg(opts []domain.Option) questionMsg {
	q := buildSelectManyMsg(domain.QPromoteAgents, opts)
	q.choiceQ.Title = "Select the agents to promote"
	q.choiceQ.AllowSkip = false
	q.choiceQ.Subject = "/agents"
	return q
}

// requireAgentListOpen fails unless the agent selection is shown by its own list (not by the
// generic selection fallback) and the list shows the "Selected" summary of a multi-select.
func requireAgentListOpen(t *testing.T, m *rootModel) {
	t.Helper()
	if m.selectOverlay != nil || !strings.Contains(m.View(), "Selected 0") {
		t.Fatalf("the agent selection is not shown by the multi-select agent list:\n%s", m.View())
	}
}

// ---------------------------------------------------------------------------
// Agent selection
// ---------------------------------------------------------------------------

func TestBatchRouting_AgentSelection_OpensTheAgentListNotTheGenericFallback(t *testing.T) {
	m := newBatchRoutingModel(100, 30)

	m.Update(agentSelectionMsg(promoteAgentOptions()))

	if m.screen != screenQuestion {
		t.Fatalf("screen = %v after the agent selection; want screenQuestion", m.screen)
	}
	if m.selectOverlay != nil {
		t.Error("selectOverlay is non-nil; the agent selection needs the multi-select agent list, not the generic fallback")
	}
	view := m.View()
	for _, o := range promoteAgentOptions() {
		if !strings.Contains(view, o.Label) {
			t.Errorf("agent list does not show %q:\n%s", o.Label, view)
		}
	}
	if !strings.Contains(view, "already in Team Layer (team)") {
		t.Errorf("agent list does not show the existing-layer hint:\n%s", view)
	}
}

func TestBatchRouting_AgentSelection_DeliversTheSelectedFileNamesInOptionOrder(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := agentSelectionMsg(promoteAgentOptions())
	m.Update(q)

	sendKeys(m, keyDown, keyDown, keySpace) // gamma
	sendKeys(m, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyUp}, keySpace) // alpha
	sendKeys(m, keyEnter)

	a := reply(t, q).multiChoiceAns
	if a.Status != domain.Answered || len(a.OptionIDs) != 2 || a.OptionIDs[0] != "alpha.md" || a.OptionIDs[1] != "gamma.md" {
		t.Errorf("answer = %+v; want Answered with [alpha.md gamma.md]", a)
	}
	if m.screen != screenRunning {
		t.Errorf("screen = %v after confirming the selection; want screenRunning", m.screen)
	}
}

func TestBatchRouting_AgentSelection_OneAgentIsAnsweredLikeAnyBatch(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := agentSelectionMsg(promoteAgentOptions())
	m.Update(q)
	requireAgentListOpen(t, m)

	sendKeys(m, keyDown, keySpace, keyEnter)

	a := reply(t, q).multiChoiceAns
	if a.Status != domain.Answered || len(a.OptionIDs) != 1 || a.OptionIDs[0] != "beta.md" {
		t.Errorf("answer = %+v; want Answered with exactly [beta.md]", a)
	}
}

func TestBatchRouting_AgentSelection_EmptyConfirmationIsRefused(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := agentSelectionMsg(promoteAgentOptions())
	m.Update(q)

	sendKeys(m, keyEnter)

	requireNoReply(t, q, "enter with nothing selected must not answer")
	if m.screen != screenQuestion || !strings.Contains(m.View(), screens.MsgNoAgentsSelected) {
		t.Errorf("screen = %v; want the list to stay with the validation message:\n%s", m.screen, m.View())
	}
}

func TestBatchRouting_AgentSelection_EscAnswersCancelled(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := agentSelectionMsg(promoteAgentOptions())
	m.Update(q)
	requireAgentListOpen(t, m)
	sendKeys(m, keySpace)

	sendKeys(m, keyEsc)

	if got := reply(t, q).multiChoiceAns.Status; got != domain.Cancelled {
		t.Errorf("status = %q after esc; want Cancelled (a partial selection is not delivered)", got)
	}
	if m.screen != screenRunning {
		t.Errorf("screen = %v after esc; want screenRunning (the abort goes through the reply)", m.screen)
	}
}

func manyAgentOptions(n int) []domain.Option {
	opts := make([]domain.Option, n)
	for i := range opts {
		name := "agent-" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		opts[i] = domain.Option{ID: name + ".md", Label: name, Description: strings.Repeat("detail line\n", 12)}
	}
	return opts
}

func TestBatchRouting_AgentSelection_AlwaysFitsTheTerminalHeight(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 12}, {40, 10}, {120, 40}} {
		m := newBatchRoutingModel(size.w, size.h)
		m.Update(agentSelectionMsg(manyAgentOptions(30)))

		for i := 0; i < 30; i++ {
			sendKeys(m, keySpace, keyDown)
			if n := strings.Count(m.View(), "\n") + 1; n > size.h {
				t.Fatalf("%dx%d: view has %d lines after selecting %d agents; want at most %d", size.w, size.h, n, i+1, size.h)
			}
		}
		if !strings.Contains(m.View(), "Selected 30") {
			t.Errorf("%dx%d: summary does not state that 30 agents are selected:\n%s", size.w, size.h, m.View())
		}
	}
}

func TestBatchRouting_AgentSelection_ResizeKeepsTheListWithinTheTerminal(t *testing.T) {
	m := newBatchRoutingModel(120, 40)
	m.Update(agentSelectionMsg(manyAgentOptions(30)))

	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})

	if n := strings.Count(m.View(), "\n") + 1; n > 10 {
		t.Errorf("view has %d lines after the terminal shrank to 10 lines", n)
	}
}

func TestBatchRouting_AgentSelection_ShortListLeavesNoTallEmptyArea(t *testing.T) {
	m := newBatchRoutingModel(100, 60)
	m.Update(agentSelectionMsg(promoteAgentOptions()))
	requireAgentListOpen(t, m)

	if n := strings.Count(strings.TrimRight(m.View(), "\n "), "\n") + 1; n > 20 {
		t.Errorf("a 3-row list takes %d lines on a 60 line terminal; want no tall empty area", n)
	}
}

// ---------------------------------------------------------------------------
// Recommended tier: the model selection dialog
// ---------------------------------------------------------------------------

func tierQuestionMsg() questionMsg {
	q := buildSelectOneMsg(domain.QPromoteRecommendedTier, []domain.Option{
		{ID: "heavy", Label: "heavy"}, {ID: "light", Label: "light"},
	})
	q.choiceQ.Title = "Recommended tier for model opus (3 agents)"
	q.choiceQ.Subject = "opus"
	q.choiceQ.AllowSkip = true
	q.choiceQ.AllowSkipAll = true
	q.choiceQ.AllowCustom = true
	q.choiceQ.CustomPrompt = "Enter a custom tier name"
	return q
}

func TestBatchRouting_Tier_OpensTheModelSelectionDialog(t *testing.T) {
	m := newBatchRoutingModel(100, 30)

	m.Update(tierQuestionMsg())

	if m.modelOverlay == nil {
		t.Fatal("modelOverlay is nil after the tier question; want the model selection dialog")
	}
	if m.selectOverlay != nil {
		t.Error("selectOverlay is non-nil; the tier question must not use the generic selection")
	}
	view := collapseSpaces(m.View())
	for _, want := range []string{"Recommended tier for model opus (3 agents)", "heavy", "light",
		"Enter a custom tier name", "ctrl+k skip", "ctrl+x skip all"} {
		if !strings.Contains(view, want) {
			t.Errorf("dialog does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "custom model ID") {
		t.Errorf("dialog uses model-ID wording for a tier question:\n%s", view)
	}
}

func TestBatchRouting_Tier_CtrlKSkipsThisModel(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := tierQuestionMsg()
	m.Update(q)

	sendKeys(m, keyCtrlK)

	if got := reply(t, q).choiceAns.Status; got != domain.SkippedOne {
		t.Errorf("status = %q after ctrl+k; want SkippedOne", got)
	}
	if m.screen != screenRunning || m.modelOverlay != nil {
		t.Errorf("screen = %v, modelOverlay nil = %v after ctrl+k; want the dialog closed", m.screen, m.modelOverlay == nil)
	}
}

func TestBatchRouting_Tier_CtrlXSkipsForTheRestOfTheBatch(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := tierQuestionMsg()
	m.Update(q)

	sendKeys(m, keyCtrlX)

	if got := reply(t, q).choiceAns.Status; got != domain.SkippedAll {
		t.Errorf("status = %q after ctrl+x; want SkippedAll", got)
	}
}

func TestBatchRouting_Tier_EnterAnswersTheFocusedTier(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := tierQuestionMsg()
	m.Update(q)

	sendKeys(m, keyDown, keyEnter)

	if a := reply(t, q).choiceAns; a.Status != domain.Answered || a.OptionID != "light" {
		t.Errorf("answer = %+v; want Answered with tier light", a)
	}
}

func TestBatchRouting_Tier_CustomEntryAnswersWithTheTypedTier(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := tierQuestionMsg()
	m.Update(q)

	sendKeys(m, keyDown, keyDown, keyEnter) // the custom entry row
	typeRunes(m, "ultra")
	sendKeys(m, keyEnter)

	if a := reply(t, q).choiceAns; a.Status != domain.Answered || a.Custom != "ultra" {
		t.Errorf("answer = %+v; want Answered with custom tier ultra", a)
	}
}

func TestBatchRouting_Tier_EscAnswersCancelled(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := tierQuestionMsg()
	m.Update(q)

	sendKeys(m, keyEsc)

	if got := reply(t, q).choiceAns.Status; got != domain.Cancelled {
		t.Errorf("status = %q after esc; want Cancelled", got)
	}
}

