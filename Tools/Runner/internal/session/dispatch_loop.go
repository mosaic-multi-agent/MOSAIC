package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// runDispatchLoop arms the ManualDispatch flag and executes the dispatch loop
// until the engine signals completion, stop, or an error occurs. All loop
// state is held in rs, which must have been fully populated by the run-start
// sequence before this function is called.
func (s *sessionImpl) runDispatchLoop(ctx context.Context, rs *runStartCtx) (domain.RunOutcome, error) {
	s.manualDispatchPending = rs.config.ManualDispatch
	s.scheduleDefect = rs.resumedDefect
	s.droppedSchedules = nil

	for {
		if s.consultOutputs.set {
			rs.lastOutputArtifacts = s.consultOutputs.paths
			s.consultOutputs = consultOutputs{}
		}
		// A defective produced schedule is acted on before the engine routes
		// onwards: the engine never sees a block row to dispatch against it.
		if s.scheduleDefect != nil {
			out, done, err := s.handleScheduleDefect(ctx, rs)
			if done {
				return out, err
			}
			continue
		}
		if out, done, handled, err := s.runRepairDispatch(ctx, rs); handled {
			if done {
				return out, err
			}
			continue
		}
		if out, done, handled, err := s.runResumeRedispatch(ctx, rs); handled {
			if done {
				return out, err
			}
			continue
		}
		decision := engine.Next(engine.NextInput{
			Workflow:            rs.admitted,
			Schedules:           rs.stages,
			RetrySchedules:      s.droppedSchedules,
			State:               rs.state,
			LastResponse:        rs.lastResponse,
			Agents:              rs.agents,
			Seq:                 rs.seq,
			Now:                 s.deps.Clock.Now(),
			StageSources:        rs.stageSources,
			Mode:                rs.state.Mode,
			LastOutputArtifacts: rs.lastOutputArtifacts,
		})

		if decision.Dispatch != nil {
			out, done, _, err := s.handleEngineDispatch(ctx, rs, decision)
			if done {
				return out, err
			}
			continue
		}

		if decision.Complete != nil {
			return domain.RunOutcome{
				Status:  domain.RunCompleted,
				Message: "run completed successfully",
			}, nil
		}

		if decision.Consult != nil {
			out, done, err := s.handleEngineConsult(ctx, rs)
			if done {
				return out, err
			}
			continue
		}

		if decision.Deviation != nil {
			out, done, err := s.handleEngineDeviation(ctx, rs, decision)
			if done {
				return out, err
			}
			continue
		}

		if decision.Stop != nil {
			return domain.RunOutcome{
				Status:  domain.RunStopped,
				Message: "engine stop: " + decision.Stop.Reason,
			}, nil
		}

		return domain.RunOutcome{Status: domain.RunFailed, Message: "engine returned nil decision"}, nil
	}
}

// handleEngineConsult handles an engine Consult decision by calling
// consultRoute and returning (out, done, err) to the dispatch loop.
func (s *sessionImpl) handleEngineConsult(ctx context.Context, rs *runStartCtx) (domain.RunOutcome, bool, error) {
	if s.deps.Routing == nil && !s.manualResolverReady() {
		return domain.RunOutcome{
			Status:  domain.RunFailed,
			Message: "orchestrated mode requires a RoutingConsultant but none was wired",
		}, true, nil
	}
	done, out, err := s.consultRoute(ctx, nil, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
	return out, done, err
}

// handleEngineDeviation handles an engine Deviation decision by calling
// consultRoute and returning (out, done, err) to the dispatch loop.
func (s *sessionImpl) handleEngineDeviation(ctx context.Context, rs *runStartCtx, decision domain.EngineDecision) (domain.RunOutcome, bool, error) {
	s.deps.Debug.Log(domain.EventSessionDeviation, "engine returned deviation",
		domain.F("kind", string(decision.Deviation.Info.Kind)),
	)
	if s.deps.Routing == nil && !s.manualResolverReady() {
		msg := fmt.Sprintf("deviation: no routing consultant configured (kind=%s)", decision.Deviation.Info.Kind)
		s.deps.Debug.Log(domain.EventSessionDeviationUnresolved, msg)
		return domain.RunOutcome{Status: domain.RunDeviationUnresolved, Message: msg}, true, nil
	}
	done, out, err := s.consultRoute(ctx, &decision.Deviation.Info, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
	return out, done, err
}

// handleEngineDispatch handles an engine Dispatch decision: prepares the
// request, invokes the harness (with raw-text bypass), runs HITL compliance
// verification, and applies the accepted response.
//
// Returns (out, done, cont, err): done=true means the run is terminal;
// cont=true means the outer loop should continue to the next engine.Next call.
func (s *sessionImpl) handleEngineDispatch(ctx context.Context, rs *runStartCtx, decision domain.EngineDecision) (domain.RunOutcome, bool, bool, error) {
	if len(decision.Dispatch.Steps) == 0 {
		return domain.RunOutcome{Status: domain.RunFailed, Message: "engine returned empty dispatch"}, true, false, nil
	}
	if out, done, diverted, err := s.divertEngineDispatch(ctx, rs, decision.Dispatch.Steps[0]); diverted {
		return out, done, true, err
	}
	step := s.prepareAutoDispatchRequest(ctx, rs, decision.Dispatch.Steps[0])
	// An engine dispatch is counted but never blocked. Mechanical re-dispatches
	// are not counted at all: their own bounds (PARTIALLY_DONE limit, E501
	// budget) own them, and they must not eat into the guard's allowance for
	// the dispatches a consultant requests afterwards.
	if step.Retry == domain.RetryNone {
		rs.antiLoop.recordDispatch(step.RowIndex, step.Agent.Identifier)
	}
	rs.antiLoop.escalations = 0

	if s.deps.StopRequested() {
		s.deps.Debug.Log(domain.EventSessionStopObserved, "graceful stop observed; not dispatching",
			domain.F("checkpoint", StopCheckpointEngineStep),
		)
		return domain.RunOutcome{Status: domain.RunStopped, Message: "run stopped: graceful stop confirmed"}, true, false, nil
	}

	step = s.applyHITLToEngineStep(step)
	s.beginOutputs(ctx, rs.config.RunFolder, step.Request.OutputArtifacts)
	response, invokeErr := s.invokeAndLog(ctx, step.Agent, step.Request)
	if invokeErr != nil {
		var bypassResp domain.ProtocolResponse
		var bypassStep domain.DispatchStep
		var done, cont bool
		var out domain.RunOutcome
		var err error
		bypassResp, bypassStep, done, cont, out, err = s.handleAutoHarnessErrorAndBypass(ctx, rs, step, invokeErr)
		if done || cont {
			return out, done, cont, err
		}
		// Bypass succeeded; use the bypass response and updated step for the HITL check.
		response = bypassResp
		step = bypassStep
	}

	result := s.runAutoDispatchHITL(ctx, rs, step, response)
	if result.done {
		return result.outcome, true, false, result.err
	}
	if !result.accepted {
		return domain.RunOutcome{}, false, true, nil
	}
	out, done, err := s.postAutoDispatchApply(ctx, rs, result.hitlStep, result.hitlResponse, result.hitlAttemptSeq, result.written, result.routed, result.routedResp)
	return out, done, false, err
}

// prepareAutoDispatchRequest fills in the auto-routed dispatch request fields:
// task description (with pre-consultation advice), RunID, and run-scoped
// artifact paths. It also logs the dispatch start and sends the progress notice.
func (s *sessionImpl) prepareAutoDispatchRequest(ctx context.Context, rs *runStartCtx, step domain.DispatchStep) domain.DispatchStep {
	if step.Request.TaskDescription == "" {
		step.Request.TaskDescription = domain.GenericTaskDescription
	}
	adv, _ := s.deps.Guidance.Current()
	if adv.TaskDescription != "" {
		step.Request.TaskDescription += "\n\n" + adv.TaskDescription
	}
	if adv.Constraints != "" {
		if step.Request.Constraints != "" {
			step.Request.Constraints += "\n\n" + adv.Constraints
		} else {
			step.Request.Constraints = adv.Constraints
		}
	}
	step.Request.RunID = rs.state.RunID
	folder := domain.RunScopedFolder(rs.state.RunID) + "/"
	step.Request.InputArtifacts = expandInputWildcards(rs.config.RunFolder, rs.state.RunID,
		resolveToRunScoped(step.Request.InputArtifacts, folder))
	step.Request.OutputArtifacts = resolveToRunScoped(step.Request.OutputArtifacts, folder)
	s.deps.Debug.Log(domain.EventSessionDispatchStart, "dispatching step",
		domain.F("agent", step.Request.AgentInstanceID),
		domain.F("phase", step.Phase),
		domain.F("stage", step.Stage),
		domain.F("row", strconv.Itoa(int(domain.WorkflowRowFromIndex(step.RowIndex)))),
	)
	s.deps.Interact.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeInfo,
		Title:   step.Request.AgentInstanceID,
		Message: fmt.Sprintf("phase=%s iteration=%q status=running", step.Phase, iterationNoticeValue(rs.admitted.Table, step.RowIndex, step.Stage)),
	})
	return step
}

// handleAutoHarnessErrorAndBypass handles a harness-level error from an
// auto-routed dispatch. It persists the failed attempt as a HARNESS_ERROR row,
// attempts a raw-text bypass, then lets the engine re-dispatch while the
// HARNESS_ERROR budget lasts and otherwise calls consultRoute.
//
// Returns: (response, step, done, cont, out, err). On bypass success, done and
// cont are both false and the returned response/step are the bypass values.
// On consultRoute handling, cont=true. On terminal outcome, done=true.
func (s *sessionImpl) handleAutoHarnessErrorAndBypass(ctx context.Context, rs *runStartCtx, step domain.DispatchStep, invokeErr error) (domain.ProtocolResponse, domain.DispatchStep, bool, bool, domain.RunOutcome, error) {
	if ctx.Err() != nil {
		return domain.ProtocolResponse{}, step, true, false, domain.RunOutcome{Status: domain.RunStopped, Message: "run stopped: context cancelled"}, nil
	}
	if isForceStopped(invokeErr) {
		done, out, err := s.autoForceStop(ctx, rs, attemptOfStep(step, step.Request), step.Retry == domain.RetryNone)
		return domain.ProtocolResponse{}, step, done, true, out, err
	}
	attempt := failedAttempt{agent: step.Agent, req: step.Request, phase: step.Phase, stage: step.Stage, rowIndex: step.RowIndex}
	harnessResp, recErr := s.recordHarnessFailure(ctx, &rs.state, attempt, invokeErr)
	if recErr != nil {
		return domain.ProtocolResponse{}, step, true, false, domain.RunOutcome{Status: domain.RunFailed, Message: recErr.Error()}, recErr
	}
	rs.seq = rs.state.GlobalSequence
	if s.deps.Routing == nil {
		msg := fmt.Sprintf("harness error: no routing consultant configured: %s", harnessResp.StatusMessage)
		s.deps.Debug.Log(domain.EventSessionDeviationUnresolved, msg)
		return domain.ProtocolResponse{}, step, true, false, domain.RunOutcome{Status: domain.RunDeviationUnresolved, Message: msg}, nil
	}
	// Raw-text bypass: one direct retry (one attempt of the HARNESS_ERROR
	// budget in the mechanical-retry modes) before the next step is decided.
	bypass := s.attemptRawTextBypass(ctx, &rs.state, rs.table, attempt, invokeErr, &rs.antiLoop)
	switch bypass.kind {
	case bypassSucceeded:
		step.Request = bypass.req
		return bypass.resp, step, false, false, domain.RunOutcome{}, nil
	case bypassForceStopped:
		done, out, err := s.autoForceStop(ctx, rs, attemptOfStep(step, bypass.req), true)
		return domain.ProtocolResponse{}, step, done, true, out, err
	case bypassRecordFailed:
		return domain.ProtocolResponse{}, step, true, false, domain.RunOutcome{Status: domain.RunFailed, Message: bypass.err.Error()}, bypass.err
	case bypassFailed:
		rs.seq = rs.state.GlobalSequence
	}
	rs.lastResponse = &harnessResp
	if isMechanicalRetryMode(rs.state.Mode) && harnessErrorBudgetLeft(rs.state, rs.table, step.RowIndex, step.Stage) {
		// Budget remains: the engine decision re-dispatches the same row and stage.
		return domain.ProtocolResponse{}, step, false, true, domain.RunOutcome{}, nil
	}
	deviationInfo := harnessErrorDeviation(attempt, harnessResp, rs.state)
	done, out, outErr := s.consultRoute(ctx, &deviationInfo, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
	return domain.ProtocolResponse{}, step, done, true, out, outErr
}

// postAutoDispatchApply applies the HITL-accepted response to the artifact,
// sends the progress notice, evaluates infrastructure triggers, and performs
// stage-set re-derivation.
func (s *sessionImpl) postAutoDispatchApply(ctx context.Context, rs *runStartCtx, hitlStep domain.DispatchStep, hitlResp domain.ProtocolResponse, hitlAttemptSeq int, written []string, routed *domain.RoutedOutcome, routedResp *domain.ProtocolResponse) (domain.RunOutcome, bool, error) {
	completedStep := domain.CompletedStep{
		Seq:              hitlAttemptSeq,
		AgentInstance:    hitlStep.Request.AgentInstanceID,
		Phase:            hitlStep.Phase,
		Stage:            hitlStep.Stage,
		WorkflowRow:      domain.WorkflowRowFromIndex(hitlStep.RowIndex),
		Status:           hitlResp.StatusCode,
		ErrorCode:        hitlResp.ErrorCode,
		Summary:          hitlResp.StatusMessage,
		Timestamp:        s.deps.Clock.Now(),
		Inputs:           formatInputs(hitlStep.Request.InputArtifacts),
		WrittenArtifacts: written,
		Routed:           routed,
	}
	newState, err := s.applyAndAnnounce(ctx, rs.state, completedStep)
	if err != nil {
		s.deps.Debug.Log(domain.EventSessionApplyFailed, err.Error())
		return domain.RunOutcome{Status: domain.RunFailed, Message: err.Error()}, true, err
	}
	rs.state = newState
	s.deps.Debug.Log(domain.EventSessionStepDone, "step applied to artifact",
		domain.F("agent", completedStep.AgentInstance),
		domain.F("status", string(completedStep.Status)),
		domain.F("error_code", string(completedStep.ErrorCode)),
	)
	rs.seq = hitlAttemptSeq
	rs.lastResponse = &hitlResp
	if routedResp != nil {
		// Routing follows the original attempt, not the repairing re-dispatch.
		rs.lastResponse = routedResp
	}
	rs.lastOutputArtifacts = written
	s.deps.Interact.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeInfo,
		Title:   completedStep.AgentInstance,
		Message: fmt.Sprintf("phase=%s iteration=%q status=%s", completedStep.Phase, iterationNoticeValue(rs.admitted.Table, hitlStep.RowIndex, completedStep.Stage), string(completedStep.Status)),
	})
	orchDir := filepath.Dir(rs.config.OrchestratorFilePath)
	if !completedStep.IsInfrastructure {
		out, done, trigErr := s.postDispatchTriggers(ctx, rs, completedStep, hitlStep.RowIndex, orchDir)
		if done {
			return out, true, trigErr
		}
	}
	// Schedule re-derivation for the blocks this row produces or writes.
	s.rereadAccepted(ctx, rs.admitted, rs.config.RunFolder, hitlStep.RowIndex, hitlStep.Stage,
		hitlResp, &rs.stages)
	return domain.RunOutcome{}, false, nil
}

// postDispatchTriggers evaluates infrastructure-agent triggers after a
// completed workflow step. Returns (out, done, err) where done=true means the
// run is terminal (halt or stop).
func (s *sessionImpl) postDispatchTriggers(ctx context.Context, rs *runStartCtx, completedStep domain.CompletedStep, rowIdx int, orchDir string) (domain.RunOutcome, bool, error) {
	halt, trigStopped, reviewConsult, trigErr := s.evaluateTriggers(
		ctx, &rs.state, &rs.seq, completedStep, rs.prevWorkflowStep,
		rs.declaredInfraAgents, rs.config,
		buildActiveAgentsFilter(rs.declaredInfraAgents, rs.config.InfraClassSelections),
		orchDir, rowIdx, rs.admitted, rs.stages,
		true, // only HITL-accepted steps reach trigger evaluation
	)
	if trigErr != nil {
		if ctx.Err() != nil {
			return domain.RunOutcome{Status: domain.RunStopped, Message: "run stopped: context cancelled"}, true, nil
		}
		return domain.RunOutcome{Status: domain.RunFailed, Message: trigErr.Error()}, true, trigErr
	}
	if trigStopped {
		return domain.RunOutcome{Status: domain.RunStopped, Message: "run stopped: graceful stop confirmed"}, true, nil
	}
	if halt {
		if done, out, handled, err := s.followUpInfraForceStop(&rs.lastResponse, func() (bool, domain.RunOutcome, error) {
			return s.routeFromRS(ctx, rs)
		}); handled {
			return out, done, err
		}
		return domain.RunOutcome{Status: domain.RunStopped, Message: "infrastructure agent halted the run"}, true, nil
	}
	cp := completedStep
	rs.prevWorkflowStep = &cp
	onInfrastructureAgentTrigger()
	if s.deps.OnInfrastructureTrigger != nil {
		s.deps.OnInfrastructureTrigger()
	}
	if reviewConsult != nil && s.deps.Routing != nil {
		syntheticResp := domain.ProtocolResponse{StatusMessage: reviewConsult.StatusMessage}
		rs.lastResponse = &syntheticResp
		done, out, consultErr := s.consultRoute(ctx, nil, &rs.state, &rs.seq,
			&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
			rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
		if consultErr != nil {
			return domain.RunOutcome{Status: domain.RunFailed, Message: consultErr.Error()}, true, consultErr
		}
		if done {
			return out, true, nil
		}
	}
	return domain.RunOutcome{}, false, nil
}
