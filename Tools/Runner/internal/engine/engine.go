// Package engine is the pure, side-effect-free decision core of the runner.
// It answers "what runs next" given the admitted workflow, stage set, and
// current artifact state.
//
// Purity constraint: Next and ResumePoint import no I/O packages and never
// call time.Now(), os.*, net.*, os/exec.*, or math/rand.*. All time-varying
// inputs arrive as parameters. The time package is imported only for the
// time.Time parameter type; no system-clock reads occur.
package engine

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// Next is the routing decision. It is pure: no I/O, no clock read, no
// filesystem access, no randomness. Every mode-conditional input arrives in
// NextInput.
//
// Behaviour by in.Mode:
//
//	ExecutionModeOrchestrated — Next never produces a routing decision.
//	  It returns EngineDecision{Consult: &ConsultDecision{Trigger:
//	  ConsultTriggerOrchestratedMode, ...}} for every call, including the
//	  first call of a new run.
//
//	ExecutionModeAuto — On StatusSUCCESS, routes by the row's typed On Success
//	  entry in the run scope and by the block's own progression inside a
//	  block (a block row's On Success is not read). After PARTIALLY_DONE (within
//	  PartiallyDoneRedispatchLimit) and BLOCKED with error code E501 (within
//	  E501AttemptLimit) it re-dispatches the same assignment mechanically. On
//	  any other non-SUCCESS status, including StatusCOMPLETED_NEEDS_ACTION, and
//	  once a bound or budget is used up, it returns a DeviationDecision. The
//	  On-Findings auto-route never fires.
//
//	ExecutionModeAutoReview — As ExecutionModeAuto, plus: on
//	  StatusCOMPLETED_NEEDS_ACTION where the row's typed On Findings entry
//	  names a row, returns a DispatchDecision targeting that row,
//	  with in.LastOutputArtifacts injected into the dispatched step's
//	  Request.InputArtifacts (see review artifact injection below). When the
//	  reviewer's COMPLETED_NEEDS_ACTION count at the current phase and stage
//	  reaches state.ReviewLoopLimit (0 = no limit), the engine returns a
//	  DeviationReviewLoopLimit deviation instead of auto-routing.
//
// The On Findings target is the row the typed entry names; a "-" entry has no
// target and deviates. A route-back to a row of the same block keeps the current
// iteration.
//
// Review artifact injection applies to the auto-review COMPLETED_NEEDS_ACTION
// auto-route-back and to nothing else. A SUCCESS-routed dispatch never carries
// injected artifacts. The dispatched step's InputArtifacts are the table row's
// entries in table order, followed by the entries of in.LastOutputArtifacts
// that are not already present, in their given order. Comparison is on the
// exact path string as dispatched.
//
// DispatchDecision.Steps continues to hold exactly one element.
func Next(in NextInput) domain.EngineDecision {
	// Unpack for readability.
	workflow := in.Workflow
	sc := ScheduleContext{Schedules: in.Schedules}
	state := in.State
	lastResponse := in.LastResponse

	// ExecutionModeUnset is a programming error: surface it as an ambiguous-route
	// deviation rather than silently picking a mode or panicking.
	if in.Mode == domain.ExecutionModeUnset {
		return domain.EngineDecision{Deviation: &domain.DeviationDecision{
			Info: domain.DeviationInfo{
				Kind:          domain.DeviationAmbiguousRoute,
				ArtifactState: state,
			},
		}}
	}

	// ExecutionModeOrchestrated: the engine never produces a routing decision.
	if in.Mode == domain.ExecutionModeOrchestrated {
		return orchestratedDecision(in)
	}

	// No prior invocations: initial dispatch.
	if state.CurrentState.LastAgent == "" {
		return initialDispatch(in, sc)
	}

	// Determine the response status and the response for deviation assembly.
	var status domain.StatusCode
	var resp domain.ProtocolResponse
	if lastResponse != nil {
		status = lastResponse.StatusCode
		resp = *lastResponse
	} else {
		status = state.CurrentState.LastStatus
	}

	// Locate the row that was last completed.
	// Check for a routing error (e.g. unresolvable approach) before the generic
	// "could not determine current row" check -- the approach error is more specific.
	pos, rowFindErr := currentPosition(workflow, state)
	if rowFindErr != nil {
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: rowFindErr.Error(), Err: rowFindErr}}
	}
	currentRowIdx := pos.rowIdx
	if currentRowIdx < 0 {
		return domain.EngineDecision{Stop: &domain.StopDecision{
			Reason: fmt.Sprintf("could not determine current row from artifact state (LastAgent=%q)",
				state.CurrentState.LastAgent),
		}}
	}
	currentRow := workflow.Table.Rows[currentRowIdx]

	if status != domain.StatusSUCCESS {
		return nonSuccessDecision(in, sc, pos, currentRow, status, resp)
	}

	// SUCCESS: route by the row's scope.
	return routeSuccess(in, sc, pos, currentRow, resp)
}
