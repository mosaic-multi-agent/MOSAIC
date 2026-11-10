package domain

import "strings"

// HarnessFailureKind names the root cause of a HARNESS_ERROR.
type HarnessFailureKind string

const (
	HarnessFailureTimeout             HarnessFailureKind = "timeout"
	HarnessFailureProcess             HarnessFailureKind = "process-failure"
	HarnessFailureUnparseable         HarnessFailureKind = "unparseable-response"
	HarnessFailureEmpty               HarnessFailureKind = "empty-response"
	HarnessFailureGHCPAgentDeselected HarnessFailureKind = "ghcp-agent-deselected"
	HarnessFailureGHCPSessionError    HarnessFailureKind = "ghcp-session-error"
	HarnessFailureStreamError         HarnessFailureKind = "stream-error"
	HarnessFailureOther               HarnessFailureKind = "other"
)

// HarnessFailure is one classified harness-level failure.
type HarnessFailure struct {
	Kind   HarnessFailureKind
	Detail string // may contain line breaks
}

// Summary returns the failure kind followed by its detail, always on a single
// line: every line break in Detail is replaced by a space.
func (f HarnessFailure) Summary() string {
	if f.Detail == "" {
		return string(f.Kind)
	}
	detail := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(f.Detail)
	return string(f.Kind) + ": " + detail
}

// HarnessFailureResponse is the response recorded for a harness-level
// failure: StatusCode HARNESS_ERROR, ErrorCode ErrorNone, StatusMessage and
// ErrorReason both f.Summary(), AgentInstanceID and RunID as given.
func HarnessFailureResponse(agentInstanceID, runID string, f HarnessFailure) ProtocolResponse {
	summary := f.Summary()
	return ProtocolResponse{
		AgentInstanceID: agentInstanceID,
		RunID:           runID,
		StatusCode:      StatusHARNESS_ERROR,
		StatusMessage:   summary,
		ErrorCode:       ErrorNone,
		ErrorReason:     summary,
	}
}

// HarnessErrorResponse builds the synthetic response recorded for a
// harness-level failure at the sites that still record the old shape:
// StatusCode BLOCKED, ErrorCode E501, StatusMessage
// and ErrorReason both err.Error(), AgentInstanceID and RunID as given.
func HarnessErrorResponse(agentInstanceID, runID string, err error) ProtocolResponse {
	msg := err.Error()
	return ProtocolResponse{
		AgentInstanceID: agentInstanceID,
		RunID:           runID,
		StatusCode:      "BLOCKED",
		StatusMessage:   msg,
		ErrorCode:       "E501",
		ErrorReason:     msg,
	}
}

// LastErrorReasonFor returns the last_error_reason value for a consultation
// triggered by resp: nil when resp is nil or its StatusCode is neither BLOCKED nor HARNESS_ERROR;
// otherwise a pointer to resp.ErrorReason verbatim (possibly "").
func LastErrorReasonFor(resp *ProtocolResponse) *string {
	if resp == nil || (resp.StatusCode != StatusBLOCKED && resp.StatusCode != StatusHARNESS_ERROR) {
		return nil
	}
	reason := resp.ErrorReason
	return &reason
}

// ConfigErrorResponse is the response recorded for an infrastructure agent
// whose definition cannot be resolved: StatusCode CONFIG_ERROR, ErrorCode
// ErrorNone, StatusMessage and ErrorReason both
// "agent definition not resolved: " + err.Error().
func ConfigErrorResponse(agentInstanceID, runID string, err error) ProtocolResponse {
	msg := "agent definition not resolved: " + err.Error()
	return ProtocolResponse{
		AgentInstanceID: agentInstanceID,
		RunID:           runID,
		StatusCode:      StatusCONFIG_ERROR,
		StatusMessage:   msg,
		ErrorCode:       ErrorNone,
		ErrorReason:     msg,
	}
}
