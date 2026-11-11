package session_test

// A resume of the incident run: row 77 of the Execution Log split over three
// lines, rows 78 to 83 after it. The session repairs it through the orchestrator
// at run start and carries out the dispatch the orchestrator returns, instead
// of working out a resume position from the damaged file.

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
)

func incidentRig(t *testing.T, steps ...repairStep) *repairRig {
	t.Helper()
	return newRepairRig(t, steps, withDamagedRun(t, startOpts{}))
}

func TestSession_ResumeOfTheIncidentArtifact_MakesARepairConsultationCarryingTheDamageReport(t *testing.T) {
	rig := incidentRig(t, repairStep{edit: repairIncident, reply: stopReply("repaired")})

	rig.start(t)

	if rig.cons.calls() < 1 {
		t.Fatal("want a routing consultation at run start, got none")
	}
	req := rig.cons.request(0)
	report := requireDamageReport(t, req)
	requireStartTriggersNull(t, req)
	wantLine := lineNumberOf(t, incidentDamagedText(t), splitRowFirstLine)
	found := false
	for _, p := range report.Problems {
		if p.Section == "ExecutionLog" && p.Line == wantLine {
			found = true
		}
	}
	if !found {
		t.Errorf("want an ExecutionLog problem at the split row (line %d), got %+v", wantLine, report.Problems)
	}
	if want := filepath.Join(rig.evidDir, "Orchestration.last-good.md"); report.LastGoodCopy != want {
		t.Errorf("want the last-good copy %q in the report, got %q", want, report.LastGoodCopy)
	}
	if want := dispatchlog.RunLogPath(filepath.Dir(rig.run.runFolder), testRunID); report.DispatchLog != want {
		t.Errorf("want the run's dispatch log %q in the report, got %q", want, report.DispatchLog)
	}
}

func TestSession_ResumeOfTheIncidentArtifact_AcceptsTheRepairAndCarriesOutItsDispatch(t *testing.T) {
	rig := incidentRig(t,
		repairStep{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
		repairStep{reply: stopReply("done")},
	)

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if want := []string{"agent-a#84"}; !slices.Equal(invokedInstances(rig.f), want) {
		t.Errorf("want the returned dispatch carried out as the step after global_sequence 83 %v, got %v", want, invokedInstances(rig.f))
	}
	if rig.cons.calls() != 2 {
		t.Errorf("want the repair consultation then the routing consultation after the step, got %d consultations", rig.cons.calls())
	}
	requireNoDamageReport(t, rig.cons.request(1), "consultation after the accepted repair")
	if n := len(rig.user.asked); n != 0 {
		t.Errorf("want the run to continue without asking the user, got %d questions", n)
	}
}

func TestSession_ResumeOfTheIncidentArtifact_KeepsAll83RowsAndMovesTheStrayTextToWorkflowNotes(t *testing.T) {
	rig := incidentRig(t,
		repairStep{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
		repairStep{reply: stopReply("done")},
	)

	rig.start(t)

	state := readLog(t, rig.run.artifactPath)
	if len(state.ExecutionLog) != 84 {
		t.Fatalf("want the 83 repaired rows plus the dispatched step, got %d rows", len(state.ExecutionLog))
	}
	for i, e := range state.ExecutionLog {
		if e.Seq != i+1 {
			t.Fatalf("want rows in sequence, row %d has Seq %d", i+1, e.Seq)
		}
	}
	if last := state.ExecutionLog[83]; last.Agent != "agent-a#84" {
		t.Errorf("want the dispatched step as the last row, got %q", last.Agent)
	}
	inNotes := false
	for _, n := range state.WorkflowNotes {
		if strings.Contains(n.Note, "continuation of the split row") {
			inNotes = true
		}
	}
	if !inNotes {
		t.Errorf("want the stray text in Workflow Notes, got %+v", state.WorkflowNotes)
	}
}

func TestSession_ResumeOfTheIncidentArtifact_SavesTheDamagedCopyBeforeTheRepairConsultation(t *testing.T) {
	rig := incidentRig(t, repairStep{edit: repairIncident, reply: stopReply("repaired")})

	rig.start(t)

	rig.cons.request(0) // fails the test cleanly when no consultation was made
	if names := rig.cons.copiesAtCall(0); len(names) != 1 {
		t.Fatalf("want one damaged copy on disk when the repair consultation began, got %v", names)
	} else if got := readText(t, filepath.Join(rig.evidDir, names[0])); got != incidentDamagedText(t) {
		t.Error("want the damaged copy to hold the artifact as it was found")
	}
}

func lineNumberOf(t *testing.T, doc, prefix string) int {
	t.Helper()
	for i, l := range strings.Split(doc, "\n") {
		if strings.HasPrefix(l, prefix) {
			return i + 1
		}
	}
	t.Fatalf("test setup: no line starting with %q", prefix)
	return 0
}
