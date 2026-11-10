package artifact_test

// Tests that every structural damage the reader finds is reported as an
// artifact damage error naming the section, the file path (through ParseAt and
// the store) and, where the parser can tell, the line; and that a present but
// unreadable WorkflowNotes section is reported, not dropped.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const refusalPath = "C:/work/Orchestration-x/Orchestration.md"

// refusalCase is one broken variant of the canonical fixture.
type refusalCase struct {
	name       string
	section    string
	mutate     func(doc string) string
	lineNeedle string // when set, Line must be the line holding this text in the mutated document
}

func removeCloseTag(section string) func(string) string {
	return func(doc string) string {
		return strings.Replace(doc, "</"+section+">", "", 1)
	}
}

func replaceOnce(old, repl string) func(string) string {
	return func(doc string) string { return strings.Replace(doc, old, repl, 1) }
}

var refusalCases = []refusalCase{
	{name: "ExecutionLog closing tag missing", section: "ExecutionLog", mutate: removeCloseTag("ExecutionLog")},
	{name: "Artifacts closing tag missing", section: "Artifacts", mutate: removeCloseTag("Artifacts")},
	{name: "WorkflowNotes closing tag missing", section: "WorkflowNotes", mutate: removeCloseTag("WorkflowNotes")},
	{
		name: "ExecutionLog row with an extra cell", section: "ExecutionLog",
		mutate:     replaceOnce("| 5 | requirements-review#5 |", "| 5 | extra | requirements-review#5 |"),
		lineNeedle: "| 5 | extra | requirements-review#5 |",
	},
	{
		name: "Artifacts row with an extra cell", section: "Artifacts",
		mutate:     replaceOnce("| Research.md | RESEARCH |", "| Research.md | RESEARCH | extra | x |"),
		lineNeedle: "| Research.md | RESEARCH | extra | x |",
	},
	{
		name: "WorkflowNotes row with an extra cell", section: "WorkflowNotes",
		mutate:     replaceOnce("| 0 | User instruction:", "| 0 | extra | User instruction:"),
		lineNeedle: "| 0 | extra | User instruction:",
	},
	{
		name: "WorkflowNotes header without a Note column", section: "WorkflowNotes",
		mutate: replaceOnce("| Seq | Note |", "| Seq | Text |"),
	},
}

func canonicalMutated(t *testing.T, c refusalCase) string {
	t.Helper()
	base := canonicalCloseTags(loadCloseTagFixture(t))
	doc := c.mutate(base)
	if doc == base {
		t.Fatal("test setup: mutation did not change the document")
	}
	return doc
}

func TestParseAt_Damage_NamesPathAndSection(t *testing.T) {
	for _, c := range refusalCases {
		t.Run(c.name, func(t *testing.T) {
			doc := canonicalMutated(t, c)

			_, err := artifact.ParseAt(refusalPath, []byte(doc))

			if err == nil {
				t.Fatal("ParseAt succeeded, want a damage error")
			}
			d := asDamageError(t, err)
			if d.Resource != refusalPath {
				t.Errorf("Resource = %q, want %q", d.Resource, refusalPath)
			}
			if len(problemLines(d, c.section)) == 0 {
				t.Errorf("no problem in section %q: %+v", c.section, d.Problems)
			}
			for _, want := range []string{refusalPath, c.section} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error text %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}

func TestParse_Damage_HasSectionAndNoPath(t *testing.T) {
	for _, c := range refusalCases {
		t.Run(c.name, func(t *testing.T) {
			_, err := artifact.Parse([]byte(canonicalMutated(t, c)))

			if err == nil {
				t.Fatal("Parse succeeded, want a damage error")
			}
			d := asDamageError(t, err)
			if len(problemLines(d, c.section)) == 0 {
				t.Errorf("no problem in section %q: %+v", c.section, d.Problems)
			}
			if d.Resource != "" {
				t.Errorf("Resource = %q, want empty from Parse", d.Resource)
			}
		})
	}
}

func TestParseAt_Damage_LineIsTheOffendingRow(t *testing.T) {
	for _, c := range refusalCases {
		if c.lineNeedle == "" {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			doc := canonicalMutated(t, c)
			want := lineOf(t, doc, c.lineNeedle)

			_, err := artifact.ParseAt(refusalPath, []byte(doc))

			requireDamageProblem(t, asDamageError(t, err), c.section, want)
		})
	}
}

func TestParseAt_Damage_LineIsTheSameForCRLF(t *testing.T) {
	c := refusalCases[3]
	doc := canonicalMutated(t, c)
	want := lineOf(t, doc, c.lineNeedle)

	_, err := artifact.ParseAt(refusalPath, []byte(lineEndings[1].conv(doc)))

	requireDamageProblem(t, asDamageError(t, err), c.section, want)
}

func TestParse_UnreadableWorkflowNotes_IsReportedNotDropped(t *testing.T) {
	// An opening tag with nothing that can close it used to read as "no notes".
	doc := canonicalCloseTags(loadCloseTagFixture(t))
	doc = removeCloseTag("WorkflowNotes")(doc)

	state, err := artifact.Parse([]byte(doc))

	if err == nil {
		t.Fatalf("Parse succeeded with %d notes, want a WorkflowNotes damage error", len(state.WorkflowNotes))
	}
	d := asDamageError(t, err)
	if len(problemLines(d, "WorkflowNotes")) == 0 {
		t.Errorf("no problem in WorkflowNotes: %+v", d.Problems)
	}
}

func TestParse_AbsentWorkflowNotes_MeansNoNotes(t *testing.T) {
	doc := canonicalCloseTags(loadCloseTagFixture(t))
	i := strings.Index(doc, "<WorkflowNotes")
	if i < 0 {
		t.Fatal("test setup: fixture has no WorkflowNotes section")
	}

	state, err := artifact.Parse([]byte(doc[:i]))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(state.WorkflowNotes) != 0 {
		t.Errorf("notes = %d, want 0", len(state.WorkflowNotes))
	}
}

func TestParseAt_ExistingFixtureDamage_NamesItsPathAndSection(t *testing.T) {
	fixtures := []struct{ name, section string }{
		{"no-type.md", "frontmatter"},
		{"truncated.md", "ExecutionLog"},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			path := fixturePath(f.name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			_, perr := artifact.ParseAt(path, data)

			d := asDamageError(t, perr)
			if d.Resource != path {
				t.Errorf("Resource = %q, want %q", d.Resource, path)
			}
			if len(problemLines(d, f.section)) == 0 {
				t.Errorf("no problem in section %q: %+v", f.section, d.Problems)
			}
		})
	}
}

func TestRefusalError_Error_AppendsSectionAndLineOnlyWhenSet(t *testing.T) {
	base := domain.RefusalError{Component: "artifact", Resource: "p.md", Reason: "bad", Row: domain.NoWorkflowRow}
	plain := base
	full := base
	full.Section, full.Line = "Artifacts", 7

	if got, want := plain.Error(), `artifact: refused "p.md": bad`; got != want {
		t.Errorf("without Section/Line: %q, want unchanged %q", got, want)
	}
	for _, want := range []string{" [section Artifacts]", " [line 7]"} {
		if !strings.Contains(full.Error(), want) {
			t.Errorf("%q does not contain %q", full.Error(), want)
		}
	}
}

func TestParseAt_FrontmatterDamage_IsSectionFrontmatter(t *testing.T) {
	path := fixturePath("no-type.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	_, perr := artifact.ParseAt(path, data)

	d := asDamageError(t, perr)
	if len(problemLines(d, "frontmatter")) == 0 {
		t.Errorf("no problem in section frontmatter: %+v", d.Problems)
	}
	if d.Resource != path {
		t.Errorf("Resource = %q, want %q", d.Resource, path)
	}
}

func TestParseAt_NonDamageResultsAreNotAltered(t *testing.T) {
	// A success must stay a success and carry the same state as Parse.
	doc := []byte(canonicalCloseTags(loadCloseTagFixture(t)))

	viaParse, err1 := artifact.Parse(doc)
	viaParseAt, err2 := artifact.ParseAt(refusalPath, doc)

	if err1 != nil || err2 != nil {
		t.Fatalf("Parse/ParseAt errors: %v / %v", err1, err2)
	}
	if viaParse.RunID != viaParseAt.RunID || len(viaParse.ExecutionLog) != len(viaParseAt.ExecutionLog) {
		t.Error("ParseAt state differs from Parse state")
	}
}

func TestStoreRead_UnreadableSection_DamageNamesFileAndSection(t *testing.T) {
	doc := removeCloseTag("WorkflowNotes")(canonicalCloseTags(loadCloseTagFixture(t)))
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := artifact.NewFileStore(path).Read(context.Background())

	d := asDamageError(t, err)
	if d.Resource != path {
		t.Errorf("Resource = %q, want %q", d.Resource, path)
	}
	if len(problemLines(d, "WorkflowNotes")) == 0 {
		t.Errorf("no problem in WorkflowNotes: %+v", d.Problems)
	}
}
