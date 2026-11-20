package tui

// promote_batch_agentlist_edge_test.go covers boundaries of the batch agent selection at the
// root level: a list with exactly one promotable agent, and Esc while the "no agents selected"
// message is displayed.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestBatchRouting_AgentSelection_SingleRowList_SpaceEnterAnswersThatAgent(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := agentSelectionMsg([]domain.Option{{ID: "only.md", Label: "only-agent", Description: "only.md"}})
	m.Update(q)
	requireAgentListOpen(t, m)

	sendKeys(m, keySpace, keyEnter)

	a := reply(t, q).multiChoiceAns
	if a.Status != domain.Answered || len(a.OptionIDs) != 1 || a.OptionIDs[0] != "only.md" {
		t.Errorf("answer = %+v; want Answered with exactly [only.md]", a)
	}
}

func TestBatchRouting_AgentSelection_EscWhileTheNoSelectionMessageShowsAnswersCancelled(t *testing.T) {
	m := newBatchRoutingModel(100, 30)
	q := agentSelectionMsg(promoteAgentOptions())
	m.Update(q)
	sendKeys(m, keyEnter) // nothing selected: the validation message is displayed
	requireNoReply(t, q, "enter with nothing selected must not answer")

	sendKeys(m, keyEsc)

	if got := reply(t, q).multiChoiceAns.Status; got != domain.Cancelled {
		t.Errorf("status = %q after esc with the message shown; want Cancelled", got)
	}
}
