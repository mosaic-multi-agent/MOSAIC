package tui

// mcp_split_routing_test.go verifies the TUI side of the MCP name split question of the
// transform: the select-one question lists the possible splits in the given order, selecting
// one answers it with that option's server, "s" skips the name, "S" skips every remaining name
// and Esc cancels the run. The keys match the model selection question, which has the same
// skip and skip-all shape.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
)

// buildSplitMsg is the question the transform asks for an ambiguous OpenCode name.
func buildSplitMsg() questionMsg {
	msg := buildSelectOneMsg(domain.QMCPNameSplit, []domain.Option{
		{ID: "tracker", Label: "tracker / get_issue"},
		{ID: "tracker_get", Label: "tracker_get / issue"},
	})
	msg.choiceQ.Subject = "tracker_get_issue"
	msg.choiceQ.Title = "How does tracker_get_issue split into server and tool?"
	msg.choiceQ.AllowSkip = true
	msg.choiceQ.AllowSkipAll = true
	return msg
}

// answerOf returns the choice answer on the reply channel, failing when there is none.
func answerOf(t *testing.T, msg questionMsg) domain.ChoiceAnswer {
	t.Helper()
	select {
	case ans := <-msg.reply:
		return ans.choiceAns
	default:
		t.Fatal("reply channel has no message; the question was not answered")
		return domain.ChoiceAnswer{}
	}
}

func TestMcpSplitRouting_QuestionIsShownWithTheSplitsInOrder(t *testing.T) {
	m := newRoutingModel()

	m.Update(buildSplitMsg())

	if m.screen != screenQuestion {
		t.Fatalf("screen = %v after the split question; want screenQuestion", m.screen)
	}
	view := m.View()
	first, second := strings.Index(view, "tracker / get_issue"), strings.Index(view, "tracker_get / issue")
	if first < 0 || second < 0 || first > second {
		t.Errorf("the view must list \"tracker / get_issue\" before \"tracker_get / issue\", got:\n%s", view)
	}
}

func TestMcpSplitRouting_EnterAnswersWithTheFirstSplitServer(t *testing.T) {
	m := newRoutingModel()
	msg := buildSplitMsg()
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenRunning {
		t.Errorf("screen = %v after answering; want screenRunning", m.screen)
	}
	if ans := answerOf(t, msg); ans.Status != domain.Answered || ans.OptionID != "tracker" {
		t.Errorf("answer = %+v, want Answered with tracker", ans)
	}
}

func TestMcpSplitRouting_DownThenEnterAnswersWithTheSecondSplitServer(t *testing.T) {
	m := newRoutingModel()
	msg := buildSplitMsg()
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if ans := answerOf(t, msg); ans.Status != domain.Answered || ans.OptionID != "tracker_get" {
		t.Errorf("answer = %+v, want Answered with tracker_get", ans)
	}
}

func TestMcpSplitRouting_LowerSSkipsTheName(t *testing.T) {
	m := newRoutingModel()
	msg := buildSplitMsg()
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if ans := answerOf(t, msg); ans.Status != domain.SkippedOne {
		t.Errorf("answer = %+v, want SkippedOne", ans)
	}
	if m.screen != screenRunning {
		t.Errorf("screen = %v after skipping; want screenRunning", m.screen)
	}
}

func TestMcpSplitRouting_UpperSSkipsEveryRemainingName(t *testing.T) {
	m := newRoutingModel()
	msg := buildSplitMsg()
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	if ans := answerOf(t, msg); ans.Status != domain.SkippedAll {
		t.Errorf("answer = %+v, want SkippedAll", ans)
	}
}

func TestMcpSplitRouting_SkipKeysDoNothingWhenTheQuestionDoesNotAllowThem(t *testing.T) {
	m := newRoutingModel()
	msg := buildSplitMsg()
	msg.choiceQ.AllowSkip, msg.choiceQ.AllowSkipAll = false, false
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	if m.screen != screenQuestion {
		t.Errorf("screen = %v; the question must stay open when skipping is not allowed", m.screen)
	}
	select {
	case ans := <-msg.reply:
		t.Errorf("unexpected answer %+v", ans.choiceAns)
	default:
	}
}

func TestMcpSplitRouting_EscCancelsTheRun(t *testing.T) {
	m := newRoutingModel()
	msg := buildSplitMsg()
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if ans := answerOf(t, msg); ans.Status != domain.Cancelled {
		t.Errorf("answer = %+v, want Cancelled", ans)
	}
}
