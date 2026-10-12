package planstages

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

// ScheduleRequest names one block schedule to read.
type ScheduleRequest struct {
	Path       string            // absolute path of the source file to read
	Source     string            // declared Source as written in **Blocks:** (named in defects)
	Section    string            // heading text after "## "
	Block      domain.BlockName  // the block; the table's first column must be headed by it
	Grouped    bool              // true when the block's rows carry groups (V3, V6 apply)
	Approaches []domain.Approach // approach tokens the approach table declares for this block (V6)
}

// scheduleColumns holds the column indices of a schedule table; -1 = absent.
type scheduleColumns struct {
	depends, hitl, approach int
}

// ReadBlockSchedule reads a WorkflowFormat 2.16 block schedule (V1-V6).
// Iterations are returned in schedule order with IDs and Depends On IDs
// verbatim (cells trimmed). An empty table is a valid empty schedule.
// Every defect, including a missing or unreadable file, is returned as
// *domain.ScheduleDefectError; a missing file unwraps to fs.ErrNotExist.
func ReadBlockSchedule(req ScheduleRequest) (domain.BlockSchedule, error) {
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return domain.BlockSchedule{}, readDefect(req, err)
	}

	tbl, derr := locateScheduleTable(req, data)
	if derr != nil {
		return domain.BlockSchedule{}, derr
	}

	cols := scheduleColumns{
		depends:  tbl.Column("Depends On"),
		hitl:     tbl.Column("HITL"),
		approach: tbl.Column("Approach"),
	}
	if req.Grouped && cols.approach == -1 {
		return domain.BlockSchedule{}, scheduleDefect(req, "V3", 0, "Approach",
			`grouped block schedule has no "Approach" column`)
	}

	iterations, derr := readIterations(req, tbl, cols)
	if derr != nil {
		return domain.BlockSchedule{}, derr
	}
	return domain.BlockSchedule{
		Block:      req.Block,
		Source:     req.Source,
		Section:    req.Section,
		Iterations: iterations,
	}, nil
}

// readDefect classifies a file read failure as a missing or unreadable source.
func readDefect(req ScheduleRequest, err error) *domain.ScheduleDefectError {
	de := scheduleDefect(req, "", 0, req.Source, "source file cannot be read")
	de.Cause = err
	if errors.Is(err, fs.ErrNotExist) {
		de.Rule = "V1"
		de.Detail = "source file does not exist"
	}
	return de
}

func scheduleDefect(req ScheduleRequest, rule string, row int, token, detail string) *domain.ScheduleDefectError {
	return &domain.ScheduleDefectError{
		Rule:    rule,
		Block:   req.Block,
		Source:  req.Source,
		Section: req.Section,
		Row:     row,
		Token:   token,
		Detail:  detail,
	}
}

// locateScheduleTable finds the one matching section heading and the table
// that follows it before the next level-1 or level-2 heading (V1).
func locateScheduleTable(req ScheduleRequest, data []byte) (mdtable.Table, *domain.ScheduleDefectError) {
	heading := "## " + req.Section
	lines := bytes.Split(data, []byte("\n"))

	// Byte offset of the line after the heading, and of the next level-1/2 heading.
	headingEnd, matches := -1, 0
	offset := 0
	for _, raw := range lines {
		if strings.TrimSpace(string(raw)) == heading {
			matches++
			if headingEnd < 0 {
				headingEnd = offset + len(raw) + 1
			}
		}
		offset += len(raw) + 1
	}
	if matches == 0 {
		return mdtable.Table{}, scheduleDefect(req, "V1", 0, heading, "no matching section heading")
	}
	if matches > 1 {
		return mdtable.Table{}, scheduleDefect(req, "V1", 0, heading, "section heading appears more than once")
	}

	searchEnd := nextTopHeading(lines, headingEnd, len(data))
	if headingEnd > searchEnd {
		headingEnd = searchEnd
	}
	tbl, _, _, err := mdtable.ParseAt(data[:searchEnd], headingEnd)
	if err != nil {
		return mdtable.Table{}, scheduleDefect(req, "V1", 0, heading, "no schedule table in the section")
	}
	if tbl.Header[0] != string(req.Block) {
		return mdtable.Table{}, scheduleDefect(req, "V1", 0, tbl.Header[0],
			"first column is not headed by the block name")
	}
	return tbl, nil
}

// nextTopHeading returns the byte offset of the first level-1 or level-2
// heading line starting at or after from, or end when there is none.
func nextTopHeading(lines [][]byte, from, end int) int {
	offset := 0
	for _, raw := range lines {
		if offset >= from {
			t := strings.TrimSpace(string(raw))
			if strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") {
				return offset
			}
		}
		offset += len(raw) + 1
	}
	return end
}

// readIterations validates and converts the table rows, top to bottom. Within
// a row the cells are checked in the order ID, Depends On, HITL, Approach.
func readIterations(req ScheduleRequest, tbl mdtable.Table, cols scheduleColumns) ([]domain.ScheduleIteration, *domain.ScheduleDefectError) {
	earlier := make(map[string]bool, len(tbl.Rows))
	iterations := make([]domain.ScheduleIteration, 0, len(tbl.Rows))

	for i, row := range tbl.Rows {
		n := i + 1
		id := strings.TrimSpace(row[0])
		if !validID(id) {
			return nil, scheduleDefect(req, "V2", n, id,
				"ID must contain only letters, digits and '-' and start with a letter or digit")
		}
		if earlier[asciiLower(id)] {
			return nil, scheduleDefect(req, "V2", n, id, "ID repeats an earlier row's ID (compared ignoring case)")
		}

		it := domain.ScheduleIteration{ID: domain.IterationID(id)}
		if cols.depends != -1 {
			deps, derr := parseDependsOn(req, n, row[cols.depends], earlier)
			if derr != nil {
				return nil, derr
			}
			it.DependsOn = deps
		}
		if cols.hitl != -1 {
			cell := strings.TrimSpace(row[cols.hitl])
			if cell != "TRUE" && cell != "FALSE" {
				return nil, scheduleDefect(req, "V5", n, cell, "HITL must be exactly TRUE or FALSE")
			}
			it.HITL = cell == "TRUE"
		}
		if req.Grouped {
			approach, derr := checkApproach(req, n, row[cols.approach])
			if derr != nil {
				return nil, derr
			}
			it.Approach = approach
		}

		earlier[asciiLower(id)] = true
		iterations = append(iterations, it)
	}
	return iterations, nil
}

// parseDependsOn reads one Depends On cell against the IDs of earlier rows (V4).
func parseDependsOn(req ScheduleRequest, row int, raw string, earlier map[string]bool) ([]domain.IterationID, *domain.ScheduleDefectError) {
	cell := strings.TrimSpace(raw)
	if cell == "-" {
		return nil, nil
	}
	if cell == "" {
		return nil, scheduleDefect(req, "V4", row, "", "Depends On cell is empty; write '-' for none")
	}

	seen := make(map[string]bool)
	var deps []domain.IterationID
	for _, part := range strings.Split(cell, ",") {
		el := strings.TrimSpace(part)
		switch {
		case el == "":
			return nil, scheduleDefect(req, "V4", row, cell, "Depends On cell has an empty entry")
		case !earlier[asciiLower(el)]:
			return nil, scheduleDefect(req, "V4", row, el, "Depends On entry is not the ID of an earlier row")
		case seen[asciiLower(el)]:
			return nil, scheduleDefect(req, "V4", row, el, "Depends On entry is repeated")
		}
		seen[asciiLower(el)] = true
		deps = append(deps, domain.IterationID(el))
	}
	return deps, nil
}

// checkApproach validates one grouped-block Approach cell (V3, then V6).
func checkApproach(req ScheduleRequest, row int, raw string) (domain.Approach, *domain.ScheduleDefectError) {
	cell := strings.TrimSpace(raw)
	if cell == "" || cell == "-" {
		return "", scheduleDefect(req, "V3", row, cell, "grouped block row has no Approach")
	}
	for _, a := range req.Approaches {
		if string(a) == cell {
			return a, nil
		}
	}
	return "", scheduleDefect(req, "V6", row, cell, "Approach is not declared for the block")
}

// validID reports whether id is non-empty, made of ASCII letters, digits and
// '-', and does not start with '-'.
func validID(id string) bool {
	if id == "" || id[0] == '-' {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		isAlnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !isAlnum && c != '-' {
			return false
		}
	}
	return true
}

// asciiLower lower-cases ASCII letters only, so that non-ASCII characters never
// fold onto an ID character.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
