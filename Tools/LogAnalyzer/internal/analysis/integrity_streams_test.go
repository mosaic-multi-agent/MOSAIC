package analysis_test

// Tests for the overlapping invocation start check on in-memory streams.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const osFolder = domain.AgentInstanceID("planner#3")

func osStream(hint domain.AgentInstanceID, events ...domain.Event) analysis.Stream {
	ref := agentStream(testRunRef, hint)
	ref.Path = "/logs/run/" + string(hint) + "/03_events.jsonl"
	return analysis.Stream{Ref: ref, Events: events}
}

func osStart(id domain.AgentInstanceID, sec int) domain.Event {
	return rmInvStart(id, "planner", rmAt(sec))
}

func osEnd(id domain.AgentInstanceID, sec int) domain.Event {
	return rmInvEnd(id, "ok", "model-a", rmAt(sec))
}

func TestOverlappingStart_TwoStartsWithNoEndBetween_OneFindingWithFolderIdAsActor(t *testing.T) {
	// The events name a different id than the folder: the folder id is the actor.
	s := osStream(osFolder, osStart("written-in-event#1", 0), osStart("written-in-event#1", 10))

	got := analysis.OverlappingStartFindings([]analysis.Stream{s})

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Kind != domain.FindingOverlappingInvocationStart {
		t.Errorf("kind = %q, want %q", f.Kind, domain.FindingOverlappingInvocationStart)
	}
	if f.Severity != domain.SeverityWarning {
		t.Errorf("severity = %v, want warning", f.Severity)
	}
	if f.Run != testRunRef {
		t.Errorf("run = %+v, want %+v", f.Run, testRunRef)
	}
	if f.Actor != domain.AgentInstance(osFolder) {
		t.Errorf("actor = %+v, want the folder id %q", f.Actor, osFolder)
	}
	if f.Path != s.Ref.Path {
		t.Errorf("path = %q, want %q", f.Path, s.Ref.Path)
	}
}

func TestOverlappingStart_StartEndStartEnd_SequentialResumeGivesNoFinding(t *testing.T) {
	s := osStream(osFolder, osStart(osFolder, 0), osEnd(osFolder, 5), osStart(osFolder, 10), osEnd(osFolder, 15))

	if got := analysis.OverlappingStartFindings([]analysis.Stream{s}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestOverlappingStart_ThreeStartsWithoutEnd_OneFindingPerExtraStart(t *testing.T) {
	s := osStream(osFolder, osStart(osFolder, 0), osStart(osFolder, 5), osStart(osFolder, 10))

	got := analysis.OverlappingStartFindings([]analysis.Stream{s})

	if len(got) != 2 {
		t.Errorf("findings = %+v, want two", got)
	}
}

func TestOverlappingStart_EndClosesTheOpenStartWhateverItsId(t *testing.T) {
	s := osStream(osFolder, osStart(osFolder, 0), osEnd("someone-else#9", 5), osStart(osFolder, 10))

	if got := analysis.OverlappingStartFindings([]analysis.Stream{s}); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestOverlappingStart_OverlapAfterAResume_IsStillFound(t *testing.T) {
	s := osStream(osFolder,
		osStart(osFolder, 0), osEnd(osFolder, 5),
		osStart(osFolder, 10), osStart(osFolder, 12))

	got := analysis.OverlappingStartFindings([]analysis.Stream{s})

	if len(got) != 1 {
		t.Errorf("findings = %+v, want one", got)
	}
}

func TestOverlappingStart_EndBeforeAnyStartThenTwoStarts_OneFinding(t *testing.T) {
	s := osStream(osFolder, osEnd(osFolder, 0), osStart(osFolder, 5), osStart(osFolder, 10))

	got := analysis.OverlappingStartFindings([]analysis.Stream{s})

	if len(got) != 1 {
		t.Errorf("findings = %+v, want one: a stray end opens nothing", got)
	}
}

func TestOverlappingStart_EachStreamIsExaminedOnItsOwn(t *testing.T) {
	// Starts in two different folders never overlap each other.
	a := osStream("planner#1", osStart("planner#1", 0))
	b := osStream("planner#2", osStart("planner#2", 1))
	bad := osStream("planner#3", osStart("planner#3", 2), osStart("planner#3", 3))

	got := analysis.OverlappingStartFindings([]analysis.Stream{a, b, bad})

	if len(got) != 1 || got[0].Actor != domain.AgentInstance("planner#3") {
		t.Errorf("findings = %+v, want exactly one, for planner#3", got)
	}
}

func TestOverlappingStart_OrchestratorStream_IsNotChecked(t *testing.T) {
	s := analysis.Stream{
		Ref:    orchStream(testRunRef),
		Events: []domain.Event{osStart("planner#1", 0), osStart("planner#1", 1)},
	}

	if got := analysis.OverlappingStartFindings([]analysis.Stream{s}); len(got) != 0 {
		t.Errorf("findings = %+v, want none: only agent folder streams are examined", got)
	}
}

func TestOverlappingStart_Finding_IsNotDataLoss(t *testing.T) {
	s := osStream(osFolder, osStart(osFolder, 0), osStart(osFolder, 5))

	got := analysis.OverlappingStartFindings([]analysis.Stream{s})

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want one", got)
	}
	if domain.NewQualitySummary(got).Incomplete() {
		t.Error("overlapping_invocation_start must not make the report incomplete")
	}
}
