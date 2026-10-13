package engine_test

// Selection of the candidate artifact paths of an ended iteration: the
// Input and Output entries of the block's own rows that carry the block's
// fill-in, resolved with the iteration ID, in table order. Pure: no file
// system, no wildcard expansion.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// candRow builds a routing row with the given Input and Output entries.
func candRow(in, out []string) domain.RoutingRow {
	return domain.RoutingRow{InputArtifacts: in, OutputArtifacts: out}
}

// candBlock is a block named name over rows [start, end).
func candBlock(name string, start, end int) domain.AdmittedBlock {
	return domain.AdmittedBlock{Name: domain.BlockName(name), StartRow: start, EndRow: end}
}

// requireCandidates asserts the selection succeeds with exactly want.
func requireCandidates(t *testing.T, table domain.RoutingTable, block domain.AdmittedBlock, id string, want []string) {
	t.Helper()
	got, err := engine.IterationArtifactCandidates(table, block, domain.IterationID(id))
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want candidates %q, got %q", want, got)
	}
}

// groupedStageRows are the rows of a build workflow's Stage block, across the
// Test and Implementation groups.
func groupedStageRows() []domain.RoutingRow {
	planAndProgress := []string{"Stage-{Stage}/Plan.md", "ContractsDesign.md", "Stage-{Stage}/PlanProgress.md"}
	return []domain.RoutingRow{
		candRow([]string{"Stage-{Stage}/Plan.md", "ContractsDesign.md"}, []string{"Stage-{Stage}/PlanProgress.md"}),
		candRow(planAndProgress, []string{"Stage-{Stage}/tests-review-tdd.md"}),
		candRow(planAndProgress, []string{"Stage-{Stage}/PlanProgress.md"}),
		candRow(planAndProgress, []string{"Stage-{Stage}/implementation-review.md"}),
	}
}

func TestIterationArtifactCandidates_GroupedStageBlock_ReturnsFillInPathsOnceInTableOrder(t *testing.T) {
	table := domain.RoutingTable{Rows: groupedStageRows()}

	requireCandidates(t, table, candBlock("Stage", 0, 4), "2", []string{
		"Stage-2/Plan.md",
		"Stage-2/PlanProgress.md",
		"Stage-2/tests-review-tdd.md",
		"Stage-2/implementation-review.md",
	})
}

func TestIterationArtifactCandidates_RowsOutsideTheBlock_AreExcluded(t *testing.T) {
	rows := []domain.RoutingRow{
		candRow(nil, []string{"Stage-{Stage}/before.md"}),
		candRow([]string{"Stage-{Stage}/Plan.md"}, []string{"Stage-{Stage}/inside.md"}),
		candRow(nil, []string{"Stage-{Stage}/after.md"}),
	}
	table := domain.RoutingTable{Rows: rows}

	// The block holds only the middle row: EndRow is exclusive, so the row
	// directly after it, which uses the same fill-in, is not part of it.
	requireCandidates(t, table, candBlock("Stage", 1, 2), "1", []string{
		"Stage-1/Plan.md",
		"Stage-1/inside.md",
	})
}

func TestIterationArtifactCandidates_NonStageBlock_ReturnsItsOwnFillInPaths(t *testing.T) {
	rows := []domain.RoutingRow{
		candRow(nil, []string{"ProductPlan.md"}),
		candRow([]string{"ProductPlan.md"}, []string{"built-{Product}.md"}),
		candRow([]string{"built-{Product}.md"}, []string{"checked-{Product}.md"}),
	}
	table := domain.RoutingTable{Rows: rows}

	requireCandidates(t, table, candBlock("Product", 1, 3), "A", []string{
		"built-A.md",
		"checked-A.md",
	})
}

func TestIterationArtifactCandidates_EntriesWithoutTheFillIn_AreExcluded(t *testing.T) {
	rows := []domain.RoutingRow{
		candRow(
			[]string{"ContractsDesign.md", "*-ProductMap.md", "shared/{Other}-notes.md"},
			[]string{"notes-{Topic}.md", "Summary.md"},
		),
	}
	table := domain.RoutingTable{Rows: rows}

	requireCandidates(t, table, candBlock("Topic", 0, 1), "Pricing", []string{"notes-Pricing.md"})
}

func TestIterationArtifactCandidates_FillInWithWildcard_ReturnedResolvedWithStarKeptAtItsPosition(t *testing.T) {
	rows := []domain.RoutingRow{
		candRow([]string{"TopicPlan.md"}, []string{"notes-{Topic}.md"}),
		candRow([]string{"*-{Topic}-Findings.md", "*-ProductMap.md"}, []string{"summary-{Topic}.md"}),
	}
	table := domain.RoutingTable{Rows: rows}

	requireCandidates(t, table, candBlock("Topic", 0, 2), "Pricing", []string{
		"notes-Pricing.md",
		"*-Pricing-Findings.md",
		"summary-Pricing.md",
	})
}

func TestIterationArtifactCandidates_EveryOccurrenceOfTheFillIn_IsReplaced(t *testing.T) {
	rows := []domain.RoutingRow{
		candRow(nil, []string{"Stage-{Stage}/log-{Stage}.md"}),
	}
	table := domain.RoutingTable{Rows: rows}

	requireCandidates(t, table, candBlock("Stage", 0, 1), "3", []string{"Stage-3/log-3.md"})
}

func TestIterationArtifactCandidates_NothingQualifies_ReturnsNoCandidatesAndNoError(t *testing.T) {
	rows := []domain.RoutingRow{
		candRow([]string{"Plan.md", "*-ProductMap.md"}, []string{"Result.md"}),
	}
	table := domain.RoutingTable{Rows: rows}

	requireCandidates(t, table, candBlock("Stage", 0, 1), "1", nil)
}

func TestIterationArtifactCandidates_InvalidInput_ReturnsError(t *testing.T) {
	table := domain.RoutingTable{Rows: groupedStageRows()}
	cases := []struct {
		name      string
		block     domain.AdmittedBlock
		iteration domain.IterationID
	}{
		{"empty iteration", candBlock("Stage", 0, 4), ""},
		{"empty block name", candBlock("", 0, 4), "1"},
		{"negative start row", candBlock("Stage", -1, 4), "1"},
		{"end row beyond the table", candBlock("Stage", 0, 5), "1"},
		{"start row after end row", candBlock("Stage", 3, 2), "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := engine.IterationArtifactCandidates(table, tc.block, tc.iteration)

			if err == nil {
				t.Errorf("want an error, got none (candidates %q)", got)
			}
		})
	}
}
