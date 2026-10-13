package planstages_test

// Tests for reading valid block schedules of every supported shape.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/planstages"
)

func TestReadBlockSchedule_PlannerShape_ReadsAllColumnsInOrder(t *testing.T) {
	req := stageRequest("sched-planner-tdd-soft.md", true, groupedApproaches...)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil, Approach: "TDD"},
		{ID: "2", HITL: true, DependsOn: []domain.IterationID{"1"}, Approach: "Implementation-First"},
		{ID: "3", HITL: false, DependsOn: []domain.IterationID{"1", "2"}, Approach: "TDD"},
	})
}

func TestReadBlockSchedule_CopiesBlockSourceAndSectionFromRequest(t *testing.T) {
	req := stageRequest("sched-planner-tdd-soft.md", true, groupedApproaches...)
	req.Source = "Stage-{Plan}/Plan.md"

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	if got.Block != req.Block || got.Source != req.Source || got.Section != req.Section {
		t.Errorf("want block/source/section %q/%q/%q, got %q/%q/%q",
			req.Block, req.Source, req.Section, got.Block, got.Source, got.Section)
	}
}

func TestReadBlockSchedule_AuditShapeWithoutDependsOn_HasNoDependencies(t *testing.T) {
	req := stageRequest("sched-audit-no-depends.md", true, groupedApproaches...)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil, Approach: "Tests-Only"},
		{ID: "2", HITL: true, DependsOn: nil, Approach: "Implementation-Only"},
	})
}

func TestReadBlockSchedule_OtherSectionAndBlockName_NonNumericIDsVerbatim(t *testing.T) {
	req := planstages.ScheduleRequest{
		Path:    planstagesFixture("sched-generation-section.md"),
		Source:  "Plan.md",
		Section: "Generation",
		Block:   "Generation",
	}

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "A", HITL: false, DependsOn: nil},
		{ID: "b-2", HITL: true, DependsOn: []domain.IterationID{"A"}},
		{ID: "Acme", HITL: false, DependsOn: []domain.IterationID{"A", "b-2"}},
	})
}

func TestReadBlockSchedule_UngroupedBlock_IgnoresApproachColumn(t *testing.T) {
	// Approach cells here are "-" and free text; neither is a defect.
	req := stageRequest("sched-ungrouped-with-approach.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil, Approach: ""},
		{ID: "2", HITL: true, DependsOn: []domain.IterationID{"1"}, Approach: ""},
	})
}

func TestReadBlockSchedule_NoHITLColumn_AllFalse(t *testing.T) {
	req := stageRequest("sched-no-hitl-col.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil},
		{ID: "2", HITL: false, DependsOn: []domain.IterationID{"1"}},
	})
}

func TestReadBlockSchedule_EmptyTable_IsValidEmptySchedule(t *testing.T) {
	req := stageRequest("sched-empty.md", true, groupedApproaches...)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	if len(got.Iterations) != 0 {
		t.Errorf("want no iterations, got %#v", got.Iterations)
	}
	if got.Block != req.Block {
		t.Errorf("Block: want %q, got %q", req.Block, got.Block)
	}
}

func TestReadBlockSchedule_LevelThreeHeadingBeforeTable_DoesNotEndSearch(t *testing.T) {
	req := stageRequest("sched-level3-before-table.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil},
	})
}

func TestReadBlockSchedule_DependsOnEntryDifferingInCase_AcceptedAndStoredVerbatim(t *testing.T) {
	req := stageRequest("sched-depends-case-insensitive.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "Acme", HITL: false, DependsOn: nil},
		{ID: "B", HITL: false, DependsOn: []domain.IterationID{"acme"}},
	})
}

func TestReadBlockSchedule_DependsOnWithSpacesAroundSeparator_ElementsTrimmed(t *testing.T) {
	req := stageRequest("sched-depends-padded-separators.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil},
		{ID: "2", HITL: false, DependsOn: nil},
		{ID: "3", HITL: false, DependsOn: []domain.IterationID{"1", "2"}},
	})
}

func TestReadBlockSchedule_HeadingWithSurroundingWhiteSpace_StillMatches(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
	}{
		{"trailing spaces", "sched-heading-trailing-space.md"},
		{"leading spaces", "sched-heading-leading-space.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := stageRequest(tc.fixture, false)

			got, err := planstages.ReadBlockSchedule(req)
			if err != nil {
				t.Fatalf("ReadBlockSchedule: %v", err)
			}

			requireIterations(t, got.Iterations, []domain.ScheduleIteration{
				{ID: "1", HITL: false, DependsOn: nil},
				{ID: "2", HITL: false, DependsOn: []domain.IterationID{"1"}},
			})
		})
	}
}
