package artifact_test

// Tests that the damage error reaches callers intact through the store: the same
// kind, the file path, no partial state, and a message that says the artifact is
// damaged and where.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
)

func writeDamagedFile(t *testing.T, doc string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestStoreRead_DamagedArtifact_ReturnsDamageWithPathAndNoPartialState(t *testing.T) {
	for _, c := range damageCases {
		t.Run(c.name, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)
			doc := d.String()
			path := writeDamagedFile(t, doc)

			state, err := artifact.NewFileStore(path).Read(context.Background())

			damage := asDamageError(t, err)
			if damage.Resource != path {
				t.Errorf("Resource = %q, want %q", damage.Resource, path)
			}
			requireDamageProblem(t, damage, c.section, c.wantLine(t, doc))
			if len(state.ExecutionLog) != 0 || state.RunID != "" {
				t.Errorf("a damage error came with partial state: %+v", state)
			}
		})
	}
}

func TestStoreRead_IncidentFixture_IsDamageInExecutionLog(t *testing.T) {
	data := readFixtureBytes(t, "damaged_incident_split_row.md")
	path := writeDamagedFile(t, string(data))

	state, err := artifact.NewFileStore(path).Read(context.Background())

	damage := asDamageError(t, err)
	requireDamageProblem(t, damage, "ExecutionLog", lineOf(t, string(data), "test-writer-tdd#83 SUCCESS"))
	if len(state.ExecutionLog) != 0 {
		t.Errorf("state came back with %d log rows, want none", len(state.ExecutionLog))
	}
}

func TestDamageError_Message_SaysDamagedAndNamesFileSectionAndLine(t *testing.T) {
	d := newStrictDoc()
	d.logTail = []string{"stray prose line"}
	doc := d.String()
	path := writeDamagedFile(t, doc)

	_, err := artifact.NewFileStore(path).Read(context.Background())

	asDamageError(t, err)
	msg := err.Error()
	for name, want := range map[string]string{
		"a statement that the artifact is damaged": "damaged",
		"the file path": path,
		"the section":   "ExecutionLog",
		"the line":      strconv.Itoa(lineOf(t, doc, "stray prose line")),
	} {
		if !strings.Contains(strings.ToLower(msg), strings.ToLower(want)) {
			t.Errorf("message %q lacks %s (%q)", msg, name, want)
		}
	}
}

func TestDamageError_MessageFromParse_NamesEveryProblemLine(t *testing.T) {
	d := newStrictDoc()
	d.logRows = append(d.logRows, rawLogRow("2", "worker#2", "repeated-seq-marker"))
	d.notesTail = []string{"stray notes line"}
	doc := d.String()

	_, err := artifact.Parse([]byte(doc))

	asDamageError(t, err)
	for _, needle := range []string{"repeated-seq-marker", "stray notes line"} {
		line := strconv.Itoa(lineOf(t, doc, needle))
		if !strings.Contains(err.Error(), line) {
			t.Errorf("message %q does not name line %s (%s)", err.Error(), line, needle)
		}
	}
}

func TestStoreRead_AfterTheDamageIsFixed_ReadsSuccessfully(t *testing.T) {
	d := newStrictDoc()
	d.logTail = []string{"stray prose line"}
	path := writeDamagedFile(t, d.String())
	store := artifact.NewFileStore(path)
	if _, err := store.Read(context.Background()); err == nil {
		t.Fatal("Read succeeded on a damaged artifact")
	}
	if err := os.WriteFile(path, []byte(newStrictDoc().String()), 0o644); err != nil {
		t.Fatalf("repair: %v", err)
	}

	state, err := store.Read(context.Background())

	if err != nil {
		t.Fatalf("Read after repair: %v", err)
	}
	if len(state.ExecutionLog) != 2 {
		t.Errorf("log rows = %d, want 2", len(state.ExecutionLog))
	}
}

func TestStoreRead_Pre30Layout_KeepsErrPre30Layout(t *testing.T) {
	path := writeDamagedFile(t, string(v3Document("Stage", "Test.1", "stage", "Test.1")))

	_, err := artifact.NewFileStore(path).Read(context.Background())

	requirePre30Layout(t, err)
}
