package deviation

import (
	"context"
	"strings"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// stepAskMessage asks for the optional message that travels with the handed
// over decision. The text is passed on verbatim; an answer without a
// non-whitespace character means no message. Esc steps back to the row step.
func (d *manualDialogue) stepAskMessage(ctx context.Context) error {
	ans, cancelled, err := d.askText(ctx, interaction.TextQuestion{
		Question: interaction.Question{
			ID:     domain.QuestionManualAskMessage,
			Prompt: "Enter a message for the orchestrator (optional, leave empty for none):",
		},
		Multiline: true,
	})
	if err != nil {
		return err
	}
	if cancelled {
		d.step = stepRow
		return nil
	}
	instr := &domain.AskOrchestratorInstruction{}
	if strings.TrimSpace(ans.Text) != "" {
		text := ans.Text
		instr.UserMessage = &text
	}
	d.result = &domain.RoutingInstruction{AskOrchestrator: instr}
	return nil
}
