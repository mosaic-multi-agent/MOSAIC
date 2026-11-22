package domain_test

// Tests for domain.RoutingTable.FirstRowInputs: the artifacts the first row
// declares, which decide the seed input wording.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

func tableWithFirstInputs(first []string) domain.RoutingTable {
	return domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, AgentID: "first", InputArtifacts: first, OutputArtifacts: []string{"Out.md"}},
		{Index: 1, AgentID: "second", InputArtifacts: []string{"Other.md"}},
	}}
}

func TestFirstRowInputs_ReturnsFirstRowInputsInOrder(t *testing.T) {
	table := tableWithFirstInputs([]string{"Requirements.md", "Research.md"})

	got := table.FirstRowInputs()

	want := []string{"Requirements.md", "Research.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FirstRowInputs() = %v, want %v", got, want)
	}
}

func TestFirstRowInputs_KeepsInputTheRowAlsoWrites(t *testing.T) {
	table := domain.RoutingTable{Rows: []domain.RoutingRow{
		{InputArtifacts: []string{"Requirements.md"}, OutputArtifacts: []string{"Requirements.md"}},
	}}

	got := table.FirstRowInputs()

	if !reflect.DeepEqual(got, []string{"Requirements.md"}) {
		t.Errorf("FirstRowInputs() = %v, want the self-written input kept", got)
	}
}

func TestFirstRowInputs_PreservesTemplatesAndWildcardsVerbatim(t *testing.T) {
	table := tableWithFirstInputs([]string{"Stage-{Stage}/Plan.md", "Stage-*/Plan.md"})

	got := table.FirstRowInputs()

	want := []string{"Stage-{Stage}/Plan.md", "Stage-*/Plan.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FirstRowInputs() = %v, want %v", got, want)
	}
}

func TestFirstRowInputs_FirstRowWithoutInputs_IsNilEvenWhenLaterRowsHaveInputs(t *testing.T) {
	table := tableWithFirstInputs(nil)

	if got := table.FirstRowInputs(); got != nil {
		t.Errorf("FirstRowInputs() = %v, want nil", got)
	}
}

func TestFirstRowInputs_NoRows_IsNil(t *testing.T) {
	var table domain.RoutingTable

	if got := table.FirstRowInputs(); got != nil {
		t.Errorf("FirstRowInputs() = %v, want nil", got)
	}
}

func TestFirstRowInputs_ReturnsACopy(t *testing.T) {
	table := tableWithFirstInputs([]string{"Requirements.md"})

	got := table.FirstRowInputs()
	got[0] = "changed.md"

	if table.Rows[0].InputArtifacts[0] != "Requirements.md" {
		t.Errorf("mutating the result changed the table: %v", table.Rows[0].InputArtifacts)
	}
}
