package analysis_test

// Tests for the transcript capture gap check on in-memory streams and
// transcript facts.
//
// Rules under test:
//   - Side (a): transcript assistant ids with no usage record in the folder's own
//     stream give one finding stating their count. Side (b): the stream's usage
//     records (position-hash ids and empty ids excluded) whose id is not in the
//     transcript give one finding stating their count.
//   - Agent comparisons apply only to streams with exactly one invocation_start.
//   - The orchestrator is compared for side (a) only, whatever its starts.
//   - Only claude-code transcripts are compared; facts without a matching stream
//     are skipped; the actor is the folder id.

import (
	"regexp"
	"strconv"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	tgFolder     = domain.AgentInstanceID("planner#9")
	tgOtherAgent = domain.AgentInstanceID("coder#8")
	tgRawPath    = "/logs/run/planner#9/04_session.raw"
	tgOrchPath   = "/logs/run/00_orchestrator_session.raw"
)

// tgHasNumber reports whether text contains n as a whole number.
func tgHasNumber(text string, n int) bool {
	return regexp.MustCompile(`(^|[^0-9])` + strconv.Itoa(n) + `([^0-9]|$)`).MatchString(text)
}

// tgUsages builds usage_record events of the stream owner with the given record ids.
func tgUsages(owner domain.AgentInstanceID, recordIDs ...string) []domain.Event {
	var events []domain.Event
	for _, id := range recordIDs {
		events = append(events, usageRecordEvent(owner, id, "model-a", domain.TokenUsage{Input: domain.Tokens(1)}))
	}
	return events
}

// tgAgent builds the stream of an agent folder holding one invocation_start and
// the given usage record ids.
func tgAgent(hint domain.AgentInstanceID, recordIDs ...string) analysis.Stream {
	events := append([]domain.Event{rmInvStart(hint, "planner", rmAt(0))}, tgUsages(hint, recordIDs...)...)
	return analysis.Stream{Ref: agentStream(testRunRef, hint), Events: events}
}

// tgOrchestrator builds the orchestrator stream holding the given usage record ids.
func tgOrchestrator(recordIDs ...string) analysis.Stream {
	return analysis.Stream{Ref: orchStream(testRunRef), Events: tgUsages("", recordIDs...)}
}

func tgAgentFacts(hint domain.AgentInstanceID, ids ...string) domain.TranscriptFacts {
	return domain.TranscriptFacts{
		Run: testRunRef, Owner: domain.StreamAgentInstance, InstanceHint: hint,
		Path:   "/logs/run/" + string(hint) + "/04_session.raw",
		Format: domain.TranscriptClaudeCodeJSONL, AssistantIDs: ids,
	}
}

func tgOrchFacts(ids ...string) domain.TranscriptFacts {
	return domain.TranscriptFacts{
		Run: testRunRef, Owner: domain.StreamOrchestrator, Path: tgOrchPath,
		Format: domain.TranscriptClaudeCodeJSONL, AssistantIDs: ids,
	}
}

func tgIDs(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix + strconv.Itoa(i+1)
	}
	return out
}

func tgGaps(streams []analysis.Stream, facts ...domain.TranscriptFacts) []domain.Finding {
	return analysis.TranscriptCaptureGapFindings(streams, facts)
}

func TestCaptureGap_SingleStartWithOneTranscriptMessageLackingUsage_OneFindingStatingCountOne(t *testing.T) {
	s := tgAgent(tgFolder, "m1")
	facts := tgAgentFacts(tgFolder, "m1", "m2")

	got := tgGaps([]analysis.Stream{s}, facts)

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Kind != domain.FindingTranscriptCaptureGap || f.Severity != domain.SeverityWarning {
		t.Errorf("finding = %+v, want a transcript_capture_gap warning", f)
	}
	if f.Run != testRunRef || f.Path != facts.Path {
		t.Errorf("run/path = %+v / %q, want %+v / %q", f.Run, f.Path, testRunRef, facts.Path)
	}
	if f.Actor != domain.AgentInstance(tgFolder) {
		t.Errorf("actor = %+v, want the folder id %q", f.Actor, tgFolder)
	}
	if !tgHasNumber(f.Detail, 1) {
		t.Errorf("detail %q does not state the count 1", f.Detail)
	}
}

func TestCaptureGap_ActorIsTheFolderIdNotTheIdWrittenInTheEvents(t *testing.T) {
	events := []domain.Event{rmInvStart("written-in-event#1", "planner", rmAt(0))}
	events = append(events, tgUsages("written-in-event#1", "m1")...)
	s := analysis.Stream{Ref: agentStream(testRunRef, tgFolder), Events: events}

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1", "m2"))

	if len(got) != 1 || got[0].Actor != domain.AgentInstance(tgFolder) {
		t.Fatalf("findings = %+v, want one finding with the folder id as actor", got)
	}
}

func TestCaptureGap_SeveralTranscriptMessagesLackingUsage_OneFindingStatingTheCount(t *testing.T) {
	s := tgAgent(tgFolder, "m1")

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, append([]string{"m1"}, tgIDs("t", 4)...)...))

	if len(got) != 1 || !tgHasNumber(got[0].Detail, 4) {
		t.Fatalf("findings = %+v, want exactly one stating the count 4", got)
	}
}

func TestCaptureGap_UsageRecordsMissingFromTheTranscript_OneFindingStatingTheCount(t *testing.T) {
	s := tgAgent(tgFolder, append([]string{"m1"}, tgIDs("x", 6)...)...)

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1"))

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one (side b only)", got)
	}
	if got[0].Kind != domain.FindingTranscriptCaptureGap || got[0].Actor != domain.AgentInstance(tgFolder) {
		t.Errorf("finding = %+v, want a transcript_capture_gap for the folder id", got[0])
	}
	if !tgHasNumber(got[0].Detail, 6) {
		t.Errorf("detail %q does not state the count 6", got[0].Detail)
	}
}

func TestCaptureGap_BothSidesDiffer_OneFindingPerSideEachStatingItsOwnCount(t *testing.T) {
	s := tgAgent(tgFolder, append([]string{"m1"}, tgIDs("x", 7)...)...)

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, append([]string{"m1"}, tgIDs("t", 4)...)...))

	if len(got) != 2 {
		t.Fatalf("findings = %+v, want two (one per side)", got)
	}
	four, seven := 0, 0
	for _, f := range got {
		switch {
		case tgHasNumber(f.Detail, 4) && !tgHasNumber(f.Detail, 7):
			four++
		case tgHasNumber(f.Detail, 7) && !tgHasNumber(f.Detail, 4):
			seven++
		}
	}
	if four != 1 || seven != 1 {
		t.Errorf("details = %q / %q, want one stating 4 and one stating 7", got[0].Detail, got[1].Detail)
	}
}

func TestCaptureGap_EveryTranscriptMessageHasAUsageRecord_NoFinding(t *testing.T) {
	s := tgAgent(tgFolder, "m1", "m2", "m3")

	if got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m3", "m2", "m1")); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestCaptureGap_DuplicateUsageRecordIds_AreComparedAsADistinctSet(t *testing.T) {
	s := tgAgent(tgFolder, "m1", "m1", "x1", "x1", "x1")

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1"))

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one (side b)", got)
	}
	if !tgHasNumber(got[0].Detail, 1) || tgHasNumber(got[0].Detail, 3) {
		t.Errorf("detail %q, want the distinct count 1, not the record count 3", got[0].Detail)
	}
}

func TestCaptureGap_UsageRecordWithEmptyId_IsNotCompared(t *testing.T) {
	s := tgAgent(tgFolder, "m1", "")

	if got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1")); len(got) != 0 {
		t.Errorf("findings = %+v, want none: an empty record id is not a usage id", got)
	}
}

func TestCaptureGap_PositionHashUsageRecords_AreNotComparedAsSideB(t *testing.T) {
	s := tgAgent(tgFolder, "m1", "pos:12", "pos:13")

	if got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1")); len(got) != 0 {
		t.Errorf("findings = %+v, want none: pos: ids are not compared", got)
	}
}

func TestCaptureGap_PositionHashUsageRecordDoesNotCoverATranscriptMessage(t *testing.T) {
	s := tgAgent(tgFolder, "pos:12")

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1"))

	if len(got) != 1 || !tgHasNumber(got[0].Detail, 1) {
		t.Fatalf("findings = %+v, want exactly one side (a) finding stating 1", got)
	}
}

func TestCaptureGap_StreamWithSeveralStarts_GivesNoFinding(t *testing.T) {
	events := []domain.Event{
		rmInvStart(tgFolder, "planner", rmAt(0)), rmInvEnd(tgFolder, "ok", "model-a", rmAt(5)),
		rmInvStart(tgFolder, "planner", rmAt(10)),
	}
	events = append(events, tgUsages(tgFolder, "m1", "x1")...)
	s := analysis.Stream{Ref: agentStream(testRunRef, tgFolder), Events: events}

	got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1", "m2"))

	if len(got) != 0 {
		t.Errorf("findings = %+v, want none: a resumed invocation overwrote the transcript", got)
	}
}

func TestCaptureGap_StreamWithoutAStart_GivesNoFinding(t *testing.T) {
	s := analysis.Stream{Ref: agentStream(testRunRef, tgFolder), Events: tgUsages(tgFolder, "m1")}

	if got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1", "m2")); len(got) != 0 {
		t.Errorf("findings = %+v, want none: only streams with exactly one start are compared", got)
	}
}

func TestCaptureGap_FactsOfAnotherFolder_AreMatchedOnlyToTheirOwnStream(t *testing.T) {
	a := tgAgent(tgFolder, "m1")
	b := tgAgent(tgOtherAgent, "m1", "m2")

	got := tgGaps([]analysis.Stream{a, b}, tgAgentFacts(tgFolder, "m1", "m2"))

	if len(got) != 1 || got[0].Actor != domain.AgentInstance(tgFolder) {
		t.Errorf("findings = %+v, want one finding for %q: another stream's records do not count", got, tgFolder)
	}
}

func TestCaptureGap_FactsWithoutAMatchingStream_AreSkipped(t *testing.T) {
	s := tgAgent(tgOtherAgent, "m1")

	if got := tgGaps([]analysis.Stream{s}, tgAgentFacts(tgFolder, "m1", "m2", "m3")); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestCaptureGap_FactsOfAnotherRun_AreNotMatchedToTheStream(t *testing.T) {
	s := tgAgent(tgFolder, "m1")
	facts := tgAgentFacts(tgFolder, "m1", "m2")
	facts.Run = domain.NamedRun("20260102T000000Z-ffff")

	if got := tgGaps([]analysis.Stream{s}, facts); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestCaptureGap_OtherNativeFormats_AreNotCompared(t *testing.T) {
	s := tgAgent(tgFolder, "x1", "x2")
	openCode := tgAgentFacts(tgFolder)
	openCode.Format = domain.TranscriptOpenCodeMessages
	unknown := tgAgentFacts(tgFolder, "m1", "m2")
	unknown.Format = "some-other-format"

	if got := tgGaps([]analysis.Stream{s}, openCode, unknown); len(got) != 0 {
		t.Errorf("findings = %+v, want none: other formats are not a finding", got)
	}
}

func TestCaptureGap_Orchestrator_TranscriptMessagesWithoutUsage_OneFindingForTheOrchestrator(t *testing.T) {
	s := tgOrchestrator("m1")
	facts := tgOrchFacts(append([]string{"m1"}, tgIDs("t", 4)...)...)

	got := tgGaps([]analysis.Stream{s}, facts)

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Kind != domain.FindingTranscriptCaptureGap || f.Severity != domain.SeverityWarning ||
		f.Run != testRunRef || f.Actor != domain.Orchestrator() || f.Path != tgOrchPath {
		t.Errorf("finding = %+v, want a warning for the orchestrator of %+v at %q", f, testRunRef, tgOrchPath)
	}
	if !tgHasNumber(f.Detail, 4) {
		t.Errorf("detail %q does not state the count 4", f.Detail)
	}
}

func TestCaptureGap_Orchestrator_UsageRecordsBeyondTheTranscript_AreNotAFinding(t *testing.T) {
	// The shared orchestrator transcript may lag or hold another session: side (b)
	// is never raised for it.
	s := tgOrchestrator(append([]string{"m1"}, tgIDs("x", 5)...)...)

	if got := tgGaps([]analysis.Stream{s}, tgOrchFacts("m1")); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestCaptureGap_Orchestrator_IsComparedWhateverItsInvocationStarts(t *testing.T) {
	events := []domain.Event{rmInvStart("", "orchestrator", rmAt(0)), rmInvStart("", "orchestrator", rmAt(5))}
	events = append(events, tgUsages("", "m1")...)
	s := analysis.Stream{Ref: orchStream(testRunRef), Events: events}

	got := tgGaps([]analysis.Stream{s}, tgOrchFacts("m1", "m2"))

	if len(got) != 1 {
		t.Errorf("findings = %+v, want exactly one: the start rule is for agents only", got)
	}
}

func TestCaptureGap_OrchestratorFacts_AreNotMatchedToAnAgentStream(t *testing.T) {
	// The agent's usage records cover m1 and m2, the orchestrator stream has none.
	agent := tgAgent(tgFolder, "m1", "m2")
	orch := tgOrchestrator()

	got := tgGaps([]analysis.Stream{agent, orch}, tgOrchFacts("m1", "m2"))

	if len(got) != 1 || got[0].Actor != domain.Orchestrator() || !tgHasNumber(got[0].Detail, 2) {
		t.Errorf("findings = %+v, want one orchestrator finding stating 2", got)
	}
}
