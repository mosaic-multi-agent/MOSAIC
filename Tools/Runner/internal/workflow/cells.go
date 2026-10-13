package workflow

// Strict parsing of the routing table's identity cells: Phase, Subagent, HITL,
// Input and Output, and the placement of fill-ins. The routing cells (On
// Success, On Findings, Waits For) are resolved against the Subagent cells in
// targets.go.

import (
	"strconv"
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

// Column names of the routing table (WorkflowFormat 2.16 sec. 6.1).
const (
	phaseColumn      = "Phase"
	subagentColumn   = "Subagent"
	hitlColumn       = "HITL"
	onSuccessColumn  = "On Success"
	onFindingsColumn = "On Findings"
	waitsForColumn   = "Waits For"
	inputColumn      = "Input"
	outputColumn     = "Output"
)

// strictRequiredColumns are the columns every 2.16 routing table carries (T2).
var strictRequiredColumns = []string{
	phaseColumn, subagentColumn, hitlColumn, onSuccessColumn, onFindingsColumn, inputColumn, outputColumn,
}

// routingColumns holds the position of each routing-table column in the
// header; an optional column that is absent holds -1.
type routingColumns struct {
	phase, subagent, hitl, onSuccess, onFindings, input, output int
	waitsFor, row                                               int
	header                                                      []string
}

func locateColumns(t mdtable.Table) routingColumns {
	return routingColumns{
		phase:      t.Column(phaseColumn),
		subagent:   t.Column(subagentColumn),
		hitl:       t.Column(hitlColumn),
		onSuccess:  t.Column(onSuccessColumn),
		onFindings: t.Column(onFindingsColumn),
		input:      t.Column(inputColumn),
		output:     t.Column(outputColumn),
		waitsFor:   t.Column(waitsForColumn),
		row:        t.Column(rowColumnName),
		header:     t.Header,
	}
}

// parseRowIdentities decodes the Phase, Subagent, HITL, Input and Output cells
// of every data row and checks the cell-level rules that need no other row,
// plus the uniqueness of the Subagent cells (T8). The routing cells of the
// returned rows are filled in by resolveRoutingCells.
func parseRowIdentities(t mdtable.Table, c routingColumns, workflowID string) ([]domain.RoutingRow, error) {
	rows := make([]domain.RoutingRow, 0, len(t.Rows))
	firstRowOf := map[string]int{}
	for i, raw := range t.Rows {
		n := i + 1
		if err := refuseEmptyCell(raw, c, n, workflowID); err != nil {
			return nil, err
		}
		phase := raw[c.phase]
		parsed, defect := parsePhaseCell(phase)
		if defect != nil {
			return nil, tableRefusal(workflowID, defect.rule, n, phase, defect.reason)
		}
		if err := checkFillIns(raw, c, parsed, n, workflowID); err != nil {
			return nil, err
		}
		agent := raw[c.subagent]
		agentID, qualifier, ok := splitSubagentCell(agent)
		if !ok {
			return nil, tableRefusal(workflowID, "T6", n, agent,
				"Subagent cell must be a kebab-case agent id, optionally followed directly by a kebab-case qualifier in parentheses")
		}
		if first, dup := firstRowOf[agent]; dup {
			return nil, tableRefusal(workflowID, "T8", n, agent,
				"Subagent cell is already held by row "+strconv.Itoa(first)+"; every Subagent cell must be unique")
		}
		firstRowOf[agent] = n
		hitl, ok := parseHITLCell(raw[c.hitl])
		if !ok {
			return nil, tableRefusal(workflowID, "T7", n, raw[c.hitl], "HITL must be exactly TRUE or FALSE")
		}
		rows = append(rows, domain.RoutingRow{
			Index:           i,
			Phase:           phase,
			PhaseParsed:     parsed,
			Agent:           agent,
			AgentID:         agentID,
			Qualifier:       qualifier,
			HITL:            hitl,
			InputArtifacts:  splitArtifacts(raw[c.input]),
			OutputArtifacts: splitArtifacts(raw[c.output]),
		})
	}
	return rows, nil
}

// refuseEmptyCell refuses a data row holding an empty cell (T12). The Row
// column is validated separately.
func refuseEmptyCell(raw []string, c routingColumns, row int, workflowID string) error {
	for col, cell := range raw {
		if col == c.row || cell != "" {
			continue
		}
		return tableRefusal(workflowID, "T12", row, c.header[col],
			"the "+c.header[col]+" cell is empty; write - where a column has no value")
	}
	return nil
}

// splitSubagentCell splits a Subagent cell into its bare agent id and its mode
// qualifier ("" when absent). ok is false unless the cell is a kebab-case id,
// optionally followed directly by a non-empty kebab-case qualifier in
// parentheses.
func splitSubagentCell(cell string) (agentID, qualifier string, ok bool) {
	open := strings.IndexByte(cell, '(')
	if open < 0 {
		return cell, "", isKebabCase(cell)
	}
	if !strings.HasSuffix(cell, ")") {
		return "", "", false
	}
	agentID, qualifier = cell[:open], cell[open+1:len(cell)-1]
	if !isKebabCase(agentID) || !isKebabCase(qualifier) {
		return "", "", false
	}
	return agentID, qualifier, true
}

// isKebabCase reports whether s is runs of lowercase letters and digits joined
// by single "-", starting with a letter.
func isKebabCase(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', isDigit(c):
		case c == '-' && s[i-1] != '-':
		default:
			return false
		}
	}
	return true
}

// parseHITLCell decodes a HITL cell: exactly TRUE or FALSE (T7).
func parseHITLCell(cell string) (value, ok bool) {
	switch cell {
	case "TRUE":
		return true, true
	case "FALSE":
		return false, true
	}
	return false, false
}
