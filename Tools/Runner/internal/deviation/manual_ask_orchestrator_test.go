package deviation_test

// Tests for the "ask the orchestrator" option of the manual routing dialogue's
// row question: offered only when the request says so, placed right before the
// stop option, and ending the dialogue with a request for an orchestrator
// consultation.

import (
	"slices"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

func askOrchestratorRequest(offer bool) domain.ConsultationRequest {
	req := manualRequest(schedulesUpTo(3))
	req.OfferAskOrchestrator = offer
	return req
}

func rowQuestionOf(t *testing.T, d *dialogue) interaction.ChoiceQuestion {
	t.Helper()
	return d.oneQ(domain.QuestionManualRow, 0)
}

func TestManualDialogue_AskOrchestratorOffered_RowQuestionListsItJustBeforeStop(t *testing.T) {
	d := newDialogue(t).standard("1", "")
	d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualStopOptionID)}

	_, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := optionIDs(rowQuestionOf(t, d))
	n := len(ids)
	if n < 3 {
		t.Fatalf("want rows plus two extra options, got %v", ids)
	}
	if ids[n-1] != domain.ManualStopOptionID || ids[n-2] != domain.ManualAskOrchestratorOptionID {
		t.Errorf("want ask-orchestrator immediately before stop at the end, got %v", ids)
	}
	if got := slices.Index(ids, domain.ManualAskOrchestratorOptionID); got != n-2 {
		t.Errorf("want ask-orchestrator listed once, at index %d, got index %d", n-2, got)
	}
	label := rowQuestionOf(t, d).Options[n-2].Label
	if label != "Ask the orchestrator to decide" {
		t.Errorf("want label %q, got %q", "Ask the orchestrator to decide", label)
	}
}

func TestManualDialogue_AskOrchestratorNotOffered_RowQuestionIsUnchanged(t *testing.T) {
	d := newDialogue(t).standard("1", "")
	d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualStopOptionID)}

	_, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(false))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := optionIDs(rowQuestionOf(t, d))
	if slices.Contains(ids, domain.ManualAskOrchestratorOptionID) {
		t.Errorf("want no ask-orchestrator option when not offered, got %v", ids)
	}
	if want := len(stagedRoutingTable().Rows) + 1; len(ids) != want {
		t.Errorf("want %d options (rows plus stop), got %v", want, ids)
	}
	if ids[len(ids)-1] != domain.ManualStopOptionID {
		t.Errorf("want stop last, got %v", ids)
	}
}

func TestManualDialogue_ChoosingAskOrchestrator_EndsAfterTheMessageStepWithAnAskInstruction(t *testing.T) {
	d := newDialogue(t).standard("3", "1")
	d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualAskOrchestratorOptionID)}
	d.text[domain.QuestionManualAskMessage] = []textFn{say("")}

	instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

	if err != nil {
		t.Fatalf("want a routing instruction, not a failure; got %v", err)
	}
	if instr.AskOrchestrator == nil || instr.Dispatch != nil || instr.Stop != nil {
		t.Fatalf("want exactly an ask-orchestrator instruction, got %+v", instr)
	}
	requireSequence(t, d, domain.QuestionManualRow, domain.QuestionManualAskMessage)
}

func TestManualDialogue_AskOrchestratorOffered_OtherChoicesBehaveAsBefore(t *testing.T) {
	t.Run("a row still leads to the full dialogue and a dispatch", func(t *testing.T) {
		d := newDialogue(t).standard("4", "")
		instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

		disp := requireDispatch(t, instr, err)
		if disp.Agent != "reviewer" || instr.AskOrchestrator != nil {
			t.Errorf("want a plain reviewer dispatch, got %+v", instr)
		}
		requireSequence(t, d,
			domain.QuestionManualRow, domain.QuestionManualTask, domain.QuestionManualInputs,
			domain.QuestionManualOutputs, domain.QuestionManualConstraints, domain.QuestionManualHITL)
	})
	t.Run("stop still stops", func(t *testing.T) {
		d := newDialogue(t).standard("4", "")
		d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualStopOptionID)}
		instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

		if err != nil || instr.Stop == nil || instr.AskOrchestrator != nil {
			t.Fatalf("want exactly a stop instruction, got %+v, %v", instr, err)
		}
	})
	t.Run("Esc at the row step still ends as an abandonment", func(t *testing.T) {
		d := newDialogue(t).standard("4", "")
		d.one[domain.QuestionManualRow] = []oneFn{cancelOne}
		instr, err := consultManual(t, d, stagedRoutingTable(), askOrchestratorRequest(true))

		if err == nil {
			t.Fatalf("want a failure for Esc, got %+v", instr)
		}
		if instr.Dispatch != nil || instr.Stop != nil || instr.AskOrchestrator != nil {
			t.Errorf("want no instruction, got %+v", instr)
		}
	})
}
