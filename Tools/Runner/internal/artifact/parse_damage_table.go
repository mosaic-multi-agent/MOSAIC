package artifact

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"mosaic-common/mdtable"
)

// sectionTable is the table of one tagged section, read together with the
// facts the strict checks need: the file line of the header and of each row.
type sectionTable struct {
	table      mdtable.Table
	headerLine int   // file line of the header row
	rowLines   []int // file line of each data row, parallel to table.Rows
	cells      []int // cells each data row really has, parallel to table.Rows
}

// readSectionTable reads the table of a closed section and reports every
// structural problem it finds as damage of that section:
//   - no table, or a header whose column count differs from the separator's;
//   - a data row with fewer or more cells than the header;
//   - a non-blank line between the last row and the closing tag (the table ends
//     there, and nothing after that line is read as a row).
//
// An empty section is a table with no rows. The result is usable only when ok.
func readSectionTable(sc sectionScan, d *damage) (st sectionTable, ok bool) {
	if len(bytes.TrimSpace(sc.content)) == 0 {
		return st, true
	}
	t, shape, err := mdtable.ParseShaped(sc.content)
	if err != nil {
		problem := err.Error()
		if errors.Is(err, mdtable.ErrNoTable) {
			problem = "no table found"
		}
		d.addBody(sc.name, sc.fileLine(1), problem)
		return st, false
	}

	st.table = t
	st.headerLine = sc.fileLine(shape.HeaderLine)
	for i, n := range shape.RowCells {
		line := sc.fileLine(shape.RowLines[i])
		st.rowLines = append(st.rowLines, line)
		st.cells = append(st.cells, n)
		if n != len(t.Header) {
			d.addBody(sc.name, line, "row has "+strconv.Itoa(n)+" cells, the header has "+strconv.Itoa(len(t.Header)))
		}
	}
	if stray := sc.fileLine(shape.EndLine); stray != 0 {
		d.addBody(sc.name, stray, "line is not a table row; the table ends before the closing tag")
	}
	return st, true
}

// requireColumns reports a damage problem at the header line when the table
// lacks one of the named columns.
func (st sectionTable) requireColumns(section string, d *damage, columns ...string) bool {
	if len(st.table.Header) == 0 {
		return true // an empty section has no header to check
	}
	var missing []string
	for _, c := range columns {
		if st.table.Column(c) < 0 {
			missing = append(missing, c)
		}
	}
	if len(missing) == 0 {
		return true
	}
	d.addBody(section, st.headerLine, "table header lacks the column(s) "+strings.Join(missing, ", "))
	return false
}
