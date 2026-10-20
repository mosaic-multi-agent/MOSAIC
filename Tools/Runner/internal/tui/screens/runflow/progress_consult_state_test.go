package runflow

// The consultation row of the progress screen: state, removal and independence
// from the step rows.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

var (
	routingNoKind    = domain.ConsultationActivity{Context: domain.ConsultContextRouting}
	routingHarnessKd = domain.ConsultationActivity{Context: domain.ConsultContextRouting, Kind: domain.DeviationHarnessError}
	preConsultation  = domain.ConsultationActivity{Context: domain.ConsultContextPreConsultation}
)

func TestProgressScreen_StartConsultation_ReportsTheActivity(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.StartConsultation(routingHarnessKd)

	got, ok := s.Consultation()
	if !ok || got != routingHarnessKd {
		t.Errorf("Consultation() = %+v, %v; want %+v, true", got, ok, routingHarnessKd)
	}
}

func TestProgressScreen_Consultation_NoneShown_ReportsFalse(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	if _, ok := s.Consultation(); ok {
		t.Error("Consultation() reports a row on a fresh screen")
	}
}

func TestProgressScreen_EndConsultation_RemovesTheRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.StartConsultation(routingNoKind)
	if _, ok := s.Consultation(); !ok {
		t.Fatal("scenario did not run as intended: no consultation row is shown")
	}

	s.EndConsultation()

	if got, ok := s.Consultation(); ok {
		t.Errorf("Consultation() = %+v, true after EndConsultation; want no row", got)
	}
}

func TestProgressScreen_EndConsultation_NoneShown_IsNoOp(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#1", Status: "running"})

	s.EndConsultation()

	if got := s.Rows(); len(got) != 1 || got[0].Status != "running" {
		t.Errorf("Rows() = %+v; want the running step row untouched", got)
	}
}

func TestProgressScreen_StartConsultation_ReplacesTheShownRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.StartConsultation(routingNoKind)

	s.StartConsultation(preConsultation)

	got, ok := s.Consultation()
	if !ok || got != preConsultation {
		t.Errorf("Consultation() = %+v, %v; want the replacing activity %+v", got, ok, preConsultation)
	}
	s.EndConsultation()
	if _, still := s.Consultation(); still {
		t.Error("one EndConsultation left a stray consultation row behind a replaced one")
	}
}

func TestProgressScreen_Rows_NeverIncludeTheConsultationRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "done#1", Phase: "PLANNING", Status: "SUCCESS"})
	want := s.Rows()

	s.StartConsultation(routingHarnessKd)
	if _, ok := s.Consultation(); !ok {
		t.Fatal("scenario did not run as intended: no consultation row is shown")
	}

	if got := s.Rows(); !reflect.DeepEqual(got, want) {
		t.Errorf("Rows() = %+v while a consultation runs; want only the step rows %+v", got, want)
	}
}

func TestProgressScreen_CompleteRow_DoesNotCompleteOrRemoveTheConsultationRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#1", Status: "running"})
	s.StartConsultation(routingNoKind)

	s.CompleteRow("SUCCESS")

	if got := s.Rows(); len(got) != 1 || got[0].Status != "SUCCESS" {
		t.Errorf("Rows() = %+v; want the step row completed", got)
	}
	if _, ok := s.Consultation(); !ok {
		t.Error("a step completion removed the consultation row")
	}
}

func TestProgressScreen_EndConsultation_DoesNotCompleteTheRunningStepRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#1", Status: "running"})
	s.StartConsultation(routingNoKind)
	if _, ok := s.Consultation(); !ok {
		t.Fatal("scenario did not run as intended: no consultation row is shown")
	}

	s.EndConsultation()
	s.CompleteRow("BLOCKED")

	if got := s.Rows(); len(got) != 1 || got[0].Status != "BLOCKED" {
		t.Errorf("Rows() = %+v; want the step row still running until its own completion", got)
	}
}

func TestProgressScreen_RecordStep_DoesNotTouchTheConsultationRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.StartConsultation(routingNoKind)

	s.RecordStep(ProgressRow{AgentInstance: "agent-a#1", Status: "SUCCESS"})

	if _, ok := s.Consultation(); !ok {
		t.Error("RecordStep removed the consultation row")
	}
	if got := s.Rows(); len(got) != 1 {
		t.Errorf("Rows() = %+v; want the recorded row appended", got)
	}
}

func TestProgressScreen_SetHistory_RemovesTheConsultationRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.StartConsultation(routingNoKind)
	if _, ok := s.Consultation(); !ok {
		t.Fatal("scenario did not run as intended: no consultation row is shown")
	}

	s.SetHistory([]ProgressRow{{AgentInstance: "done#1", Status: "SUCCESS"}})

	if _, ok := s.Consultation(); ok {
		t.Error("SetHistory left a consultation row behind")
	}
}
