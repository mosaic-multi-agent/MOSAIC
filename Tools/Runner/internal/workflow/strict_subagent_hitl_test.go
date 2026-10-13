package workflow_test

// Strict parsing of the Subagent cell (kebab-case id with an optional mode
// qualifier, unique across rows: T6, T8) and the HITL cell (exactly TRUE or
// FALSE: T7).

import (
	"testing"
)

func TestParseStrict_SubagentCell_SplitsIntoWholeCellBareIdAndQualifier(t *testing.T) {
	cases := []struct {
		cell          string
		wantAgentID   string
		wantQualifier string
	}{
		{"build-review", "build-review", ""},
		{"build-review(tests)", "build-review", "tests"},
		{"codebase-research(re-check)", "codebase-research", "re-check"},
		{"tdd2", "tdd2", ""},
		{"tdd2(phase2)", "tdd2", "phase2"},
	}
	for _, tc := range cases {
		t.Run(tc.cell, func(t *testing.T) {
			table := mustParseStrict(t, table7(plain7("PLANNING", "agent-a"), plain7("REVIEW", tc.cell)))

			row := table.Rows[1]
			if row.Agent != tc.cell {
				t.Errorf("Agent: want the whole cell %q, got %q", tc.cell, row.Agent)
			}
			if row.AgentID != tc.wantAgentID {
				t.Errorf("AgentID: want %q, got %q", tc.wantAgentID, row.AgentID)
			}
			if row.Qualifier != tc.wantQualifier {
				t.Errorf("Qualifier: want %q, got %q", tc.wantQualifier, row.Qualifier)
			}
		})
	}
}

func TestParseStrict_SameAgentWithDistinctQualifiers_IsAccepted(t *testing.T) {
	table := mustParseStrict(t, table7(
		plain7("EXECUTION", "build-review(tests)"),
		plain7("EXECUTION", "build-review(impl)"),
		plain7("REVIEW", "build-review"),
	))

	wantCells := []string{"build-review(tests)", "build-review(impl)", "build-review"}
	for i, row := range table.Rows {
		if row.Agent != wantCells[i] || row.AgentID != "build-review" {
			t.Errorf("Rows[%d]: want cell %q and bare id build-review, got Agent %q AgentID %q",
				i, wantCells[i], row.Agent, row.AgentID)
		}
	}
	if table.Rows[0].Qualifier != "tests" || table.Rows[1].Qualifier != "impl" || table.Rows[2].Qualifier != "" {
		t.Errorf("Qualifiers: want tests, impl, empty; got %q, %q, %q",
			table.Rows[0].Qualifier, table.Rows[1].Qualifier, table.Rows[2].Qualifier)
	}
}

func TestParseStrict_MalformedSubagentCell_IsRefusedByT6(t *testing.T) {
	cases := map[string]string{
		"space before qualifier":   "build-review (tests)",
		"uppercase id":             "Build-Review",
		"uppercase qualifier":      "build-review(Tests)",
		"empty qualifier":          "build-review()",
		"suffix after qualifier":   "build-review(tests)x",
		"space inside qualifier":   "build-review(a b)",
		"two qualifiers":           "build-review(tests)(impl)",
		"doubled hyphen":           "build--review",
		"leading hyphen":           "-build-review",
		"trailing hyphen":          "build-review-",
		"leading digit":            "2fast",
		"underscore":               "build_review",
		"unclosed qualifier":       "build-review(tests",
		"qualifier with doubled -": "build-review(a--b)",
		"qualifier with leading -": "build-review(-tests)",
		"qualifier with a fill-in": "build-review({Stage})",
		"trailing star":            "build-review*",
		"dash alone (none marker)": "-",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(table7(plain7("PLANNING", "agent-a"), plain7("REVIEW", cell)))

			re := requireRefusal(t, err, refusalWant{anyOf: []string{"T6", "P11"}, row: 2})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_MalformedSubagentCell_ReportsT6AndTheCellAsToken(t *testing.T) {
	cases := []string{"build-review (tests)", "Build-Review", "build-review()", "build-review(tests)x"}
	for _, cell := range cases {
		t.Run(cell, func(t *testing.T) {
			_, err := parseStrict(table7(plain7("PLANNING", "agent-a"), plain7("REVIEW", cell)))

			requireRefusal(t, err, refusalWant{rule: "T6", row: 2, token: cell})
		})
	}
}

func TestParseStrict_DuplicateSubagentCell_IsRefusedByT8(t *testing.T) {
	cases := map[string][][]string{
		"bare id twice": {
			plain7("PLANNING", "agent-a"), plain7("PLANNING", "agent-b"), plain7("REVIEW", "agent-a"),
		},
		"qualified cell twice": {
			plain7("PLANNING", "build-review(tests)"), plain7("REVIEW", "build-review(tests)"),
		},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(table7(rows...))

			re := requireRefusal(t, err, refusalWant{rule: "T8"})
			requireRowIn(t, re, 1, len(rows))
			if re.Token == "" {
				t.Error("refusal must name the duplicated cell as its token")
			}
		})
	}
}

func TestParseStrict_HITLCell_TrueAndFalseDecodeToBool(t *testing.T) {
	table := mustParseStrict(t, table7(
		row7("PLANNING", "agent-a", "TRUE", "NEXT", "-"),
		row7("REVIEW", "agent-b", "FALSE", "COMPLETE", "-"),
	))

	if !table.Rows[0].HITL || table.Rows[1].HITL {
		t.Errorf("HITL: want true then false, got %v then %v", table.Rows[0].HITL, table.Rows[1].HITL)
	}
}

func TestParseStrict_HITLCellOtherThanTrueOrFalse_IsRefusedByT7(t *testing.T) {
	cases := map[string]string{
		"dash":       "-",
		"mixed case": "True",
		"lowercase":  "false",
		"typo":       "TURE",
		"yes":        "YES",
		"checkbox":   "✅",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(threeRowsWith("HITL", cell))

			re := requireRefusal(t, err, refusalWant{rule: "T7", row: 2, token: cell})
			requireRowOrToken(t, re)
		})
	}
}
