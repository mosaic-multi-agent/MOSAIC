package artifact_test

// Builders for the strict-reading tests. A strictDoc is a compliant artifact
// whose pieces a test damages one at a time; String renders it.

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const strictLogHeader = "| Seq | Agent | Phase | Iteration | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint |\n" +
	"| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"

// logRow renders a compliant Execution Log row: the Agent is worker#seq.
func logRow(seq int) string {
	n := strconv.Itoa(seq)
	return rawLogRow(n, "worker#"+n, "row "+n)
}

// rawLogRow renders a ten-cell Execution Log row from raw Seq and Agent cells.
// The summary is a unique marker tests can use to find the row's line.
func rawLogRow(seqCell, agentCell, summary string) string {
	return "| " + seqCell + " | " + agentCell + " | EXECUTION | - | 1 | SUCCESS | 2026-01-01T01:00:00Z | " + summary + " | - | - |"
}

type strictDoc struct {
	lastAgent    string // raw current_state.last_agent value, e.g. `"worker#2"` or `null`
	globalSeq    int
	logRows      []string // lines after the header
	logTail      []string // lines between the last log row and the closing tag
	registry     []string
	registryTail []string
	notes        []string
	notesTail    []string
}

// newStrictDoc is a compliant artifact: worker#1 and worker#2 logged, the last
// of them the last_agent.
func newStrictDoc() strictDoc {
	return strictDoc{
		lastAgent: `"worker#2"`,
		globalSeq: 2,
		logRows:   []string{logRow(1), logRow(2)},
		registry:  []string{"| Plan.md | PLANNING | worker#1 |"},
		notes:     []string{"| 1 | a note |"},
	}
}

func (d strictDoc) String() string {
	var b strings.Builder
	b.WriteString("---\n" +
		"type: orchestration-artifact\n" +
		"run_id: " + testRunID + "\n" +
		"workflow: test\n" +
		"workflow_version: \"1.0\"\n" +
		"task: \"test\"\n" +
		"started: 2026-01-01T00:00:00Z\n" +
		"last_updated: 2026-01-01T01:00:00Z\n" +
		"global_sequence: " + strconv.Itoa(d.globalSeq) + "\n" +
		"checkpoints: disabled\n" +
		"current_state:\n" +
		"  phase: EXECUTION\n" +
		"  iteration: null\n" +
		"  last_status: SUCCESS\n" +
		"  last_agent: " + d.lastAgent + "\n" +
		"  error_code: null\n" +
		"---\n\n")
	section := func(name, header string, rows, tail []string) {
		b.WriteString("<" + name + " type=\"core\">\n\n" + header + "\n")
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
		for _, l := range tail {
			b.WriteString(l + "\n")
		}
		b.WriteString("\n</" + name + ">\n\n")
	}
	section("ExecutionLog", strictLogHeader, d.logRows, d.logTail)
	section("Artifacts", "| Artifact | Created In | Created By |\n| --- | --- | --- |", d.registry, d.registryTail)
	section("WorkflowNotes", "| Seq | Note |\n| --- | --- |", d.notes, d.notesTail)
	return b.String()
}

// crlfEnding looks the CRLF line-ending style up by name.
func crlfEnding(t *testing.T) lineEnding {
	t.Helper()
	for _, e := range lineEndings {
		if e.name == "CRLF" {
			return e
		}
	}
	t.Fatal("test setup: no CRLF line ending")
	return lineEnding{}
}

// readFixtureBytes returns the bytes of a fixture under testdata/artifact.
func readFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return data
}

// parseDamaged parses the document and asserts the result is a damage error and
// that no partial state came back with it.
func parseDamaged(t *testing.T, doc string) *domain.ArtifactDamageError {
	t.Helper()
	state, err := artifact.Parse([]byte(doc))
	if err == nil {
		t.Fatalf("Parse succeeded with %d log rows, want a damage error\n%s", len(state.ExecutionLog), doc)
	}
	d := asDamageError(t, err)
	if len(state.ExecutionLog) != 0 || len(state.ArtifactRegistry) != 0 || len(state.WorkflowNotes) != 0 || state.RunID != "" {
		t.Errorf("a damage error came with partial state: %+v", state)
	}
	return d
}
