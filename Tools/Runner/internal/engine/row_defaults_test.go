package engine_test

// Tests for the shared row default resolution used by engine-routed and
// consultation-routed dispatches.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func stagedDefaultsRow() domain.RoutingRow {
	return domain.RoutingRow{
		Index:           2,
		Agent:           "test-writer",
		AgentID:         "test-writer",
		Phase:           "EXECUTION.[Stage].Test",
		PhaseParsed:     domain.PhaseParsed{Name: "EXECUTION", Blocks: []domain.BlockName{"Stage"}, IsStaged: true, Group: "Test"},
		InputArtifacts:  []string{"Stage-*/Plan.md", "Stage-{Stage}/notes.md"},
		OutputArtifacts: []string{"Stage-{Stage}/tests.md", "Stage-*/Plan.md"},
	}
}

func defaultsSchedules(stage2HITL bool) domain.BlockSchedules {
	return newTestSchedules(
		domain.ScheduleIteration{ID: "1", Approach: "TDD"},
		domain.ScheduleIteration{ID: "2", HITL: stage2HITL, Approach: "TDD"},
	)
}

// defaultsContext is the schedule context of a resolution that reads only the
// current schedules.
func defaultsContext(schedules domain.BlockSchedules) engine.ScheduleContext {
	return engine.ScheduleContext{Schedules: schedules}
}

func TestResolveRowDefaults_StagedRow_FillsInInputsKeepsWildcardsAndOrsStageHITL(t *testing.T) {
	got, err := engine.ResolveRowDefaultsForIteration(stagedDefaultsRow(), "2", defaultsContext(defaultsSchedules(true)))

	if err != nil {
		t.Fatalf("ResolveRowDefaults: unexpected error %v", err)
	}
	wantIn := []string{"Stage-*/Plan.md", "Stage-2/notes.md"}
	if !reflect.DeepEqual(got.InputArtifacts, wantIn) {
		t.Errorf("inputs = %v, want %v", got.InputArtifacts, wantIn)
	}
	wantOut := []string{"Stage-2/tests.md", "Stage-*/Plan.md"}
	if !reflect.DeepEqual(got.OutputArtifacts, wantOut) {
		t.Errorf("outputs = %v, want %v", got.OutputArtifacts, wantOut)
	}
	if !got.HITL {
		t.Errorf("HITL = false, want true from the plan stage")
	}
}

func TestResolveRowDefaults_RowHITLWithoutStageHITL_IsTrue(t *testing.T) {
	row := stagedDefaultsRow()
	row.HITL = true

	got, err := engine.ResolveRowDefaultsForIteration(row, "1", defaultsContext(defaultsSchedules(false)))

	if err != nil {
		t.Fatalf("ResolveRowDefaults: unexpected error %v", err)
	}
	if !got.HITL {
		t.Errorf("HITL = false, want true from the row")
	}
}

func TestResolveRowDefaults_NoHITLAnywhere_IsFalse(t *testing.T) {
	got, err := engine.ResolveRowDefaultsForIteration(stagedDefaultsRow(), "1", defaultsContext(defaultsSchedules(true)))

	if err != nil {
		t.Fatalf("ResolveRowDefaults: unexpected error %v", err)
	}
	if got.HITL {
		t.Errorf("HITL = true for stage 1 (no HITL on row or stage 1), want false")
	}
	if len(got.InputArtifacts) == 0 {
		t.Errorf("want resolved inputs for a valid staged row, got none")
	}
}

func TestResolveRowDefaults_NonStagedRow_PassesStageWildcardThroughForTheSession(t *testing.T) {
	row := domain.RoutingRow{
		Agent:          "final-review",
		AgentID:        "final-review",
		Phase:          "REVIEW",
		PhaseParsed:    domain.PhaseParsed{Name: "REVIEW"},
		InputArtifacts: []string{"Stage-*/Plan.md"},
	}

	got, err := engine.ResolveRowDefaultsForIteration(row, "", engine.ScheduleContext{
		Schedules: defaultsSchedules(false),
	})

	if err != nil {
		t.Fatalf("ResolveRowDefaults: unexpected error %v", err)
	}
	want := []string{"Stage-*/Plan.md"}
	if !reflect.DeepEqual(got.InputArtifacts, want) {
		t.Errorf("inputs = %v, want %v", got.InputArtifacts, want)
	}
}

func TestResolveRowDefaults_NonStagedRowWithRowHITL_IsTrue(t *testing.T) {
	row := domain.RoutingRow{
		Agent:       "final-review",
		AgentID:     "final-review",
		Phase:       "REVIEW",
		PhaseParsed: domain.PhaseParsed{Name: "REVIEW"},
		HITL:        true,
	}

	got, err := engine.ResolveRowDefaultsForIteration(row, "", defaultsContext(defaultsSchedules(false)))

	if err != nil {
		t.Fatalf("ResolveRowDefaults: unexpected error %v", err)
	}
	if !got.HITL {
		t.Errorf("HITL = false, want true from the row")
	}
}

func TestResolveRowDefaults_StageArgumentErrors(t *testing.T) {
	nonStaged := domain.RoutingRow{Agent: "final-review", AgentID: "final-review", PhaseParsed: domain.PhaseParsed{Name: "REVIEW"}}
	cases := []struct {
		name      string
		row       domain.RoutingRow
		iteration domain.IterationID
		schedules domain.BlockSchedules
		want      error
	}{
		{"staged row without stage", stagedDefaultsRow(), "", defaultsSchedules(false), engine.ErrRowDefaultsStageRequired},
		{"staged row with stage missing from set", stagedDefaultsRow(), "7", defaultsSchedules(false), engine.ErrRowDefaultsStageNotInSet},
		{"staged row with no schedules", stagedDefaultsRow(), "1", nil, engine.ErrRowDefaultsStageNotInSet},
		{"non-staged row with a stage", nonStaged, "1", defaultsSchedules(false), engine.ErrRowDefaultsStageOnNonStagedRow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := engine.ResolveRowDefaultsForIteration(tc.row, tc.iteration, defaultsContext(tc.schedules))

			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want one wrapping %v", err, tc.want)
			}
			if !reflect.DeepEqual(got, domain.DispatchDefaults{}) {
				t.Errorf("want zero defaults on error, got %+v", got)
			}
		})
	}
}
