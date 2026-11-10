package engine_test

// New-API counterparts of the stage-set helpers in engine_helpers_test.go and
// row_drift_helpers_test.go: they build per-block schedules with iteration IDs
// (block Stage, IDs written as plain numbers) and drive Next and ResumePoint
// through the schedule-based API.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// newTestSchedules returns the schedules of the one block Stage holding the
// given iterations in schedule order.
func newTestSchedules(iterations ...domain.ScheduleIteration) domain.BlockSchedules {
	return domain.BlockSchedules{"Stage": domain.BlockSchedule{
		Block:      "Stage",
		Source:     "Plan.md",
		Section:    "Stages",
		Iterations: iterations,
	}}
}

// singleSchedules returns schedules with one iteration "1" of the given approach.
func singleSchedules(approach domain.Approach) domain.BlockSchedules {
	return newTestSchedules(domain.ScheduleIteration{ID: "1", Approach: approach})
}

// twoSchedules returns schedules with iterations "1" and "2", both of the
// given approach.
func twoSchedules(approach domain.Approach) domain.BlockSchedules {
	return newTestSchedules(
		domain.ScheduleIteration{ID: "1", Approach: approach},
		domain.ScheduleIteration{ID: "2", Approach: approach},
	)
}

// nextAfterLogIDs calls engine.Next in auto-review mode for the step that
// ended the log, with the given schedules.
func nextAfterLogIDs(
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	log *runLog,
) domain.EngineDecision {
	last := log.last()
	return engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        log.state(),
		LastResponse: responseFor(last),
		Agents:       agents,
		Seq:          last.Seq,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})
}

// engineNextAutoIDs calls engine.Next in auto-review mode with an explicit
// state and the given schedules.
func engineNextAutoIDs(
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	state domain.ArtifactState,
	resp *domain.ProtocolResponse,
) domain.EngineDecision {
	return engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: resp,
		Agents:       buildVerifiedAgents(),
		Seq:          state.GlobalSequence,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})
}

// requireNextStepIDs calls nextAfterLogIDs and requires a single dispatch.
func requireNextStepIDs(
	t *testing.T,
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	log *runLog,
) domain.DispatchStep {
	t.Helper()
	return requireDispatch(t, nextAfterLogIDs(aw, schedules, agents, log))
}

// resumeLogIDs calls ResumePointFromSchedules with the checkpoint
// infrastructure agent declared.
func resumeLogIDs(
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	log *runLog,
	interrupted bool,
) (domain.ResumeInfo, error) {
	infra := domain.NewInfraAgentSet(declaredCheckpointInfraAgent())
	return engine.ResumePointFromSchedules(aw, schedules, resumeState(log, interrupted), infra)
}
