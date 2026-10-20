package domain

import "context"

// StepRecordObserver receives each Execution Log entry the session records.
// Called only from the session goroutine. Implementations never return an
// error, never panic, and never block longer than Interaction.Notify does.
type StepRecordObserver interface {
	// StepRecorded announces one entry exactly as the store recorded it
	// (the form a subsequent ArtifactStore.Read returns for that entry).
	StepRecorded(ctx context.Context, entry ExecutionLogEntry)
}

// NopStepRecordObserver discards every announcement.
type NopStepRecordObserver struct{}

// StepRecorded discards the announcement.
func (NopStepRecordObserver) StepRecorded(context.Context, ExecutionLogEntry) {}

// ConsultationActivity says what a running consultation is, so the progress
// screen can label it.
type ConsultationActivity struct {
	// Context is ConsultContextRouting or ConsultContextPreConsultation.
	Context ConsultContext
	// Kind is the deviation kind of a routing consultation; empty when there
	// is no deviation and for pre-consultation.
	Kind DeviationKind
}

// ConsultationObserver receives the start and end of every consultation the
// orchestrator agent answers. Called only from the session goroutine;
// implementations never return an error, never panic, and never block longer
// than Interaction.Notify does.
type ConsultationObserver interface {
	ConsultationStarted(ctx context.Context, c ConsultationActivity)
	ConsultationEnded(ctx context.Context)
}

// NopConsultationObserver discards every announcement.
type NopConsultationObserver struct{}

// ConsultationStarted discards the announcement.
func (NopConsultationObserver) ConsultationStarted(context.Context, ConsultationActivity) {}

// ConsultationEnded discards the announcement.
func (NopConsultationObserver) ConsultationEnded(context.Context) {}
