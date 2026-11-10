package tui

// Helpers of the row-parity tests: a script of the messages a session produces
// (running notices, step-record announcements, completion notices) fed to the
// root model, and the comparison of the live rows with the rows rebuilt from
// the Execution Log the script recorded.

import (
	"fmt"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// liveScript is an ordered list of session output messages together with the
// Execution Log entries the script recorded.
type liveScript struct {
	msgs []tea.Msg
	log  []domain.ExecutionLogEntry
}

func iterationNotice(agent, phase, iteration, status string) questionMsg {
	return noticeMsg(agent, fmt.Sprintf("phase=%s iteration=%q status=%s", phase, iteration, status))
}

// running adds the notice announcing that agent starts.
func (s *liveScript) running(agent, phase, iteration string) *liveScript {
	s.msgs = append(s.msgs, iterationNotice(agent, phase, iteration, "running"))
	return s
}

// record adds the announcement of one recorded entry.
func (s *liveScript) record(e domain.ExecutionLogEntry) *liveScript {
	e.Seq = len(s.log) + 1
	s.log = append(s.log, e)
	s.msgs = append(s.msgs, stepRecordedMsg{entry: e})
	return s
}

// completion adds a completion notice.
func (s *liveScript) completion(agent, phase, iteration, status string) *liveScript {
	s.msgs = append(s.msgs, iterationNotice(agent, phase, iteration, status))
	return s
}

func entry(agent, phase, stage string, status domain.StatusCode) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{Agent: agent, Phase: phase, Stage: stage, Status: status}
}

// newParityModel is a root model on the progress screen with no rows.
func newParityModel() *rootModel {
	m := newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	return m
}

func (s *liveScript) feed(m *rootModel) {
	for _, msg := range s.msgs {
		m.Update(msg)
	}
}

// liveRows returns the progress rows without their measured elapsed time.
func liveRows(m *rootModel) []progressRow {
	rows := m.progressScreen.Rows()
	for i := range rows {
		rows[i].Elapsed = 0
	}
	return rows
}

// requireRowsEqualRebuilt asserts that the live rows equal the rows rebuilt
// from the script's Execution Log and that no row is running.
func requireRowsEqualRebuilt(t *testing.T, m *rootModel, s *liveScript) {
	t.Helper()
	if len(s.log) == 0 {
		t.Fatal("scenario did not run as intended: the script recorded no entry")
	}
	got, want := liveRows(m), historyRows(s.log)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("live rows =\n%+v\nwant the rows rebuilt from the Execution Log\n%+v", got, want)
	}
	for _, row := range got {
		if row.Status == "running" {
			t.Errorf("row %q is still running once no step is running", row.AgentInstance)
		}
	}
}
