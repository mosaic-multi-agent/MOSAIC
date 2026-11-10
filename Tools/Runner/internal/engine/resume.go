package engine

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// ResumePointFromSchedules determines where to continue from a parsed artifact.
// Pure function -- no I/O, no side effects.
//
// Position is resolved from the last *workflow* entry in the execution log:
// trailing infrastructure entries are skipped, in sequence order, using the
// recognition rule (agent absent from workflow.Table, present in infra).
// infra may be nil, meaning no infrastructure agents are declared.
//
// If the artifact has no execution log entries, or none that is a workflow
// entry, returns the first row.
// If the last *workflow* logged invocation completed cleanly (its agent
// matches current_state.last_agent), returns the row after it. Trailing
// infrastructure entries appearing after it are not an interruption.
// If a mismatch is detected between the last workflow execution log entry
// and current_state, the last workflow step was interrupted in-flight and
// must be re-dispatched (ResumeInfo.RerunLast = true, FR-33).
//
// Returns *domain.PositionUnresolvedError when the position cannot be
// determined.
func ResumePointFromSchedules(
	workflow domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	state domain.ArtifactState,
	infra domain.InfraAgentSet,
) (domain.ResumeInfo, error) {
	if len(state.ExecutionLog) == 0 {
		// Fresh start: resume from the first row.
		return domain.ResumeInfo{
			RowIndex:   0,
			Phase:      firstRowPhase(workflow),
			Stage:      "",
			GroupIndex: -1,
			Seq:        0,
			RerunLast:  false,
		}, nil
	}

	lastWorkflowEntry, found, findErr := lastWorkflowLogEntry(workflow, state.ExecutionLog, infra)
	if findErr != nil {
		return domain.ResumeInfo{}, fmt.Errorf("resume: %w", findErr)
	}
	if !found {
		// The log holds no workflow entry at all (only infrastructure activity
		// on record so far): resume from the first row, as if fresh.
		return domain.ResumeInfo{
			RowIndex:   0,
			Phase:      firstRowPhase(workflow),
			Stage:      "",
			GroupIndex: -1,
			Seq:        0,
			RerunLast:  false,
		}, nil
	}

	// Interruption detection: the last WORKFLOW entry doesn't match
	// CurrentState. Trailing infrastructure entries appearing after it are
	// not an interruption.
	interrupted := lastWorkflowEntry.Agent != state.CurrentState.LastAgent

	if interrupted {
		// The last workflow log entry was dispatched but not recorded in
		// CurrentState. Re-run the interrupted row.
		pos, err := resumePosition(workflow, lastWorkflowEntry)
		if err != nil {
			return domain.ResumeInfo{}, err
		}
		rowIdx := pos.rowIdx
		row := workflow.Table.Rows[rowIdx]
		groupIdx := -1
		if row.PhaseParsed.IsStaged {
			groupIdx = groupIndexOfRow(workflow, rowIdx)
		}
		block := blockOfRow(workflow, rowIdx)
		iteration := domain.IterationID("")
		if block != "" {
			iteration = pos.iteration
		}
		return domain.ResumeInfo{
			RowIndex:   rowIdx,
			Phase:      row.Phase,
			Stage:      lastWorkflowEntry.Stage,
			Block:      block,
			Iteration:  iteration,
			GroupIndex: groupIdx,
			Seq:        state.GlobalSequence,
			RerunLast:  true,
		}, nil
	}

	// Clean completion: advance to the next row after the last workflow step.
	pos, err := resumePosition(workflow, lastWorkflowEntry)
	if err != nil {
		return domain.ResumeInfo{}, err
	}
	return resumeAfterScopedRow(workflow, schedules, state.ExecutionLog, pos, state.GlobalSequence)
}

// scheduleSpelling returns the ID as the block's schedule spells it when the
// recorded ID matches an entry ignoring case, and the recorded ID otherwise.
func scheduleSpelling(schedules domain.BlockSchedules, block domain.BlockName, id domain.IterationID) domain.IterationID {
	sched, ok := schedules[block]
	if !ok {
		return id
	}
	if idx := iterationIndex(sched, id); idx >= 0 {
		return sched.Iterations[idx].ID
	}
	return id
}

// lastWorkflowLogEntry returns the most recent execution log entry produced
// by a workflow participant, skipping any trailing infrastructure entries
// (recognised by derivation: absent from the routing table, present in infra).
//
// found is false when the log holds no workflow entry at all -- only
// infrastructure activity is on record so far.
//
// Returns *domain.PositionUnresolvedError when a trailing entry is neither a
// workflow participant nor a recognised infrastructure agent: the position
// cannot be determined in that case. infra may be nil, meaning no
// infrastructure agents are declared.
func lastWorkflowLogEntry(
	workflow domain.AdmittedWorkflow,
	log []domain.ExecutionLogEntry,
	infra domain.InfraAgentSet,
) (entry domain.ExecutionLogEntry, found bool, err error) {
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		agentName := extractAgentName(e.Agent)
		if isWorkflowParticipant(workflow, agentName) {
			return e, true, nil
		}
		if infra != nil && infra.IsInfrastructureAgent(e.Agent) {
			continue // trailing infrastructure entry: keep looking
		}
		return domain.ExecutionLogEntry{}, false, &domain.PositionUnresolvedError{
			AgentInstance: e.Agent,
			Phase:         e.Phase,
			Stage:         e.Stage,
			Cause:         domain.CauseAgentNotInWorkflow,
		}
	}
	return domain.ExecutionLogEntry{}, false, nil
}

// isWorkflowParticipant reports whether agentName names a routing table
// participant for this workflow, in any row.
func isWorkflowParticipant(workflow domain.AdmittedWorkflow, agentName string) bool {
	for _, row := range workflow.Table.Rows {
		if row.BareAgentID() == agentName {
			return true
		}
	}
	return false
}
