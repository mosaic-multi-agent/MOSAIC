package session_test

// Tests for the run artifact's dispatch field at run start: this Runner runs
// sequentially, so a resume on an artifact whose dispatch field is present and
// anything other than exactly "sequential" is refused before any write or
// dispatch, while an artifact without the field resumes normally.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// dispatchResumeFixture builds a commit-enabled resume whose setup is still
// pending (so a supported artifact would dispatch the commit setup agent
// first), with the given dispatch field recorded.
func dispatchResumeFixture(t *testing.T, dispatch domain.DispatchField) (f *harness.MockAdapter, store *memStore, cfg domain.RunConfig, run func() (domain.RunOutcome, error)) {
	t.Helper()
	ses, f, store, orchPath := newCommitSession(t)
	store.state = commitResumeState()
	store.state.Dispatch = dispatch
	store.exists = true
	queueSuccessfulSetupAndWorkflow(f, "mosaic/run/dispatch-mode")

	cfg = baseCommitConfig(orchPath)
	markResume(&cfg)
	cfg.Supplied.CommitBranchVariant = true
	run = func() (domain.RunOutcome, error) { return ses.Start(context.Background(), cfg) }
	return f, store, cfg, run
}

func TestSession_Start_Resume_UnsupportedDispatch_RefusedBeforeAnyWriteOrDispatch(t *testing.T) {
	cases := []struct {
		name     string
		dispatch domain.DispatchField
		wantText string
	}{
		{"parallel", domain.DispatchField{Present: true, Value: "parallel"}, "parallel"},
		{"invalid value", domain.DispatchField{Present: true, Value: "concurrent"}, "concurrent"},
		{"blank value", domain.DispatchField{Present: true, Value: ""}, ""},
		{"case variant", domain.DispatchField{Present: true, Value: "Sequential"}, "Sequential"},
		{"padded value", domain.DispatchField{Present: true, Value: " sequential"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, store, _, run := dispatchResumeFixture(t, tc.dispatch)
			before := store.state

			got, err := run()

			msg := requireRefused(t, got, err)
			if !strings.Contains(msg, "dispatch") {
				t.Errorf("want the refusal to name the dispatch field, got %q", msg)
			}
			if !strings.Contains(strings.ToLower(msg), "sequential") {
				t.Errorf("want the refusal to say the Runner runs sequentially, got %q", msg)
			}
			if tc.wantText != "" && !strings.Contains(msg, tc.wantText) {
				t.Errorf("want the refusal to show the recorded value %q, got %q", tc.wantText, msg)
			}
			requireNothingWrittenOrDispatched(t, f, store)
			if len(store.BranchCalls) != 0 {
				t.Errorf("want no SetCommitBranch call, got %v", store.BranchCalls)
			}
			if !reflect.DeepEqual(before, store.state) {
				t.Errorf("want the stored artifact untouched by a refused run\nbefore: %+v\nafter:  %+v", before, store.state)
			}
		})
	}
}

func TestSession_Start_Resume_UnsupportedDispatch_PlainWorkflowRefusedWithoutInvocation(t *testing.T) {
	ses, f, store, orchPath := newLinearSession(t)
	store.state = domain.ArtifactState{
		RunID:       testRunID,
		Workflow:    "linear",
		Task:        "test task",
		Dispatch:    domain.DispatchField{Present: true, Value: "parallel"},
		RunSettings: domain.RunSettings{Mode: domain.ExecutionModeAuto},
	}
	store.exists = true
	cfg := baseLinearConfig(orchPath)
	markResume(&cfg)

	got, err := ses.Start(context.Background(), cfg)

	msg := requireRefused(t, got, err)
	if !strings.Contains(msg, "dispatch") {
		t.Errorf("want the refusal to name the dispatch field, got %q", msg)
	}
	requireNothingWrittenOrDispatched(t, f, store)
}

func TestSession_Start_Resume_SequentialDispatch_Proceeds(t *testing.T) {
	f, store, _, run := dispatchResumeFixture(t, domain.DispatchField{Present: true, Value: domain.DispatchSequential})

	got, err := run()

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(f.Invocations()) != 3 {
		t.Errorf("want the commit setup and both workflow agents dispatched, got %d invocations", len(f.Invocations()))
	}
	if want := (domain.DispatchField{Present: true, Value: domain.DispatchSequential}); store.state.Dispatch != want {
		t.Errorf("Dispatch after the run = %+v, want %+v", store.state.Dispatch, want)
	}
}

func TestSession_Start_Resume_AbsentDispatch_ProceedsAndStaysAbsent(t *testing.T) {
	f, store, _, run := dispatchResumeFixture(t, domain.DispatchField{})

	got, err := run()

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(f.Invocations()) != 3 {
		t.Errorf("want the commit setup and both workflow agents dispatched, got %d invocations", len(f.Invocations()))
	}
	if store.state.Dispatch.Present {
		t.Errorf("want dispatch left absent after the run, got %+v", store.state.Dispatch)
	}
}
