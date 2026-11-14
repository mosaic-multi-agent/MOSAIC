package artifact_test

// Tests that the reader refuses a damaged Orchestration.md with a damage error
// naming the section and the file line of each problem, instead of parsing
// around the damage. Assertions cover the error kind, the section and the line;
// never the wording.

import (
	"os"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
)

// damageCase is one way to damage a compliant document.
type damageCase struct {
	name    string
	mutate  func(d *strictDoc)
	section string
	needle  string // text unique to the line the first problem must be reported at
}

func (c damageCase) wantLine(t *testing.T, doc string) int {
	t.Helper()
	return lineOf(t, doc, c.needle)
}

var damageCases = []damageCase{
	{
		name: "ExecutionLog row with fewer cells than the header",
		mutate: func(d *strictDoc) {
			d.logRows = append(d.logRows, "| 3 | worker#3 | EXECUTION | - | 1 | SUCCESS | short-row-marker |")
		},
		section: "ExecutionLog", needle: "short-row-marker",
	},
	{
		name:    "Artifacts row with fewer cells than the header",
		mutate:  func(d *strictDoc) { d.registry = append(d.registry, "| Out.md | short-row-marker |") },
		section: "Artifacts", needle: "short-row-marker",
	},
	{
		name:    "WorkflowNotes row with fewer cells than the header",
		mutate:  func(d *strictDoc) { d.notes = append(d.notes, "| short-row-marker |") },
		section: "WorkflowNotes", needle: "short-row-marker",
	},
	{
		name:    "ExecutionLog line without a leading pipe before the closing tag",
		mutate:  func(d *strictDoc) { d.logTail = []string{"stray prose line"} },
		section: "ExecutionLog", needle: "stray prose line",
	},
	{
		name:    "ExecutionLog indented line before the closing tag",
		mutate:  func(d *strictDoc) { d.logTail = []string{" indented stray line"} },
		section: "ExecutionLog", needle: "indented stray line",
	},
	{
		name:    "ExecutionLog stray line after a blank line",
		mutate:  func(d *strictDoc) { d.logTail = []string{"", "", "stray after blank"} },
		section: "ExecutionLog", needle: "stray after blank",
	},
	{
		name:    "Artifacts line without a leading pipe before the closing tag",
		mutate:  func(d *strictDoc) { d.registryTail = []string{"stray prose line"} },
		section: "Artifacts", needle: "stray prose line",
	},
	{
		name:    "WorkflowNotes line without a leading pipe before the closing tag",
		mutate:  func(d *strictDoc) { d.notesTail = []string{"stray prose line"} },
		section: "WorkflowNotes", needle: "stray prose line",
	},
	{
		name:    "repeated Seq",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("2", "worker#2", "repeated-seq-marker")) },
		section: "ExecutionLog", needle: "repeated-seq-marker",
	},
	{
		name: "lower Seq after a higher one",
		mutate: func(d *strictDoc) {
			d.logRows = append(d.logRows, rawLogRow("5", "worker#5", "five"), rawLogRow("3", "worker#3", "lower-seq-marker"))
		},
		section: "ExecutionLog", needle: "lower-seq-marker",
	},
	{
		name:    "non-integer Seq",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("x3", "worker#3", "bad-seq-marker")) },
		section: "ExecutionLog", needle: "bad-seq-marker",
	},
	{
		name:    "decimal Seq",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("3.5", "worker#3", "bad-seq-marker")) },
		section: "ExecutionLog", needle: "bad-seq-marker",
	},
	{
		name: "free-text Agent",
		mutate: func(d *strictDoc) {
			d.logRows = append(d.logRows, rawLogRow("3", "the orchestrator note about this row", "free-text-marker"))
		},
		section: "ExecutionLog", needle: "free-text-marker",
	},
	{
		name: "free text after a valid instance id in the Agent cell",
		mutate: func(d *strictDoc) {
			d.logRows = append(d.logRows, rawLogRow("3", "worker#3 SUCCESS ...", "trailing-text-marker"))
		},
		section: "ExecutionLog", needle: "trailing-text-marker",
	},
	{
		name: "legacy Agent whose suffix differs from the row's Seq",
		mutate: func(d *strictDoc) {
			d.logRows = append(d.logRows, rawLogRow("3", "orchestrator#1", "legacy-suffix-marker"))
		},
		section: "ExecutionLog", needle: "legacy-suffix-marker",
	},
	{
		name:    "Agent name containing whitespace",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("3", "my worker#3", "space-name-marker")) },
		section: "ExecutionLog", needle: "space-name-marker",
	},
	{
		name:    "Agent without a name",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("3", "#3", "empty-name-marker")) },
		section: "ExecutionLog", needle: "empty-name-marker",
	},
	{
		name:    "Agent without a numeric suffix",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("3", "worker", "no-suffix-marker")) },
		section: "ExecutionLog", needle: "no-suffix-marker",
	},
	{
		name:    "last_agent naming an instance with no Execution Log row",
		mutate:  func(d *strictDoc) { d.lastAgent = `"worker#9"` },
		section: "frontmatter", needle: "last_agent:",
	},
	{
		name:    "last_agent naming a row that does not exist although the Seq is logged",
		mutate:  func(d *strictDoc) { d.lastAgent = `"other#2"` },
		section: "frontmatter", needle: "last_agent:",
	},
}

func TestParse_DamageCases_AreRefusedAtTheRightSectionAndLine(t *testing.T) {
	for _, c := range damageCases {
		t.Run(c.name, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)
			doc := d.String()

			damage := parseDamaged(t, doc)

			requireDamageProblem(t, damage, c.section, c.wantLine(t, doc))
		})
	}
}

func TestParseAt_DamageCases_CarryTheFilePath(t *testing.T) {
	for _, c := range damageCases {
		t.Run(c.name, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)
			doc := d.String()

			_, err := artifact.ParseAt(refusalPath, []byte(doc))

			damage := asDamageError(t, err)
			if damage.Resource != refusalPath {
				t.Errorf("Resource = %q, want %q", damage.Resource, refusalPath)
			}
			requireDamageProblem(t, damage, c.section, c.wantLine(t, doc))
		})
	}
}

// A damage line is a file line: CRLF line endings do not move it.
func TestParse_DamageLine_IsTheSameForCRLF(t *testing.T) {
	for _, c := range damageCases {
		t.Run(c.name, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)
			doc := d.String()

			_, err := artifact.Parse([]byte(crlfEnding(t).conv(doc)))

			requireDamageProblem(t, asDamageError(t, err), c.section, c.wantLine(t, doc))
		})
	}
}

// The table ends at the first non-table line: the lines after it are not parsed
// as rows, so a well-formed row below the stray line adds no problem of its own.
func TestParse_RowsAfterAStrayLine_AreNotParsedAsRows(t *testing.T) {
	d := newStrictDoc()
	d.logTail = []string{"stray prose line", logRow(3)}
	doc := d.String()

	damage := parseDamaged(t, doc)

	want := lineOf(t, doc, "stray prose line")
	if got := problemLines(damage, "ExecutionLog"); len(got) != 1 || got[0] != want {
		t.Errorf("ExecutionLog problem lines = %v, want only the stray line %d", got, want)
	}
}

// Whether a derived last_agent problem is also reported for a damaged log is
// unspecified; only the log damage itself is required.
func TestParse_LastAgentWithoutRow_StillReportsTheLogDamage(t *testing.T) {
	d := newStrictDoc()
	d.lastAgent = `"worker#9"`
	d.logTail = []string{"stray prose line"}

	damage := parseDamaged(t, d.String())

	if got := problemLines(damage, "ExecutionLog"); len(got) == 0 {
		t.Error("the log damage was not reported")
	}
}

func TestParse_BrokenFrontmatter_IsDamageInTheFrontmatterSection(t *testing.T) {
	cases := map[string]func(doc string) string{
		"unparseable YAML": func(doc string) string {
			return strings.Replace(doc, "workflow: test\n", "workflow: [unclosed\n", 1)
		},
		"type missing": func(doc string) string {
			return strings.Replace(doc, "type: orchestration-artifact\n", "", 1)
		},
		"wrong type": func(doc string) string {
			return strings.Replace(doc, "type: orchestration-artifact\n", "type: something-else\n", 1)
		},
		"invalid checkpoints value": func(doc string) string {
			return strings.Replace(doc, "checkpoints: disabled\n", "checkpoints: maybe\n", 1)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			doc := mutate(newStrictDoc().String())

			damage := parseDamaged(t, doc)

			if len(problemLines(damage, "frontmatter")) == 0 {
				t.Errorf("no problem in section frontmatter: %+v", damage.Problems)
			}
		})
	}
}

func TestParse_MissingClosingTag_IsDamageInThatSection(t *testing.T) {
	for _, section := range tolerantSections {
		t.Run(section, func(t *testing.T) {
			doc := strings.Replace(newStrictDoc().String(), "</"+section+">", "", 1)

			damage := parseDamaged(t, doc)

			if len(problemLines(damage, section)) == 0 {
				t.Errorf("no problem in section %s: %+v", section, damage.Problems)
			}
		})
	}
}

func TestParse_SeveralProblemsInOneFile_AreAllReportedInFileOrder(t *testing.T) {
	d := newStrictDoc()
	d.logRows = append(d.logRows,
		"| 3 | worker#3 | EXECUTION | - | 1 | SUCCESS | short-row-marker |",
		rawLogRow("3", "worker#3", "repeated-seq-marker"),
		rawLogRow("4", "free text agent", "free-text-marker"),
	)
	d.registryTail = []string{"stray registry line"}
	d.notesTail = []string{"stray notes line"}
	doc := d.String()

	damage := parseDamaged(t, doc)

	requireDamageProblem(t, damage, "ExecutionLog", lineOf(t, doc, "short-row-marker"))
	requireDamageProblem(t, damage, "ExecutionLog", lineOf(t, doc, "repeated-seq-marker"))
	requireDamageProblem(t, damage, "ExecutionLog", lineOf(t, doc, "free-text-marker"))
	requireDamageProblem(t, damage, "Artifacts", lineOf(t, doc, "stray registry line"))
	requireDamageProblem(t, damage, "WorkflowNotes", lineOf(t, doc, "stray notes line"))
	last := 0
	for _, p := range damage.Problems {
		if p.Line < last {
			t.Errorf("problems are not in file order: line %d after line %d: %+v", p.Line, last, damage.Problems)
		}
		last = p.Line
	}
}

func TestParse_SeveralProblemsAcrossFrontmatterAndTables_AreAllReported(t *testing.T) {
	d := newStrictDoc()
	d.logTail = []string{"stray prose line"}
	d.notesTail = []string{"stray notes line"}
	doc := strings.Replace(d.String(), "checkpoints: disabled\n", "checkpoints: maybe\n", 1)

	damage := parseDamaged(t, doc)

	if len(problemLines(damage, "frontmatter")) == 0 {
		t.Errorf("the frontmatter problem was not reported: %+v", damage.Problems)
	}
	requireDamageProblem(t, damage, "ExecutionLog", lineOf(t, doc, "stray prose line"))
	requireDamageProblem(t, damage, "WorkflowNotes", lineOf(t, doc, "stray notes line"))
}

// Fixture: an orchestrator edit split Execution Log row 77 into three lines, with
// rows 78 to 83 after them. The old reader ended the table at the non-pipe line,
// dropped rows 78 to 83 and took the note text for an Agent.
func TestParseAt_IncidentFixture_IsDamageInExecutionLogAtTheSplitRow(t *testing.T) {
	path := fixturePath("damaged_incident_split_row.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	doc := string(data)
	shortRow := lineOf(t, doc, "| 77 | test-writer-tdd#77 |")
	strayRow := lineOf(t, doc, "test-writer-tdd#83 SUCCESS")
	noteLine := lineOf(t, doc, "continuation of the split row")

	state, perr := artifact.ParseAt(path, data)

	damage := asDamageError(t, perr)
	if damage.Resource != path {
		t.Errorf("Resource = %q, want %q", damage.Resource, path)
	}
	requireDamageProblem(t, damage, "ExecutionLog", shortRow)
	requireDamageProblem(t, damage, "ExecutionLog", strayRow)
	requireDamageProblem(t, damage, "ExecutionLog", noteLine)
	for _, line := range problemLines(damage, "ExecutionLog") {
		if line > noteLine {
			t.Errorf("a problem is reported at line %d, after the line that ended the table (%d): rows 78-83 must not be parsed", line, noteLine)
		}
	}
	if len(state.ExecutionLog) != 0 {
		t.Errorf("state came back with %d log rows, want none (no partial state)", len(state.ExecutionLog))
	}
}

func TestParseAt_IncidentFixture_FirstProblemIsAtTheSplitRow(t *testing.T) {
	path := fixturePath("damaged_incident_split_row.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	_, perr := artifact.ParseAt(path, data)

	damage := asDamageError(t, perr)
	if first, want := damage.Problems[0], lineOf(t, string(data), "| 77 | test-writer-tdd#77 |"); first.Section != "ExecutionLog" || first.Line != want {
		t.Errorf("first problem = %+v, want ExecutionLog at line %d", first, want)
	}
}
