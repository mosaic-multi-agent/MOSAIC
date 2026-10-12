package workflow

import (
	"bytes"
	"errors"
	"strings"

	"mosaic-common/mdtable"
)

// Reserved headings that only 2.16 uses.
const (
	blocksHeading = "**Blocks:**"
	groupsHeading = "**Groups:**"
)

// bracketName returns the name inside "[Name]".
func bracketName(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		return s[1 : len(s)-1], true
	}
	return "", false
}

// reservedTables are the tables the region may hold besides the routing
// table, each located by its reserved heading.
type reservedTables struct {
	blocks *mdtable.Table
	groups *mdtable.Table
	// starts holds the byte offset of every table claimed by a heading.
	starts map[int]bool
}

// headingLine is a reserved heading found in the region.
type headingLine struct {
	text string
	end  int // byte offset just after the heading line
}

// findReservedHeadings returns every line at or after from whose trimmed text
// is one of the reserved headings, in region order.
func findReservedHeadings(content []byte, from int) []headingLine {
	var found []headingLine
	offset := 0
	for _, line := range bytes.Split(content, []byte("\n")) {
		next := offset + len(line) + 1
		trimmed := strings.TrimSpace(string(line))
		if offset >= from && (trimmed == blocksHeading || trimmed == groupsHeading) {
			found = append(found, headingLine{text: trimmed, end: next})
		}
		offset = next
	}
	return found
}

// retiredExecutionGroupsHeading is the 1.x heading of the approach table; the
// parser refuses it and names **Groups:** instead.
const retiredExecutionGroupsHeading = "**Execution Groups:**"

// hasExecutionGroupsHeading reports whether content holds a line whose trimmed
// text is the retired 1.x approach-table heading.
func hasExecutionGroupsHeading(content []byte) bool {
	for _, line := range bytes.Split(content, []byte("\n")) {
		if strings.TrimSpace(string(line)) == retiredExecutionGroupsHeading {
			return true
		}
	}
	return false
}

// locateReservedTables finds the table that follows each reserved heading
// after offset from. A heading must appear once and be followed, past blank
// lines only, by a table (P3).
func locateReservedTables(content []byte, from int, workflowID string) (reservedTables, error) {
	res := reservedTables{starts: map[int]bool{}}
	seen := map[string]bool{}
	for _, h := range findReservedHeadings(content, from) {
		if seen[h.text] {
			return res, tableRefusal(workflowID, "P3", 0, h.text, "reserved heading "+h.text+" appears more than once")
		}
		seen[h.text] = true
		tbl, start, ok := tableDirectlyAfter(content, h.end)
		if !ok {
			return res, tableRefusal(workflowID, "P3", 0, h.text, "reserved heading "+h.text+" is not followed by a table")
		}
		res.starts[start] = true
		if h.text == blocksHeading {
			res.blocks = &tbl
		} else {
			res.groups = &tbl
		}
	}
	return res, nil
}

// tableDirectlyAfter returns the table whose first line is the first
// non-blank line at or after offset.
func tableDirectlyAfter(content []byte, offset int) (mdtable.Table, int, bool) {
	for offset < len(content) {
		end := bytes.IndexByte(content[offset:], '\n')
		lineEnd := len(content)
		if end >= 0 {
			lineEnd = offset + end + 1
		}
		if strings.TrimSpace(string(content[offset:lineEnd])) != "" {
			break
		}
		offset = lineEnd
	}
	if offset >= len(content) {
		return mdtable.Table{}, 0, false
	}
	tbl, start, _, err := mdtable.ParseAt(content, offset)
	if err != nil || start != offset {
		return mdtable.Table{}, 0, false
	}
	return tbl, start, true
}

// checkNoOtherTables refuses (T1) any table after the routing table that is
// not the table of a reserved heading.
func checkNoOtherTables(content []byte, from int, res reservedTables, workflowID string) error {
	offset := from
	for offset < len(content) {
		tbl, start, end, err := mdtable.ParseAt(content, offset)
		if err != nil {
			if errors.Is(err, mdtable.ErrNoTable) {
				return nil
			}
			return tableRefusal(workflowID, "T1", 0, "malformed table",
				"the region holds a table that cannot be read: "+err.Error())
		}
		if !res.starts[start] {
			return tableRefusal(workflowID, "T1", 0, strings.Join(tbl.Header, " | "),
				"besides the routing table the region holds only the Blocks and Groups tables")
		}
		if end <= offset {
			return nil
		}
		offset = end
	}
	return nil
}

// refuseLegacyFillIn refuses {StageNumber} in an Input or Output cell.
func refuseLegacyFillIn(row int, cell, workflowID string) error {
	if strings.Contains(cell, "{"+legacyStageNumber+"}") {
		return tableRefusal(workflowID, "", row, "{"+legacyStageNumber+"}",
			"1.x fill-in {StageNumber} is not WorkflowFormat 2.16 notation")
	}
	return nil
}

// firstTableIsRouting reports whether t is a routing table: it has a Phase or
// a Subagent column.
func firstTableIsRouting(t mdtable.Table) bool {
	return t.Column("Phase") >= 0 || t.Column("Subagent") >= 0
}
