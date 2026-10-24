package agentdeploy

import "encoding/json"

// ReasonUnresolvedModels is the deployment tool's reason code for a
// strict-models failure, passed through unchanged in DeployError.Reason.
const ReasonUnresolvedModels = "unresolved-models"

// wireFailureEnvelope is the stdout document the deployment tool writes under
// --output json when a deploy fails with a structured reason.
type wireFailureEnvelope struct {
	Error   bool   `json:"error"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

// decodeFailureEnvelope returns the failure envelope in stdout, or false when
// stdout is not one.
func decodeFailureEnvelope(stdout []byte) (wireFailureEnvelope, bool) {
	var env wireFailureEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil || !env.Error || env.Reason == "" {
		return wireFailureEnvelope{}, false
	}
	return env, true
}

// newDeployFailure builds the DeployError for a non-zero exit: the tool's
// failure envelope when stdout carries one (so the message names the agent and
// tier), otherwise the undifferentiated stderr-based failure.
func newDeployFailure(stdout, stderr []byte, exitCode int) *DeployError {
	if env, ok := decodeFailureEnvelope(stdout); ok {
		return &DeployError{
			Reason:      env.Reason,
			Detail:      env.Detail,
			ToolMessage: env.Message,
			ExitCode:    exitCode,
		}
	}
	return &DeployError{ToolMessage: string(stderr), ExitCode: exitCode}
}
