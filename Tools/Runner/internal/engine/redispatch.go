package engine

import (
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// RedispatchInput carries everything UnacceptedAttemptRedispatch reads.
type RedispatchInput struct {
	Workflow  domain.AdmittedWorkflow
	Schedules domain.BlockSchedules
	State     domain.ArtifactState
	Infra     domain.InfraAgentSet // may be nil: no infrastructure agents declared
	Agents    map[string]domain.AgentReference
	// Seq is the last allocated global sequence; the step's instance is
	// "<bare agent id>#<Seq+1>".
	Seq int
}

// recordedInputsSeparator separates the artifact paths in the Inputs column of
// an Execution Log row.
const recordedInputsSeparator = ", "

// UnacceptedAttemptRedispatch returns the step that re-dispatches the trailing
// unaccepted workflow attempt of State.ExecutionLog.
//
// ok is false, with a zero step and nil error, exactly when
// ResumePointFromSchedules(in.Workflow, in.Schedules, in.State, in.Infra)
// reports RerunLast == false.
//
// The step is a fresh dispatch of the attempt's row and iteration (row
// defaults for outputs and HITL), except that its input artifacts are the
// Inputs recorded on the log entry. State is never modified.
func UnacceptedAttemptRedispatch(in RedispatchInput) (step domain.DispatchStep, ok bool, err error) {
	info, err := ResumePointFromSchedules(in.Workflow, in.Schedules, in.State, in.Infra)
	if err != nil {
		return domain.DispatchStep{}, false, err
	}
	if !info.RerunLast {
		return domain.DispatchStep{}, false, nil
	}
	entry, _, err := lastWorkflowLogEntry(in.Workflow, in.State.ExecutionLog, in.Infra)
	if err != nil {
		return domain.DispatchStep{}, false, fmt.Errorf("resume: %w", err)
	}

	row := in.Workflow.Table.Rows[info.RowIndex]
	defaults, err := ResolveRowDefaultsForIteration(row, info.Iteration, ScheduleContext{Schedules: in.Schedules})
	if err != nil {
		return domain.DispatchStep{}, false, fmt.Errorf("resume: re-dispatch of %s: %w", entry.Agent, err)
	}

	agentID := row.BareAgentID()
	recordedStage := ""
	if row.PhaseParsed.IsStaged {
		recordedStage = recordedIterationValue(row.PhaseParsed.Group, info.Iteration)
	}
	return domain.DispatchStep{
		RowIndex: info.RowIndex,
		Agent:    in.Agents[agentID],
		Request: domain.ProtocolRequest{
			AgentInstanceID: fmt.Sprintf("%s#%d", agentID, in.Seq+1),
			TaskDescription: qualifierTaskDescription(row),
			InputArtifacts:  splitRecordedInputs(entry.Inputs),
			OutputArtifacts: defaults.OutputArtifacts,
			HumanInTheLoop:  defaults.HITL,
		},
		EffectiveHITL: defaults.HITL,
		Phase:         row.PhaseParsed.Name,
		Stage:         recordedStage,
		Retry:         domain.RetryNone,
	}, true, nil
}

// splitRecordedInputs splits the Inputs column value of a log row into artifact
// paths, in recorded order. An empty value gives an empty list.
func splitRecordedInputs(recorded string) []string {
	if recorded == "" || recorded == "-" {
		return []string{}
	}
	return strings.Split(recorded, recordedInputsSeparator)
}
