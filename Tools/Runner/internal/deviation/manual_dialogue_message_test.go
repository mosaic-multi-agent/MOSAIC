package deviation_test

// Tests for the message step that follows "Ask the orchestrator to decide" in
// the manual routing dialogue: a multi-line, optional text step whose answer
// travels with the ask instruction; Esc steps back to the row step.

import (
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
)

// askingDialogue picks the ask-orchestrator option at the row step and answers
// the message step with msg.
func askingDialogue(t *testing.T, msg textFn) *dialogue {
	d := newDialogue(t).standard("3", "1")
	d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualAskOrchestratorOptionID)}
	d.text[domain.QuestionManualAskMessage] = []textFn{msg}
	return d
}

func requireAskInstruction(t *testing.T, instr domain.RoutingInstruction, err error) *domain.AskOrchestratorInstruction {
	t.Helper()
	if err != nil {
		t.Fatalf("want an ask instruction, got error: %v", err)
	}
	if instr.AskOrchestrator == nil || instr.Dispatch != nil || instr.Stop != nil {
		t.Fatalf("want exactly an ask-orchestrator instruction, got %+v", instr)
	}
	return instr.AskOrchestrator
}

func TestManualDialogue_ChoosingAskOrchestrator_AsksAMultilineOptionalMessageNext(t *testing.T) {
	d := askingDialogue(t, say("hello"))

	_, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireSequence(t, d, domain.QuestionManualRow, domain.QuestionManualAskMessage)
	qs := d.textQs[domain.QuestionManualAskMessage]
	if len(qs) != 1 {
		t.Fatalf("want the message step asked once, got %d", len(qs))
	}
	if !qs[0].Multiline {
		t.Error("want a multi-line message step")
	}
	if qs[0].Default != "" {
		t.Errorf("want an empty default, got %q", qs[0].Default)
	}
	if qs[0].Validate != nil {
		if err := qs[0].Validate(""); err != nil {
			t.Errorf("want the empty text accepted (optional step), got %v", err)
		}
	}
}

func TestManualDialogue_MessageAnswered_AttachesTheTextVerbatim(t *testing.T) {
	cases := map[string]string{
		"single line": "Go to implementation.",
		"multi-line":  "Stop routing back to planning.\nGo to implementation.\n\n  indented line  ",
		"surrounding": "  keep my spaces \n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			d := askingDialogue(t, say(text))

			instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

			ask := requireAskInstruction(t, instr, err)
			if ask.UserMessage == nil || *ask.UserMessage != text {
				t.Errorf("want the message %q verbatim, got %v", text, ask.UserMessage)
			}
		})
	}
}

func TestManualDialogue_MessageWithoutNonWhitespace_AttachesNoMessage(t *testing.T) {
	for name, text := range map[string]string{"empty": "", "spaces": "   ", "line breaks": "\n \n\t"} {
		t.Run(name, func(t *testing.T) {
			d := askingDialogue(t, say(text))

			instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

			ask := requireAskInstruction(t, instr, err)
			requireSequence(t, d, domain.QuestionManualRow, domain.QuestionManualAskMessage)
			if ask.UserMessage != nil {
				t.Errorf("want no message, got %q", *ask.UserMessage)
			}
		})
	}
}

func TestManualDialogue_EscAtTheMessageStep_ReturnsToTheRowStep(t *testing.T) {
	d := askingDialogue(t, cancelText)
	d.text[domain.QuestionManualAskMessage] = []textFn{cancelText, say("second try")}

	instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	ask := requireAskInstruction(t, instr, err)
	requireSequence(t, d,
		domain.QuestionManualRow, domain.QuestionManualAskMessage,
		domain.QuestionManualRow, domain.QuestionManualAskMessage)
	if ask.UserMessage == nil || *ask.UserMessage != "second try" {
		t.Errorf("want the message of the second pass only, got %v", ask.UserMessage)
	}
}

func TestManualDialogue_EscAtTheMessageStep_LetsTheUserPickAnotherRow(t *testing.T) {
	d := newDialogue(t).standard("4", "")
	d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualAskOrchestratorOptionID), pick("4")}
	d.text[domain.QuestionManualAskMessage] = []textFn{cancelText}

	instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	disp := requireDispatch(t, instr, err)
	if disp.Agent != "reviewer" || instr.AskOrchestrator != nil {
		t.Errorf("want a plain reviewer dispatch after stepping back, got %+v", instr)
	}
}

func TestManualDialogue_MessageStepAnswerUnavailable_IsAnInteractionFailure(t *testing.T) {
	d := askingDialogue(t, func(interaction.TextQuestion) interaction.TextAnswer {
		return interaction.TextAnswer{Status: interaction.Unsupported}
	})

	instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	requireEnding(t, instr, err, domain.ConsultFailInteractionUnavailable)
	if instr.AskOrchestrator != nil {
		t.Errorf("want no ask instruction, got %+v", instr)
	}
}

func TestManualDialogue_MessageStepNotReached_WhenAskOrchestratorIsNotChosen(t *testing.T) {
	d := newDialogue(t).standard("4", "")

	instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	requireDispatch(t, instr, err)
	if len(d.textQs[domain.QuestionManualAskMessage]) != 0 {
		t.Error("want no message step on a plain dispatch")
	}
}

func TestManualDialogue_RowAskThenEscForever_EndsAtThePromptBound(t *testing.T) {
	d := askingDialogue(t, cancelText)

	instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	requireEnding(t, instr, err, domain.ConsultFailManualBoundExceeded)
	if len(d.calls) != deviation.ManualPromptLimit {
		t.Errorf("want exactly %d questions before the bound ends the dialogue, got %d",
			deviation.ManualPromptLimit, len(d.calls))
	}
}
