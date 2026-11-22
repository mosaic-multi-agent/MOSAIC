package tui

// custom_tool_validation_test.go verifies, through the root model, that the custom tool
// question refuses an answer that does not fit the list syntax inline: the prompt stays open,
// nothing is sent to the run, the refusal names the entry, and a corrected answer is accepted.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

func customToolValidatedMsg() questionMsg {
	msg := buildTextMsg(domain.QCustomTool)
	msg.textQ.Subject = "content_search"
	msg.textQ.Validate = customtool.Validate
	return msg
}

func TestRootModel_CustomToolPrompt_InvalidAnswer_KeepsPromptOpenAndNamesTheEntry(t *testing.T) {
	m := newRoutingModel()
	qMsg := customToolValidatedMsg()
	m.Update(qMsg)

	typeIntoRoot(m, "github/get_pr, github/a/b")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenQuestion || m.textOverlay == nil {
		t.Fatalf("screen = %v, overlay open = %v; want the prompt to stay open", m.screen, m.textOverlay != nil)
	}
	select {
	case ans := <-qMsg.reply:
		t.Fatalf("an invalid answer was sent to the run: %+v", ans.textAns)
	default:
	}
	if view := m.View(); !strings.Contains(view, "github/a/b") {
		t.Errorf("the refusal does not name the entry github/a/b:\n%s", view)
	}
}

func TestRootModel_CustomToolPrompt_CorrectedAnswer_IsSentAfterARefusal(t *testing.T) {
	m := newRoutingModel()
	qMsg := customToolValidatedMsg()
	m.Update(qMsg)
	typeIntoRoot(m, "github/get_pr,")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // refused: empty entry

	typeIntoRoot(m, "jira")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	select {
	case ans := <-qMsg.reply:
		if ans.textAns.Text != "github/get_pr,jira" {
			t.Errorf("answer text = %q, want %q", ans.textAns.Text, "github/get_pr,jira")
		}
	default:
		t.Fatal("the corrected answer was not sent to the run")
	}
	if m.screen != screenRunning {
		t.Errorf("screen = %v after the corrected answer; want screenRunning", m.screen)
	}
}
