package deviation_test

// Tests for a manual dialogue opened by the in-run manual dispatch request
// (ConsultationRequest.OfferCancelRequest): Esc at the row step cancels the
// request with a distinguishable, non-failure result, the row step carries the
// Esc hint (next to an invalid-answer reason), and later steps still step back.

import (
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// cancelHint is the key fragment of the Esc hint, not its full wording.
const cancelHint = "Esc cancels"

// consultCancellable runs the dialogue over the staged table with the request
// marked as cancellable (or not).
func consultCancellable(t *testing.T, d *dialogue, cancellable bool) (domain.RoutingInstruction, error) {
	t.Helper()
	req := manualRequest(schedulesUpTo(3))
	req.OfferCancelRequest = cancellable
	return consultManual(t, d, stagedRoutingTable(), req)
}

func TestManualDialogue_CancellableRequest_EscAtTheRowStep_CancelsTheRequestWithoutAFailure(t *testing.T) {
	d := newDialogue(t).standard("3", "2").cancelFirst(domain.QuestionManualRow)

	instr, err := consultCancellable(t, d, true)

	if err != nil {
		t.Fatalf("want a cancel result and no error, got %v", err)
	}
	if instr.CancelRequest == nil {
		t.Fatalf("want a cancel-request instruction, got %+v", instr)
	}
	if instr.Dispatch != nil || instr.Stop != nil || instr.AskOrchestrator != nil {
		t.Errorf("want only the cancel field set, got %+v", instr)
	}
	requireSequence(t, d, domain.QuestionManualRow)
}

func TestManualDialogue_CancellableRequest_EscAfterAnInvalidRowAnswer_StillCancels(t *testing.T) {
	d := newDialogue(t).standard("3", "2")
	d.one[domain.QuestionManualRow] = []oneFn{pick("99"), cancelOne}

	instr, err := consultCancellable(t, d, true)

	if err != nil || instr.CancelRequest == nil {
		t.Fatalf("want a cancel result, got %+v, err %v", instr, err)
	}
	requireSequence(t, d, domain.QuestionManualRow, domain.QuestionManualRow)
}

func TestManualDialogue_NonCancellableRequest_EscAtTheRowStep_StillAbandons(t *testing.T) {
	d := newDialogue(t).standard("3", "2").cancelFirst(domain.QuestionManualRow)

	instr, err := consultCancellable(t, d, false)

	requireEnding(t, instr, err, domain.ConsultFailUserAbandoned)
	if instr.CancelRequest != nil {
		t.Errorf("want no cancel-request instruction without the offer, got %+v", instr)
	}
}

func TestManualDialogue_CancellableRequest_EscAtALaterStep_StepsBackAndRoutingCompletes(t *testing.T) {
	cases := []struct {
		name     string
		row      string
		stage    string
		cancelAt interaction.QuestionID
		wantNext interaction.QuestionID
	}{
		{"stage goes back to row", "3", "2", domain.QuestionManualStage, domain.QuestionManualRow},
		{"task of a non-staged row goes back to row", "4", "", domain.QuestionManualTask, domain.QuestionManualRow},
		{"inputs go back to task", "3", "2", domain.QuestionManualInputs, domain.QuestionManualTask},
		{"HITL goes back to constraints", "3", "2", domain.QuestionManualHITL, domain.QuestionManualConstraints},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard(tc.row, tc.stage).cancelFirst(tc.cancelAt)

			instr, err := consultCancellable(t, d, true)

			if got := questionAfterFirst(d.calls, tc.cancelAt); got != tc.wantNext {
				t.Errorf("after Esc at %s want %s asked next, got %s (sequence %v)", tc.cancelAt, tc.wantNext, got, d.calls)
			}
			requireDispatch(t, instr, err)
		})
	}
}

func TestManualDialogue_CancellableRequest_RowStepShowsTheEscHint(t *testing.T) {
	d := newDialogue(t).standard("3", "2")

	instr, err := consultCancellable(t, d, true)

	requireDispatch(t, instr, err)
	if got := d.oneQ(domain.QuestionManualRow, 0).Detail; !strings.Contains(got, cancelHint) {
		t.Errorf("want the Esc hint on the row step, got detail %q", got)
	}
}

func TestManualDialogue_NonCancellableRequest_RowStepShowsNoEscHint(t *testing.T) {
	d := newDialogue(t).standard("3", "2")

	instr, err := consultCancellable(t, d, false)

	requireDispatch(t, instr, err)
	if got := d.oneQ(domain.QuestionManualRow, 0).Detail; got != "" {
		t.Errorf("want an empty row detail without the offer, got %q", got)
	}
}

func TestManualDialogue_CancellableRequest_OnlyTheRowStepShowsTheEscHint(t *testing.T) {
	d := newDialogue(t).standard("3", "2")

	instr, err := consultCancellable(t, d, true)

	requireDispatch(t, instr, err)
	if got := d.oneQ(domain.QuestionManualStage, 0).Detail; strings.Contains(got, cancelHint) {
		t.Errorf("want no hint on the stage step, got %q", got)
	}
	if got := d.textQ(domain.QuestionManualTask, 0).Detail; strings.Contains(got, cancelHint) {
		t.Errorf("want no hint on the task step, got %q", got)
	}
}

func TestManualDialogue_CancellableRequest_InvalidAnswerReasonAndHintAreBothShownOnTheRowStep(t *testing.T) {
	d := newDialogue(t).standard("3", "2")
	d.one[domain.QuestionManualRow] = []oneFn{pick("99"), pick("4")}

	instr, err := consultCancellable(t, d, true)

	requireDispatch(t, instr, err)
	if len(d.notices) == 0 || d.notices[0].Level != interaction.NoticeWarning || d.notices[0].Message == "" {
		t.Fatalf("want a non-empty warning notice for the invalid row answer, got %+v", d.notices)
	}
	got := d.oneQ(domain.QuestionManualRow, 1).Detail
	if !strings.Contains(got, d.notices[0].Message) || !strings.Contains(got, cancelHint) {
		t.Errorf("want the reason %q and the hint in the detail, got %q", d.notices[0].Message, got)
	}
	if first := d.oneQ(domain.QuestionManualRow, 0).Detail; strings.Contains(first, d.notices[0].Message) {
		t.Errorf("want the first row question free of the later reason, got %q", first)
	}
}

func TestManualDialogue_NonCancellableRequest_InvalidAnswerReasonAloneIsShownOnTheRowStep(t *testing.T) {
	d := newDialogue(t).standard("3", "2")
	d.one[domain.QuestionManualRow] = []oneFn{pick("99"), pick("4")}

	instr, err := consultCancellable(t, d, false)

	requireDispatch(t, instr, err)
	if got := d.oneQ(domain.QuestionManualRow, 1).Detail; got != d.notices[0].Message {
		t.Errorf("want the detail to be exactly the reason %q, got %q", d.notices[0].Message, got)
	}
}
