package artifact

import (
	"strconv"
	"strings"
	"unicode"
)

// checkLogRows checks the Seq and Agent cell of every Execution Log row and
// reports each violation as damage at the row's line:
//   - Seq is a decimal integer, strictly greater than the previous row's Seq
//     (a gap is fine: consumed-but-unlogged sequence numbers are legitimate);
//   - Agent is "{AgentName}#{Seq}" with the row's own Seq as the numeric suffix.
//
// A row that is too short is already reported; its cells are checked as far as
// it has them, so its Seq still counts as the previous Seq of the next row.
func checkLogRows(sc sectionScan, st sectionTable, d *damage) {
	seqCol := st.table.Column("Seq")
	agentCol := st.table.Column("Agent")
	prev, havePrev := 0, false
	for i, row := range st.table.Rows {
		line := st.rowLines[i]
		seq, seqOK := 0, false
		if seqCol < st.cells[i] {
			seqCell := cellText(row[seqCol])
			if n, ok := decimalInt(seqCell); ok {
				seq, seqOK = n, true
				if havePrev && n <= prev {
					d.addBody(sc.name, line, "Seq "+seqCell+" is not above the previous row's Seq "+strconv.Itoa(prev))
				}
				prev, havePrev = n, true
			} else {
				d.addBody(sc.name, line, "Seq "+strconv.Quote(seqCell)+" is not an integer")
			}
		}
		if agentCol < st.cells[i] {
			if problem := agentProblem(cellText(row[agentCol]), seq, seqOK); problem != "" {
				d.addBody(sc.name, line, problem)
			}
		}
	}
}

// decimalInt reads a non-empty run of ASCII digits as an integer.
func decimalInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// agentProblem describes why agent is not an agent instance id of the row whose
// Seq is seq ("{AgentName}#{Seq}"); "" when it is one. The suffix is compared to
// the row's Seq only when that Seq could be read.
func agentProblem(agent string, seq int, seqOK bool) string {
	quoted := strconv.Quote(agent)
	hash := strings.LastIndex(agent, "#")
	if hash < 0 {
		return "Agent " + quoted + " is not an agent instance id (AgentName#Seq)"
	}
	name, suffix := agent[:hash], agent[hash+1:]
	if name == "" || strings.Contains(name, "#") || strings.ContainsFunc(name, unicode.IsSpace) {
		return "Agent " + quoted + " is not an agent instance id (AgentName#Seq)"
	}
	if n, ok := decimalInt(suffix); !ok || strconv.Itoa(n) != suffix {
		return "Agent " + quoted + " has no numeric Seq suffix"
	}
	if seqOK && suffix != strconv.Itoa(seq) {
		return "Agent " + quoted + " does not end in the row's own Seq " + strconv.Itoa(seq)
	}
	return ""
}
