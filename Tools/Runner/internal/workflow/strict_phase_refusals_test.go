package workflow_test

// Strict Phase cell refusals: P1 (group without a block reference, more than
// one segment after the last block reference), P2 (group token characters),
// P10 (block references directly after the phase name, no block twice in one
// row) and P12 (phase name characters, INIT/COMPLETED reserved).

import "testing"

// parsePhaseCell parses a table whose second row carries the phase cell (so a
// refusal about it must name row 2) and returns the parse error.
func parsePhaseCell(phase string) error {
	rows := []string{strictRow("PLANNING", "agent-a"), strictRow(phase, "agent-b")}
	_, err := parseStrict(strictRegion(rows, strictBlocksTable))
	return err
}

func TestParseStrict_P1_GroupWithoutBlockReference_IsRefused(t *testing.T) {
	cases := []string{
		"EXECUTION.Test",
		"EXECUTION.Test.Implementation",
		"EXECUTION.[Stage].A.B",
		"EXECUTION.[Stage].Test.Implementation",
		"EXECUTION.[Domain].[Stage].A.B",
	}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			err := parsePhaseCell(phase)

			re := requireRefusal(t, err, refusalWant{rule: "P1", row: 2, token: phase})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P2_GroupTokenCharacters_AreRefused(t *testing.T) {
	cases := []string{
		"EXECUTION.[Stage].T st",
		"EXECUTION.[Stage].Te_st",
		"EXECUTION.[Stage].-Test",
		"EXECUTION.[Stage].Test!",
	}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			err := parsePhaseCell(phase)

			requireRefusal(t, err, refusalWant{rule: "P2", row: 2, token: phase})
		})
	}
}

func TestParseStrict_P2_EmptyGroupSegment_IsRefused(t *testing.T) {
	err := parsePhaseCell("EXECUTION.[Stage].")

	requireRefusal(t, err, refusalWant{anyOf: []string{"P1", "P2"}, row: 2, token: "EXECUTION.[Stage]."})
}

func TestParseStrict_P10_BlockReferenceNotDirectlyAfterPhaseName_IsRefused(t *testing.T) {
	cases := []string{
		"EXECUTION.Test.[Stage]",
		"EXECUTION.Foo.[Domain].[Stage]",
	}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			err := parsePhaseCell(phase)

			// A group (or any segment) before a block reference breaks both
			// "a group follows a block reference" and "block references
			// directly follow the phase name".
			requireRefusal(t, err, refusalWant{anyOf: []string{"P1", "P10"}, row: 2, token: phase})
		})
	}
}

func TestParseStrict_P10_BlockAppearsTwiceInOneRow_IsRefused(t *testing.T) {
	cases := []string{
		"EXECUTION.[Stage].[Stage]",
		"EXECUTION.[Stage].[Stage].Test",
		"EXECUTION.[Domain].[Stage].[Domain]",
	}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			err := parsePhaseCell(phase)

			requireRefusal(t, err, refusalWant{rule: "P10", row: 2, token: phase})
		})
	}
}

func TestParseStrict_MalformedBlockReference_IsRefused(t *testing.T) {
	// A block reference is "." then "[", a block name, "]". Any other shape is
	// not a block reference and must not be read as one (or as a plain phase).
	cases := map[string]string{
		"unclosed bracket":          "EXECUTION.[Stage",
		"empty brackets":            "EXECUTION.[]",
		"text after the bracket":    "EXECUTION.[Stage]x",
		"no dot before the bracket": "EXECUTION[Stage]",
		"whitespace in brackets":    "EXECUTION.[ Stage ]",
	}
	for name, phase := range cases {
		t.Run(name, func(t *testing.T) {
			err := parsePhaseCell(phase)

			re := requireRefusal(t, err, refusalWant{row: 2, token: phase})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P12_PhaseNameCharacters_AreRefused(t *testing.T) {
	cases := []string{
		"execution",
		"Execution.[Stage]",
		"1PLAN",
		"_PLAN",
		"PLAN-NING",
		"PLAN ING",
	}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			err := parsePhaseCell(phase)

			requireRefusal(t, err, refusalWant{rule: "P12", row: 2, token: phase})
		})
	}
}

func TestParseStrict_P12_ReservedPhaseNames_AreRefused(t *testing.T) {
	cases := []string{"INIT", "COMPLETED", "INIT.[Stage]", "COMPLETED.[Stage].Test"}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			err := parsePhaseCell(phase)

			requireRefusal(t, err, refusalWant{rule: "P12", row: 2, token: phase})
		})
	}
}

func TestParseStrict_P12_NamesLikeReservedOnesAreAccepted(t *testing.T) {
	// INIT and COMPLETED are reserved as whole names only.
	rows := strictPhases("INITIAL", "COMPLETED_RUN", "INIT2")

	table := mustParseStrict(t, strictRegion(rows))

	if len(table.Rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(table.Rows))
	}
}

func TestParseStrict_PhaseRefusal_NamesTheOffendingRowPosition(t *testing.T) {
	// The defective cell is in row 4 of 5: the refusal names row 4, so a
	// parser that reports the first or last row is caught.
	rows := strictPhases("PLANNING", "RESEARCH", "DESIGN", "EXECUTION.Test", "REVIEW")

	_, err := parseStrict(strictRegion(rows))

	requireRefusal(t, err, refusalWant{rule: "P1", row: 4, token: "EXECUTION.Test"})
}
