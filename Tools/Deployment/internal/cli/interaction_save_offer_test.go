package cli_test

// interaction_save_offer_test.go verifies that a non-interactive run is never offered to remember
// a custom tool answer: the offer question is skipped without a TODO gap, whether or not a
// pre-answer exists for it, so a run's TODO file and exit code stay as without the offer. Every
// other confirmation keeps its behaviour.

import (
	"bytes"
	"context"
	"testing"

	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

func saveOfferQuestion(subject string) domain.Question {
	return domain.Question{ID: domain.QSaveCustomTool, Subject: subject, AllowSkip: true}
}

func TestNonInteractive_SaveOffer_WithoutPreAnswer_IsSkippedWithoutAGap(t *testing.T) {
	todos := &stubTodo{}
	ix := cli.NewInteraction(cli.PreAnswers{}, todos, &bytes.Buffer{})

	ans, err := ix.Confirm(context.Background(), saveOfferQuestion("bash"))

	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if ans.Status != domain.SkippedOne {
		t.Errorf("Status = %q, want SkippedOne", ans.Status)
	}
	if !todos.Empty() {
		t.Errorf("the skipped offer recorded gaps that would change the TODO file and exit code: %+v", todos.gaps)
	}
}

func TestNonInteractive_SaveOffer_WithAPreAnswer_IsStillSkippedAndNeverAccepted(t *testing.T) {
	todos := &stubTodo{}
	answers := cli.PreAnswers{Values: map[domain.QuestionID]map[string]string{
		domain.QSaveCustomTool: {"bash": "true"},
	}}
	ix := cli.NewInteraction(answers, todos, &bytes.Buffer{})

	ans, err := ix.Confirm(context.Background(), saveOfferQuestion("bash"))

	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if ans.Status != domain.SkippedOne || ans.Confirm {
		t.Errorf("answer = %+v, want SkippedOne and not confirmed: a non-interactive run never saves", ans)
	}
	if !todos.Empty() {
		t.Errorf("recorded gaps: %+v", todos.gaps)
	}
}

func TestNonInteractive_OtherConfirmations_KeepTheirBehaviour(t *testing.T) {
	todos := &stubTodo{}
	ix := cli.NewInteraction(cli.PreAnswers{}, todos, &bytes.Buffer{})

	ans, err := ix.Confirm(context.Background(), domain.Question{ID: domain.QExternalOptIn, Subject: "x"})

	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if ans.Status != domain.SkippedOne {
		t.Errorf("Status = %q, want SkippedOne", ans.Status)
	}
	if !todos.hasGapKind(domain.GapSkippedFile) {
		t.Error("a confirmation without a pre-answer must still record its gap")
	}
}
