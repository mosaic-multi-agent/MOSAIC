package session

import (
	"context"
	"fmt"

	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
)

// dispatchInfraAgent dispatches a single infrastructure agent that has fired
// its trigger (matched names every trigger that matched in this pass) and
// records the result in the Execution Log. It returns (halt, stop,
// reviewSignal, err) matching the semantics of evaluateTriggers.
func (s *sessionImpl) dispatchInfraAgent(
	ctx context.Context,
	state *domain.ArtifactState,
	seq *int,
	completedStep domain.CompletedStep,
	agent domain.DeclaredInfraAgent,
	matched []string,
	config domain.RunConfig,
	orchDir string,
	boundaries triggerBoundaries,
) (haltRun bool, stopRequested bool, reviewSignal *reviewConsultSignal, err error) {
	infraSeq := *seq + 1
	instanceID := fmt.Sprintf("%s#%d", agent.Name, infraSeq)
	req := buildInfraRequest(agent, matched, instanceID, state.RunID, config.RunFolder, completedStep, boundaries)

	agentRef, resolveErr := agentresolve.ResolveOne(orchDir, agent.Name)
	var response domain.ProtocolResponse
	if resolveErr != nil {
		// Cannot locate the agent definition file; treat as non-SUCCESS and
		// apply the on_failure policy without dispatching.
		response = domain.HarnessErrorResponse(req.AgentInstanceID, state.RunID, resolveErr)
	} else {
		// Graceful-stop checkpoint before dispatching.
		if s.deps.StopRequested() {
			s.deps.Debug.Log(domain.EventSessionStopObserved, "graceful stop observed; not dispatching",
				domain.F("checkpoint", StopCheckpointInfraDispatch),
			)
			return false, true, nil, nil
		}

		var invokeErr error
		response, invokeErr = s.invokeAndLog(ctx, agentRef, req)
		if invokeErr != nil {
			if ctx.Err() != nil {
				return true, false, nil, ctx.Err()
			}
			// Harness-level error: treat as non-SUCCESS and apply on_failure policy.
			response = domain.HarnessErrorResponse(req.AgentInstanceID, state.RunID, invokeErr)
		}
	}

	// Extract a checkpoint content-reference from checkpoint-class responses.
	checkpoint := ""
	if agent.Class == "checkpoint" {
		checkpoint = extractCheckpointRef(response.StatusMessage)
	}

	// Record the completed infrastructure step in the Execution Log.
	infraStep := domain.CompletedStep{
		Seq:              infraSeq,
		AgentInstance:    req.AgentInstanceID,
		Phase:            completedStep.Phase,
		Stage:            completedStep.Stage,
		Status:           response.StatusCode,
		Inputs:           formatInputs(req.InputArtifacts),
		Summary:          response.StatusMessage,
		Timestamp:        s.deps.Clock.Now(),
		Checkpoint:       checkpoint,
		IsInfrastructure: true,
	}
	if response.StatusCode == domain.StatusBLOCKED {
		infraStep.ErrorCode = response.ErrorCode
	}
	newState, applyErr := s.applyAndAnnounce(ctx, *state, infraStep)
	if applyErr != nil {
		return false, false, nil, applyErr
	}
	*state = newState
	*seq = infraSeq

	// Apply on_failure policy for non-SUCCESS outcomes.
	if response.StatusCode != domain.StatusSUCCESS {
		if agent.OnFailure == "halt" {
			return true, false, nil, nil
		}
		// continue policy: record the failure and proceed.
	} else if agent.Class == "review" {
		// Accumulate review signal: overwrite on each successful review so
		// the last successful review's message is used for the consultation.
		reviewSignal = &reviewConsultSignal{StatusMessage: response.StatusMessage}
	}
	return false, false, reviewSignal, nil
}
