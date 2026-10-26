package session_test

// Tests that every Runner-log event the session writes with a row field uses
// the 1-based workflow-row numbering (the value the Execution Log records as
// WorkflowRow). Each scenario uses a row whose zero-based index and 1-based
// number differ, so a raw table index fails the assertion.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// entriesFor returns the captured entries of one event, in order.
func (r *sessionRecordingLogger) entriesFor(event string) []sessionLogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []sessionLogEntry
	for _, e := range r.entries {
		if e.Event == event {
			out = append(out, e)
		}
	}
	return out
}

// fieldOf returns the value of the named field of an entry, "" when absent.
func fieldOf(e sessionLogEntry, key string) string {
	for _, f := range e.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// requireRowForAgent asserts that at least one entry names an agent instance
// starting with agentPrefix, and that each such entry carries row wantRow.
func requireRowForAgent(t *testing.T, entries []sessionLogEntry, agentPrefix, wantRow string) {
	t.Helper()
	found := 0
	for _, e := range entries {
		if !strings.HasPrefix(fieldOf(e, "agent"), agentPrefix) {
			continue
		}
		found++
		if got := fieldOf(e, "row"); got != wantRow {
			t.Errorf("%s entry for %s: want row=%s (1-based), got row=%q", e.Event, agentPrefix, wantRow, got)
		}
	}
	if found == 0 {
		t.Errorf("scenario did not run as intended: no entry for agent %q among %d entries", agentPrefix, len(entries))
	}
}

func TestRowNumbering_EngineDispatchStart_LogsOneBasedRow(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			logger := &sessionRecordingLogger{}
			ses, f, _, orchPath := newLinearSessionWithDebug(t, logger)
			f.Queue("agent-a", successEntry("a done"))
			f.Queue("agent-b", successEntry("b done"))

			ses.Start(context.Background(), configForMode(orchPath, mode)) //nolint:errcheck

			starts := logger.entriesFor(domain.EventSessionDispatchStart)
			requireRowForAgent(t, starts, "agent-a#", "1")
			requireRowForAgent(t, starts, "agent-b#", "2")
		})
	}
}

func TestRowNumbering_ConsultantRoutedDispatchStart_LogsOneBasedRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "first", 0)
	consultant.queueDispatch("agent-b", "second", 1)
	consultant.queueStop("done")
	logger := &sessionRecordingLogger{}
	ses, f, _, orchPath := newOrchestratedSessionWithDebug(t, consultant, logger)
	f.Queue("agent-a", successEntry("a done"))
	f.Queue("agent-b", successEntry("b done"))

	ses.Start(context.Background(), baseOrchestratedConfig(orchPath)) //nolint:errcheck

	starts := logger.entriesFor(domain.EventSessionDispatchStart)
	requireRowForAgent(t, starts, "agent-a#", "1")
	requireRowForAgent(t, starts, "agent-b#", "2")
}

func TestRowNumbering_InvalidDispatchTarget_LogsOneBasedRow(t *testing.T) {
	// agent-a does not hold row 2 (index 1): the target is invalid.
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "wrong agent for the row", 1)
	logger := &sessionRecordingLogger{}
	ses, _, _, orchPath := newOrchestratedSessionWithDebug(t, consultant, logger)

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	var invalid []sessionLogEntry
	for _, e := range logger.entriesFor(domain.EventSessionConsultFailed) {
		if strings.Contains(e.Message, "dispatch target invalid") {
			invalid = append(invalid, e)
		}
	}
	if len(invalid) != 1 {
		t.Fatalf("want one invalid-dispatch-target entry, got %d among %v", len(invalid), logger.allEvents())
	}
	if row := fieldOf(invalid[0], "row"); row != "2" {
		t.Errorf("want row=2 (1-based), got row=%q", row)
	}
}

func TestRowNumbering_ScheduleDefectRedispatch_LogsOneBasedRow(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			logger := &sessionRecordingLogger{}
			rig := newLoggedBlockRig(t, logger, "planner", "auditor", "merger")
			rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
			rig.queueDefectivePlanner(1)
			rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			rig.queueOKTimes("auditor", 2)
			rig.queueOK("merger")

			rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode)) //nolint:errcheck

			var defect []sessionLogEntry
			for _, e := range logger.entriesFor(domain.EventSessionDeviation) {
				if strings.Contains(e.Message, "block schedule defective") {
					defect = append(defect, e)
				}
			}
			if len(defect) != 1 {
				t.Fatalf("want one schedule-defect entry, got %d among %v", len(defect), logger.allEvents())
			}
			// The producing planner holds the first routing row.
			if row := fieldOf(defect[0], "row"); row != "1" {
				t.Errorf("want row=1 (1-based, the planner's row), got row=%q", row)
			}
		})
	}
}

func TestRowNumbering_ConsultantStopDiscard_LogsAndNamesOneBasedRow(t *testing.T) {
	d, orchPath := newLinearDiscardRun(t)
	d.inner.queueDispatch("agent-b", "do the work", 1)

	got, _ := d.ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	discarded := d.logger.entriesFor(domain.EventSessionConsultDiscarded)
	if len(discarded) != 1 {
		t.Fatalf("want one discard entry, got %d among %v", len(discarded), d.logger.allEvents())
	}
	if row := fieldOf(discarded[0], "row"); row != "2" {
		t.Errorf("want row=2 (1-based), got row=%q", row)
	}
	if !strings.Contains(got.Message, "workflow row 2") {
		t.Errorf("want the outcome message to name workflow row 2, got %q", got.Message)
	}
	warnings := d.warningNotices()
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "workflow row 2") {
		t.Errorf("want one warning notice naming workflow row 2, got %+v", warnings)
	}
}
