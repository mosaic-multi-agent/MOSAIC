package tui

// The consultation row on the progress screen as the root model drives it from
// the session's consultation announcements: shown while a consultation runs,
// removed when it ends, kept apart from the step rows, and absent from the rows
// that equal the Execution Log.

import (
	"testing"

	"mosaic-run/internal/domain"
)

var (
	consultRouting = domain.ConsultationActivity{Context: domain.ConsultContextRouting}
	consultDefect  = domain.ConsultationActivity{Context: domain.ConsultContextRouting, Kind: domain.DeviationScheduleDefect}
	consultPre     = domain.ConsultationActivity{Context: domain.ConsultContextPreConsultation}
)

// consultStart adds the announcement that a consultation began.
func (s *liveScript) consultStart(a domain.ConsultationActivity) *liveScript {
	s.msgs = append(s.msgs, consultStartedMsg{activity: a})
	return s
}

// consultEnd adds the announcement that the consultation ended.
func (s *liveScript) consultEnd() *liveScript {
	s.msgs = append(s.msgs, consultEndedMsg{})
	return s
}

// feedChecked feeds the script and, after every consultation start, asserts the
// consultation row is shown with that activity.
func (s *liveScript) feedChecked(t *testing.T, m *rootModel) {
	t.Helper()
	for _, msg := range s.msgs {
		m.Update(msg)
		if started, ok := msg.(consultStartedMsg); ok {
			requireConsultation(t, m, started.activity)
		}
	}
}

// requireConsultation asserts the screen shows the consultation row for want.
func requireConsultation(t *testing.T, m *rootModel, want domain.ConsultationActivity) {
	t.Helper()
	got, ok := m.progressScreen.Consultation()
	if !ok || got != want {
		t.Fatalf("Consultation() = %+v, %v; want %+v, true", got, ok, want)
	}
}

// requireNoConsultation asserts the screen shows no consultation row.
func requireNoConsultation(t *testing.T, m *rootModel) {
	t.Helper()
	if got, ok := m.progressScreen.Consultation(); ok {
		t.Fatalf("Consultation() = %+v, true; want no consultation row", got)
	}
}

func TestConsultRows_Start_ShowsTheRowWithItsKind(t *testing.T) {
	m := newParityModel()

	(&liveScript{}).consultStart(consultDefect).feed(m)

	requireConsultation(t, m, consultDefect)
}

func TestConsultRows_End_RemovesTheRow(t *testing.T) {
	m := newParityModel()

	(&liveScript{}).consultStart(consultRouting).feed(m)
	requireConsultation(t, m, consultRouting)
	(&liveScript{}).consultEnd().feed(m)

	requireNoConsultation(t, m)
}

func TestConsultRows_RowIsNotAStepRow(t *testing.T) {
	m := newParityModel()

	(&liveScript{}).consultStart(consultRouting).feed(m)

	requireConsultation(t, m, consultRouting)
	if rows := m.progressScreen.Rows(); len(rows) != 0 {
		t.Errorf("Rows() = %+v while a consultation runs; want no step row", rows)
	}
}

func TestConsultRows_ConsultationThatDispatches_LeavesOnlyTheDispatchedRunningRow(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#1", "PLANNING", "", "SUCCESS").
		consultStart(consultRouting).
		consultEnd().
		running("agent-b#2", "EXECUTION", "")
	m := newParityModel()

	s.feedChecked(t, m)

	requireNoConsultation(t, m)
	rows := liveRows(m)
	if len(rows) != 2 || rows[0].Status != "SUCCESS" || rows[1].AgentInstance != "agent-b#2" || rows[1].Status != "running" {
		t.Errorf("rows = %+v; want agent-a finished and only agent-b running", rows)
	}
}

func TestConsultRows_ConsultationThatStops_LeavesNoRowAndRowsEqualRebuilt(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#1", "PLANNING", "", "SUCCESS").
		consultStart(consultRouting).
		consultEnd()
	m := newParityModel()

	s.feedChecked(t, m)

	requireNoConsultation(t, m)
	requireRowsEqualRebuilt(t, m, s)
}

func TestConsultRows_BackToBackConsultations_LeaveNoStrayRow(t *testing.T) {
	s := (&liveScript{}).
		consultStart(consultRouting).consultEnd().
		consultStart(consultDefect).consultEnd().
		record(entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)).
		consultStart(consultRouting).consultEnd()
	m := newParityModel()

	s.feedChecked(t, m)

	requireNoConsultation(t, m)
	requireRowsEqualRebuilt(t, m, s)
}

func TestConsultRows_NestedStart_ShowsOneRowAndOneEndRemovesIt(t *testing.T) {
	m := newParityModel()

	(&liveScript{}).consultStart(consultRouting).consultStart(consultDefect).feed(m)
	requireConsultation(t, m, consultDefect)
	(&liveScript{}).consultEnd().feed(m)

	requireNoConsultation(t, m)
}

func TestConsultRows_EndWithoutStart_IsHarmless(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		consultEnd()
	m := newParityModel()

	s.feed(m)

	rows := liveRows(m)
	if len(rows) != 1 || rows[0].Status != "running" {
		t.Errorf("rows = %+v; want the running step row untouched by a stray end", rows)
	}
}

func TestConsultRows_SubagentCompletionNotice_NeverCompletesTheConsultationRow(t *testing.T) {
	m := newParityModel()
	(&liveScript{}).running("agent-a#1", "PLANNING", "").consultStart(consultRouting).feed(m)

	(&liveScript{}).completion("agent-a#1", "PLANNING", "", "SUCCESS").feed(m)

	requireConsultation(t, m, consultRouting)
	rows := liveRows(m)
	if len(rows) != 1 || rows[0].Status != "SUCCESS" {
		t.Errorf("rows = %+v; want the subagent row completed", rows)
	}
}

func TestConsultRows_SubagentRecordedRow_NeverOverwritesTheConsultationRow(t *testing.T) {
	m := newParityModel()
	(&liveScript{}).consultStart(consultRouting).feed(m)

	(&liveScript{}).record(entry("agent-a#1", "PLANNING", "", domain.StatusBLOCKED)).feed(m)

	requireConsultation(t, m, consultRouting)
	if rows := liveRows(m); len(rows) != 1 || rows[0].AgentInstance != "agent-a#1" {
		t.Errorf("rows = %+v; want the recorded row appended beside the consultation row", rows)
	}
}

func TestConsultRows_EndOfConsultation_NeverCompletesTheRunningSubagentRow(t *testing.T) {
	m := newParityModel()
	(&liveScript{}).running("agent-a#1", "PLANNING", "").consultStart(consultRouting).feed(m)
	requireConsultation(t, m, consultRouting)

	(&liveScript{}).consultEnd().feed(m)

	rows := liveRows(m)
	if len(rows) != 1 || rows[0].Status != "running" {
		t.Errorf("rows = %+v; want the subagent row still running after the consultation ended", rows)
	}
}

func TestConsultRows_PreConsultationOnNewRun_ShowsAndRemovesTheRow(t *testing.T) {
	m := newParityModel()

	(&liveScript{}).consultStart(consultPre).feed(m)
	requireConsultation(t, m, consultPre)
	s := (&liveScript{}).consultEnd().
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#1", "PLANNING", "", "SUCCESS")
	s.feed(m)

	requireNoConsultation(t, m)
	requireRowsEqualRebuilt(t, m, s)
}

func TestConsultRows_PreConsultationEndedByStop_LeavesNoRowAndNoStepRow(t *testing.T) {
	m := newParityModel()

	(&liveScript{}).consultStart(consultPre).feed(m)
	requireConsultation(t, m, consultPre)
	(&liveScript{}).consultEnd().feed(m)

	requireNoConsultation(t, m)
	if rows := m.progressScreen.Rows(); len(rows) != 0 {
		t.Errorf("Rows() = %+v; want none", rows)
	}
}

func TestConsultRows_FullRunWithConsultations_RowsEqualRebuiltAndNoConsultationRemains(t *testing.T) {
	s := (&liveScript{}).
		consultStart(consultPre).consultEnd().
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusBLOCKED)).
		completion("agent-a#1", "PLANNING", "", "BLOCKED").
		consultStart(consultDefect).consultEnd().
		running("agent-a#2", "PLANNING", "").
		record(entry("agent-a#2", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#2", "PLANNING", "", "SUCCESS").
		consultStart(consultRouting).consultEnd().
		running("agent-b#3", "EXECUTION", "").
		record(entry("agent-b#3", "EXECUTION", "", domain.StatusSUCCESS)).
		completion("agent-b#3", "EXECUTION", "", "SUCCESS").
		consultStart(consultRouting).consultEnd()
	m := newParityModel()

	s.feedChecked(t, m)

	requireNoConsultation(t, m)
	requireRowsEqualRebuilt(t, m, s)
}

func TestConsultRows_WithoutProgressScreen_AnnouncementsAreNoOps(t *testing.T) {
	m := newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})
	m.progressScreen = nil

	(&liveScript{}).consultStart(consultRouting).consultEnd().feed(m)

	if m.progressScreen != nil {
		t.Error("a consultation announcement created a progress screen")
	}
}
