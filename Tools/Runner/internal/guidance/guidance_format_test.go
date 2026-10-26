package guidance_test

// Tests for the guidance file format: what Format writes, what Parse accepts
// (including hand-edited files), and the round trip of arbitrary text.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/guidance"
)

func TestFormat_WritesCommentsThenBothSectionsInOrder(t *testing.T) {
	g := domain.Guidance{TaskDescription: "task line 1\ntask line 2", Constraints: "be careful"}

	got := string(guidance.Format(g))

	want := "# MOSAIC Runner standing guidance for this run.\n" +
		"# Appended to every engine-routed dispatch. Edit the text under each heading;\n" +
		"# the Runner reads this file when a session starts.\n" +
		"[task_description]\n" +
		"task line 1\ntask line 2\n" +
		"\n" +
		"[constraints]\n" +
		"be careful\n"
	if got != want {
		t.Errorf("want the pinned file layout:\n%q\ngot:\n%q", want, got)
	}
}

func TestFormat_EmptyGuidance_StillWritesBothHeadersAndEndsWithOneNewline(t *testing.T) {
	got := string(guidance.Format(domain.Guidance{}))

	if !strings.Contains(got, "[task_description]\n") || !strings.Contains(got, "[constraints]\n") {
		t.Errorf("want both headers even for empty guidance, got %q", got)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n\n") {
		t.Errorf("want the file to end with a single newline, got %q", got)
	}
}

func TestFormat_FieldLineLookingLikeHeader_IsEscapedWithLeadingBackslash(t *testing.T) {
	g := domain.Guidance{TaskDescription: "a\n[constraints]\nb", Constraints: "\\already"}

	got := string(guidance.Format(g))

	if !strings.Contains(got, "\n\\[constraints]\n") {
		t.Errorf("want the header-like field line written with one extra leading backslash, got %q", got)
	}
	if !strings.Contains(got, "\n\\\\already\n") {
		t.Errorf("want a field line starting with a backslash written with one extra backslash, got %q", got)
	}
}

func TestParse_RoundTrip_ReturnsNormalizedGuidance(t *testing.T) {
	cases := map[string]domain.Guidance{
		"empty":                {},
		"only task":            {TaskDescription: "only task"},
		"only constraints":     {Constraints: "only constraints"},
		"multi-line":           {TaskDescription: "one\ntwo\nthree", Constraints: "x\ny"},
		"blank-line paragraph": {TaskDescription: "p1\n\np2\n\n\np3", Constraints: "c1\n\nc2"},
		"hash lines":           {TaskDescription: "# not a comment\n## heading", Constraints: "#also content"},
		"header lookalike":     {TaskDescription: "[task_description]\n[constraints]", Constraints: "[constraints]  \n[task_description]\t"},
		"backslash lines":      {TaskDescription: "\\[task_description]\n\\\\x\n\\", Constraints: "\\"},
		"leading whitespace":   {TaskDescription: "  indented\n\tTabbed", Constraints: " lead"},
		"leading blank lines":  {TaskDescription: "\n\nafter blanks", Constraints: "\nx"},
		"crlf input":           {TaskDescription: "a\r\nb\rc", Constraints: "d\r\n"},
		"trailing newlines":    {TaskDescription: "text\n\n\n", Constraints: "\n"},
		"non-ascii":            {TaskDescription: "caf\u00e9 \u2192 ok", Constraints: "\u65e5\u672c\u8a9e"},
	}
	for name, g := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := guidance.Parse(guidance.Format(g))

			if err != nil {
				t.Fatalf("want Parse(Format(g)) to succeed, got %v", err)
			}
			if want := g.Normalized(); got != want {
				t.Errorf("want round trip %+v, got %+v", want, got)
			}
		})
	}
}

func TestParse_RoundTrip_KeepsFieldsSeparate(t *testing.T) {
	g := domain.Guidance{TaskDescription: "T", Constraints: "C"}

	got, err := guidance.Parse(guidance.Format(g))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.TaskDescription != "T" || got.Constraints != "C" {
		t.Errorf("want task_description=T constraints=C, got %+v", got)
	}
}

func TestParse_HandEditedFile_ReadsBothFields(t *testing.T) {
	data := "# my own note\n\n[task_description]\nUse British spelling.\n# kept: a hash line inside a field\n\n[constraints]\nNo new dependencies.\n"

	got, err := guidance.Parse([]byte(data))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	want := domain.Guidance{
		TaskDescription: "Use British spelling.\n# kept: a hash line inside a field",
		Constraints:     "No new dependencies.",
	}
	if got != want {
		t.Errorf("want %+v, got %+v", want, got)
	}
}

func TestParse_HeadersInEitherOrder(t *testing.T) {
	got, err := guidance.Parse([]byte("[constraints]\nC\n[task_description]\nT\n"))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.TaskDescription != "T" || got.Constraints != "C" {
		t.Errorf("want task_description=T constraints=C, got %+v", got)
	}
}

func TestParse_MissingHeader_MeansEmptyField(t *testing.T) {
	got, err := guidance.Parse([]byte("[task_description]\nonly this\n"))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.TaskDescription != "only this" || got.Constraints != "" {
		t.Errorf("want only task_description set, got %+v", got)
	}
}

func TestParse_WindowsLineBreaksAndHeaderTrailingWhitespace(t *testing.T) {
	got, err := guidance.Parse([]byte("[task_description] \t\r\nline1\r\nline2\r\n[constraints]\r\nc\r\n"))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.TaskDescription != "line1\nline2" || got.Constraints != "c" {
		t.Errorf("want line breaks read as \\n, got %+v", got)
	}
}

func TestParse_EmptyOrCommentsOnly_GivesZeroGuidance(t *testing.T) {
	for name, data := range map[string]string{"empty": "", "blank": "\n\n", "comments": "# a\n\n# b\n"} {
		t.Run(name, func(t *testing.T) {
			got, err := guidance.Parse([]byte(data))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if got != (domain.Guidance{}) {
				t.Errorf("want zero guidance, got %+v", got)
			}
		})
	}
}

func TestParse_TextBeforeFirstHeader_IsFormatErrorWithLineNumber(t *testing.T) {
	_, err := guidance.Parse([]byte("# ok\n\nstray text\n[task_description]\nx\n"))

	var fe *guidance.FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("want a *FormatError, got %v", err)
	}
	if fe.Line != 3 {
		t.Errorf("want the error on line 3, got line %d", fe.Line)
	}
}

func TestParse_DuplicateHeader_IsFormatError(t *testing.T) {
	_, err := guidance.Parse([]byte("[constraints]\na\n[task_description]\nb\n[constraints]\nc\n"))

	var fe *guidance.FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("want a *FormatError for a duplicate header, got %v", err)
	}
	if fe.Line != 5 {
		t.Errorf("want the error on line 5, got line %d", fe.Line)
	}
}

func TestParse_EscapedLine_LosesOneLeadingBackslash(t *testing.T) {
	got, err := guidance.Parse([]byte("[task_description]\n\\[constraints]\n\\\\x\n"))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.TaskDescription != "[constraints]\n\\x" || got.Constraints != "" {
		t.Errorf("want the escaped lines unescaped inside task_description, got %+v", got)
	}
}
