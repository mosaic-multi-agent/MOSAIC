package artifact_test

// Regression tests for what the strict reader keeps accepting or keeps handling
// in its own way: layouts and values that are not damage, the pre-3.0 layout,
// and the run-identity refusals. These pass against the lenient reader too; they
// pin that tightening it did not take these away.

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

func mustParseStrict(t *testing.T, doc string) domain.ArtifactState {
	t.Helper()
	state, err := artifact.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, doc)
	}
	return state
}

func TestParse_CompliantDocument_IsReadInFull(t *testing.T) {
	state := mustParseStrict(t, newStrictDoc().String())

	if len(state.ExecutionLog) != 2 || len(state.ArtifactRegistry) != 1 || len(state.WorkflowNotes) != 1 {
		t.Fatalf("counts: log=%d registry=%d notes=%d, want 2/1/1",
			len(state.ExecutionLog), len(state.ArtifactRegistry), len(state.WorkflowNotes))
	}
	if state.CurrentState.LastAgent != "worker#2" {
		t.Errorf("LastAgent = %q, want worker#2", state.CurrentState.LastAgent)
	}
}

func TestParse_ClosingTagAfterContent_IsNotDamage(t *testing.T) {
	for _, section := range tolerantSections {
		t.Run(section, func(t *testing.T) {
			doc := joinCloseTag(t, newStrictDoc().String(), section)

			state := mustParseStrict(t, doc)

			if len(state.ExecutionLog) != 2 || len(state.ArtifactRegistry) != 1 || len(state.WorkflowNotes) != 1 {
				t.Errorf("rows were lost: log=%d registry=%d notes=%d",
					len(state.ExecutionLog), len(state.ArtifactRegistry), len(state.WorkflowNotes))
			}
		})
	}
}

func TestParse_CRLFLineEndings_AreReadLikeLF(t *testing.T) {
	doc := newStrictDoc().String()

	lf := mustParseStrict(t, doc)
	crlf := mustParseStrict(t, lineEndings[1].conv(doc))

	if !reflect.DeepEqual(lf, crlf) {
		t.Errorf("CRLF state differs from LF state\n lf: %+v\ncrlf: %+v", lf, crlf)
	}
}

func TestParse_GlobalSequenceBehindTheLastLoggedSeq_IsNotDamageAndIsCorrectedUp(t *testing.T) {
	d := newStrictDoc()
	d.globalSeq = 0

	state := mustParseStrict(t, d.String())

	if state.GlobalSequence != 2 {
		t.Errorf("GlobalSequence = %d, want 2 (the highest logged Seq)", state.GlobalSequence)
	}
}

func TestParse_GapsInSeq_AreNotDamage(t *testing.T) {
	d := newStrictDoc()
	d.logRows = []string{logRow(1), logRow(3), logRow(7)}
	d.lastAgent = `"worker#7"`
	d.globalSeq = 9

	state := mustParseStrict(t, d.String())

	if len(state.ExecutionLog) != 3 || state.ExecutionLog[2].Seq != 7 {
		t.Errorf("log = %+v, want rows 1, 3 and 7", state.ExecutionLog)
	}
}

func TestParse_EmptyLastAgent_IsNotDamage(t *testing.T) {
	cases := map[string]func(d *strictDoc){
		"null before any invocation":  func(d *strictDoc) { d.lastAgent = "null"; d.logRows = nil; d.globalSeq = 0 },
		"empty string before any run": func(d *strictDoc) { d.lastAgent = `""`; d.logRows = nil; d.globalSeq = 0 },
		// A step that was not accepted leaves a row but does not move current_state.
		"null beside logged rows": func(d *strictDoc) { d.lastAgent = "null" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := newStrictDoc()
			mutate(&d)

			state := mustParseStrict(t, d.String())

			if state.CurrentState.LastAgent != "" {
				t.Errorf("LastAgent = %q, want empty", state.CurrentState.LastAgent)
			}
		})
	}
}

func TestParse_LastAgentOfAnEarlierRow_IsNotDamage(t *testing.T) {
	d := newStrictDoc()
	d.lastAgent = `"worker#1"`

	state := mustParseStrict(t, d.String())

	if state.CurrentState.LastAgent != "worker#1" {
		t.Errorf("LastAgent = %q, want worker#1", state.CurrentState.LastAgent)
	}
}

func TestParse_BlankLinesAroundTables_AreNotDamage(t *testing.T) {
	d := newStrictDoc()
	d.logTail = []string{"", "  ", "\t"}
	d.registryTail = []string{"", " "}
	d.notesTail = []string{"", ""}

	state := mustParseStrict(t, d.String())

	if len(state.ExecutionLog) != 2 {
		t.Errorf("log rows = %d, want 2", len(state.ExecutionLog))
	}
}

func TestParse_Pre30Layout_KeepsItsOwnRefusalAndIsNotDamage(t *testing.T) {
	_, err := artifact.Parse(v3Document("Stage", "Test.1", "stage", "Test.1"))

	requirePre30Layout(t, err)
	var d *domain.ArtifactDamageError
	if errors.As(err, &d) {
		t.Errorf("the pre-3.0 layout was reported as damage: %v", err)
	}
}

// A kept refusal is returned alone even when the file is also damaged: no repair
// can fix the layout.
func TestParse_Pre30LayoutWithOtherDamage_StillYieldsErrPre30Layout(t *testing.T) {
	doc := strings.Replace(string(v3Document("Stage", "Test.1", "stage", "Test.1")),
		"</ExecutionLog>", "stray prose line\n</ExecutionLog>", 1)

	_, err := artifact.Parse([]byte(doc))

	requirePre30Layout(t, err)
	var d *domain.ArtifactDamageError
	if errors.As(err, &d) {
		t.Errorf("the pre-3.0 layout came back as damage: %v", err)
	}
}

func TestParse_RunIdentityRefusals_AreUnchangedAndNotDamage(t *testing.T) {
	cases := map[string]string{
		"absent":    "",
		"empty":     "run_id: \"\"\n",
		"malformed": "run_id: not-a-run-id\n",
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			doc := strings.Replace(newStrictDoc().String(), "run_id: "+testRunID+"\n", line, 1)

			_, err := artifact.Parse([]byte(doc))

			re := asRefusalError(t, err)
			var idErr *domain.RunIdentityError
			if !errors.As(re.Cause, &idErr) {
				t.Errorf("Cause = %T, want *domain.RunIdentityError", re.Cause)
			}
			var d *domain.ArtifactDamageError
			if errors.As(err, &d) {
				t.Errorf("a run-identity refusal was reported as damage: %v", err)
			}
		})
	}
}

// The identity refusal is returned alone even when the file is also damaged.
func TestParse_RunIdentityRefusalWithOtherDamage_StillYieldsTheRefusal(t *testing.T) {
	d := newStrictDoc()
	d.logTail = []string{"stray prose line"}
	doc := strings.Replace(d.String(), "run_id: "+testRunID+"\n", "", 1)

	_, err := artifact.Parse([]byte(doc))

	re := asRefusalError(t, err)
	var idErr *domain.RunIdentityError
	if !errors.As(re.Cause, &idErr) {
		t.Errorf("Cause = %T, want *domain.RunIdentityError", re.Cause)
	}
	var dmg *domain.ArtifactDamageError
	if errors.As(err, &dmg) {
		t.Errorf("the identity refusal came back as damage: %v", err)
	}
}

// An Execution Log written without an Inputs column is an older layout that is
// accepted as before: no new header strictness.
func TestParse_ExecutionLogWithoutInputsColumn_IsStillAccepted(t *testing.T) {
	state, err := artifact.Parse(readFixtureBytes(t, "no-inputs-column.md"))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(state.ExecutionLog) != 1 || state.ExecutionLog[0].Inputs != "" {
		t.Errorf("log = %+v, want one row with empty Inputs", state.ExecutionLog)
	}
}

func TestParse_DamageIsNeverAPositionUnresolvedError(t *testing.T) {
	for _, c := range damageCases {
		t.Run(c.name, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)

			_, err := artifact.Parse([]byte(d.String()))

			var pu *domain.PositionUnresolvedError
			if errors.As(err, &pu) {
				t.Errorf("damage reported as a position-unresolved error: %v", err)
			}
			if err == nil {
				t.Fatal("Parse succeeded, want a damage error")
			}
			asDamageError(t, err)
		})
	}
}
