package engine

import (
	"errors"
	"fmt"

	"mosaic-run/internal/domain"
)

// position is the workflow position derived from the Execution Log entry of
// the step that just ran. Live routing (Next) and resume (ResumePoint) both
// derive row and stage through resolvePosition, so they cannot disagree.
type position struct {
	entry     domain.ExecutionLogEntry
	rowIdx    int // zero-based routing table row; -1 when the entry cannot be identified
	stage     string
	iteration domain.IterationID    // the ID read from stage; "" when stage is empty or unreadable
	value     domain.IterationValue // stage read with the row's block and group; zero outside every block
}

// currentPosition derives the position of the step that last ran from the
// artifact state. The entry is the last Execution Log entry of the exact
// instance recorded as LastAgent; when the log holds none, an entry built from
// the current state (which records no workflow row) stands in.
//
// It returns an error when the recorded agent is not a workflow participant,
// or when resolvePosition refuses the entry.
func currentPosition(workflow domain.AdmittedWorkflow, state domain.ArtifactState) (position, error) {
	cs := state.CurrentState
	if !isWorkflowParticipant(workflow, extractAgentName(cs.LastAgent)) {
		// Not an ambiguity to fall back on: the recorded agent genuinely is not
		// a participant, and the stop must name this as the cause.
		return position{rowIdx: -1}, &domain.PositionUnresolvedError{
			AgentInstance: cs.LastAgent,
			Phase:         cs.Phase,
			Stage:         cs.Stage,
			Cause:         domain.CauseAgentNotInWorkflow,
		}
	}
	return resolvePosition(workflow, lastEntryOfAgent(state))
}

// lastEntryOfAgent returns the last log entry whose Agent equals the exact
// instance id in CurrentState.LastAgent, or the synthetic entry built from
// CurrentState when there is none.
func lastEntryOfAgent(state domain.ArtifactState) domain.ExecutionLogEntry {
	cs := state.CurrentState
	for i := len(state.ExecutionLog) - 1; i >= 0; i-- {
		if state.ExecutionLog[i].Agent == cs.LastAgent {
			return state.ExecutionLog[i]
		}
	}
	return domain.ExecutionLogEntry{
		Agent:       cs.LastAgent,
		Phase:       cs.Phase,
		Stage:       cs.Stage,
		WorkflowRow: domain.NoWorkflowRow,
	}
}

// resolvePosition derives row and stage from one log entry.
//
// Whenever the entry records a workflow row, that row is the position for
// every row type, validated against the table. Only an entry that records no
// row falls back to resolving by agent and phase, in which case rowIdx is -1
// (with a nil error) when no row matches.
func resolvePosition(workflow domain.AdmittedWorkflow, entry domain.ExecutionLogEntry) (position, error) {
	if entry.WorkflowRow != domain.NoWorkflowRow {
		return recordedPosition(workflow, entry)
	}
	return fallbackPosition(workflow, entry)
}

// recordedPosition validates the row recorded on entry against the table: the
// row exists and holds the entry's bare agent id. For a row of a block the
// entry must also carry a stage whose group is the row's group.
func recordedPosition(workflow domain.AdmittedWorkflow, entry domain.ExecutionLogEntry) (position, error) {
	idx := entry.WorkflowRow.Index()
	if idx < 0 || idx >= len(workflow.Table.Rows) {
		return position{rowIdx: -1}, recordedRowError(entry, domain.CauseRecordedRowInvalid)
	}
	row := workflow.Table.Rows[idx]
	if row.BareAgentID() != extractAgentName(entry.Agent) {
		return position{rowIdx: -1}, recordedRowError(entry, domain.CauseRecordedRowInvalid)
	}
	pos := position{entry: entry, rowIdx: idx, stage: entry.Stage}
	value, err := domain.ReadIterationValue(entry.Stage, row)
	if err != nil {
		var verr *domain.IterationValueError
		if errors.As(err, &verr) && verr.Kind == domain.IterationValueMissing {
			return position{rowIdx: -1}, &domain.PositionUnresolvedError{
				AgentInstance: entry.Agent,
				Phase:         entry.Phase,
				Stage:         entry.Stage,
				Cause:         domain.CauseStagedRowWithoutStage,
				RecordedRow:   entry.WorkflowRow,
				Group:         row.PhaseParsed.Group,
			}
		}
		return position{rowIdx: -1}, recordedRowError(entry, domain.CauseRecordedRowInvalid)
	}
	pos.value = value
	pos.iteration = value.ID
	return pos, nil
}

// fallbackPosition resolves an entry that records no workflow row by its bare
// agent id and phase. Whether a candidate row belongs to a block decides how
// the entry's stage is read: an entry with no stage names a row outside every
// block (the first such row), and an entry with a stage value names the row of
// a block whose group is the stage value's group. An agent that fills several
// block rows of one group cannot be resolved without a recorded row.
func fallbackPosition(workflow domain.AdmittedWorkflow, entry domain.ExecutionLogEntry) (position, error) {
	agentName := extractAgentName(entry.Agent)
	phase := domain.RecordedPhaseName(entry.Phase)
	pos := position{entry: entry, rowIdx: -1, stage: entry.Stage}

	var plain, inBlock []int
	for _, row := range workflow.Table.Rows {
		if row.BareAgentID() != agentName || (row.PhaseParsed.Name != phase && row.Phase != entry.Phase) {
			continue
		}
		if blockOfRow(workflow, row.Index) != "" {
			inBlock = append(inBlock, row.Index)
		} else {
			plain = append(plain, row.Index)
		}
	}

	candidates := inBlock
	// "No iteration" check while choosing the row: no value is split here, so
	// the recorded text is compared as is.
	if entry.Stage == "" || entry.Stage == "-" {
		if len(plain) > 0 {
			pos.rowIdx = plain[0]
			return pos, nil
		}
	} else {
		// The rows of the agent that read the recorded value narrow the
		// candidates (one agent may fill rows of several groups through mode
		// qualifiers); when none reads it, all stay.
		var readable []int
		for _, idx := range inBlock {
			if _, err := domain.ReadIterationValue(entry.Stage, workflow.Table.Rows[idx]); err == nil {
				readable = append(readable, idx)
			}
		}
		if len(readable) > 0 {
			candidates = readable
		}
	}
	switch len(candidates) {
	case 0:
		if len(plain) > 0 {
			pos.rowIdx = plain[0]
		}
		return pos, nil
	case 1:
		// The value is read with the one matched row; a value that row does not
		// accept leaves no row matching the recorded phase and value.
		value, err := domain.ReadIterationValue(entry.Stage, workflow.Table.Rows[candidates[0]])
		if err != nil {
			return position{rowIdx: -1}, recordedRowError(entry, domain.CauseNoMatchingRow)
		}
		pos.rowIdx = candidates[0]
		pos.value = value
		pos.iteration = value.ID
		return pos, nil
	}
	return position{rowIdx: -1}, recordedRowError(entry, domain.CauseNoRecordedRow)
}

func recordedRowError(entry domain.ExecutionLogEntry, cause domain.PositionUnresolvedCause) error {
	return &domain.PositionUnresolvedError{
		AgentInstance: entry.Agent,
		Phase:         entry.Phase,
		Stage:         entry.Stage,
		Cause:         cause,
		RecordedRow:   entry.WorkflowRow,
	}
}

// resumePosition resolves the position of a workflow log entry for ResumePoint.
// An entry that cannot be identified at all is an error naming agent and phase;
// no row is ever guessed.
func resumePosition(workflow domain.AdmittedWorkflow, entry domain.ExecutionLogEntry) (position, error) {
	pos, err := resolvePosition(workflow, entry)
	if err == nil && pos.rowIdx < 0 {
		err = fmt.Errorf("row not found for log entry agent=%q phase=%q", entry.Agent, entry.Phase)
	}
	if err != nil {
		return position{rowIdx: -1}, fmt.Errorf("resume: %w", err)
	}
	return pos, nil
}

// resumeAfterNonExecution is the resume point after a cleanly completed row of
// the run scope: the row immediately after it, or the entry point of the block
// that row begins. A block with no usable schedule leaves the positional next
// row.
func resumeAfterNonExecution(
	workflow domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	currentRowIdx int,
	seq int,
) domain.ResumeInfo {
	nextRowIdx := currentRowIdx + 1
	if nextRowIdx >= len(workflow.Table.Rows) {
		return domain.ResumeInfo{RowIndex: nextRowIdx, GroupIndex: -1, Seq: seq}
	}
	adv, err := newRouter(workflow, schedules, false).enterRow(nextRowIdx)
	if err != nil || adv.NoSchedule != "" || adv.Complete {
		nextRow := workflow.Table.Rows[nextRowIdx]
		return domain.ResumeInfo{
			RowIndex:   nextRowIdx,
			Phase:      nextRow.Phase,
			Block:      blockOfRow(workflow, nextRowIdx),
			GroupIndex: -1,
			Seq:        seq,
		}
	}
	return resumeInfoAt(workflow, adv, seq)
}

// resumeInfoAt is the resume point of a computed position.
func resumeInfoAt(workflow domain.AdmittedWorkflow, adv advance, seq int) domain.ResumeInfo {
	if adv.Complete || adv.RowIndex < 0 || adv.RowIndex >= len(workflow.Table.Rows) {
		return domain.ResumeInfo{RowIndex: len(workflow.Table.Rows), GroupIndex: -1, Seq: seq}
	}
	row := workflow.Table.Rows[adv.RowIndex]
	groupIdx := -1
	if row.PhaseParsed.IsStaged {
		groupIdx = groupIndexOfRow(workflow, adv.RowIndex)
	}
	return domain.ResumeInfo{
		RowIndex:   adv.RowIndex,
		Phase:      row.Phase,
		Stage:      recordedIterationValue(row.PhaseParsed.Group, adv.Iteration),
		Block:      blockOfRow(workflow, adv.RowIndex),
		Iteration:  adv.Iteration,
		GroupIndex: groupIdx,
		Seq:        seq,
	}
}
