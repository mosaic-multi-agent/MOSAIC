package tui

import (
	"errors"
	"os"
	"path/filepath"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/resumecheck"
	"mosaic-run/internal/runselect"
	"mosaic-run/internal/tui/screens/runflow"
)

// readRunState reads and parses the chosen run's artifact. A new run, and a run
// with no folder, has none to read.
func (m *rootModel) readRunState() (domain.ArtifactState, error) {
	if m.selections.isNewRun || m.selections.runFolder == "" {
		return domain.ArtifactState{}, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(m.selections.runFolder, "Orchestration.md"))
	if err != nil {
		return domain.ArtifactState{}, err
	}
	return artifact.Parse(data)
}

// recordedWorkflowVersion returns the workflow version recorded in the run's
// artifact frontmatter, or the empty version when there is nothing to read: a
// new run has no prior artifact, and a resumed run whose artifact is missing,
// unreadable, or unparseable has no recorded version to compare against.
func (m *rootModel) recordedWorkflowVersion() domain.WorkflowVersion {
	state, err := m.readRunState()
	if err != nil {
		return ""
	}
	return state.WorkflowVersion
}

// selectedWorkflowVersion returns the version declared by the workflow region
// the user selected, or the empty version when the orchestrator file declares
// none for it.
func (m *rootModel) selectedWorkflowVersion() domain.WorkflowVersion {
	for _, wf := range m.workflows {
		if wf.Info.ID == m.selections.workflowID {
			return wf.Info.Version
		}
	}
	return ""
}

// announceIdentity builds the runselect.Identity used to render the
// chosen-run announcement from the currently selected run. For a resumed run it
// attempts to read the recorded position from the run's artifact; a read or
// parse failure simply leaves Position nil, which runselect.Announce handles
// without panicking.
func (m *rootModel) announceIdentity() runselect.Identity {
	id := runselect.Identity{
		RunID:     m.selections.runID,
		RunFolder: m.selections.runFolder,
		IsNewRun:  m.selections.isNewRun,
	}
	state, err := m.readRunState()
	if err != nil {
		return id
	}
	id.Position = &runselect.Position{
		Phase:       state.CurrentState.Phase,
		Stage:       state.CurrentState.Stage,
		LastAgent:   state.CurrentState.LastAgent,
		LastUpdated: state.LastUpdated,
	}
	return id
}

// needsRunnerAdoption reports whether the resumed run's artifact records no
// runner settings (a native-created artifact), so the configuration wizard has
// to ask for them once. A new run, and an artifact that cannot be read or
// parsed, never needs adoption here; the session refuses the latter.
func (m *rootModel) needsRunnerAdoption() bool {
	state, err := m.readRunState()
	return err == nil && state.Mode == domain.ExecutionModeUnset
}

// commitSetupPending reports whether the resumed run enables commits but
// records no commit branch, so the wizard must ask for the branch variant
// explicitly. A new run, and an artifact that cannot be read or parsed, never
// has a pending setup here; the session refuses the latter.
func (m *rootModel) commitSetupPending() bool {
	state, err := m.readRunState()
	return err == nil && state.Commits && state.CommitBranch == ""
}

// artifactRefusal applies the artifact-only early refusal checks to the chosen
// run. It returns nil for a new run and for a run that may be resumed.
func (m *rootModel) artifactRefusal() error {
	if m.selections.isNewRun || m.selections.runFolder == "" {
		return nil
	}
	state, readErr := m.readRunState()
	return resumecheck.CheckArtifact(m.selections.runID, state, readErr)
}

// deploymentRefusal applies the deployed-files early refusal checks to the
// chosen run against what the orchestrator file declares. A deployment that
// cannot be enumerated is left to the session's own run-start checks.
func (m *rootModel) deploymentRefusal(orchPath string, regions []domain.WorkflowRegion) error {
	if m.selections.isNewRun || m.selections.runFolder == "" || orchPath == "" {
		return nil
	}
	state, err := m.readRunState()
	if err != nil {
		return nil
	}
	agents, err := orchfile.EnumerateInfrastructureAgents(orchPath)
	if err != nil {
		return nil
	}
	return resumecheck.CheckDeployment(m.selections.runID, state,
		resumecheck.Deployment{Workflows: regions, InfraAgents: agents})
}

// showEarlyRefusal ends the setup on the done screen with err: a refusal is a
// refused run, anything else a failed one.
func (m *rootModel) showEarlyRefusal(err error) {
	status := domain.RunFailed
	var refusal *resumecheck.Refusal
	if errors.As(err, &refusal) {
		status = domain.RunRefused
	}
	m.doneScreen = runflow.NewDoneScreen(
		domain.RunOutcome{Status: status, Message: err.Error()},
		"", m.width, m.height, stylesFromTheme(m.theme),
	)
	m.screen = screenDone
}
