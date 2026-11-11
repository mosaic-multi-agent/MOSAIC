package artifact_test

// Tests for the recorded frontmatter values of a damaged run: the artifact's
// own values when its frontmatter is intact, otherwise those of the run's
// last-good copy when that copy is usable, otherwise none. The run is laid out
// as the Runner keeps it: {ws}/Orchestration-{id}/Orchestration.md and the copy
// in {ws}/RunnerLogs/{id}/.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const otherRunID = "20260101T000000Z-abcd"

// damagedRun is a run folder with its evidence directory.
type damagedRun struct {
	artifactPath string
	lastGoodPath string
}

func newDamagedRun(t *testing.T) damagedRun {
	t.Helper()
	ws := t.TempDir()
	path := filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")
	dir, ok := artifact.EvidenceDir(path)
	if !ok {
		t.Fatal("test setup: no evidence location for a run-scoped path")
	}
	return damagedRun{artifactPath: path, lastGoodPath: filepath.Join(dir, "Orchestration.last-good.md")}
}

// frontmatterDamaged breaks the frontmatter of a document.
func frontmatterDamaged(doc string) string {
	return strings.Replace(doc, "type: orchestration-artifact\n", "type: [unclosed\n", 1)
}

// bodyDamaged leaves a stray line between the Execution Log table and its tag.
func bodyDamaged() string {
	d := newStrictDoc()
	d.logTail = []string{"stray prose line"}
	return d.String()
}

func withWorkflow(doc, workflow string) string {
	return strings.Replace(doc, "workflow: test\n", "workflow: "+workflow+"\n", 1)
}

// readDamage writes the artifact, reads it and returns the damage error.
func (r damagedRun) readDamage(t *testing.T, doc string) error {
	t.Helper()
	writeFileT(t, r.artifactPath, doc)
	_, err := artifact.ParseAt(r.artifactPath, []byte(doc))
	var damage *domain.ArtifactDamageError
	if !errors.As(err, &damage) {
		t.Fatalf("test setup: want a damage error reading the artifact, got %T (%v)", err, err)
	}
	return err
}

func (r damagedRun) writeLastGood(t *testing.T, doc string) {
	t.Helper()
	writeFileT(t, r.lastGoodPath, doc)
}

func TestDamagedRunFrontmatter_DamageOutsideTheFrontmatter_ReturnsTheArtifactsOwnValues(t *testing.T) {
	r := newDamagedRun(t)
	doc := withWorkflow(bodyDamaged(), "own-flow")
	readErr := r.readDamage(t, doc)
	// A last-good copy recording something else must not be preferred.
	r.writeLastGood(t, withWorkflow(newStrictDoc().String(), "copy-flow"))

	state, ok := artifact.DamagedRunFrontmatter(r.artifactPath, readErr)

	if !ok {
		t.Fatal("want the recorded values of a damage outside the frontmatter, got none")
	}
	if state.Workflow != "own-flow" || state.RunID != testRunID {
		t.Errorf("want the artifact's own workflow and run_id, got %q and %q", state.Workflow, state.RunID)
	}
}

func TestDamagedRunFrontmatter_DamageOutsideTheFrontmatterWithoutACopy_ReturnsTheOwnValues(t *testing.T) {
	r := newDamagedRun(t)
	readErr := r.readDamage(t, withWorkflow(bodyDamaged(), "own-flow"))

	state, ok := artifact.DamagedRunFrontmatter(r.artifactPath, readErr)

	if !ok || state.Workflow != "own-flow" {
		t.Errorf("want the own values without any copy, got %q (found %v)", state.Workflow, ok)
	}
}

func TestDamagedRunFrontmatter_WrappedDamageError_IsFound(t *testing.T) {
	r := newDamagedRun(t)
	readErr := r.readDamage(t, withWorkflow(bodyDamaged(), "own-flow"))

	state, ok := artifact.DamagedRunFrontmatter(r.artifactPath, fmt.Errorf("reading the run: %w", readErr))

	if !ok || state.Workflow != "own-flow" {
		t.Errorf("want the values through a wrapped damage error, got %q (found %v)", state.Workflow, ok)
	}
}

func TestDamagedRunFrontmatter_DamagedFrontmatterWithAUsableCopy_ReturnsTheCopysValues(t *testing.T) {
	tests := []struct {
		name string
		copy func(t *testing.T) string
	}{
		{"intact copy", func(t *testing.T) string { return withWorkflow(newStrictDoc().String(), "copy-flow") }},
		{"copy damaged outside its frontmatter", func(t *testing.T) string {
			return withWorkflow(bodyDamaged(), "copy-flow")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newDamagedRun(t)
			readErr := r.readDamage(t, frontmatterDamaged(withWorkflow(newStrictDoc().String(), "own-flow")))
			r.writeLastGood(t, tc.copy(t))

			state, ok := artifact.DamagedRunFrontmatter(r.artifactPath, readErr)

			if !ok {
				t.Fatal("want the last-good copy's values, got none")
			}
			if state.Workflow != "copy-flow" || state.RunID != testRunID {
				t.Errorf("want the copy's workflow and run_id, got %q and %q", state.Workflow, state.RunID)
			}
		})
	}
}

func TestDamagedRunFrontmatter_ReadsOnly_NeitherFileIsWritten(t *testing.T) {
	r := newDamagedRun(t)
	damaged := frontmatterDamaged(newStrictDoc().String())
	readErr := r.readDamage(t, damaged)
	good := withWorkflow(newStrictDoc().String(), "copy-flow")
	r.writeLastGood(t, good)

	if _, ok := artifact.DamagedRunFrontmatter(r.artifactPath, readErr); !ok {
		t.Fatal("test setup: want the copy's values found")
	}

	for path, want := range map[string]string{r.artifactPath: damaged, r.lastGoodPath: good} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("want %s left as it was (err %v)", path, err)
		}
	}
}

func TestDamagedRunFrontmatter_DamagedFrontmatterWithoutAUsableCopy_ReturnsNone(t *testing.T) {
	tests := []struct {
		name string
		copy func(t *testing.T) (doc string, present bool)
	}{
		{"no copy", func(t *testing.T) (string, bool) { return "", false }},
		{"empty copy", func(t *testing.T) (string, bool) { return "", true }},
		{"copy not an artifact", func(t *testing.T) (string, bool) { return "not an artifact\n", true }},
		{"copy with damaged frontmatter", func(t *testing.T) (string, bool) {
			return frontmatterDamaged(newStrictDoc().String()), true
		}},
		{"copy of another run", func(t *testing.T) (string, bool) {
			return strings.Replace(newStrictDoc().String(), "run_id: "+testRunID, "run_id: "+otherRunID, 1), true
		}},
		{"copy with a malformed run_id", func(t *testing.T) (string, bool) {
			return strings.Replace(newStrictDoc().String(), "run_id: "+testRunID, "run_id: not-a-run-id", 1), true
		}},
		{"copy in the pre-3.0 layout", func(t *testing.T) (string, bool) {
			doc := strings.Replace(newStrictDoc().String(), "| Iteration |", "| Stage |", 1)
			return strings.Replace(doc, "  iteration: null", "  stage: null", 1), true
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newDamagedRun(t)
			readErr := r.readDamage(t, frontmatterDamaged(newStrictDoc().String()))
			if doc, present := tc.copy(t); present {
				r.writeLastGood(t, doc)
			}

			state, ok := artifact.DamagedRunFrontmatter(r.artifactPath, readErr)

			if ok || state.RunID != "" || state.Workflow != "" {
				t.Errorf("want none, got %+v (found %v)", state, ok)
			}
		})
	}
}

func TestDamagedRunFrontmatter_PathWithoutAnEvidenceLocation_ReturnsNoneForDamagedFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	doc := frontmatterDamaged(newStrictDoc().String())
	writeFileT(t, path, doc)
	_, readErr := artifact.ParseAt(path, []byte(doc))

	state, ok := artifact.DamagedRunFrontmatter(path, readErr)

	if ok {
		t.Errorf("want none without an evidence location, got %+v", state)
	}
}

func TestDamagedRunFrontmatter_ReadErrorThatIsNotDamage_ReturnsNone(t *testing.T) {
	r := newDamagedRun(t)
	r.readDamage(t, frontmatterDamaged(newStrictDoc().String()))
	r.writeLastGood(t, newStrictDoc().String())

	for name, readErr := range map[string]error{
		"nil":            nil,
		"not exist":      os.ErrNotExist,
		"other":          errors.New("disk failure"),
		"domain refusal": &domain.RefusalError{Reason: "pre-3.0 layout"},
	} {
		t.Run(name, func(t *testing.T) {
			state, ok := artifact.DamagedRunFrontmatter(r.artifactPath, readErr)

			if ok {
				t.Errorf("want none for a read error that is not damage, got %+v", state)
			}
		})
	}
}
