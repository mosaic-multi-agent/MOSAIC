package workflow_test

// Strict parsing exposes typed routing entries: every On Success, On Findings
// and Waits For target is resolved to the one row holding that Subagent cell.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

// successRows renders three plain rows (agent-a, agent-b, agent-c) where row 1
// carries the given On Success cell.
func successRows(onSuccess string) string {
	return table7(
		row7("PLANNING", "agent-a", "FALSE", onSuccess, "-"),
		plain7("PLANNING", "agent-b"),
		plain7("REVIEW", "agent-c"),
	)
}

func TestParseStrict_OnSuccessTarget_ResolvesToTheRowAboveAndBelow(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("PLANNING", "agent-a", "FALSE", "agent-c", "-"),
		row7("PLANNING", "agent-b", "FALSE", "NEXT", "-"),
		row7("REVIEW", "agent-c", "FALSE", "agent-b", "-"),
	))

	below := table.Rows[0].Success
	if below.Kind != domain.SuccessTarget || below.Target != ref("agent-c", 2) || below.Raw != "agent-c" {
		t.Errorf("target below: want target agent-c at index 2, got %+v", below)
	}
	above := table.Rows[2].Success
	if above.Kind != domain.SuccessTarget || above.Target != ref("agent-b", 1) {
		t.Errorf("target above: want target agent-b at index 1, got %+v", above)
	}
}

func TestParseStrict_OnSuccessBlockEntry_CarriesTheBlockName(t *testing.T) {
	region := table7(
		row7("PLANNING", "agent-a", "FALSE", "[Stage]", "-"),
		row7("EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "-"),
	) + strictBlocksTable

	table := mustParseStrict(t, region)

	got := table.Rows[0].Success
	if got.Kind != domain.SuccessBlockEntry || got.Block != "Stage" || got.Raw != "[Stage]" {
		t.Errorf("want block entry of Stage with raw [Stage], got %+v", got)
	}
}

func TestParseStrict_OnSuccessFork_ListsEveryTargetInCellOrder(t *testing.T) {
	cases := map[string]string{
		"comma and space": "agent-b, agent-c",
		"comma only":      "agent-b,agent-c",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			table := mustParseStrict(t, successRows(cell))

			got := table.Rows[0].Success
			want := []domain.RowRef{ref("agent-b", 1), ref("agent-c", 2)}
			if got.Kind != domain.SuccessFork || !reflect.DeepEqual(got.Fork, want) || got.Raw != cell {
				t.Errorf("want fork %+v with raw %q, got %+v", want, cell, got)
			}
		})
	}
}

func TestParseStrict_OnSuccessCompleteAndNext_HaveTheirOwnKinds(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("PLANNING", "agent-a", "FALSE", "NEXT", "-"),
		row7("REVIEW", "agent-b", "FALSE", "COMPLETE", "-"),
	))

	if got := table.Rows[0].Success; got.Kind != domain.SuccessNext || got.Raw != "NEXT" {
		t.Errorf("NEXT: want kind next with raw NEXT, got %+v", got)
	}
	if got := table.Rows[1].Success; got.Kind != domain.SuccessComplete || got.Raw != "COMPLETE" {
		t.Errorf("COMPLETE: want kind complete with raw COMPLETE, got %+v", got)
	}
}

func TestParseStrict_OnFindings_TargetResolvesAndDashMeansNone(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("PLANNING", "agent-a", "FALSE", "NEXT", "-"),
		row7("REVIEW", "agent-b", "FALSE", "COMPLETE", "agent-a"),
	))

	none := table.Rows[0].Findings
	if none.Kind != domain.FindingsNone || none.Raw != "-" {
		t.Errorf("dash: want kind none with raw -, got %+v", none)
	}
	target := table.Rows[1].Findings
	if target.Kind != domain.FindingsTarget || target.Target != ref("agent-a", 0) || target.Raw != "agent-a" {
		t.Errorf("target: want agent-a at index 0, got %+v", target)
	}
}

func TestParseStrict_WaitsFor_ListResolvesAndDashIsNil(t *testing.T) {
	table := mustParseStrict(t, table8(joinRows("agent-b, agent-c")...))

	want := []domain.RowRef{ref("agent-b", 1), ref("agent-c", 2)}
	if !reflect.DeepEqual(table.Rows[3].WaitsFor, want) {
		t.Errorf("join WaitsFor: want %+v, got %+v", want, table.Rows[3].WaitsFor)
	}
	if table.Rows[0].WaitsFor != nil || table.Rows[1].WaitsFor != nil {
		t.Errorf("a %q cell must give nil WaitsFor, got %+v and %+v", "-", table.Rows[0].WaitsFor, table.Rows[1].WaitsFor)
	}
}

func TestParseStrict_WaitsFor_SingleTargetResolves(t *testing.T) {
	rows := joinRows("agent-b")

	table := mustParseStrict(t, table8(rows...))

	if want := []domain.RowRef{ref("agent-b", 1)}; !reflect.DeepEqual(table.Rows[3].WaitsFor, want) {
		t.Errorf("WaitsFor: want %+v, got %+v", want, table.Rows[3].WaitsFor)
	}
}

func TestParseStrict_TableWithoutWaitsForColumn_GivesNilWaitsFor(t *testing.T) {
	table := mustParseStrict(t, successRows("NEXT"))

	for i, row := range table.Rows {
		if row.WaitsFor != nil {
			t.Errorf("Rows[%d].WaitsFor: want nil without the column, got %+v", i, row.WaitsFor)
		}
	}
}

func TestParseStrict_QualifiedTargets_ResolveToExactlyThatRow(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("PLANNING", "planner", "FALSE", "build-review(tests)", "-"),
		row7("EXECUTION", "build-review(tests)", "FALSE", "build-review(impl)", "planner"),
		row7("EXECUTION", "build-review(impl)", "FALSE", "COMPLETE", "build-review(tests)"),
	))

	if got := table.Rows[0].Success.Target; got != ref("build-review(tests)", 1) {
		t.Errorf("row 1 On Success: want build-review(tests) at index 1, got %+v", got)
	}
	if got := table.Rows[1].Success.Target; got != ref("build-review(impl)", 2) {
		t.Errorf("row 2 On Success: want build-review(impl) at index 2, got %+v", got)
	}
	if got := table.Rows[2].Findings.Target; got != ref("build-review(tests)", 1) {
		t.Errorf("row 3 On Findings: want build-review(tests) at index 1, got %+v", got)
	}
}

func TestParseStrict_BareTarget_ResolvesToTheBareRowNotTheQualifiedOne(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("RESEARCH", "codebase-research(architecture)", "FALSE", "codebase-research", "-"),
		row7("RESEARCH", "codebase-research", "FALSE", "COMPLETE", "-"),
	))

	got := table.Rows[0].Success
	if got.Kind != domain.SuccessTarget || got.Target != ref("codebase-research", 1) {
		t.Errorf("want the bare row (index 1), got %+v", got)
	}
}

func TestParseStrict_ForkOfQualifiedCells_ResolvesEachEntryWhole(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("RESEARCH", "codebase-research", "FALSE", "codebase-research(architecture), codebase-research(contracts)", "-"),
		row7("RESEARCH", "codebase-research(architecture)", "FALSE", "COMPLETE", "-"),
		row7("RESEARCH", "codebase-research(contracts)", "FALSE", "COMPLETE", "-"),
	))

	want := []domain.RowRef{ref("codebase-research(architecture)", 1), ref("codebase-research(contracts)", 2)}
	if got := table.Rows[0].Success.Fork; !reflect.DeepEqual(got, want) {
		t.Errorf("Fork: want %+v, got %+v", want, got)
	}
}

func TestParseStrict_FillInOfTheRowsOwnBlock_IsAcceptedInInputAndOutput(t *testing.T) {
	rows := [][]string{
		plain7("PLANNING", "agent-a"),
		{"EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "-", "Stage-{Stage}/Plan.md", "Stage-{Stage}/Progress.md"},
	}

	table := mustParseStrict(t, table7(rows...)+strictBlocksTable)

	row := table.Rows[1]
	if !equalStrings(row.InputArtifacts, []string{"Stage-{Stage}/Plan.md"}) ||
		!equalStrings(row.OutputArtifacts, []string{"Stage-{Stage}/Progress.md"}) {
		t.Errorf("fill-ins must be kept verbatim, got in %v out %v", row.InputArtifacts, row.OutputArtifacts)
	}
}

func TestParseStrict_FillInsOfEveryEnclosingBlock_AreAcceptedOnANestedRow(t *testing.T) {
	rows := [][]string{
		plain7("PLANNING", "agent-a"),
		{"EXECUTION.[Domain].[Stage]", "agent-b", "FALSE", "COMPLETE", "-", "{Domain}/{Stage}/Plan.md", "{Domain}/Stage-{Stage}/Progress.md"},
	}
	content := table7(rows...) + blocksTable("| Domain | SystemDesign.md | Domains |", "| Stage | {Domain}/Plan.md | Stages |")

	table := mustParseStrict(t, content)

	row := table.Rows[1]
	if !equalStrings(row.InputArtifacts, []string{"{Domain}/{Stage}/Plan.md"}) ||
		!equalStrings(row.OutputArtifacts, []string{"{Domain}/Stage-{Stage}/Progress.md"}) {
		t.Errorf("fill-ins must be kept verbatim, got in %v out %v", row.InputArtifacts, row.OutputArtifacts)
	}
}

