package session

import (
	"context"
	"strconv"

	"mosaic-run/internal/domain"
)

// reviewConsultSignal carries the status message from the last successful
// review-class infrastructure agent in an evaluateTriggers pass. When
// non-nil, the caller must perform a routing consultation via consultRoute,
// supplying StatusMessage as last_status_message.
type reviewConsultSignal struct {
	StatusMessage string
}

// lastInfraSeqInLog returns the Seq of the most recent execution log entry
// whose agent identifier matches agentName. Returns -1 when no matching entry
// is found (the agent has never been dispatched).
func lastInfraSeqInLog(agentName string, log []domain.ExecutionLogEntry) int {
	for i := len(log) - 1; i >= 0; i-- {
		if extractAgentIdentifier(log[i].Agent) == agentName {
			return log[i].Seq
		}
	}
	return -1
}

// infraTriggerFires reports whether the given trigger should fire after the
// workflow step described by completedStep.
//
// currentSeq is the global sequence after the most recent Store.Apply call
// (including any infra dispatches that have already completed during this
// evaluation pass). log is the current execution log, used for
// INVOCATION_INTERVAL interval arithmetic.
//
// boundaries carries the completed step's table row, the admitted workflow,
// the schedule and the run mode; ITERATION_END and PHASE_END are answered from them
// and from log, which holds the completed step.
//
// ITERATION_END and PHASE_END additionally require that the completed step returned
// SUCCESS and that hitlAccepted is true (the step passed HITL verification
// where HITL applies). Interval and manual triggers ignore both conditions.
func infraTriggerFires(
	trigger domain.DeclaredInfraTrigger,
	currentSeq int,
	log []domain.ExecutionLogEntry,
	agentName string,
	completedStep domain.CompletedStep,
	boundaries triggerBoundaries,
	hitlAccepted bool,
) bool {
	switch trigger.Trigger {
	case domain.InfraTriggerInvocationInterval:
		param, convErr := strconv.Atoi(trigger.Param)
		if convErr != nil || param <= 0 {
			return false
		}
		lastSeq := lastInfraSeqInLog(agentName, log)
		if lastSeq < 0 {
			// No prior dispatch row: fires when global_sequence >= param.
			return currentSeq >= param
		}
		return (currentSeq - lastSeq) >= param

	case domain.InfraTriggerIterationEnd:
		if !boundaryConditionsMet(completedStep, hitlAccepted) {
			return false
		}
		// Prospective semantics: fires when the completed step is the last
		// step of its stage (look-ahead), not by comparing against a prior step.
		// ITERATION_END only applies to EXECUTION-phase steps (Stage != ""); the
		// comparison is a presence check, not a split of the value.
		if completedStep.Stage == "" {
			return false
		}
		value, readErr := boundaries.readValue(completedStep.Stage)
		if readErr != nil {
			return false
		}
		return boundaries.endsIteration(value.ID, log)

	case domain.InfraTriggerPhaseEnd:
		if !boundaryConditionsMet(completedStep, hitlAccepted) {
			return false
		}
		// Prospective semantics: fires when the completed step is the last
		// step of its phase (look-ahead), not by comparing against a prior step.
		// An unreadable value leaves the zero value: a step outside every
		// block records none, and the phase boundary is then judged without
		// an iteration. (ITERATION_END above returns false instead: it needs one.)
		value, _ := boundaries.readValue(completedStep.Stage)
		return boundaries.endsPhase(value.ID, log)

	case domain.InfraTriggerManual:
		// MANUAL triggers never fire automatically.
		return false

	default:
		return false
	}
}

// boundaryConditionsMet reports whether a completed step qualifies to fire
// ITERATION_END or PHASE_END: its routed status must be SUCCESS and it must have
// been accepted by HITL verification. Rejected attempts and non-SUCCESS
// routed outcomes never fire; a gate-discharging re-dispatch is judged by the
// original attempt's status.
func boundaryConditionsMet(step domain.CompletedStep, hitlAccepted bool) bool {
	return hitlAccepted && step.RoutedStatus() == domain.StatusSUCCESS
}

// evaluateTriggers checks all declared infrastructure agent triggers against
// the current artifact state after a workflow step completes. Agents are
// evaluated in declaration order; each agent fires at most once per
// evaluation even if multiple triggers match.
//
// hitlAccepted carries the HITL verification outcome of completedStep; it gates
// ITERATION_END and PHASE_END only.
//
// Dispatch is performed synchronously: each matching agent's invocation
// completes (including its Execution Log row via Store.Apply) before the
// next declared agent's triggers are evaluated.
//
// Returns (true, false, nil, nil) when an on_failure=halt agent stops the run.
// Returns (false, false, nil, non-nil) on unexpected infrastructure errors.
// Returns (false, true, nil, nil) when a graceful stop is confirmed between two
// declared agents' dispatches within this evaluation pass.
// Returns (false, false, non-nil, nil) when all evaluations complete and at
// least one review-class agent succeeded.
// Returns (false, false, nil, nil) when all evaluations complete without a
// halt, stop, or successful review.
func (s *sessionImpl) evaluateTriggers(
	ctx context.Context,
	state *domain.ArtifactState,
	seq *int,
	completedStep domain.CompletedStep,
	prevWorkflowStep *domain.CompletedStep,
	declared []domain.DeclaredInfraAgent,
	config domain.RunConfig,
	activeAgents map[string]bool,
	orchDir string,
	rowIdx int,
	admitted domain.AdmittedWorkflow,
	stages domain.BlockSchedules,
	hitlAccepted bool,
) (haltRun bool, stopRequested bool, reviewConsult *reviewConsultSignal, err error) {
	var reviewSignal *reviewConsultSignal
	for _, agent := range declared {
		// Restore-class agents are never dispatched by automatic trigger
		// evaluation; they act only on explicit manual instruction (MANUAL
		// trigger). The exclusion is by class so that any restore-class agent
		// (e.g. a future checkpoint-restore-s3) is automatically excluded
		// without a code change.
		if agent.Class == "restore" {
			continue
		}

		// activeAgents filter: when non-nil, only listed agents are evaluated.
		if activeAgents != nil && !activeAgents[agent.Name] {
			continue
		}

		// Activation gating: checkpoint-class agents require checkpoints to be
		// enabled for the run. Other classes are always active.
		if agent.Class == "checkpoint" && !config.Checkpoints {
			continue
		}

		// Collect every declared trigger that fires; the agent is still
		// dispatched once per evaluation pass, carrying all of them.
		boundaries := triggerBoundaries{Workflow: admitted, Stages: stages, RowIndex: rowIdx, Mode: config.Mode}
		var matched []string
		for _, trigger := range agent.Triggers {
			if infraTriggerFires(trigger, *seq, state.ExecutionLog, agent.Name, completedStep, boundaries, hitlAccepted) {
				matched = append(matched, trigger.Trigger)
			}
		}
		if len(matched) == 0 {
			continue
		}

		halt, stop, rev, trigErr := s.dispatchInfraAgent(ctx, state, seq, completedStep, agent, matched, config, orchDir, boundaries)
		if trigErr != nil {
			return false, false, nil, trigErr
		}
		if stop {
			return false, true, nil, nil
		}
		if halt {
			return true, false, nil, nil
		}
		if rev != nil {
			reviewSignal = rev
		}
		_ = prevWorkflowStep // kept for future use
	}
	return false, false, reviewSignal, nil
}
