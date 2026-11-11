package mdtable

import "errors"

// Shape describes where a parsed table sits in its input, line by line, and
// how many cells each data row really had before Parse padded or cut it to the
// header width. All line numbers are 1-based within the parsed input.
type Shape struct {
	HeaderLine int   // line of the header row
	RowLines   []int // line of each data row, parallel to Table.Rows
	RowCells   []int // cells found in each data row (escape-aware), parallel to Table.Rows
	EndLine    int   // first line after the table; len(lines)+1 when the table runs to the end
}

// ParseShaped is Parse that also returns the Shape of the table. Rows in the
// returned Table are padded or cut to the header width exactly as Parse does, so
// a caller that cares about short or long rows compares Shape.RowCells with
// len(Table.Header).
func ParseShaped(data []byte) (Table, Shape, error) {
	t, start, end, err := parseAt(data, 0, false)
	if err != nil {
		return Table{}, Shape{}, err
	}
	lines, offsets := splitLines(data)

	header := -1
	endIdx := len(lines)
	for i, off := range offsets {
		if off == start {
			header = i
		}
		if off == end {
			endIdx = i
		}
	}
	if header < 0 || endIdx < header+2 {
		return Table{}, Shape{}, errors.New("mdtable: table position not found in input")
	}

	shape := Shape{HeaderLine: header + 1, EndLine: endIdx + 1}
	for j := header + 2; j < endIdx; j++ {
		shape.RowLines = append(shape.RowLines, j+1)
		shape.RowCells = append(shape.RowCells, len(parseRow(lines[j])))
	}
	return t, shape, nil
}
