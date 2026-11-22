package app_test

// promote_tool_answer_single_test.go covers how single-file Promote consumes the answer to the
// generic tool name question: a padded name is trimmed, a whitespace-only answer is an empty
// answer (the harness name is carried verbatim), and skip, skip all, cancel and empty answers
// keep carrying the harness name verbatim.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// textCapture is a Stub that also records every text question it is asked, in full.
type textCapture struct {
	*interactiontest.Stub
	asked []domain.TextQuestion
}

func (c *textCapture) AskText(ctx context.Context, q domain.TextQuestion) (domain.TextAnswer, error) {
	c.asked = append(c.asked, q)
	return c.Stub.AskText(ctx, q)
}

// promoteTools runs single-file Promote over an agent declaring the given unmapped harness
// tools, with the scripted answers, and returns the result and the capturing interaction.
func promoteTools(t *testing.T, script *interactiontest.Builder, tools ...string) (app.PromoteResult, *textCapture) {
	t.Helper()
	mosaicRoot := writeMosaicRoot(t)
	srcPath := filepath.Join(t.TempDir(), "tool-test-agent.md")
	if err := os.WriteFile(srcPath, eligibleAgentWithHarnessTools(tools...), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	capture := &textCapture{Stub: script.Build()}
	deps := newToolResolutionPromoteDeps(t, capture.Stub, mosaicRoot)
	deps.Interaction = capture

	result, err := app.New(deps).Promote(context.Background(), app.PromoteRequest{
		FilePath:  srcPath,
		Category:  "TestCategory",
		HarnessID: toolResolutionHarness.ID,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	return result, capture
}

// hasWhitespace reports whether s holds any whitespace rune.
func hasWhitespace(s string) bool { return strings.ContainsFunc(s, unicode.IsSpace) }

// requireNoSpacedTool fails when any tool name holds whitespace.
func requireNoSpacedTool(t *testing.T, tools []string) {
	t.Helper()
	for _, tool := range tools {
		if hasWhitespace(tool) {
			t.Errorf("tools = %q, want no tool name with whitespace", tools)
			return
		}
	}
}

// TestPromote_PromoteToolAnswer_PaddedNameIsTrimmed verifies a single name with surrounding
// whitespace lands in the tools list trimmed, and is not reported as verbatim.
func TestPromote_PromoteToolAnswer_PaddedNameIsTrimmed(t *testing.T) {
	// Arrange
	script := interactiontest.NewBuilder().
		AnswerText(domain.QPromoteCustomTool, "my-custom-tool", "  file-reader \t")

	// Act
	result, _ := promoteTools(t, script, "my-custom-tool")

	// Assert
	if !batchContains(result.Tools, "file-reader") {
		t.Errorf("Tools = %q, want the trimmed name file-reader", result.Tools)
	}
	requireNoSpacedTool(t, result.Tools)
	if batchContains(result.VerbatimTools, "my-custom-tool") {
		t.Errorf("VerbatimTools = %q, want my-custom-tool absent after an answer", result.VerbatimTools)
	}
}

// TestPromote_PromoteToolAnswer_WhitespaceOnlyCarriesHarnessNameVerbatim verifies a
// whitespace-only answer is treated as empty: the harness name is carried and reported as
// verbatim, and no tool named with spaces appears.
func TestPromote_PromoteToolAnswer_WhitespaceOnlyCarriesHarnessNameVerbatim(t *testing.T) {
	// Arrange
	script := interactiontest.NewBuilder().
		AnswerText(domain.QPromoteCustomTool, "my-custom-tool", "   \t ")

	// Act
	result, _ := promoteTools(t, script, "my-custom-tool")

	// Assert
	if !batchContains(result.Tools, "my-custom-tool") {
		t.Errorf("Tools = %q, want my-custom-tool carried verbatim", result.Tools)
	}
	if !batchContains(result.VerbatimTools, "my-custom-tool") {
		t.Errorf("VerbatimTools = %q, want my-custom-tool listed", result.VerbatimTools)
	}
	requireNoSpacedTool(t, result.Tools)
}

// TestPromote_PromoteToolAnswer_EmptyAnswerCarriesHarnessNameVerbatim verifies an empty answer
// keeps the harness name, listed as verbatim.
func TestPromote_PromoteToolAnswer_EmptyAnswerCarriesHarnessNameVerbatim(t *testing.T) {
	// Arrange
	script := interactiontest.NewBuilder().
		AnswerText(domain.QPromoteCustomTool, "my-custom-tool", "")

	// Act
	result, _ := promoteTools(t, script, "my-custom-tool")

	// Assert
	if !batchContains(result.Tools, "my-custom-tool") || !batchContains(result.VerbatimTools, "my-custom-tool") {
		t.Errorf("Tools = %q, VerbatimTools = %q, want my-custom-tool in both", result.Tools, result.VerbatimTools)
	}
}

// TestPromote_PromoteToolAnswer_SkipAndCancelCarryHarnessNameVerbatim verifies skip and cancel
// keep the harness name, listed as verbatim, and a cancel does not abort the promote.
func TestPromote_PromoteToolAnswer_SkipAndCancelCarryHarnessNameVerbatim(t *testing.T) {
	cases := map[string]*interactiontest.Builder{
		"skip":   interactiontest.NewBuilder(),
		"cancel": interactiontest.NewBuilder().AnswerCancelledText(domain.QPromoteCustomTool, "my-custom-tool"),
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			result, _ := promoteTools(t, script, "my-custom-tool")

			// Assert
			if !batchContains(result.Tools, "my-custom-tool") || !batchContains(result.VerbatimTools, "my-custom-tool") {
				t.Errorf("Tools = %q, VerbatimTools = %q, want my-custom-tool in both", result.Tools, result.VerbatimTools)
			}
		})
	}
}

// TestPromote_PromoteToolAnswer_SkipAllCarriesRemainingToolsVerbatimWithoutAsking verifies skip
// all keeps this and every later tool name and asks no further tool question.
func TestPromote_PromoteToolAnswer_SkipAllCarriesRemainingToolsVerbatimWithoutAsking(t *testing.T) {
	// Arrange
	script := interactiontest.NewBuilder().AnswerSkipAll(domain.QPromoteCustomTool, "tool-one")

	// Act
	result, capture := promoteTools(t, script, "tool-one", "tool-two")

	// Assert
	for _, tool := range []string{"tool-one", "tool-two"} {
		if !batchContains(result.Tools, tool) || !batchContains(result.VerbatimTools, tool) {
			t.Errorf("Tools = %q, VerbatimTools = %q, want %s in both", result.Tools, result.VerbatimTools, tool)
		}
	}
	if n := capture.CountAsked(domain.QPromoteCustomTool, "tool-two"); n != 0 {
		t.Errorf("tool-two asked %d times after skip all, want 0", n)
	}
}

// TestPromote_PromoteToolAnswer_SameNameForSeveralToolsAppearsOnce verifies two harness tools
// given the same name, differently padded, yield that name once.
func TestPromote_PromoteToolAnswer_SameNameForSeveralToolsAppearsOnce(t *testing.T) {
	// Arrange
	script := interactiontest.NewBuilder().
		AnswerText(domain.QPromoteCustomTool, "tool-one", " shared").
		AnswerText(domain.QPromoteCustomTool, "tool-two", "shared  ")

	// Act
	result, _ := promoteTools(t, script, "tool-one", "tool-two")

	// Assert
	n := 0
	for _, tool := range result.Tools {
		if tool == "shared" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Tools = %q, want shared exactly once (got %d)", result.Tools, n)
	}
}
