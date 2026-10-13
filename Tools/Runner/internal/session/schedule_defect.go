package session

// A block schedule that a producing row, or a row of the block that writes it
// while the block runs, produced and that is defective is never silently used.
// The defect is a defect of the producing subagent: the Runner re-dispatches
// that row itself, in every run mode and without consulting the orchestrator,
// with a message naming the defect and the non-compliant artifact among the
// inputs. The re-dispatches are bounded by a limit counted from the Execution
// Log alone; once it is used up the session consults the routing consultant
// with the defect description as the last error reason.

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// pendingScheduleDefect is a judged schedule defect awaiting its re-dispatch or
// consultation. It is handled at the top of the dispatch loop, before the
// engine routes onwards.
type pendingScheduleDefect struct {
	// rowIndex is the zero-based routing row whose completed step triggered the
	// re-read that found the defect.
	rowIndex int
	// stage is the recorded stage value of that step, "" outside every block.
	stage string
	// defects are the blocks found defective by that re-read.
	defects []blockDefect
	// response is the producer's accepted response.
	response domain.ProtocolResponse
	// ran holds the schedules the producer's attempt ran under; the re-dispatch
	// resolves the row's defaults over them. Nil takes the session's schedules.
	ran domain.BlockSchedules
}

// rereadAccepted re-reads the schedules affected by the step of row rowIdx that
// the HITL compliance loop accepted, and judges them. A defective schedule of a
// produced or seeded block is dropped from stages, and a SUCCESS response
// leaves the defect pending for the dispatch loop. Any other status keeps
// today's handling (a warning; the previous schedule stays in use).
func (s *sessionImpl) rereadAccepted(
	ctx context.Context,
	admitted domain.AdmittedWorkflow,
	runFolder string,
	rowIdx int,
	stage string,
	resp domain.ProtocolResponse,
	stages *domain.BlockSchedules,
) {
	judge := resp.StatusCode == domain.StatusSUCCESS
	before := *stages
	res := s.rereadBlocks(ctx, admitted, runFolder, rowIdx, before, judge)
	if res.changed {
		*stages = res.schedules
	}
	// A block read successfully again is accepted again; a block dropped for
	// a defect keeps its last accepted schedule, which a re-dispatched writer
	// of the block resolves its defaults over.
	for name := range s.droppedSchedules {
		if _, held := res.schedules[name]; held {
			delete(s.droppedSchedules, name)
		}
	}
	if len(res.defects) == 0 {
		return
	}
	ran := before
	if len(s.droppedSchedules) > 0 {
		ran = make(domain.BlockSchedules, len(before)+len(s.droppedSchedules))
		for name, sched := range before {
			ran[name] = sched
		}
		for name, sched := range s.droppedSchedules {
			if _, held := ran[name]; !held {
				ran[name] = sched
			}
		}
	}
	for _, d := range res.defects {
		if sched, held := ran[d.block.Name]; held {
			if s.droppedSchedules == nil {
				s.droppedSchedules = domain.BlockSchedules{}
			}
			s.droppedSchedules[d.block.Name] = sched
		}
	}
	s.scheduleDefect = &pendingScheduleDefect{
		rowIndex: rowIdx,
		stage:    stage,
		defects:  res.defects,
		response: resp,
		ran:      ran,
	}
}

// noteResumedDefect disposes of a defective schedule read of produced block b
// at resume. When the last routed entry of the log is a SUCCESS of one of b's
// producing rows, the defect is that producer's and is left for the dispatch
// loop to act on, as for a judged attempt; the entry is the accepted attempt.
// In any other log state in which a routed step ran, no attempt of this run is
// waiting for judgement, and the returned refusal ends the resume. A log with
// no routed entry holds no attempt at all: the file is a stale one that the
// producer, which has not run yet, replaces, so the read keeps being a
// warning and leaves no schedule (handled false).
func (rs *runStartCtx) noteResumedDefect(b domain.AdmittedBlock, cause error) (handled bool, refusal *domain.RefusalError) {
	log := rs.existingState.ExecutionLog
	routed := false
	for i := len(log) - 1; i >= 0; i-- {
		entry := log[i]
		if entry.WorkflowRow == domain.NoWorkflowRow {
			continue
		}
		routed = true
		if entry.Status != domain.StatusSUCCESS {
			break
		}
		for _, p := range b.ProducingRows {
			if entry.WorkflowRow != domain.WorkflowRowFromIndex(p) {
				continue
			}
			if rs.resumedDefect == nil {
				rs.resumedDefect = &pendingScheduleDefect{
					rowIndex: p,
					stage:    entry.Stage,
					response: domain.ProtocolResponse{
						AgentInstanceID: entry.Agent,
						StatusCode:      domain.StatusSUCCESS,
						StatusMessage:   entry.Summary,
					},
				}
			}
			rs.resumedDefect.defects = append(rs.resumedDefect.defects, blockDefect{block: b, err: cause})
			return true, nil
		}
		break
	}
	if !routed {
		return false, nil
	}
	return true, scheduleDefectRefusal(rs.config, b, cause, "produced schedule")
}

// handleScheduleDefect acts on the pending schedule defect: the producing row
// is re-dispatched while re-dispatches remain, and the routing consultant is
// consulted afterwards. Returns (out, done, err) for the dispatch loop.
func (s *sessionImpl) handleScheduleDefect(ctx context.Context, rs *runStartCtx) (domain.RunOutcome, bool, error) {
	p := s.scheduleDefect
	s.scheduleDefect = nil
	row, ok := rowAtIndex(rs.admitted.Table, p.rowIndex)
	if !ok {
		msg := fmt.Sprintf("schedule defect names routing row %d, which the table does not have", p.rowIndex)
		return domain.RunOutcome{Status: domain.RunFailed, Message: msg}, true, nil
	}
	attempts := engine.ScheduleDefectAttempts(rs.state.ExecutionLog, row, p.stage)
	if attempts-1 >= engine.ScheduleDefectRedispatchLimit {
		return s.consultScheduleDefect(ctx, rs, p, row)
	}
	step, err := s.scheduleDefectStep(rs, p, row)
	if err != nil {
		return domain.RunOutcome{Status: domain.RunFailed, Message: err.Error()}, true, nil
	}
	s.deps.Debug.Log(domain.EventSessionDeviation, "block schedule defective; re-dispatching its producer",
		domain.F("agent", step.Agent.Identifier),
		domain.F("row", fmt.Sprint(row.Index)),
		domain.F("attempts", fmt.Sprint(attempts)),
	)
	out, done, _, err := s.handleEngineDispatch(ctx, rs, domain.EngineDecision{
		Dispatch: &domain.DispatchDecision{Steps: []domain.DispatchStep{step}},
	})
	return out, done, err
}

// scheduleDefectStep builds the re-dispatch of the producing row: its defaults
// at the iteration it ran at, the non-compliant artifacts among its inputs and
// the fix message in the task description.
func (s *sessionImpl) scheduleDefectStep(rs *runStartCtx, p *pendingScheduleDefect, row domain.RoutingRow) (domain.DispatchStep, error) {
	var iteration domain.IterationID
	if _, inBlock := row.Block(); inBlock {
		value, readErr := domain.ReadIterationValue(p.stage, row)
		if readErr != nil {
			return domain.DispatchStep{}, fmt.Errorf("schedule defect: recorded stage %q of row %d names no iteration", p.stage, row.Index)
		}
		iteration = value.ID
	}
	schedules := p.ran
	if schedules == nil {
		schedules = rs.stages
	}
	defaults, err := resolveRowDefaults(row, iteration, schedules)
	if err != nil {
		return domain.DispatchStep{}, fmt.Errorf("schedule defect: %w", err)
	}
	inputs := slices.Clone(defaults.InputArtifacts)
	for _, d := range p.defects {
		if !slices.Contains(inputs, d.block.Source) && fileExists(blockSchedulePath(rs.config.RunFolder, d.block)) {
			inputs = append(inputs, d.block.Source)
		}
	}
	task := domain.GenericTaskDescription
	if row.Qualifier != "" {
		task += "\n\n" + engine.QualifierFocusPrefix + " " + row.Qualifier
	}
	task += "\n\n" + p.fixMessage()
	agentID := row.BareAgentID()
	return domain.DispatchStep{
		RowIndex: row.Index,
		Agent:    rs.agents[agentID],
		Request: domain.ProtocolRequest{
			AgentInstanceID: fmt.Sprintf("%s#%d", agentID, rs.seq+1),
			TaskDescription: task,
			InputArtifacts:  inputs,
			OutputArtifacts: defaults.OutputArtifacts,
			HumanInTheLoop:  defaults.HITL,
		},
		EffectiveHITL: defaults.HITL,
		Phase:         row.PhaseParsed.Name,
		Stage:         p.stage,
		Retry:         domain.RetryScheduleDefect,
	}, nil
}

// consultScheduleDefect consults the routing consultant after the re-dispatch
// limit is used up, with the defect description as the last error reason and
// the producer's own status message as the last status message. Without a
// routing consultant the run ends unresolved and no block row is dispatched.
func (s *sessionImpl) consultScheduleDefect(ctx context.Context, rs *runStartCtx, p *pendingScheduleDefect, row domain.RoutingRow) (domain.RunOutcome, bool, error) {
	reason := p.describe()
	if s.deps.Routing == nil && !(s.manualDispatchPending && s.deps.Manual != nil) {
		msg := fmt.Sprintf("deviation: no routing consultant configured (kind=%s): %s",
			domain.DeviationScheduleDefect, reason)
		s.deps.Debug.Log(domain.EventSessionDeviationUnresolved, msg)
		return domain.RunOutcome{Status: domain.RunDeviationUnresolved, Message: msg}, true, nil
	}
	resp := p.response
	rs.lastResponse = &resp
	info := domain.DeviationInfo{
		Kind:          domain.DeviationScheduleDefect,
		Response:      resp,
		CurrentRow:    row.Index,
		CurrentPhase:  row.PhaseParsed.Name,
		CurrentStage:  p.stage,
		ArtifactState: rs.state,
		ErrorReason:   reason,
	}
	s.deps.Debug.Log(domain.EventSessionDeviation, "block schedule still defective after the re-dispatch limit",
		domain.F("kind", string(info.Kind)),
	)
	done, out, err := s.consultRoute(ctx, &info, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
	return out, done, err
}

// describe is the defect description: for each defective block the rule (or
// that the Source is missing or unreadable), the block, the Source and Section
// and the offending row, ID or token.
func (p *pendingScheduleDefect) describe() string {
	parts := make([]string, 0, len(p.defects))
	for _, d := range p.defects {
		parts = append(parts, d.err.Error())
	}
	return strings.Join(parts, "; ")
}

// fixMessage is the task text of the re-dispatch: it states that the schedule
// is non-compliant and why, and asks for a correction in place.
func (p *pendingScheduleDefect) fixMessage() string {
	var b strings.Builder
	b.WriteString("The block schedule you produced is non-compliant and cannot be used: ")
	b.WriteString(p.describe())
	b.WriteString(".")
	for _, d := range p.defects {
		fmt.Fprintf(&b, "\nCorrect the schedule in place at the same Source %s, section %q, "+
			"so that it satisfies the block-schedule rules of the workflow format, "+
			"and complete your assignment again.", d.block.Source, d.block.Section)
	}
	return b.String()
}
