package engine_test

// Tests of the Input wildcard rule of artifact-path resolution. The engine
// matches no wildcard: every Input entry containing '*' is returned unchanged
// (after fill-ins), whatever the schedules hold, and the session lists the
// existing artifacts.

import (
	"errors"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// wildcardInputForms are the Input forms a 2.16 table may declare.
var wildcardInputForms = []string{
	"Stage-*/Plan.md",
	"*-Research.md",
	"Stage-*/implementation-review.md",
	"*/Notes.md",
}

func TestResolveArtifactsForIteration_2_16Context_InputWildcardPassesThroughUnexpanded(t *testing.T) {
	cases := []struct {
		name      string
		schedules domain.BlockSchedules
	}{
		{"one block schedule", nonNumericSchedules("Implementation-Only")},
		{"no schedules at all", nil},
		{"one block with no iterations", domain.BlockSchedules{nonNumericBlock: {Block: nonNumericBlock}}},
		{"several blocks", func() domain.BlockSchedules {
			two := nonNumericSchedules("Implementation-Only")
			two["Other"] = domain.BlockSchedule{Block: "Other", Iterations: []domain.ScheduleIteration{{ID: "X"}}}
			return two
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := engine.ScheduleContext{Schedules: tc.schedules}

			got, err := engine.ResolveArtifactsForIteration(wildcardInputForms, "", "", sc, true)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, wildcardInputForms) {
				t.Errorf("got %v, want every entry unchanged %v", got, wildcardInputForms)
			}
		})
	}
}

func TestResolveArtifactsForIteration_2_16Context_FillInsResolvedAroundWildcardEntries(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: nonNumericSchedules("Implementation-Only")}
	arts := []string{"Stage-*/Plan.md", "Stage-{Stage}/notes.md", "*-Research.md"}

	got, err := engine.ResolveArtifactsForIteration(arts, nonNumericBlock, "b-2", sc, true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Stage-*/Plan.md", "Stage-b-2/notes.md", "*-Research.md"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveArtifactsForIteration_2_16Context_UnresolvableFillInStillAnError(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: nonNumericSchedules("Implementation-Only")}

	_, err := engine.ResolveArtifactsForIteration(
		[]string{"Stage-*/Plan.md", "Stage-{Stage}/notes.md"}, nonNumericBlock, "", sc, true)

	if err == nil {
		t.Error("want an error for {Stage} without an iteration, got nil")
	}
}

func TestResolveArtifactsForIteration_2_16Context_OutputWildcardPassesThrough(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: nonNumericSchedules("Implementation-Only")}

	got, err := engine.ResolveArtifactsForIteration(wildcardInputForms, "", "", sc, false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got, wildcardInputForms) {
		t.Errorf("got %v, want every entry unchanged %v", got, wildcardInputForms)
	}
}

func TestResolveRowDefaultsForIteration_2_16Context_InputWildcardsReturnedUnexpanded(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: defaultsSchedules(false)}

	got, err := engine.ResolveRowDefaultsForIteration(stagedDefaultsRow(), "2", sc)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantIn := []string{"Stage-*/Plan.md", "Stage-2/notes.md"}
	if !slices.Equal(got.InputArtifacts, wantIn) {
		t.Errorf("inputs = %v, want %v", got.InputArtifacts, wantIn)
	}
	wantOut := []string{"Stage-2/tests.md", "Stage-*/Plan.md"}
	if !slices.Equal(got.OutputArtifacts, wantOut) {
		t.Errorf("outputs = %v, want %v", got.OutputArtifacts, wantOut)
	}
}

func TestResolveRowDefaultsForIteration_2_16Context_RowOutsideEveryBlockNeedsNoSchedule(t *testing.T) {
	row := domain.RoutingRow{
		Agent:          "final-review",
		AgentID:        "final-review",
		Phase:          "REVIEW",
		PhaseParsed:    domain.PhaseParsed{Name: "REVIEW"},
		InputArtifacts: []string{"*-Research.md"},
	}

	got, err := engine.ResolveRowDefaultsForIteration(row, "", engine.ScheduleContext{})

	if errors.Is(err, engine.ErrRowDefaultsStageNotInSet) || err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got.InputArtifacts, []string{"*-Research.md"}) {
		t.Errorf("inputs = %v, want the entry unchanged", got.InputArtifacts)
	}
}

