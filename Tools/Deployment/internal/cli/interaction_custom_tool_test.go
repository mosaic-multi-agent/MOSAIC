package cli_test

// interaction_custom_tool_test.go verifies the non-interactive handling of the custom tool
// question: without a pre-answer the question is skipped and recorded as before, a pre-answer
// that fits the list syntax is used, and a pre-answer that does not is refused like a missing
// one, with the reason recorded.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

func customToolText(subject string) domain.TextQuestion {
	return domain.TextQuestion{
		Question: domain.Question{ID: domain.QCustomTool, Subject: subject, AllowSkip: true, AllowSkipAll: true},
		Validate: customtool.Validate,
	}
}

func customToolPreAnswer(subject, text string) cli.PreAnswers {
	return cli.PreAnswers{Values: map[domain.QuestionID]map[string]string{
		domain.QCustomTool: {subject: text},
	}}
}

func TestNonInteractive_CustomTool_NoPreAnswer_SkipsAndRecordsAGap(t *testing.T) {
	todos := &stubTodo{}
	ix := cli.NewInteraction(cli.PreAnswers{}, todos, &bytes.Buffer{})

	ans, err := ix.AskText(context.Background(), customToolText("bash"))

	if err != nil {
		t.Fatalf("AskText: %v", err)
	}
	if ans.Status != domain.SkippedOne {
		t.Errorf("Status = %q, want SkippedOne", ans.Status)
	}
	if !todos.hasGapKind(domain.GapSkippedFile) {
		t.Error("no GapSkippedFile recorded for the skipped question")
	}
}

func TestNonInteractive_CustomTool_ValidPreAnswer_IsUsed(t *testing.T) {
	todos := &stubTodo{}
	ix := cli.NewInteraction(customToolPreAnswer("bash", "github/get_pr, jira"), todos, &bytes.Buffer{})

	ans, err := ix.AskText(context.Background(), customToolText("bash"))

	if err != nil {
		t.Fatalf("AskText: %v", err)
	}
	if ans.Status != domain.Answered || ans.Text != "github/get_pr, jira" {
		t.Errorf("answer = %+v, want Answered with the pre-answer text", ans)
	}
	if !todos.Empty() {
		t.Errorf("a valid pre-answer recorded gaps: %+v", todos.gaps)
	}
}

func TestNonInteractive_CustomTool_InvalidPreAnswer_IsSkippedAndRecordedWithTheReason(t *testing.T) {
	todos := &stubTodo{}
	ix := cli.NewInteraction(customToolPreAnswer("bash", "github/a/b"), todos, &bytes.Buffer{})

	ans, err := ix.AskText(context.Background(), customToolText("bash"))

	if err != nil {
		t.Fatalf("AskText: %v", err)
	}
	if ans.Status != domain.SkippedOne {
		t.Errorf("Status = %q, want SkippedOne for an answer that does not fit the syntax", ans.Status)
	}
	if !todos.hasGapKind(domain.GapSkippedFile) {
		t.Fatal("no GapSkippedFile recorded for the refused pre-answer")
	}
	found := false
	for _, g := range todos.gaps {
		if g.Kind == domain.GapSkippedFile && strings.Contains(g.Detail, "github/a/b") {
			found = true
		}
	}
	if !found {
		t.Errorf("no gap detail names the refused entry github/a/b: %+v", todos.gaps)
	}
}
