package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/resumecheck"
)

// resumePreflight is what a resume learns before any configuration question:
// the recorded artifact state, the discovered orchestrator file path and the
// infrastructure agents the deployment declares (nil when not determined).
type resumePreflight struct {
	state       domain.ArtifactState
	orchPath    string
	infraAgents []domain.DeclaredInfraAgent
}

// runResumePreflight reads the run artifact and applies both early refusal
// tiers. It writes the refusal to errOut and reports a non-ok result with the
// exit code to end with. The session keeps its own run-start checks as the
// final guard, so a deployment that cannot be enumerated here is left to it.
func runResumePreflight(runID, runFolder, harnessFlag string, infraFilter []string, errOut io.Writer) (pre resumePreflight, exitCode int, ok bool) {
	data, readErr := os.ReadFile(filepath.Join(runFolder, "Orchestration.md"))
	if readErr == nil {
		pre.state, readErr = artifact.Parse(data)
	}
	if err := resumecheck.CheckArtifact(runID, pre.state, readErr); err != nil {
		return pre, reportEarlyRefusal(err, errOut), false
	}

	orchPath, code, ok := discoverOrchestratorPath(harnessFlag, errOut)
	if !ok {
		return pre, code, false
	}
	pre.orchPath = orchPath
	if orchPath == "" {
		return pre, ExitSuccess, true
	}

	deployment, err := readDeployment(orchPath, infraFilter)
	if err != nil {
		return pre, ExitSuccess, true
	}
	pre.infraAgents = deployment.InfraAgents
	if err := resumecheck.CheckDeployment(runID, pre.state, deployment); err != nil {
		return pre, reportEarlyRefusal(err, errOut), false
	}
	return pre, ExitSuccess, true
}

// reportEarlyRefusal writes err to errOut and returns the exit code for it: a
// refusal ends as ExitRefused, any other failure as ExitFailure.
func reportEarlyRefusal(err error, errOut io.Writer) int {
	fmt.Fprintf(errOut, "error: %v\n", err)
	var refusal *resumecheck.Refusal
	if errors.As(err, &refusal) {
		return ExitRefused
	}
	return ExitFailure
}

// readDeployment enumerates what the orchestrator file declares. A non-nil
// infraFilter keeps only the named infrastructure agents, as the session does.
func readDeployment(orchPath string, infraFilter []string) (resumecheck.Deployment, error) {
	workflows, err := orchfile.EnumerateWorkflows(orchPath)
	if err != nil {
		return resumecheck.Deployment{}, err
	}
	agents, err := orchfile.EnumerateInfrastructureAgents(orchPath)
	if err != nil {
		return resumecheck.Deployment{}, err
	}
	if infraFilter != nil {
		agents = filterInfraAgents(agents, infraFilter)
	}
	return resumecheck.Deployment{Workflows: workflows, InfraAgents: agents}, nil
}

func filterInfraAgents(agents []domain.DeclaredInfraAgent, keep []string) []domain.DeclaredInfraAgent {
	keepSet := make(map[string]bool, len(keep))
	for _, name := range keep {
		keepSet[name] = true
	}
	filtered := make([]domain.DeclaredInfraAgent, 0, len(agents))
	for _, a := range agents {
		if keepSet[a.Name] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// printResumeSummary writes the recorded-settings summary. A run that records
// no mode is adopted from the supplied values, so those are shown instead.
func printResumeSummary(out io.Writer, pre resumePreflight, adopted bool, adoptedSettings domain.RunSettings) {
	state := pre.state
	if adopted {
		state.Mode = adoptedSettings.Mode
		state.PreConsultation = adoptedSettings.PreConsultation
		state.ManualResolution = adoptedSettings.ManualResolution
	}
	for _, line := range resumecheck.NewSummary(state, pre.infraAgents).Lines() {
		fmt.Fprintln(out, line)
	}
}
