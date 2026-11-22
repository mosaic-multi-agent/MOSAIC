package app_test

// promote_tool_answer_batch_test.go covers how PromoteBatch consumes the answer to the generic
// tool name question: a padded name is trimmed in every agent declaring the tool, a
// whitespace-only answer is an empty answer (the harness name is carried verbatim), and several
// harness tools given the same name appear once per agent.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestPromoteBatch_PromoteToolAnswer_PaddedNameIsTrimmedInEveryAgent verifies a padded single
// name reaches every agent declaring the tool trimmed.
func TestPromoteBatch_PromoteToolAnswer_PaddedNameIsTrimmedInEveryAgent(t *testing.T) {
	// Arrange
	ls, spy := newToolBatch(t)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = typed("  file-reader\t")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))

	// Assert
	for _, f := range []string{"a-agent.md", "b-agent.md"} {
		tools := requirePromoted(t, res, f).Result.Tools
		if !batchContains(tools, "file-reader") {
			t.Errorf("%s tools = %q, want the trimmed name file-reader", f, tools)
		}
		requireNoSpacedTool(t, tools)
	}
}

// TestPromoteBatch_PromoteToolAnswer_WhitespaceOnlyCarriesHarnessNameVerbatim verifies a
// whitespace-only answer keeps the harness name in every agent declaring it, listed as
// verbatim, and no space-named tool appears.
func TestPromoteBatch_PromoteToolAnswer_WhitespaceOnlyCarriesHarnessNameVerbatim(t *testing.T) {
	// Arrange
	ls, spy := newToolBatch(t)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = typed("   \t ")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))

	// Assert
	for _, f := range []string{"a-agent.md", "b-agent.md"} {
		got := requirePromoted(t, res, f).Result
		if !batchContains(got.Tools, "tool-one") {
			t.Errorf("%s tools = %q, want tool-one carried verbatim", f, got.Tools)
		}
		if !batchContains(got.VerbatimTools, "tool-one") {
			t.Errorf("%s VerbatimTools = %q, want tool-one listed", f, got.VerbatimTools)
		}
		requireNoSpacedTool(t, got.Tools)
	}
}

// TestPromoteBatch_PromoteToolAnswer_SameNameForSeveralToolsAppearsOncePerAgent verifies two
// harness tools given the same name, differently padded, yield that name once in the agent
// declaring both.
func TestPromoteBatch_PromoteToolAnswer_SameNameForSeveralToolsAppearsOncePerAgent(t *testing.T) {
	// Arrange
	ls, spy := newToolBatch(t)
	spy.text[askKey{domain.QPromoteCustomTool, "tool-one"}] = typed(" shared")
	spy.text[askKey{domain.QPromoteCustomTool, "tool-two"}] = typed("shared  ")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))

	// Assert
	n := 0
	tools := requirePromoted(t, res, "a-agent.md").Result.Tools
	for _, tool := range tools {
		if tool == "shared" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("a-agent tools = %q, want shared exactly once (got %d)", tools, n)
	}
	requireNoSpacedTool(t, tools)
}
