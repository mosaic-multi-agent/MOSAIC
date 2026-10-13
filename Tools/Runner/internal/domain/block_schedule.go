package domain

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"
)

// IterationID is a block-schedule ID: letters, digits, '-', starting with a
// letter or digit. Stored verbatim; compared ignoring case.
type IterationID string

// Equal reports whether the two IDs are equal ignoring case.
func (id IterationID) Equal(other IterationID) bool {
	return strings.EqualFold(string(id), string(other))
}

// ScheduleIteration is one row of a block schedule.
type ScheduleIteration struct {
	ID        IterationID
	HITL      bool
	DependsOn []IterationID // trimmed elements of the cell, verbatim, in cell order; nil = none
	Approach  Approach      // "" for an ungrouped block
}

// BlockSchedule is the schedule of one block, iterations in schedule order.
type BlockSchedule struct {
	Block      BlockName
	Source     string
	Section    string
	Iterations []ScheduleIteration
}

// Find returns the iteration whose ID equals id ignoring case (ID verbatim).
func (s BlockSchedule) Find(id IterationID) (ScheduleIteration, bool) {
	for _, it := range s.Iterations {
		if it.ID.Equal(id) {
			return it, true
		}
	}
	return ScheduleIteration{}, false
}

// IDs returns every ID in schedule order.
func (s BlockSchedule) IDs() []IterationID {
	ids := make([]IterationID, 0, len(s.Iterations))
	for _, it := range s.Iterations {
		ids = append(ids, it.ID)
	}
	return ids
}

// ScheduleDefectError reports a defective block schedule.
type ScheduleDefectError struct {
	Rule    string // "V1".."V6"; "V1" for a missing source; "" for an unreadable file
	Block   BlockName
	Source  string // declared Source
	Section string
	Row     int    // 1-based data row of the schedule table; 0 when not about a row
	Token   string // offending ID, cell, column header, "## "+Section or Source
	Detail  string // specific condition; not part of the contract
	Cause   error  // I/O cause for missing or unreadable sources; nil otherwise
}

// Error names the rule, block, source, section, row and token. The rule label
// is "missing" for a missing source, "unreadable" for any other I/O cause and
// the rule ID otherwise.
func (e *ScheduleDefectError) Error() string {
	label := e.Rule
	switch {
	case e.Cause == nil:
	case errors.Is(e.Cause, fs.ErrNotExist):
		label = "missing"
	default:
		label = "unreadable"
	}

	var b strings.Builder
	b.WriteString("block schedule defect (")
	b.WriteString(label)
	b.WriteString("): block ")
	b.WriteString(string(e.Block))
	b.WriteString(", source ")
	b.WriteString(e.Source)
	b.WriteString(", section ")
	b.WriteString(e.Section)
	if e.Row > 0 {
		b.WriteString(", row ")
		b.WriteString(strconv.Itoa(e.Row))
	}
	b.WriteString(", token ")
	b.WriteString(strconv.Quote(e.Token))
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	if e.Cause != nil && label == "unreadable" {
		b.WriteString(": ")
		b.WriteString(e.Cause.Error())
	}
	return b.String()
}

// Unwrap returns the I/O cause, if any.
func (e *ScheduleDefectError) Unwrap() error { return e.Cause }
