package tui

// interaction_confirm_detail_test.go verifies that the inline confirmation shows the question's
// Detail text, so a user asked to confirm an action (such as saving an answer to a config file)
// can see what is being confirmed.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestInlineConfirm_View_ShowsTheQuestionDetail(t *testing.T) {
	q := domain.Question{
		Title:  "Save this answer?",
		Prompt: "Remember the choice for later runs.",
		Detail: "answer-text-xyz saved to /home/user/.config/mosaic/user-config.yaml",
	}
	c := newInlineConfirm(q, DefaultTheme(), 120)

	view := c.view()

	if !strings.Contains(view, "answer-text-xyz") {
		t.Errorf("confirmation view does not show the answer in Detail:\n%s", view)
	}
	if !strings.Contains(view, "user-config.yaml") {
		t.Errorf("confirmation view does not show the config path in Detail:\n%s", view)
	}
}
