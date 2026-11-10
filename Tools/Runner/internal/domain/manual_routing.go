package domain

import "mosaic-common/interaction"

// Question IDs of the manual routing dialogue. Test doubles and frontends
// recognise a step by its ID, not by call order.
const (
	QuestionManualRow         interaction.QuestionID = "manual-routing.row"
	QuestionManualStage       interaction.QuestionID = "manual-routing.stage"
	QuestionManualTask        interaction.QuestionID = "manual-routing.task"
	QuestionManualInputs      interaction.QuestionID = "manual-routing.inputs"
	QuestionManualOutputs     interaction.QuestionID = "manual-routing.outputs"
	QuestionManualConstraints interaction.QuestionID = "manual-routing.constraints"
	QuestionManualHITL        interaction.QuestionID = "manual-routing.hitl"
	// QuestionManualAskMessage is the optional message step after the row
	// step's ManualAskOrchestratorOptionID.
	QuestionManualAskMessage interaction.QuestionID = "manual-routing.ask-message"
)

// Fixed option IDs of the manual routing dialogue.
const (
	ManualStopOptionID = "stop" // row step: stop the run
	// ManualAskOrchestratorOptionID hands the row decision to the orchestrator.
	ManualAskOrchestratorOptionID = "ask-orchestrator"
	ManualHITLDefault             = "default" // HITL step: no override
	ManualHITLOn                  = "on"      // HITL step: override to on
	ManualHITLOff                 = "off"     // HITL step: override to off
)
