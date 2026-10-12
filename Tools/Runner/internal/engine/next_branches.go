package engine

import (
	"mosaic-run/internal/domain"
)

// orchestratedDecision is the decision of ExecutionModeOrchestrated: the
// engine never routes, it always hands the decision to the consultant.
func orchestratedDecision(in NextInput) domain.EngineDecision {
	state := in.State
	if state.CurrentState.LastAgent == "" {
		// First decision of a new run: no row has been dispatched yet.
		return domain.EngineDecision{Consult: &domain.ConsultDecision{
			Trigger:       domain.ConsultTriggerOrchestratedMode,
			CurrentRow:    -1,
			ArtifactState: state,
		}}
	}
	// Subsequent call: derive position from state. Ignore currentPosition
	// errors -- the orchestrator reads the artifact directly and does not depend
	// on the engine's row resolution.
	pos, _ := currentPosition(in.Workflow, state)
	currentRowIdx := pos.rowIdx
	return domain.EngineDecision{Consult: &domain.ConsultDecision{
		Trigger:       domain.ConsultTriggerOrchestratedMode,
		CurrentRow:    currentRowIdx,
		CurrentPhase:  state.CurrentState.Phase,
		CurrentStage:  state.CurrentState.Stage,
		ArtifactState: state,
	}}
}

// nonSuccessDecision decides after a non-SUCCESS response at currentRow:
// the auto-review findings route-back, the review-loop-limit deviation, or
// the generic non-SUCCESS deviation.
func nonSuccessDecision(
	in NextInput,
	sc ScheduleContext,
	pos position,
	currentRow domain.RoutingRow,
	status domain.StatusCode,
	resp domain.ProtocolResponse,
) domain.EngineDecision {
	state := in.State
	currentRowIdx := pos.rowIdx
	if dec, ok := mechanicalRetry(in, sc, pos, status); ok {
		return dec
	}

	// COMPLETED_NEEDS_ACTION with an On Findings target -> loop-back dispatch.
	// This auto-route fires only in auto-review mode; in auto mode it deviates.
	isFindingsLoop := false
	if in.Mode == domain.ExecutionModeAutoReview && status == domain.StatusCOMPLETED_NEEDS_ACTION {
		switch currentRow.Findings.Kind {
		case domain.FindingsTarget:
			isFindingsLoop = true
		case "":
			return unroutableStop(currentRowIdx, columnOnFindings, "")
		}
	}
	if isFindingsLoop && !reviewLoopLimitReached(in.Workflow, state, pos) {
		return findingsRouteBack(in, sc, pos, currentRow)
	}

	kind := domain.DeviationNonSuccess
	if isFindingsLoop {
		// Review loop limit reached in auto-review mode: deviate instead of routing.
		kind = domain.DeviationReviewLoopLimit
	}
	return domain.EngineDecision{Deviation: &domain.DeviationDecision{
		Info: domain.DeviationInfo{
			Kind:          kind,
			Response:      resp,
			CurrentRow:    currentRowIdx,
			CurrentPhase:  currentRow.Phase,
			CurrentStage:  pos.stage,
			ArtifactState: state,
		},
	}}
}

// findingsRouteBack builds the auto-review loop-back dispatch to the row the
// typed On Findings entry names. A target in the block of the current row keeps
// the current iteration; any other target runs outside an iteration.
func findingsRouteBack(
	in NextInput,
	sc ScheduleContext,
	pos position,
	currentRow domain.RoutingRow,
) domain.EngineDecision {
	targetRowIdx := currentRow.Findings.Target.Index
	if targetRowIdx < 0 || targetRowIdx >= len(in.Workflow.Table.Rows) {
		return unroutableStop(pos.rowIdx, columnOnFindings, string(domain.FindingsTarget))
	}
	var iteration domain.IterationID
	if b, ok := in.Workflow.BlockOf(pos.rowIdx); ok && b.Contains(targetRowIdx) {
		iteration = pos.iteration
	}
	step, err := buildDispatchStep(in.Workflow, targetRowIdx, iteration,
		in.Agents, in.Seq, in.Now, sc)
	if err != nil {
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: err.Error()}}
	}
	// Inject the reviewing agent's output artifacts into the dispatched
	// step's InputArtifacts. Entries already present in the table row's
	// resolved list are not duplicated.
	step.Request.InputArtifacts = injectReviewArtifacts(in.State.RunID,
		step.Request.InputArtifacts, in.LastOutputArtifacts)
	step.Request.InputArtifacts = injectCreatorArtifacts(in.State.RunID,
		step.Request.InputArtifacts, step.Request.OutputArtifacts, in.ArtifactRegistry)
	return domain.EngineDecision{Dispatch: &domain.DispatchDecision{
		Steps: []domain.DispatchStep{step},
	}}
}

// injectCreatorArtifacts adds the creator agent's own previously-produced
// output artifacts from the registry to inputs. The comparison is against the
// step's resolved output paths (not the raw row output patterns, which may
// contain unresolved template tokens), so injection fires correctly for rows
// with templated output artifact paths.
func injectCreatorArtifacts(
	runID string,
	inputs, resolvedOutputs []string,
	registry []domain.ArtifactRegistryEntry,
) []string {
	if len(registry) == 0 {
		return inputs
	}
	outputSet := make(map[string]bool, len(resolvedOutputs))
	for _, oa := range resolvedOutputs {
		outputSet[domain.StripRunPrefix(runID, oa)] = true
	}
	var creatorArts []string
	for _, entry := range registry {
		if outputSet[domain.StripRunPrefix(runID, entry.Artifact)] {
			creatorArts = append(creatorArts, entry.Artifact)
		}
	}
	if len(creatorArts) == 0 {
		return inputs
	}
	return injectReviewArtifacts(runID, inputs, creatorArts)
}
