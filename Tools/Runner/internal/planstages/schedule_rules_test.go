package planstages_test

// Tests for V2-V6 defects and the order in which defects are reported.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/planstages"
)

type defectCase struct {
	name    string
	fixture string
	grouped bool
	rule    string
	row     int
	token   string
}

func runDefectCases(t *testing.T, cases []defectCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req planstages.ScheduleRequest
			if tc.grouped {
				req = stageRequest(tc.fixture, true, groupedApproaches...)
			} else {
				req = stageRequest(tc.fixture, false)
			}

			_, err := planstages.ReadBlockSchedule(req)

			requireDefect(t, err, req, tc.rule, tc.row, tc.token)
		})
	}
}

func TestReadBlockSchedule_InvalidOrDuplicateID_V2(t *testing.T) {
	runDefectCases(t, []defectCase{
		{"bad character", "sched-id-bad-char.md", false, "V2", 2, "a_b"},
		{"leading dash", "sched-id-leading-dash.md", false, "V2", 3, "-x"},
		{"empty ID", "sched-id-empty.md", false, "V2", 2, ""},
		{"non-ASCII letter", "sched-id-non-ascii-letter.md", false, "V2", 2, "éa"},
		{"duplicate ignoring case names later row", "sched-id-duplicate-case.md", false, "V2", 3, "acme"},
	})
}

func TestReadBlockSchedule_GroupedBlockApproachDefects_V3(t *testing.T) {
	runDefectCases(t, []defectCase{
		{"no Approach column", "sched-grouped-no-approach-col.md", true, "V3", 0, "Approach"},
		{"empty Approach cell", "sched-grouped-empty-approach.md", true, "V3", 2, ""},
		{"dash Approach cell", "sched-grouped-dash-approach.md", true, "V3", 2, "-"},
	})
}

func TestReadBlockSchedule_UngroupedBlockWithoutApproachColumn_IsValid(t *testing.T) {
	req := stageRequest("sched-grouped-no-approach-col.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}
	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil, Approach: ""},
		{ID: "2", HITL: false, DependsOn: []domain.IterationID{"1"}, Approach: ""},
	})
}

func TestReadBlockSchedule_DependsOnDefects_V4(t *testing.T) {
	runDefectCases(t, []defectCase{
		{"unknown ID", "sched-depends-unknown.md", false, "V4", 2, "9"},
		{"second element unknown", "sched-depends-second-unknown.md", false, "V4", 3, "9"},
		{"ID of a later row", "sched-depends-later-row.md", false, "V4", 1, "2"},
		{"duplicate entry", "sched-depends-duplicate.md", false, "V4", 3, "1"},
		{"duplicate entry differing in case", "sched-depends-duplicate-case.md", false, "V4", 2, "a"},
		{"empty cell", "sched-depends-empty-cell.md", false, "V4", 2, ""},
		{"empty element names whole cell", "sched-depends-empty-element.md", false, "V4", 3, "1,,2"},
		{"dash inside a list", "sched-depends-dash-in-list.md", false, "V4", 2, "-"},
		{"the row's own ID", "sched-depends-own-id.md", false, "V4", 2, "2"},
		{"trailing comma names whole cell", "sched-depends-trailing-comma.md", false, "V4", 2, "1,"},
	})
}

func TestReadBlockSchedule_HITLCellNotExactlyTrueOrFalse_V5(t *testing.T) {
	runDefectCases(t, []defectCase{
		{"mixed case True", "sched-hitl-mixed-case.md", false, "V5", 2, "True"},
		{"dash", "sched-hitl-dash.md", false, "V5", 1, "-"},
		{"lowercase true", "sched-hitl-lowercase.md", false, "V5", 2, "true"},
		{"empty cell", "sched-hitl-empty.md", false, "V5", 2, ""},
	})
}

func TestReadBlockSchedule_ApproachNotDeclaredForBlock_V6(t *testing.T) {
	runDefectCases(t, []defectCase{
		{"different case than declared", "sched-approach-undeclared.md", true, "V6", 2, "tdd"},
	})
}

func TestReadBlockSchedule_DeclaredApproachTokensAreStoredVerbatim(t *testing.T) {
	// "TDD" and "tdd" are two different declared tokens; each is kept as written.
	req := stageRequest("sched-approach-undeclared.md", true, "TDD", "tdd")

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil, Approach: "TDD"},
		{ID: "2", HITL: false, DependsOn: []domain.IterationID{"1"}, Approach: "tdd"},
	})
}

func TestReadBlockSchedule_GroupedBlockWithNoDeclaredApproaches_V6OnFirstRow(t *testing.T) {
	req := stageRequest("sched-planner-tdd-soft.md", true)

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "V6", 1, "TDD")
}

func TestReadBlockSchedule_UngroupedBlockNeverChecksApproachTokens(t *testing.T) {
	req := stageRequest("sched-approach-undeclared.md", false)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: false, DependsOn: nil, Approach: ""},
		{ID: "2", HITL: false, DependsOn: []domain.IterationID{"1"}, Approach: ""},
	})
}

func TestReadBlockSchedule_FirstDefectInCellOrderIsReported(t *testing.T) {
	// Row 2 breaks V2, V4, V5 and V6 at once; the ID comes first.
	runDefectCases(t, []defectCase{
		{"ID before Depends On, HITL and Approach", "sched-order-id-before-depends.md", true, "V2", 2, "a_b"},
	})
}

func TestReadBlockSchedule_WithinRowOrder_DependsOnThenHITLThenApproach(t *testing.T) {
	// Row 2 breaks V4 and V5; V4 comes first. A second table breaks V5 and V6; V5 comes first.
	runDefectCases(t, []defectCase{
		{"Depends On before HITL", "sched-order-depends-before-hitl.md", false, "V4", 2, "9"},
		{"HITL before Approach", "sched-order-hitl-before-approach.md", true, "V5", 2, "maybe"},
	})
}

func TestReadBlockSchedule_TableLevelDefectComesBeforeRowDefects(t *testing.T) {
	// No Approach column (V3) is reported before the bad ID in row 1 (V2).
	runDefectCases(t, []defectCase{
		{"missing Approach column first", "sched-order-table-level-first.md", true, "V3", 0, "Approach"},
		{"wrong first column before missing Approach column", "sched-order-table-level-first-v1.md", true, "V1", 0, "Name"},
	})
}
