package deviation

import (
	"encoding/json"
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// wirePreConsultProbe reads only the members that decide whether a
// pre-consultation reply is a stop. Raw values keep an absent member apart from
// null and from a value of the wrong JSON type.
type wirePreConsultProbe struct {
	Action json.RawMessage `json:"action"`
	Reason json.RawMessage `json:"reason"`
}

// parsePreConsultReply turns the extracted JSON object of a pre-consultation
// reply into advice or a stop.
//
//   - No "action" member: advice (task_description / constraints); "reason" and
//     unknown members are ignored.
//   - "action": "stop" with a non-blank string reason: a stop. Advice members
//     next to it are not decoded, so the stop wins.
//   - Any other "action" member: a failure, because pre-consultation allows
//     only a stop.
func parsePreConsultReply(extracted []byte) (domain.PreConsultationAdvice, error) {
	var probe wirePreConsultProbe
	if err := json.Unmarshal(extracted, &probe); err != nil {
		return domain.PreConsultationAdvice{}, malformedPreConsult(err)
	}

	if len(probe.Action) == 0 {
		var resp wirePreConsultResponse
		if err := json.Unmarshal(extracted, &resp); err != nil {
			return domain.PreConsultationAdvice{}, malformedPreConsult(err)
		}
		return domain.PreConsultationAdvice{
			TaskDescription: resp.TaskDescription,
			Constraints:     resp.Constraints,
		}, nil
	}

	var action string
	if err := json.Unmarshal(probe.Action, &action); err != nil || string(probe.Action) == "null" {
		return domain.PreConsultationAdvice{}, &domain.ConsultationError{
			Failure: domain.ConsultFailMalformedJSON,
			Detail:  fmt.Sprintf("pre-consultation action must be the string \"stop\", got: %s", string(probe.Action)),
		}
	}
	if action != "stop" {
		return domain.PreConsultationAdvice{}, &domain.ConsultationError{
			Failure: domain.ConsultFailUnknownAction,
			Detail:  fmt.Sprintf("pre-consultation action %q is not allowed (only \"stop\")", action),
		}
	}

	return preConsultStop(probe.Reason)
}

// preConsultStop validates the reason of a stop reply and builds the advice.
func preConsultStop(rawReason json.RawMessage) (domain.PreConsultationAdvice, error) {
	missing := &domain.ConsultationError{
		Failure: domain.ConsultFailMissingField,
		Detail:  "missing required field: reason (pre-consultation stop)",
	}
	if len(rawReason) == 0 || string(rawReason) == "null" {
		return domain.PreConsultationAdvice{}, missing
	}
	var reason string
	if err := json.Unmarshal(rawReason, &reason); err != nil {
		return domain.PreConsultationAdvice{}, &domain.ConsultationError{
			Failure: domain.ConsultFailMalformedJSON,
			Detail:  fmt.Sprintf("pre-consultation stop reason must be a string, got: %s", string(rawReason)),
			Err:     err,
		}
	}
	if strings.TrimSpace(reason) == "" {
		return domain.PreConsultationAdvice{}, missing
	}
	return domain.PreConsultationAdvice{Stop: &domain.StopInstruction{Reason: reason}}, nil
}

func malformedPreConsult(err error) error {
	return &domain.ConsultationError{
		Failure: domain.ConsultFailMalformedJSON,
		Detail:  fmt.Sprintf("malformed pre-consultation response JSON: %v", err),
		Err:     err,
	}
}
