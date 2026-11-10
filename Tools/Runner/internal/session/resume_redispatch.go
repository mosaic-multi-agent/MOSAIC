package session

import (
	"context"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// prepareResumeRedispatch derives the re-dispatch of a trailing unaccepted
// attempt from the resumed state and stores it on rs for the dispatch loop.
// current_state stays exactly as recorded. When the last attempt was accepted
// (or there is none) nothing is stored and the loop starts with the engine.
func (s *sessionImpl) prepareResumeRedispatch(rs *runStartCtx, state domain.ArtifactState) error {
	step, ok, err := engine.UnacceptedAttemptRedispatch(engine.RedispatchInput{
		Workflow:  rs.admitted,
		Schedules: rs.stages,
		State:     state,
		Infra:     domain.NewInfraAgentSet(rs.declaredInfraAgents, s.orchRef.Identifier),
		Agents:    rs.agents,
		Seq:       rs.seq,
	})
	if err != nil {
		return err
	}
	if ok {
		rs.resumeRedispatch = &step
	}
	return nil
}

// runResumeRedispatch dispatches the pending resume re-dispatch, if any,
// through the normal engine-dispatch path so that its outcome is routed as in a
// live run. handled is false when there was nothing to re-dispatch. Otherwise
// done and cont have the meaning they have for handleEngineDispatch.
func (s *sessionImpl) runResumeRedispatch(ctx context.Context, rs *runStartCtx) (out domain.RunOutcome, done bool, handled bool, err error) {
	if rs.resumeRedispatch == nil {
		return domain.RunOutcome{}, false, false, nil
	}
	step := *rs.resumeRedispatch
	rs.resumeRedispatch = nil
	decision := domain.EngineDecision{Dispatch: &domain.DispatchDecision{Steps: []domain.DispatchStep{step}}}
	out, done, _, err = s.handleEngineDispatch(ctx, rs, decision)
	return out, done, true, err
}
