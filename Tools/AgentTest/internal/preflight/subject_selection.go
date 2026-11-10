package preflight

import (
	"context"
	"errors"

	"mosaic-agent-test/internal/agentdeploy"
	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/domain"
)

// checkSubjectSelected rejects a run that has no explicit subject model
// selection. Without one, the harness default would stand in for the model
// under test.
func checkSubjectSelected(report *authoring.Report, o Overrides) {
	if o.SubjectModel != "" {
		return
	}
	report.Add(authoring.Diagnostic{
		Severity: authoring.SeverityError,
		Code:     DiagMissingSubjectModel,
		Message: "no subject model selected: pass --subject-model on the command line or choose one " +
			"on the TUI model screen; the run never falls back to the harness default model",
	})
}

// modelSelectionOf is the run's model selection as preflight received it.
func modelSelectionOf(o Overrides) domain.ModelSelection {
	return domain.ModelSelection{Subject: o.SubjectModel, Stub: o.StubModel}
}

// addStrictDeployFailure reports a strict unresolved-model failure from the
// dry-run deploy. It returns false when err is any other failure.
func addStrictDeployFailure(report *authoring.Report, defPath string, err error) bool {
	var de *agentdeploy.DeployError
	if !errors.As(err, &de) || de.Reason != agentdeploy.ReasonUnresolvedModels {
		return false
	}
	report.Add(authoring.Diagnostic{
		Severity: authoring.SeverityError,
		Code:     DiagUnresolvedTierModel,
		Path:     defPath,
		Pointer:  "subject",
		Message:  "subject declaration dry-run deploy left an agent without a model: " + de.ToolMessage,
	})
	return true
}

// dryRunDeploy performs the preflight dry-run deploy with the tier-to-model
// map of the user's selection, the same one the setup deploy sends.
func dryRunDeploy(in Input, def domain.TestDefinition) error {
	_, err := in.Deploy.Deploy(context.Background(), domain.DeployRequest{
		HarnessID:              in.HarnessID,
		WorkspaceRoot:          in.DeployScratchRoot,
		Workflows:              def.Subject.Workflows,
		InfrastructureAgentIDs: def.Subject.InfrastructureAgentIDs,
		TierModels:             modelSelectionOf(in.Overrides).TierModels(),
		DryRun:                 true,
	})
	return err
}
