package domain

import "strings"

// IterationValue is a parsed recorded Iteration. The zero value means
// "outside every block".
type IterationValue struct {
	ID    IterationID
	Group GroupName
}

// IsZero reports v.ID == "" && v.Group == "".
func (v IterationValue) IsZero() bool {
	return v.ID == "" && v.Group == ""
}

// Equal compares ID ignoring case and Group exactly.
func (v IterationValue) Equal(o IterationValue) bool {
	return strings.EqualFold(string(v.ID), string(o.ID)) && v.Group == o.Group
}

// IterationValueKind classifies why a recorded value cannot be read.
type IterationValueKind string

const (
	// IterationValueMissing: block row, recorded value "" or "-".
	IterationValueMissing IterationValueKind = "missing"
	// IterationValueOnNonBlockRow: row outside every block, value other than "" or "-".
	IterationValueOnNonBlockRow IterationValueKind = "on-non-block-row"
	// IterationValueMalformed: block row, value in no accepted form for that
	// row (wrong or missing group, extra segments, ID breaking V2).
	IterationValueMalformed IterationValueKind = "malformed"
)

// IterationValueError is a recorded value that cannot be read with its row.
type IterationValueError struct {
	Kind     IterationValueKind
	Recorded string      // the recorded value, verbatim
	Row      WorkflowRow // WorkflowRowFromIndex(row.Index) of the row passed to ReadIterationValue
	Reason   string      // free text; not part of the contract
}

func (e *IterationValueError) Error() string { return "iteration value: " + e.Reason }

// ReadIterationValue reads a recorded Iteration value using the recorded row.
//
//	Row outside every block: "" and "-" give the zero value.
//	Ungrouped block row: "{ID}" verbatim ("Stage-1" is the ID "Stage-1").
//	Grouped block row: "{ID}.{Group}"; Group is always the row's group.
//
// A value is never split without the row; anything else is an
// *IterationValueError.
func ReadIterationValue(recorded string, row RoutingRow) (IterationValue, error) {
	fail := func(kind IterationValueKind, reason string) (IterationValue, error) {
		return IterationValue{}, &IterationValueError{
			Kind:     kind,
			Recorded: recorded,
			Row:      WorkflowRowFromIndex(row.Index),
			Reason:   reason,
		}
	}
	_, inBlock := row.Block()
	none := recorded == "" || recorded == "-"
	if !inBlock {
		if none {
			return IterationValue{}, nil
		}
		return fail(IterationValueOnNonBlockRow, "row is outside every block but the value names an iteration")
	}
	if none {
		return fail(IterationValueMissing, "block row without an iteration value")
	}
	group := row.PhaseParsed.Group
	if group == "" {
		if validIterationID(recorded) {
			return IterationValue{ID: IterationID(recorded)}, nil
		}
		return fail(IterationValueMalformed, "value is not an iteration ID of an ungrouped row")
	}
	if idx := strings.LastIndex(recorded, "."); idx >= 0 &&
		GroupName(recorded[idx+1:]) == group && validIterationID(recorded[:idx]) {
		return IterationValue{ID: IterationID(recorded[:idx]), Group: group}, nil
	}
	return fail(IterationValueMalformed, "value is not an iteration of the row's group")
}

// FormatIterationValue renders the OAF 3.0 value: "{ID}", "{ID}.{Group}", or
// "-" when v.ID is empty.
func FormatIterationValue(v IterationValue) string {
	switch {
	case v.ID == "":
		return "-"
	case v.Group == "":
		return string(v.ID)
	default:
		return string(v.ID) + "." + string(v.Group)
	}
}
