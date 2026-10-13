package deviation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// currentRow is the chosen routing table row. Only valid once rowNum is set.
func (d *manualDialogue) currentRow() domain.RoutingRow {
	return d.table.Rows[d.rowNum-1]
}

// stageOrRow is the step that can correct the chosen target: the stage step
// for a staged row, else the row step.
func (d *manualDialogue) stageOrRow() manualStep {
	if d.rowNum > 0 && d.currentRow().PhaseParsed.IsStaged {
		return stepStage
	}
	return stepRow
}

// setRow records the chosen row. Choosing a different row discards the stage
// and the artifact selections made for the previous one; the task is kept.
func (d *manualDialogue) setRow(n int, choice string) {
	if n != d.rowNum {
		d.stage, d.stageChoice = "", ""
		d.inputs, d.outputs = listState{}, listState{}
	}
	d.rowNum, d.rowChoice = n, choice
}

// setStage records the chosen stage; a different stage discards the artifact
// selections made for the previous one.
func (d *manualDialogue) setStage(id domain.IterationID, choice string) {
	if id != d.stage {
		d.inputs, d.outputs = listState{}, listState{}
	}
	d.stage, d.stageChoice = id, choice
}

// rowDefaults resolves the default payload of the chosen row and stage. Without
// a defaults function the payload is empty.
func (d *manualDialogue) rowDefaults() (domain.DispatchDefaults, error) {
	if d.req.RowDefaults == nil {
		return domain.DispatchDefaults{}, nil
	}
	return d.req.RowDefaults(d.currentRow(), d.stage)
}

// ---- row ----

func (d *manualDialogue) stepRow(ctx context.Context) error {
	ans, cancelled, err := d.askOne(ctx, d.rowQuestion())
	if err != nil {
		return err
	}
	if cancelled {
		return d.abandoned("the user cancelled manual routing at the row choice")
	}
	choice := chosenValue(ans)
	if choice == "" {
		return d.unavailable("the row question was answered without a choice", nil)
	}
	if strings.EqualFold(choice, domain.ManualStopOptionID) {
		d.result = &domain.RoutingInstruction{Stop: &domain.StopInstruction{Reason: "user requested stop"}}
		return nil
	}
	n, convErr := strconv.Atoi(choice)
	if convErr != nil || n < 1 || n > len(d.table.Rows) {
		return d.invalidResult(ctx, fmt.Sprintf("row choice %q is not a row of the routing table (1 to %d)", choice, len(d.table.Rows)), stepRow)
	}
	d.setRow(n, choice)
	d.step = stepTask
	if d.currentRow().PhaseParsed.IsStaged {
		d.step = stepStage
	}
	return nil
}

func (d *manualDialogue) rowQuestion() interaction.ChoiceQuestion {
	options := make([]interaction.Option, 0, len(d.table.Rows)+1)
	for i, row := range d.table.Rows {
		options = append(options, interaction.Option{
			ID:    strconv.Itoa(i + 1),
			Label: fmt.Sprintf("[%d] %s (%s)", i+1, row.BareAgentID(), row.Phase),
		})
	}
	options = append(options, interaction.Option{ID: domain.ManualStopOptionID, Label: "Stop the run"})
	return interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:              domain.QuestionManualRow,
			Prompt:          "Choose which row to dispatch next, or stop the run:",
			AllowCustom:     true,
			CustomPrompt:    "Row number",
			DefaultOptionID: d.rowPreselection(),
		},
		Options: options,
	}
}

// rowPreselection is the earlier row answer, else the row that deviated.
func (d *manualDialogue) rowPreselection() string {
	if d.rowChoice != "" {
		return d.rowChoice
	}
	row, _, ok := d.deviationTarget()
	if !ok || row < 0 || row >= len(d.table.Rows) {
		return ""
	}
	return strconv.Itoa(row + 1)
}

// deviationTarget is the step the manual dispatch offers by default: the
// already-finished next step for a finished-work deviation, else the step that
// deviated. ok is false without a deviation.
func (d *manualDialogue) deviationTarget() (row int, stage string, ok bool) {
	dev := d.req.Deviation
	if dev == nil {
		return 0, "", false
	}
	if dev.Kind == domain.DeviationFinishedWork && dev.PlannedContinuation != nil {
		return dev.PlannedContinuation.RowIndex, dev.PlannedContinuation.Stage, true
	}
	return dev.CurrentRow, dev.CurrentStage, true
}

// ---- stage ----

// rowSchedule is the schedule of the chosen row's block in the request. ok is
// false when the request carries none for that block or it holds no iteration.
func (d *manualDialogue) rowSchedule() (sched domain.BlockSchedule, ok bool) {
	block, inBlock := d.currentRow().Block()
	if !inBlock {
		return domain.BlockSchedule{}, false
	}
	sched, ok = d.req.Schedules[block]
	return sched, ok && len(sched.Iterations) > 0
}

func (d *manualDialogue) stepStage(ctx context.Context) error {
	sched, ok := d.rowSchedule()
	if !ok {
		return d.invalidResult(ctx, "the chosen row is a staged row but no stage set is available", stepRow)
	}
	ans, cancelled, err := d.askOne(ctx, d.stageQuestion(sched))
	if err != nil {
		return err
	}
	if cancelled {
		d.step = stepRow
		return nil
	}
	choice := chosenValue(ans)
	if choice == "" {
		return d.unavailable("the stage question was answered without a choice", nil)
	}
	id, valid := stageChoiceID(sched, choice)
	if !valid {
		return d.invalidResult(ctx, fmt.Sprintf("stage choice %q is not a stage of the row's block", choice), stepStage)
	}
	d.setStage(id, choice)
	d.step = stepTask
	return nil
}

// stageChoiceID maps a stage answer to an iteration ID. An answer naming an
// iteration of the schedule (ignoring case) gives the schedule's spelling. A
// decimal number >= 1 outside the schedule is accepted as that number: the
// final check of the result rejects it with the stage named. Any other answer
// is not valid.
func stageChoiceID(sched domain.BlockSchedule, choice string) (domain.IterationID, bool) {
	if it, found := sched.Find(domain.IterationID(choice)); found {
		return it.ID, true
	}
	if n, err := strconv.Atoi(choice); err == nil && n >= 1 {
		return domain.IterationID(strconv.Itoa(n)), true
	}
	return "", false
}

func (d *manualDialogue) stageQuestion(sched domain.BlockSchedule) interaction.ChoiceQuestion {
	options := make([]interaction.Option, 0, len(sched.Iterations))
	for _, id := range sched.IDs() {
		options = append(options, interaction.Option{ID: string(id), Label: fmt.Sprintf("Stage %s", id)})
	}
	return interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:              domain.QuestionManualStage,
			Prompt:          "Choose the stage to dispatch:",
			AllowCustom:     true,
			CustomPrompt:    "Stage number",
			DefaultOptionID: d.stagePreselection(sched),
		},
		Options: options,
	}
}

// stagePreselection is the earlier stage answer, else the stage that deviated
// when it is of the chosen row's group and still in the schedule.
func (d *manualDialogue) stagePreselection(sched domain.BlockSchedule) string {
	if d.stageChoice != "" {
		return d.stageChoice
	}
	row, stage, ok := d.deviationTarget()
	if !ok || row < 0 || row >= len(d.table.Rows) {
		return ""
	}
	value, err := domain.ReadIterationValue(stage, d.table.Rows[row])
	if err != nil || value.Group != d.currentRow().PhaseParsed.Group {
		return ""
	}
	it, inSchedule := sched.Find(value.ID)
	if !inSchedule {
		return ""
	}
	return string(it.ID)
}

// ---- HITL ----

func (d *manualDialogue) stepHITL(ctx context.Context) error {
	defaults, err := d.rowDefaults()
	if err != nil {
		return d.invalidResult(ctx, "the row's defaults cannot be resolved: "+err.Error(), d.stageOrRow())
	}
	ans, cancelled, askErr := d.askOne(ctx, d.hitlQuestion(defaults.HITL))
	if askErr != nil {
		return askErr
	}
	if cancelled {
		d.step = stepConstraints
		return nil
	}
	choice := chosenValue(ans)
	switch choice {
	case "":
		return d.unavailable("the HITL question was answered without a choice", nil)
	case domain.ManualHITLDefault, domain.ManualHITLOn, domain.ManualHITLOff:
		d.hitl = choice
		return d.finish(ctx)
	}
	return d.invalidResult(ctx, fmt.Sprintf("HITL choice %q is not one of default, on, off", choice), stepHITL)
}

func (d *manualDialogue) hitlQuestion(rowHITL bool) interaction.ChoiceQuestion {
	setting := "off"
	if rowHITL {
		setting = "on"
	}
	return interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:              domain.QuestionManualHITL,
			Prompt:          "Human in the loop for this dispatch:",
			DefaultOptionID: d.hitl,
		},
		Options: []interaction.Option{
			{ID: domain.ManualHITLDefault, Label: "No override", Description: "use the row's setting (" + setting + ")"},
			{ID: domain.ManualHITLOn, Label: "On"},
			{ID: domain.ManualHITLOff, Label: "Off"},
		},
	}
}
