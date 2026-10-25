package cli

// deploy_failure.go declares the failure envelope the deploy subcommand writes to stdout
// under --output json when the run fails with app.ErrUnresolvedModels, and the deploy
// output mapping that produces it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// DeployFailure has the same field set and JSON keys as RenderFailure.
type DeployFailure struct {
	Error    bool   `json:"error"`            // always true
	Reason   string `json:"reason"`           // stable code; "unresolved-models"
	Message  string `json:"message"`          // same text as the stderr line, without "error: "
	Detail   string `json:"detail,omitempty"` // "<key> (tier <tier>)" entries joined by ", "
	ExitCode int    `json:"exitCode"`         // 1
}

// DeployReasonUnresolvedModels is the stable reason code for the strict-models failure.
const DeployReasonUnresolvedModels = "unresolved-models"

// renderDeployOutput renders a deploy result. The strict-models failure additionally writes
// a DeployFailure envelope under --output json; every other result goes through renderOutput.
func renderDeployOutput(out, errOut io.Writer, format string, summary domain.RunSummary, svcErr error) int {
	var ume *app.UnresolvedModelsError
	if !errors.As(svcErr, &ume) {
		return renderOutput(out, errOut, format, summary, svcErr)
	}
	fmt.Fprintf(errOut, "error: %v\n", svcErr)
	if strings.EqualFold(format, "json") {
		env := DeployFailure{
			Error:    true,
			Reason:   DeployReasonUnresolvedModels,
			Message:  svcErr.Error(),
			Detail:   ume.Detail(),
			ExitCode: ExitFailure,
		}
		_ = json.NewEncoder(out).Encode(env)
	}
	return ExitFailure
}
