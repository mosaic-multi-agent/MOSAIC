package orchstate_test

// Tests that the ExecutionLog section is read when its closing tag follows the
// last table row on the same line (LF and CRLF), that the tag never lands in a
// cell value, and that every malformed condition is reported with the path,
// the section, the line where known, and a reason.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/orchstate"
)

const closeTagFixture = "execlog_close_tag_after_content.md"

var eolStyles = []struct {
	name string
	conv func(string) string
}{
	{"LF", func(s string) string { return s }},
	{"CRLF", func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }},
}

// loadCloseTagFixture returns the fixture with LF line endings.
func loadCloseTagFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", closeTagFixture))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// ownLineTag puts the ExecutionLog closing tag on its own line.
func ownLineTag(doc string) string {
	return regexp.MustCompile(`(?m)(\S)[ \t]*(</ExecutionLog>)`).ReplaceAllString(doc, "$1\n$2")
}

// tagAfterContent moves the ExecutionLog closing tag to the end of its last row.
func tagAfterContent(t *testing.T, doc string) string {
	t.Helper()
	out := regexp.MustCompile(`\s+(</ExecutionLog>)`).ReplaceAllString(doc, "$1")
	if !strings.Contains(out, "|</ExecutionLog>") {
		t.Fatal("test setup: closing tag is not joined to the last row")
	}
	return out
}

func lineNumberOf(t *testing.T, doc, needle string) int {
	t.Helper()
	for i, l := range strings.Split(doc, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("test setup: no line contains %q", needle)
	return 0
}

func baselineState(t *testing.T) domain.OrchestrationState {
	t.Helper()
	state, err := orchstate.Parse([]byte(ownLineTag(loadCloseTagFixture(t))))
	if err != nil {
		t.Fatalf("Parse(canonical fixture): %v", err)
	}
	if len(state.ExecutionLog) != 8 {
		t.Fatalf("baseline rows = %d, want 8", len(state.ExecutionLog))
	}
	return state
}

func TestParse_ClosingTagAfterLastRow_ParsesToTheCanonicalState(t *testing.T) {
	base := baselineState(t)
	doc := tagAfterContent(t, ownLineTag(loadCloseTagFixture(t)))

	for _, eol := range eolStyles {
		t.Run(eol.name, func(t *testing.T) {
			got, err := orchstate.Parse([]byte(eol.conv(doc)))

			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, base) {
				t.Errorf("state differs from the canonical layout\n got: %+v\nwant: %+v", got, base)
			}
		})
	}
}

func TestParse_ClosingTagAfterLastRow_RecordedFileLayoutParses(t *testing.T) {
	base := baselineState(t)

	got, err := orchstate.Parse([]byte(eolStyles[1].conv(loadCloseTagFixture(t))))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(got, base) {
		t.Errorf("state differs from the canonical layout")
	}
}

func TestParse_ClosingTagAfterLastRow_TagIsNotPartOfTheLastRowValues(t *testing.T) {
	doc := tagAfterContent(t, ownLineTag(loadCloseTagFixture(t)))

	state, err := orchstate.Parse([]byte(doc))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(state.ExecutionLog) != 8 {
		t.Fatalf("rows = %d, want 8: the last row must be read", len(state.ExecutionLog))
	}
	last := state.ExecutionLog[len(state.ExecutionLog)-1]
	if last.Seq != 8 {
		t.Errorf("last Seq = %d, want 8", last.Seq)
	}
	if text := fmt.Sprintf("%+v", last); strings.Contains(text, "</") {
		t.Errorf("last row carries the closing tag: %s", text)
	}
}

func TestParse_ClosingTagAfterLastRow_TrailingWhitespaceAfterTagIsIgnored(t *testing.T) {
	base := baselineState(t)
	doc := tagAfterContent(t, ownLineTag(loadCloseTagFixture(t)))

	for _, trailer := range []string{"  ", "\t"} {
		got, err := orchstate.Parse([]byte(strings.Replace(doc, "</ExecutionLog>", "</ExecutionLog>"+trailer, 1)))

		if err != nil {
			t.Fatalf("Parse with trailer %q: %v", trailer, err)
		}
		if !reflect.DeepEqual(got, base) {
			t.Errorf("trailer %q: state differs from the canonical layout", trailer)
		}
	}
}

func TestParse_TagInsideALine_DoesNotEndTheExecutionLog(t *testing.T) {
	doc := ownLineTag(loadCloseTagFixture(t))
	doc = strings.Replace(doc, "Created requirements-review.md.", "Created requirements-review.md </ExecutionLog> and more.", 1)
	if !strings.Contains(doc, "</ExecutionLog> and more.") {
		t.Fatal("test setup: replacement did not apply")
	}

	state, err := orchstate.Parse([]byte(doc))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(state.ExecutionLog) != 8 {
		t.Errorf("rows = %d, want 8: a mid-line tag must not end the section", len(state.ExecutionLog))
	}
}

// ---- malformed conditions carry path, section, line and reason ----

func asMalformed(t *testing.T, err error) *orchstate.MalformedError {
	t.Helper()
	var me *orchstate.MalformedError
	if !errors.As(err, &me) {
		t.Fatalf("error is %T (%v), want *orchstate.MalformedError", err, err)
	}
	if !errors.Is(err, orchstate.ErrDocumentMalformed) {
		t.Errorf("errors.Is(err, ErrDocumentMalformed) = false for %v", err)
	}
	if me.Reason == "" {
		t.Error("Reason is empty")
	}
	return me
}

func TestParse_MissingClosingTag_IsMalformedForExecutionLog(t *testing.T) {
	doc := strings.Replace(ownLineTag(loadCloseTagFixture(t)), "</ExecutionLog>", "", 1)

	_, err := orchstate.Parse([]byte(doc))

	me := asMalformed(t, err)
	if me.Section != "ExecutionLog" {
		t.Errorf("Section = %q, want ExecutionLog", me.Section)
	}
	if me.Path != "" {
		t.Errorf("Path = %q, want empty from Parse", me.Path)
	}
}

func TestParse_InvalidRowValue_NamesSectionAndFileLine(t *testing.T) {
	doc := ownLineTag(loadCloseTagFixture(t))
	doc = strings.Replace(doc, "| 5 | requirements-review#5 |", "| five | requirements-review#5 |", 1)
	want := lineNumberOf(t, doc, "| five | requirements-review#5 |")

	for _, eol := range eolStyles {
		t.Run(eol.name, func(t *testing.T) {
			_, err := orchstate.Parse([]byte(eol.conv(doc)))

			me := asMalformed(t, err)
			if me.Section != "ExecutionLog" || me.Line != want {
				t.Errorf("Section/Line = %q/%d, want ExecutionLog/%d", me.Section, me.Line, want)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("line %d", want)) {
				t.Errorf("error text %q does not name line %d", err.Error(), want)
			}
		})
	}
}

func TestParse_UnparseableDocument_IsMalformedForDocument(t *testing.T) {
	_, err := orchstate.Parse([]byte(malformedDocument))

	me := asMalformed(t, err)
	if me.Section != "document" {
		t.Errorf("Section = %q, want document", me.Section)
	}
}

func TestParseFile_Malformed_NamesThePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	doc := strings.Replace(ownLineTag(loadCloseTagFixture(t)), "</ExecutionLog>", "", 1)
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := orchstate.ParseFile(path)

	me := asMalformed(t, err)
	if me.Path != path {
		t.Errorf("Path = %q, want %q", me.Path, path)
	}
	for _, want := range []string{path, "ExecutionLog"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error text %q does not contain %q", err.Error(), want)
		}
	}
}

func TestParseFile_TagAfterLastRow_ReadsFromDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	if err := os.WriteFile(path, []byte(eolStyles[1].conv(loadCloseTagFixture(t))), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	state, err := orchstate.ParseFile(path)

	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(state.ExecutionLog) != 8 {
		t.Errorf("rows = %d, want 8", len(state.ExecutionLog))
	}
}
