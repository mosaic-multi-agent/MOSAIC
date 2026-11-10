package engine

import (
	"fmt"
	"time"

	"mosaic-run/internal/domain"
)

// Base texts of the stop reasons for a block that has no schedule.
const (
	noScheduleAtStart = "no stage set available for staged workflow"
	noScheduleOnEntry = "entering a block but no stage set is available"
	noRowsStopReason  = "workflow has no rows"
)

// initialDispatch returns the first dispatch when no prior invocations exist:
// row 1, entering the block it begins when it begins one.
func initialDispatch(in NextInput, sc ScheduleContext) domain.EngineDecision {
	workflow := in.Workflow
	if len(workflow.Table.Rows) == 0 {
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: noRowsStopReason}}
	}
	adv, err := routerFor(in, sc).enterRow(0)
	return decisionFor(in, sc, adv, err, noScheduleAtStart)
}

// decisionFor turns a computed position into the engine's decision: a dispatch
// of the row, the completion of the run, or a stop. noScheduleBase is the base
// text of the stop reason when the position entered a block without a schedule.
func decisionFor(
	in NextInput,
	sc ScheduleContext,
	adv advance,
	err error,
	noScheduleBase string,
) domain.EngineDecision {
	switch {
	case err != nil:
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: err.Error()}}
	case adv.Complete:
		return domain.EngineDecision{Complete: &domain.CompleteDecision{FinalState: in.State.CurrentState}}
	case adv.NoSchedule != "":
		return missingScheduleStop(adv.NoSchedule, stageSourceFor(in, adv.NoSchedule), noScheduleBase)
	}
	step, stepErr := buildDispatchStep(in.Workflow, adv.RowIndex, adv.Iteration, in.Agents, in.Seq, in.Now, sc)
	if stepErr != nil {
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: stepErr.Error()}}
	}
	return domain.EngineDecision{Dispatch: &domain.DispatchDecision{Steps: []domain.DispatchStep{step}}}
}

// stageSourceFor returns the descriptor of block's schedule source: the entry
// of in.StageSources that names the block, else in.StageSource.
func stageSourceFor(in NextInput, block domain.BlockName) domain.StageSource {
	for _, src := range in.StageSources {
		if src.Block == block {
			return src
		}
	}
	return in.StageSource
}

// missingScheduleStop is the stop for a block with no schedule. The source
// descriptor, when stated, names where the schedule was looked for.
func missingScheduleStop(block domain.BlockName, src domain.StageSource, base string) domain.EngineDecision {
	cause := &domain.MissingScheduleError{Block: block, Source: src.Source, Path: src.Path, Seeded: src.Seeded, Base: base}
	return domain.EngineDecision{Stop: &domain.StopDecision{Reason: cause.Error(), Err: cause}}
}

// unroutableStop is the stop for a completed row whose routing entry is not
// routed: a zero kind, a fork outside Mode 1, or a target outside the table.
func unroutableStop(rowIdx int, column, kind string) domain.EngineDecision {
	cause := &domain.UnroutableEntryError{Row: domain.WorkflowRowFromIndex(rowIdx), Column: column, Kind: kind}
	return domain.EngineDecision{Stop: &domain.StopDecision{Reason: cause.Error(), Err: cause}}
}

// buildDispatchStep assembles a DispatchStep for the given routing table row.
// iteration is "" for a row outside every block.
func buildDispatchStep(
	workflow domain.AdmittedWorkflow,
	rowIdx int,
	iteration domain.IterationID,
	agents map[string]domain.AgentReference,
	seq int,
	now time.Time,
	sc ScheduleContext,
) (domain.DispatchStep, error) {
	_ = now // timestamp is available for future use; not inspected by current tests
	row := workflow.Table.Rows[rowIdx]
	isExecution := row.PhaseParsed.IsStaged

	defaults, err := resolveDefaults(row, blockOfRow(workflow, rowIdx), iteration, sc)
	if err != nil {
		return domain.DispatchStep{}, err
	}
	inputArts, outputArts, effectiveHITL := defaults.InputArtifacts, defaults.OutputArtifacts, defaults.HITL

	agentID := row.BareAgentID()
	agent := agents[agentID]
	instanceID := fmt.Sprintf("%s#%d", agentID, seq+1)

	// The recorded Phase/Stage are the canonical form the artifact stores:
	// the bare phase name (never the routing table's qualified string) and
	// the OAF 3.0 iteration value.
	recordedStage := ""
	if isExecution {
		recordedStage = recordedIterationValue(row.PhaseParsed.Group, iteration)
	}

	return domain.DispatchStep{
		RowIndex: rowIdx,
		Agent:    agent,
		Request: domain.ProtocolRequest{
			AgentInstanceID: instanceID,
			TaskDescription: qualifierTaskDescription(row),
			InputArtifacts:  inputArts,
			OutputArtifacts: outputArts,
			HumanInTheLoop:  effectiveHITL,
		},
		EffectiveHITL: effectiveHITL,
		Phase:         row.PhaseParsed.Name,
		Stage:         recordedStage,
	}, nil
}
