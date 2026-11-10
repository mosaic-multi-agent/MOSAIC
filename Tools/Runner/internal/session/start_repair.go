package session

import (
	"context"
	"errors"
	"fmt"

	"mosaic-run/internal/domain"
)

// resumeArtifactRefusal applies the checks a resumed artifact must pass before
// the run continues: its run identity, its dispatch value, its recorded
// iteration values and its workflow version. refused is true when one fails.
func (s *sessionImpl) resumeArtifactRefusal(rs *runStartCtx, state domain.ArtifactState) (outcome domain.RunOutcome, refused bool) {
	if idErr := checkResumeRunIdentity(state.RunID, rs.config); idErr != nil {
		return s.runIdentityRefusal(idErr, rs.config), true
	}
	if d := state.Dispatch; d.Present && d.Value != domain.DispatchSequential {
		return s.refusal(fmt.Sprintf(
			"run artifact records dispatch %q; this Runner runs sequentially and supports only \"dispatch: %s\"",
			d.Value, domain.DispatchSequential,
		)), true
	}
	if valueErr := resumeValueRefusal(state, rs.table, rs.config.RunID); valueErr != nil {
		return s.refusalCaused(valueErr.Error(), valueErr), true
	}
	// Version check for resume mode.
	if !rs.config.AllowVersionDrift && state.WorkflowVersion != rs.region.Info.Version {
		return s.refusal(fmt.Sprintf(
			"workflow version mismatch: artifact has %q, selected workflow has %q",
			state.WorkflowVersion, rs.region.Info.Version,
		)), true
	}
	return domain.RunOutcome{}, false
}

// damagedBeforeWorkflow reports the damage a run whose workflow the frontend
// could not name has, as a refusal; ok is false when the artifact is not damaged.
func (s *sessionImpl) damagedBeforeWorkflow(ctx context.Context) (domain.RunOutcome, bool) {
	_, err := s.deps.Store.Read(ctx)
	var damage *domain.ArtifactDamageError
	if !errors.As(err, &damage) {
		return domain.RunOutcome{}, false
	}
	return s.refusalCaused(damage.Error(), damage), true
}

// deferStartRepair notes the damage a resume found at the run-start read. The
// run identity the frontmatter records is checked first: a refused identity
// is never repaired. The repair consultation itself is made later, when the
// block schedules are read.
func (s *sessionImpl) deferStartRepair(rs *runStartCtx, damage *domain.ArtifactDamageError) (domain.RunOutcome, bool, error) {
	// A damaged frontmatter records no run identity to check; the identity of the
	// repaired artifact is checked after the repair.
	if damage.Frontmatter != nil {
		if idErr := checkResumeRunIdentity(damage.Frontmatter.RunID, rs.config); idErr != nil {
			return s.runIdentityRefusal(idErr, rs.config), true, nil
		}
	}
	rs.startDamage = damage
	return domain.RunOutcome{}, false, nil
}

// repairReadDamage makes the repair consultation for the damage the run-start
// read found. It leaves rs.existingState holding the repaired artifact.
func (s *sessionImpl) repairReadDamage(ctx context.Context, rs *runStartCtx) (domain.RunOutcome, bool, error) {
	damage := rs.startDamage
	rs.startDamage = nil
	state, outcome, ended := s.repairStartDamage(ctx, rs, damage, damageStand(rs.config, damage))
	if ended {
		return outcome, true, nil
	}
	rs.existingState = state
	return domain.RunOutcome{}, false, nil
}

// damageStand is the state a start repair request is built from: the values an
// intact frontmatter records, otherwise only the run's workflow and mode from
// the run configuration, which the repaired artifact must keep.
func damageStand(config domain.RunConfig, damage *domain.ArtifactDamageError) domain.ArtifactState {
	if damage.Frontmatter != nil {
		return *damage.Frontmatter
	}
	stand := domain.ArtifactState{Workflow: config.WorkflowID}
	stand.Mode = config.RunSettings.Mode
	return stand
}

// repairStartDamage asks the orchestrator to repair the damaged artifact of a
// resumed run through the repair cycle. stand is the state the request is built
// from while the artifact cannot be read. The trigger fields of the request are
// null. On acceptance the repaired artifact is read again, the resume checks
// run against it and the reply is kept for the run to carry out instead of a
// resume position; ended is true with the outcome that ends the run otherwise.
func (s *sessionImpl) repairStartDamage(
	ctx context.Context, rs *runStartCtx, damage *domain.ArtifactDamageError, stand domain.ArtifactState,
) (state domain.ArtifactState, outcome domain.RunOutcome, ended bool) {
	req := routingRequest(rs.config, nil, stand, &rs.stages, nil)
	reply, outcome, ended := s.repairDamage(ctx, req, rs.config, &damagedConsultation{damage: damage})
	if ended {
		return stand, outcome, true
	}
	state, err := s.deps.Store.Read(ctx)
	if err != nil {
		return stand, domain.RunOutcome{Status: domain.RunFailed, Message: "artifact unreadable after the repair: " + err.Error(), Cause: err}, true
	}
	if refusal, refused := s.resumeArtifactRefusal(rs, state); refused {
		return stand, refusal, true
	}
	if refusal, changed := s.repairChangedRun(stand, state); changed {
		return stand, refusal, true
	}
	rs.startRepair = &reply
	return state, domain.RunOutcome{}, false
}

// repairChangedRun refuses a repair that changed the run's workflow or mode:
// a repair restores the artifact, it does not re-decide how the run runs.
func (s *sessionImpl) repairChangedRun(before, after domain.ArtifactState) (domain.RunOutcome, bool) {
	if before.Workflow != "" && after.Workflow != before.Workflow {
		return s.refusal(fmt.Sprintf("the repaired artifact records workflow %q; the run was started with %q",
			after.Workflow, before.Workflow)), true
	}
	if before.Mode != domain.ExecutionModeUnset && after.Mode != before.Mode {
		return s.refusal(fmt.Sprintf("the repaired artifact records mode %q; the run was started with %q",
			after.Mode, before.Mode)), true
	}
	return domain.RunOutcome{}, false
}

// writeRepairingDamage performs a start-sequence artifact write of a resumed
// run. When the store refuses it because the artifact is damaged and the start
// has not been repaired yet, the damage is repaired and the write made once more.
// A second refusal is returned as the write's error: one start repairs once.
func (s *sessionImpl) writeRepairingDamage(
	ctx context.Context, rs *runStartCtx, stand domain.ArtifactState, write func() (domain.ArtifactState, error),
) (state domain.ArtifactState, outcome domain.RunOutcome, ended bool, err error) {
	state, err = write()
	var damage *domain.ArtifactDamageError
	if err == nil || rs.config.IsNewRun || rs.startRepair != nil || !errors.As(err, &damage) {
		return state, domain.RunOutcome{}, false, err
	}
	if _, outcome, ended = s.repairStartDamage(ctx, rs, damage, stand); ended {
		return stand, outcome, true, nil
	}
	state, err = write()
	return state, domain.RunOutcome{}, false, err
}

// resumeFromRepair continues a resume from the accepted start repair: its reply
// says where the run goes on, so no resume position is computed. A stop reply
// is a consultant stop.
func (s *sessionImpl) resumeFromRepair(rs *runStartCtx) (domain.RunOutcome, bool, error) {
	reply := *rs.startRepair
	if outcome, ended := s.endingInstruction(reply); ended {
		return outcome, true, nil
	}
	// The reply is validated against the table and schedules the run uses; a
	// repair that names an unusable target ends the run.
	if _, _, invalid, outcome := s.resolveConsultTarget(rs.table, reply.Dispatch, rs.agents, rs.admitted, rs.stages); invalid {
		outcome.Status = domain.RunFailed
		return outcome, true, nil
	}
	rs.state = rs.existingState
	rs.seq = rs.state.GlobalSequence
	rs.repairDispatch = reply.Dispatch
	return domain.RunOutcome{}, false, nil
}

// runRepairDispatch carries out the dispatch the accepted start repair
// returned, once, before the engine is asked for a step.
func (s *sessionImpl) runRepairDispatch(ctx context.Context, rs *runStartCtx) (out domain.RunOutcome, done bool, handled bool, err error) {
	if rs.repairDispatch == nil {
		return domain.RunOutcome{}, false, false, nil
	}
	dispatch := rs.repairDispatch
	rs.repairDispatch = nil
	done, out, err = s.consultCarryOut(ctx, dispatch, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
	return out, done, true, err
}
