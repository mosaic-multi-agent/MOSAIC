package session

import (
	"mosaic-run/internal/domain"
)

// consultPayload is the artifact and HITL payload of a consultation-routed
// dispatch, before run-scoping.
type consultPayload struct {
	Inputs  []string
	Outputs []string
	HITL    bool
}

// consultOutputs carries the outputs written by the latest consultation-routed
// step to the dispatch loop, so the next engine decision sees them. set is
// false when no consultation-routed step has been applied since the loop last
// took them.
type consultOutputs struct {
	set   bool
	paths []string
}

// resolveConsultPayload builds the payload of a consultation-routed dispatch.
// An instruction field left unset takes the row default resolved by the same
// routine the engine uses for that row and iteration; an explicit field passes
// through verbatim. effectiveStage is the recorded stage value, empty for a
// non-staged row.
func resolveConsultPayload(
	dispInstr *domain.DispatchInstruction,
	row domain.RoutingRow,
	effectiveStage string,
	stages domain.BlockSchedules,
) (consultPayload, error) {
	var iteration domain.IterationID
	if row.PhaseParsed.IsStaged && effectiveStage != "" {
		if value, err := domain.ReadIterationValue(effectiveStage, row); err == nil {
			iteration = value.ID
		}
	}
	defaults, err := resolveRowDefaults(row, iteration, stages)
	if err != nil {
		return consultPayload{}, err
	}
	payload := consultPayload{Inputs: defaults.InputArtifacts, Outputs: defaults.OutputArtifacts, HITL: defaults.HITL}
	if dispInstr.InputArtifacts != nil {
		payload.Inputs = *dispInstr.InputArtifacts
	}
	if dispInstr.OutputArtifacts != nil {
		payload.Outputs = *dispInstr.OutputArtifacts
	}
	if dispInstr.HITLOverride != nil {
		payload.HITL = *dispInstr.HITLOverride
	}
	return payload, nil
}
