package resumecheck

import (
	"fmt"
	"sort"
	"strings"

	"mosaic-run/internal/domain"
)

// Summary is the read-only summary of a run's recorded settings.
type Summary struct {
	RunID               string
	Workflow            domain.WorkflowID
	WorkflowVersion     domain.WorkflowVersion
	Mode                domain.ExecutionMode
	PreConsultation     bool
	ManualResolution    bool
	Checkpoints         bool
	CheckpointAgents    []string
	Commits             bool
	CommitBranch        string
	CommitBranchVariant domain.CommitBranchVariant
	ReviewLoopLimit     int
	InfraSelections     map[string]string
	Overrides           []domain.InfrastructureOverride
}

// NewSummary builds the settings summary from the recorded settings in state.
// infraAgents are the declared infrastructure agents, used to name the
// checkpoint agents when no selection is recorded.
func NewSummary(state domain.ArtifactState, infraAgents []domain.DeclaredInfraAgent) Summary {
	s := Summary{
		RunID:               state.RunID,
		Workflow:            state.Workflow,
		WorkflowVersion:     state.WorkflowVersion,
		Mode:                state.Mode,
		PreConsultation:     state.PreConsultation,
		ManualResolution:    state.ManualResolution,
		Checkpoints:         state.Checkpoints,
		Commits:             state.Commits,
		CommitBranch:        state.CommitBranch,
		CommitBranchVariant: state.CommitBranchVariant,
		ReviewLoopLimit:     state.ReviewLoopLimit,
		InfraSelections:     state.InfraClassSelections,
		Overrides:           state.InfrastructureOverrides,
	}
	if !state.Checkpoints {
		return s
	}
	if selected := state.InfraClassSelections["checkpoint"]; selected != "" {
		s.CheckpointAgents = []string{selected}
		return s
	}
	for _, a := range infraAgents {
		if a.Class == "checkpoint" {
			s.CheckpointAgents = append(s.CheckpointAgents, a.Name)
		}
	}
	return s
}

// Lines renders the summary as the text both frontends show.
func (s Summary) Lines() []string {
	return []string{
		"Recorded settings for run " + s.RunID,
		summaryLine("Workflow", s.workflowText()),
		summaryLine("Mode", s.modeText()),
		summaryLine("Pre-consultation", onOff(s.PreConsultation)),
		summaryLine("Manual resolution", onOff(s.ManualResolution)),
		summaryLine("Checkpoints", s.checkpointsText()),
		summaryLine("Commits", s.commitsText()),
		summaryLine("Review loop limit", s.reviewLoopText()),
		summaryLine("Infrastructure", s.selectionsText()),
		summaryLine("Overrides", s.overridesText()),
	}
}

func summaryLine(label, value string) string {
	return fmt.Sprintf("  %-20s%s", label+":", value)
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (s Summary) workflowText() string {
	return fmt.Sprintf("%s (version %s)", s.Workflow, s.WorkflowVersion)
}

func (s Summary) modeText() string {
	if s.Mode == domain.ExecutionModeUnset {
		return "not recorded"
	}
	return string(s.Mode)
}

func (s Summary) checkpointsText() string {
	if !s.Checkpoints {
		return "off"
	}
	if len(s.CheckpointAgents) == 0 {
		return "on"
	}
	return "on (" + strings.Join(s.CheckpointAgents, ", ") + ")"
}

func (s Summary) commitsText() string {
	if !s.Commits {
		return "off"
	}
	branch := "branch not set up yet"
	if s.CommitBranch != "" {
		branch = "branch " + s.CommitBranch
	}
	if s.CommitBranchVariant == "" {
		return "on (" + branch + ")"
	}
	return fmt.Sprintf("on (%s, %s)", branch, s.CommitBranchVariant)
}

func (s Summary) reviewLoopText() string {
	if s.ReviewLoopLimit <= 0 {
		return "none"
	}
	return fmt.Sprintf("%d", s.ReviewLoopLimit)
}

func (s Summary) selectionsText() string {
	if len(s.InfraSelections) == 0 {
		return "none recorded"
	}
	classes := make([]string, 0, len(s.InfraSelections))
	for class := range s.InfraSelections {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	parts := make([]string, 0, len(classes))
	for _, class := range classes {
		parts = append(parts, class+"="+s.InfraSelections[class])
	}
	return strings.Join(parts, ", ")
}

func (s Summary) overridesText() string {
	if len(s.Overrides) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(s.Overrides))
	for _, ov := range s.Overrides {
		triggers := make([]string, 0, len(ov.Triggers))
		for _, tr := range ov.Triggers {
			if tr.Param != "" {
				triggers = append(triggers, tr.Trigger+"="+tr.Param)
			} else {
				triggers = append(triggers, tr.Trigger)
			}
		}
		parts = append(parts, ov.AgentName+": "+strings.Join(triggers, ", "))
	}
	return strings.Join(parts, "; ")
}
