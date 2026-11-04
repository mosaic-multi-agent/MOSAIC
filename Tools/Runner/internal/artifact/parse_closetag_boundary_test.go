package artifact_test

// Boundary cases of the closing-tag rule: a tag inside a table row is not a
// close (any tolerant section), and a tag alone on an indented line still is.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
)

func TestParse_TagInsideATableRow_IsNotAClosingTag(t *testing.T) {
	base := baselineState(t)
	cases := []struct {
		section string
		old     string
		repl    string
	}{
		{"Artifacts", "| Research.md | RESEARCH |", "| Research.md </Artifacts> | RESEARCH |"},
		{"WorkflowNotes", "| 0 | User instruction:", "| 0 | </WorkflowNotes> User instruction:"},
	}
	for _, c := range cases {
		t.Run(c.section, func(t *testing.T) {
			doc := strings.Replace(canonicalCloseTags(loadCloseTagFixture(t)), c.old, c.repl, 1)
			if !strings.Contains(doc, c.repl) {
				t.Fatal("test setup: replacement did not apply")
			}

			state, err := artifact.Parse([]byte(doc))

			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(state.ArtifactRegistry) != len(base.ArtifactRegistry) || len(state.WorkflowNotes) != len(base.WorkflowNotes) {
				t.Errorf("registry/notes = %d/%d, want %d/%d: a tag inside a row must not end the section",
					len(state.ArtifactRegistry), len(state.WorkflowNotes), len(base.ArtifactRegistry), len(base.WorkflowNotes))
			}
		})
	}
}

func TestParse_IndentedClosingTagOnItsOwnLine_IsAccepted(t *testing.T) {
	base := baselineState(t)
	for _, sec := range tolerantSections {
		t.Run(sec, func(t *testing.T) {
			doc := strings.Replace(canonicalCloseTags(loadCloseTagFixture(t)), "\n</"+sec+">", "\n   </"+sec+">", 1)
			if !strings.Contains(doc, "   </"+sec+">") {
				t.Fatal("test setup: indentation did not apply")
			}

			got, err := artifact.Parse([]byte(doc))

			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, base) {
				t.Error("state differs from the canonical layout")
			}
		})
	}
}
