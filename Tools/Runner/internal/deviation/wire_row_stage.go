package deviation

import (
	"encoding/json"
	"fmt"
	"strconv"

	"mosaic-run/internal/domain"
)

// Wire shape of the workflow position a dispatch reply names:
//
//	"row":   1-based row number of the routing table (the Row column of the
//	         workflow table and the WorkflowRow column of the Execution Log).
//	         Required for every dispatch. A JSON integer or a decimal string.
//	"iteration": the run artifact's Iteration value for that row, a string
//	         ("A", "4", "1.Test"). Required exactly for a row in a block. It is
//	         split by the row's group in domain.ValidateDispatchIteration.
//	"stage": the pre-3.0 member; refused unless null.
//
// Absent and null both mean "not given". A value of the wrong JSON type is a
// malformed reply.

// coerceRowField decodes the "row" member. Absent or null yields NoWorkflowRow.
func coerceRowField(raw json.RawMessage) (domain.WorkflowRow, error) {
	n, ok, err := coerceIntegerField("row", raw)
	if err != nil || !ok {
		return domain.NoWorkflowRow, err
	}
	return domain.WorkflowRow(n), nil
}

// coerceIterationField decodes the "iteration" member: a JSON string holding
// the run artifact's Iteration value. Absent, null and "" yield "". The value
// is split and checked later, with the row's block and group known.
func coerceIterationField(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	if raw[0] != '"' {
		return "", malformedField("iteration", fmt.Sprintf("must be a string, got: %s", string(raw)))
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", malformedField("iteration", err.Error())
	}
	return s, nil
}

// refuseStageField rejects the pre-3.0 "stage" member. A null value is not
// given and passes.
func refuseStageField(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return malformedField("stage", "is no longer accepted; name the iteration in the \"iteration\" member")
}

// coerceIntegerField decodes a JSON integer or a decimal string. ok is false
// for an absent or null value.
func coerceIntegerField(fieldName string, raw json.RawMessage) (n int, ok bool, err error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false, nil
	}
	text := string(raw)
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, false, malformedField(fieldName, err.Error())
		}
		text = s
	}
	v, convErr := strconv.Atoi(text)
	if convErr != nil {
		return 0, false, malformedField(fieldName, fmt.Sprintf("must be an integer, got: %s", string(raw)))
	}
	return v, true, nil
}

func malformedField(fieldName, detail string) error {
	return &domain.ConsultationError{
		Failure: domain.ConsultFailMalformedJSON,
		Detail:  fmt.Sprintf("malformed routing response: %s: %s", fieldName, detail),
	}
}
