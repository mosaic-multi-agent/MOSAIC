package artifact_test

// Tests that a closing section tag following content on the same line closes
// ExecutionLog, Artifacts and WorkflowNotes, for LF and CRLF files, and that
// the tag never lands in a cell value.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
)

func TestParse_ClosingTagAfterContent_ParsesToTheCanonicalState(t *testing.T) {
	base := baselineState(t)
	canonical := canonicalCloseTags(loadCloseTagFixture(t))

	for _, section := range tolerantSections {
		for _, eol := range lineEndings {
			t.Run(section+"/"+eol.name, func(t *testing.T) {
				doc := eol.conv(joinCloseTag(t, canonical, section))

				got, err := artifact.Parse([]byte(doc))

				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				if !reflect.DeepEqual(got, base) {
					t.Errorf("state differs from the canonical layout\n got: %+v\nwant: %+v", got, base)
				}
			})
		}
	}
}

func TestParse_ClosingTagAfterContent_AllThreeSectionsAtOnce(t *testing.T) {
	base := baselineState(t)
	doc := canonicalCloseTags(loadCloseTagFixture(t))
	for _, section := range tolerantSections {
		doc = joinCloseTag(t, doc, section)
	}

	for _, eol := range lineEndings {
		t.Run(eol.name, func(t *testing.T) {
			got, err := artifact.Parse([]byte(eol.conv(doc)))

			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, base) {
				t.Errorf("state differs from the canonical layout\n got: %+v\nwant: %+v", got, base)
			}
		})
	}
}

func TestParse_ClosingTagAfterContent_TagIsNotPartOfTheLastRowValues(t *testing.T) {
	canonical := canonicalCloseTags(loadCloseTagFixture(t))

	for _, section := range tolerantSections {
		t.Run(section, func(t *testing.T) {
			state, err := artifact.Parse([]byte(joinCloseTag(t, canonical, section)))

			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if row := lastRowText(t, state, section); strings.Contains(row, "</") || strings.Contains(row, section+">") {
				t.Errorf("last %s row carries the closing tag: %s", section, row)
			}
		})
	}
}

func TestParse_ClosingTagAfterContent_RecordedFileLayoutParsesToTheCanonicalState(t *testing.T) {
	// The fixture is the recorded file as found: CRLF, ExecutionLog tag after
	// its last row.
	base := baselineState(t)
	doc := lineEndings[1].conv(loadCloseTagFixture(t))

	got, err := artifact.Parse([]byte(doc))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(got, base) {
		t.Errorf("state differs from the canonical layout\n got: %+v\nwant: %+v", got, base)
	}
}

func TestParse_ClosingTagAfterContent_TrailingWhitespaceAfterTagIsIgnored(t *testing.T) {
	base := baselineState(t)
	canonical := canonicalCloseTags(loadCloseTagFixture(t))

	for _, trailer := range []string{"  ", "\t"} {
		doc := strings.Replace(joinCloseTag(t, canonical, "ExecutionLog"), "</ExecutionLog>", "</ExecutionLog>"+trailer, 1)

		got, err := artifact.Parse([]byte(doc))

		if err != nil {
			t.Fatalf("Parse with trailer %q: %v", trailer, err)
		}
		if !reflect.DeepEqual(got, base) {
			t.Errorf("trailer %q: state differs from the canonical layout", trailer)
		}
	}
}

func TestParse_TagInsideALineIsNotAClosingTag(t *testing.T) {
	// A closing tag with text after it on the same line does not close the
	// section: the rows below it still belong to the log.
	doc := canonicalCloseTags(loadCloseTagFixture(t))
	doc = strings.Replace(doc, "| COMPLETED_NEEDS_ACTION |", "| SUCCESS |", 1)
	doc = strings.Replace(doc, "Created requirements-review.md.", "Created requirements-review.md </ExecutionLog> and more.", 1)
	if !strings.Contains(doc, "</ExecutionLog> and more.") {
		t.Fatal("test setup: replacement did not apply")
	}

	state, err := artifact.Parse([]byte(doc))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(state.ExecutionLog) != 8 {
		t.Errorf("log rows = %d, want 8: a mid-line tag must not end the section", len(state.ExecutionLog))
	}
}
