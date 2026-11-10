package artifact_test

// Helpers for the closing-tag tolerance and refusal tests. The fixture is a
// copy of a real orchestration file whose ExecutionLog closing tag follows its
// last table row on the same line.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const closeTagFixture = "execlog_close_tag_after_content.md"

var tolerantSections = []string{"ExecutionLog", "Artifacts", "WorkflowNotes"}

// lineEnding names one line-ending style a test runs under.
type lineEnding struct {
	name string
	conv func(string) string
}

var lineEndings = []lineEnding{
	{"LF", func(s string) string { return s }},
	{"CRLF", func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }},
}

// loadCloseTagFixture returns the fixture with LF line endings.
func loadCloseTagFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(fixturePath(closeTagFixture))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// canonicalCloseTags puts every closing tag of the tolerant sections on its own line.
func canonicalCloseTags(doc string) string {
	for _, sec := range tolerantSections {
		re := regexp.MustCompile(`(?m)(\S)[ \t]*(</` + sec + `>)`)
		doc = re.ReplaceAllString(doc, "$1\n$2")
	}
	return doc
}

// joinCloseTag moves the closing tag of one section onto the end of the
// section's last content line. Input must be in canonical form.
func joinCloseTag(t *testing.T, doc, section string) string {
	t.Helper()
	re := regexp.MustCompile(`\s+(</` + section + `>)`)
	out := re.ReplaceAllString(doc, "$1")
	if !strings.Contains(out, "|</"+section+">") {
		t.Fatalf("test setup: closing tag of %s is not joined to its last row", section)
	}
	return out
}

// baselineState parses the canonical form of the fixture and checks it holds
// the content the tolerance tests compare against.
func baselineState(t *testing.T) domain.ArtifactState {
	t.Helper()
	state, err := artifact.Parse([]byte(canonicalCloseTags(loadCloseTagFixture(t))))
	if err != nil {
		t.Fatalf("Parse(canonical fixture): %v", err)
	}
	if len(state.ExecutionLog) != 8 || len(state.ArtifactRegistry) < 10 || len(state.WorkflowNotes) != 2 {
		t.Fatalf("baseline counts: log=%d registry=%d notes=%d, want 8/10+/2",
			len(state.ExecutionLog), len(state.ArtifactRegistry), len(state.WorkflowNotes))
	}
	return state
}

// lastRowText renders the last row of a section for substring checks. It
// fails the test when the section holds no rows.
func lastRowText(t *testing.T, state domain.ArtifactState, section string) string {
	t.Helper()
	var rows []any
	switch section {
	case "ExecutionLog":
		for _, r := range state.ExecutionLog {
			rows = append(rows, r)
		}
	case "Artifacts":
		for _, r := range state.ArtifactRegistry {
			rows = append(rows, r)
		}
	default:
		for _, r := range state.WorkflowNotes {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		t.Fatalf("%s holds no rows", section)
	}
	return fmtRow(rows[len(rows)-1])
}

// lineOf returns the 1-based line of the first line of doc that contains needle.
func lineOf(t *testing.T, doc, needle string) int {
	t.Helper()
	for i, l := range strings.Split(doc, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("test setup: no line contains %q", needle)
	return 0
}

// fmtRow renders any table row value for substring checks.
func fmtRow(v any) string { return fmt.Sprintf("%+v", v) }
