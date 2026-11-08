package session

import (
	"context"

	"mosaic-run/internal/domain"
)

// handleRepairHarnessFailure handles a harness error of a HITL repair
// re-dispatch, identically on the engine and consultant paths. The failed
// repair is recorded as a HARNESS_ERROR row, then one raw-text bypass runs when
// the error is eligible and the anti-loop guard admits the dispatch; the
// HARNESS_ERROR budget is not checked here. A failed bypass is recorded as its
// own row. The returned response is the failed repair's HARNESS_ERROR response,
// which any following consultation sees. The caller decides what follows a
// bypass that did not succeed: a consultation at once, never a mechanical
// re-dispatch.
func (s *sessionImpl) handleRepairHarnessFailure(ctx context.Context, state *domain.ArtifactState, a failedAttempt, invokeErr error, antiLoop *antiLoopState) (domain.ProtocolResponse, bypassOutcome, error) {
	resp, err := s.recordHarnessFailure(ctx, state, a, invokeErr)
	if err != nil {
		return domain.ProtocolResponse{}, bypassOutcome{}, err
	}
	if !isRawTextHarnessError(invokeErr) || !antiLoop.recordDispatch(a.rowIndex, a.agent.Identifier) {
		return resp, bypassOutcome{kind: bypassSkipped}, nil
	}
	return resp, s.runRawTextBypass(ctx, state, a), nil
}

// consultRepairHarnessFailure handles a harness error of a HITL repair
// re-dispatch on the consultant path. It returns the consultHITLRedispatch
// result tuple: a successful bypass hands its response back to the HITL loop;
// every other outcome consults at once with a harness-error deviation (cont).
func (s *sessionImpl) consultRepairHarnessFailure(
	ctx context.Context,
	failed failedAttempt,
	rdErr error,
	state *domain.ArtifactState,
	seq *int,
	lastResponse **domain.ProtocolResponse,
	prevWorkflowStep **domain.CompletedStep,
	stages *domain.BlockSchedules,
	table domain.RoutingTable,
	agents map[string]domain.AgentReference,
	config domain.RunConfig,
	declaredInfraAgents []domain.DeclaredInfraAgent,
	admitted domain.AdmittedWorkflow,
	antiLoop *antiLoopState,
) (domain.ProtocolResponse, int, bool, bool, domain.RunOutcome, error) {
	harnessResp, bypass, err := s.handleRepairHarnessFailure(ctx, state, failed, rdErr, antiLoop)
	*seq = state.GlobalSequence
	failRun := func(e error) (domain.ProtocolResponse, int, bool, bool, domain.RunOutcome, error) {
		return domain.ProtocolResponse{}, 0, false, true, domain.RunOutcome{Status: domain.RunFailed, Message: e.Error()}, e
	}
	if err != nil {
		return failRun(err)
	}
	consult := func(info *domain.DeviationInfo) (bool, domain.RunOutcome, error) {
		return s.consultRoute(ctx, info, state, seq, lastResponse, prevWorkflowStep,
			stages, table, agents, config, declaredInfraAgents, admitted, antiLoop)
	}
	switch bypass.kind {
	case bypassSucceeded:
		return bypass.resp, bypass.seq, false, false, domain.RunOutcome{}, nil
	case bypassRecordFailed:
		return failRun(bypass.err)
	case bypassForceStopped:
		attempt := forcedAttempt{req: bypass.req, phase: failed.phase, stage: failed.stage, row: domain.WorkflowRowFromIndex(failed.rowIndex)}
		fDone, fOut, fErr := s.forceStopAttempt(ctx, attempt, true, state, seq, lastResponse, antiLoop,
			func() (bool, domain.RunOutcome, error) { return consult(nil) })
		return domain.ProtocolResponse{}, 0, true, fDone, fOut, fErr
	}
	*lastResponse = &harnessResp
	info := harnessErrorDeviation(failed, harnessResp, *state)
	info.Kind = domain.DeviationHarnessError
	crDone, crOut, crErr := consult(&info)
	return domain.ProtocolResponse{}, 0, true, crDone, crOut, crErr
}
