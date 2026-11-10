package artifact_test

// Tests for the dispatch frontmatter field: parsing (presence modelled apart
// from value), rendering (position and quoting), state-level round trips, and
// preservation through every Runner rewrite of the artifact.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// dispatchConfig is the frontmatter block placed after global_sequence: an
// optional dispatch line followed by baseline config with commits enabled (so
// SetCommitBranch accepts the artifact).
func dispatchConfig(dispatchLine string) string {
	if dispatchLine != "" {
		dispatchLine += "\n"
	}
	return dispatchLine + "checkpoints: disabled\ncommits: enabled\n"
}

func parseDispatch(t *testing.T, dispatchLine string) domain.ArtifactState {
	t.Helper()
	state, err := artifact.Parse(minimalArtifactWithConfigBytes(dispatchConfig(dispatchLine)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return state
}

func renderText(t *testing.T, state domain.ArtifactState) string {
	t.Helper()
	data, err := artifact.Render(state)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(data)
}

// frontmatterLines returns the lines of the document's frontmatter block.
func frontmatterLines(text string) []string {
	lines := strings.Split(text, "\n")
	var out []string
	for i, l := range lines {
		if i == 0 {
			continue
		}
		if l == "---" {
			break
		}
		out = append(out, l)
	}
	return out
}

// lineAfter returns the frontmatter line directly after the line starting with
// prefix, or "" when there is none.
func lineAfter(text, prefix string) string {
	lines := frontmatterLines(text)
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	return ""
}

func dispatchLineCount(text string) int {
	n := 0
	for _, l := range frontmatterLines(text) {
		if strings.HasPrefix(l, "dispatch:") {
			n++
		}
	}
	return n
}

var dispatchParseCases = []struct {
	name string
	line string
	want domain.DispatchField
}{
	{"sequential", "dispatch: sequential", domain.DispatchField{Present: true, Value: "sequential"}},
	{"parallel", "dispatch: parallel", domain.DispatchField{Present: true, Value: "parallel"}},
	{"invalid value", "dispatch: concurrent", domain.DispatchField{Present: true, Value: "concurrent"}},
	{"blank value", "dispatch:", domain.DispatchField{Present: true, Value: ""}},
	{"empty quoted", `dispatch: ""`, domain.DispatchField{Present: true, Value: ""}},
	{"null", "dispatch: null", domain.DispatchField{Present: true, Value: ""}},
	{"quoted null", `dispatch: "null"`, domain.DispatchField{Present: true, Value: "null"}},
	{"case variant", "dispatch: Sequential", domain.DispatchField{Present: true, Value: "Sequential"}},
	{"quoted sequential", `dispatch: "sequential"`, domain.DispatchField{Present: true, Value: "sequential"}},
	{"leading space kept inside quotes", `dispatch: " sequential"`, domain.DispatchField{Present: true, Value: " sequential"}},
	{"trailing space kept inside quotes", `dispatch: "sequential "`, domain.DispatchField{Present: true, Value: "sequential "}},
	{"value wrapped in quotes", `dispatch: ""x""`, domain.DispatchField{Present: true, Value: `"x"`}},
	{"absent", "", domain.DispatchField{}},
}

func TestParse_Dispatch_ReadsPresenceAndValue(t *testing.T) {
	for _, tc := range dispatchParseCases {
		t.Run(tc.name, func(t *testing.T) {
			state := parseDispatch(t, tc.line)

			if state.Dispatch != tc.want {
				t.Errorf("Dispatch = %+v, want %+v", state.Dispatch, tc.want)
			}
		})
	}
}

func TestParse_Dispatch_IsNotCarriedAsUnknownFrontmatter(t *testing.T) {
	state := parseDispatch(t, "dispatch: parallel")

	if !state.Dispatch.Present {
		t.Fatalf("Dispatch not parsed as present: %+v", state.Dispatch)
	}
	for _, e := range state.UnknownFrontmatter {
		if e.Key == "dispatch" {
			t.Errorf("dispatch carried in UnknownFrontmatter: %+v", e)
		}
	}
}

func TestParse_Dispatch_BlankIsDistinguishableFromAbsent(t *testing.T) {
	blank := parseDispatch(t, "dispatch:")
	absent := parseDispatch(t, "")

	if blank.Dispatch == absent.Dispatch {
		t.Errorf("blank dispatch %+v is not distinguishable from absent %+v", blank.Dispatch, absent.Dispatch)
	}
	if !blank.Dispatch.Present {
		t.Errorf("blank dispatch: Present = false, want true")
	}
	if absent.Dispatch.Present {
		t.Errorf("absent dispatch: Present = true, want false")
	}
}

func TestRender_Dispatch_PlacedImmediatelyAfterGlobalSequence(t *testing.T) {
	state := parseDispatch(t, "dispatch: parallel")

	text := renderText(t, state)

	if got := lineAfter(text, "global_sequence:"); got != "dispatch: parallel" {
		t.Errorf("line after global_sequence = %q, want %q\n%s", got, "dispatch: parallel", text)
	}
	if got := lineAfter(text, "dispatch:"); !strings.HasPrefix(got, "checkpoints:") {
		t.Errorf("line after dispatch = %q, want the checkpoints line", got)
	}
	if n := dispatchLineCount(text); n != 1 {
		t.Errorf("dispatch lines = %d, want exactly 1", n)
	}
}

func TestRender_Dispatch_ValueFormatting(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"sequential unquoted", "sequential", "dispatch: sequential"},
		{"parallel unquoted", "parallel", "dispatch: parallel"},
		{"invalid value unquoted", "concurrent", "dispatch: concurrent"},
		{"case variant unquoted", "Sequential", "dispatch: Sequential"},
		{"empty written as empty quotes", "", `dispatch: ""`},
		{"leading whitespace quoted", " sequential", `dispatch: " sequential"`},
		{"trailing whitespace quoted", "sequential ", `dispatch: "sequential "`},
		{"null value quoted", "null", `dispatch: "null"`},
		{"value wrapped in quotes is wrapped again", `"x"`, `dispatch: ""x""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := parseDispatch(t, "")
			state.Dispatch = domain.DispatchField{Present: true, Value: tc.value}

			text := renderText(t, state)

			if got := lineAfter(text, "global_sequence:"); got != tc.want {
				t.Errorf("line after global_sequence = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRender_Dispatch_AbsentWritesNoLine(t *testing.T) {
	state := parseDispatch(t, "")

	text := renderText(t, state)

	if n := dispatchLineCount(text); n != 0 {
		t.Errorf("dispatch lines = %d, want 0 for an artifact without the field:\n%s", n, text)
	}
}

func TestRender_Dispatch_RoundTripKeepsPresenceAndValue(t *testing.T) {
	for _, tc := range dispatchParseCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parseDispatch(t, tc.line)
			if parsed.Dispatch != tc.want {
				t.Fatalf("Parse: Dispatch = %+v, want %+v", parsed.Dispatch, tc.want)
			}

			reparsed, err := artifact.Parse([]byte(renderText(t, parsed)))
			if err != nil {
				t.Fatalf("Parse(Render(state)): %v", err)
			}

			if reparsed.Dispatch != tc.want {
				t.Errorf("after round trip Dispatch = %+v, want %+v", reparsed.Dispatch, tc.want)
			}
		})
	}
}

func TestRender_Dispatch_HandBuiltStateRoundTrips(t *testing.T) {
	values := []string{"", "sequential", "parallel", "concurrent", " sequential", "sequential ", "null", `"x"`, `""`}
	for _, v := range values {
		t.Run("value="+v, func(t *testing.T) {
			state := parseDispatch(t, "")
			state.Dispatch = domain.DispatchField{Present: true, Value: v}

			reparsed, err := artifact.Parse([]byte(renderText(t, state)))
			if err != nil {
				t.Fatalf("Parse(Render(state)): %v", err)
			}

			if reparsed.Dispatch != state.Dispatch {
				t.Errorf("after round trip Dispatch = %+v, want %+v", reparsed.Dispatch, state.Dispatch)
			}
		})
	}
}

// ---- Store: Create and rewrite paths ----

func TestCreate_Dispatch_RecordsSequentialAfterGlobalSequence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	store := artifact.NewFileStore(path)
	ctx := context.Background()

	state, err := store.Create(ctx, domain.WorkflowInfo{ID: "quick-fix", Version: "3.0"}, "task",
		domain.RunSettings{}, time.Now(), testRunID)

	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := domain.DispatchField{Present: true, Value: domain.DispatchSequential}
	if state.Dispatch != want {
		t.Errorf("returned Dispatch = %+v, want %+v", state.Dispatch, want)
	}
	if got := lineAfter(mustReadFile(t, path), "global_sequence:"); got != "dispatch: sequential" {
		t.Errorf("line after global_sequence = %q, want %q", got, "dispatch: sequential")
	}
	reread, err := store.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if reread.Dispatch != want {
		t.Errorf("persisted Dispatch = %+v, want %+v", reread.Dispatch, want)
	}
}

// dispatchRewritePaths lists every Runner rewrite of an existing artifact. Each
// runs against a store over an artifact whose state was obtained from Read.
var dispatchRewritePaths = []struct {
	name string
	run  func(ctx context.Context, store domain.ArtifactStore, state domain.ArtifactState) error
}{
	{"Apply", func(ctx context.Context, s domain.ArtifactStore, st domain.ArtifactState) error {
		step := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), nil)
		_, err := s.Apply(ctx, st, step)
		return err
	}},
	{"AdoptRunnerSettings", func(ctx context.Context, s domain.ArtifactStore, _ domain.ArtifactState) error {
		_, err := s.AdoptRunnerSettings(ctx, domain.ExecutionModeAuto, false, false, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
		return err
	}},
	{"SetPhase", func(ctx context.Context, s domain.ArtifactStore, st domain.ArtifactState) error {
		_, err := s.SetPhase(ctx, st, "COMPLETED", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
		return err
	}},
	{"SetCommitBranch", func(ctx context.Context, s domain.ArtifactStore, _ domain.ArtifactState) error {
		_, err := s.SetCommitBranch(ctx, "mosaic/run/"+testRunID, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
		return err
	}},
}

func TestRewritePaths_PresentDispatchSurvivesUnchanged(t *testing.T) {
	lines := []struct {
		name string
		line string
		want domain.DispatchField
	}{
		{"sequential", "dispatch: sequential", domain.DispatchField{Present: true, Value: "sequential"}},
		{"parallel", "dispatch: parallel", domain.DispatchField{Present: true, Value: "parallel"}},
		{"invalid", "dispatch: concurrent", domain.DispatchField{Present: true, Value: "concurrent"}},
		{"blank", "dispatch:", domain.DispatchField{Present: true, Value: ""}},
		{"case variant", "dispatch: Sequential", domain.DispatchField{Present: true, Value: "Sequential"}},
	}
	for _, rw := range dispatchRewritePaths {
		for _, ln := range lines {
			t.Run(rw.name+"/"+ln.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "Orchestration.md")
				if err := os.WriteFile(path, minimalArtifactWithConfigBytes(dispatchConfig(ln.line)), 0o600); err != nil {
					t.Fatalf("write artifact: %v", err)
				}
				store := artifact.NewFileStore(path)
				ctx := context.Background()
				before, err := store.Read(ctx)
				if err != nil {
					t.Fatalf("Read: %v", err)
				}
				if before.Dispatch != ln.want {
					t.Fatalf("Read: Dispatch = %+v, want %+v", before.Dispatch, ln.want)
				}

				if err := rw.run(ctx, store, before); err != nil {
					t.Fatalf("%s: %v", rw.name, err)
				}

				after, err := store.Read(ctx)
				if err != nil {
					t.Fatalf("Read after %s: %v", rw.name, err)
				}
				if after.Dispatch != ln.want {
					t.Errorf("Dispatch after %s = %+v, want %+v", rw.name, after.Dispatch, ln.want)
				}
				if n := dispatchLineCount(mustReadFile(t, path)); n != 1 {
					t.Errorf("dispatch lines on disk after %s = %d, want 1", rw.name, n)
				}
			})
		}
	}
}

func TestRewritePaths_AbsentDispatchStaysAbsent(t *testing.T) {
	for _, rw := range dispatchRewritePaths {
		t.Run(rw.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Orchestration.md")
			if err := os.WriteFile(path, minimalArtifactWithConfigBytes(dispatchConfig("")), 0o600); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			store := artifact.NewFileStore(path)
			ctx := context.Background()
			before, err := store.Read(ctx)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}

			if err := rw.run(ctx, store, before); err != nil {
				t.Fatalf("%s: %v", rw.name, err)
			}

			after, err := store.Read(ctx)
			if err != nil {
				t.Fatalf("Read after %s: %v", rw.name, err)
			}
			if after.Dispatch.Present {
				t.Errorf("Dispatch after %s = %+v, want absent", rw.name, after.Dispatch)
			}
			if n := dispatchLineCount(mustReadFile(t, path)); n != 0 {
				t.Errorf("dispatch lines on disk after %s = %d, want 0", rw.name, n)
			}
		})
	}
}
