package domain_test

// Tests for adding a user-interaction tool to settings and for tool-name validation.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestValidateToolName_RejectsEmptyWhitespaceAndControlCharacters(t *testing.T) {
	for _, name := range []string{"", "   ", "\t", "a\nb", "a\x00b", "a\tb", "a\rb"} {
		err := domain.ValidateToolName(name)
		if !errors.Is(err, domain.ErrInvalidToolName) {
			t.Errorf("ValidateToolName(%q) = %v, want ErrInvalidToolName", name, err)
		}
	}
}

func TestValidateToolName_AcceptsRealisticNamesAndPatterns(t *testing.T) {
	for _, name := range []string{"AskUserQuestion", "mcp__contact-user__*", "vscode/askQuestions", "ask_user"} {
		if err := domain.ValidateToolName(name); err != nil {
			t.Errorf("ValidateToolName(%q) = %v, want nil", name, err)
		}
	}
}

func TestWithUserInteractionTool_AppendsToExistingList(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	got, err := s.WithUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask")
	if err != nil {
		t.Fatalf("WithUserInteractionTool: %v", err)
	}

	want := []string{"AskUserQuestion", "mcp__x__ask"}
	if !reflect.DeepEqual(got.UserInteractionTools[domain.HarnessClaudeCode], want) {
		t.Errorf("claude-code list = %v, want %v", got.UserInteractionTools[domain.HarnessClaudeCode], want)
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask") {
		t.Error("the added tool must be recognised as user-interaction")
	}
}

func TestWithUserInteractionTool_DoesNotMutateReceiver(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	got, err := s.WithUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask")
	if err != nil {
		t.Fatalf("WithUserInteractionTool: %v", err)
	}

	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask") {
		t.Fatal("precondition: the copy must contain the added tool")
	}
	if s.IsUserInteractionTool(domain.HarnessClaudeCode, "mcp__x__ask") {
		t.Error("the receiver must stay unchanged; the method returns a copy")
	}
}

func TestWithUserInteractionTool_DuplicateIsNoOp(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	got, err := s.WithUserInteractionTool(domain.HarnessClaudeCode, "AskUserQuestion")
	if err != nil {
		t.Fatalf("WithUserInteractionTool: %v", err)
	}

	want := []string{"AskUserQuestion"}
	if !reflect.DeepEqual(got.UserInteractionTools[domain.HarnessClaudeCode], want) {
		t.Errorf("claude-code list = %v, want %v", got.UserInteractionTools[domain.HarnessClaudeCode], want)
	}
}

func TestWithUserInteractionTool_HarnessWithoutListStartsFromItsDefaults(t *testing.T) {
	s := domain.AnalysisSettings{
		UserInteractionTools: map[domain.Harness][]string{domain.HarnessOpenCode: {"custom"}},
	}

	got, err := s.WithUserInteractionTool(domain.HarnessGHCPCLI, "my_ask")
	if err != nil {
		t.Fatalf("WithUserInteractionTool: %v", err)
	}

	want := []string{"ask_user", "my_ask"}
	if !reflect.DeepEqual(got.UserInteractionTools[domain.HarnessGHCPCLI], want) {
		t.Errorf("ghcp-cli list = %v, want %v", got.UserInteractionTools[domain.HarnessGHCPCLI], want)
	}
	if !reflect.DeepEqual(got.UserInteractionTools[domain.HarnessOpenCode], []string{"custom"}) {
		t.Errorf("opencode list changed: %v", got.UserInteractionTools[domain.HarnessOpenCode])
	}
}

func TestWithUserInteractionTool_HarnessWithoutDefaultsGetsOnlyTheName(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	got, err := s.WithUserInteractionTool("brand-new-harness", "ask")
	if err != nil {
		t.Fatalf("WithUserInteractionTool: %v", err)
	}

	if !reflect.DeepEqual(got.UserInteractionTools["brand-new-harness"], []string{"ask"}) {
		t.Errorf("list = %v, want [ask]", got.UserInteractionTools["brand-new-harness"])
	}
}

func TestWithUserInteractionTool_EmptyHarnessCheckedBeforeName(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	_, err := s.WithUserInteractionTool("", "")

	if !errors.Is(err, domain.ErrInvalidHarness) {
		t.Errorf("err = %v, want ErrInvalidHarness (harness is checked first)", err)
	}
}

func TestWithUserInteractionTool_InvalidNameIsRejected(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	_, err := s.WithUserInteractionTool(domain.HarnessClaudeCode, "bad\nname")

	if !errors.Is(err, domain.ErrInvalidToolName) {
		t.Errorf("err = %v, want ErrInvalidToolName", err)
	}
	if s.IsUserInteractionTool(domain.HarnessClaudeCode, "bad\nname") {
		t.Error("receiver must be unchanged on error")
	}
}
