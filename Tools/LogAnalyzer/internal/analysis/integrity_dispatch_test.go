package analysis_test

// Tests for the dispatch-without-invocation check on in-memory streams.
//
// Rules under test:
//   - A subagent dispatch on a named run's orchestrator stream naming an id that
//     the run has no invocation for gives one warning finding per dispatch.
//   - An invocation is a folder (agent stream) of the run with that id, or an
//     invocation / agent of the run aggregate with that id.
//   - Dispatches without a decoded id, dispatches on the unattributable bucket
//     and on agent streams are never checked; an invocation without a dispatch
//     is not a finding.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	dpRunID    = "20260102T000000Z-dddd"
	dpMissing  = domain.AgentInstanceID("planner#3")
	dpExisting = domain.AgentInstanceID("coder#7")
)

var dpOtherRun = domain.NamedRun(dpRunID)

// dpDispatch builds a tool_call_start naming the given instance id ("" = none).
func dpDispatch(callID string, id domain.AgentInstanceID, sec int) domain.Event {
	ev := rmToolStart(callID, "Agent", rmAt(sec))
	ev.ToolCallStart.DispatchedInstance = id
	return ev
}

func dpOrch(run domain.RunRef, events ...domain.Event) analysis.Stream {
	ref := orchStream(run)
	ref.Path = "/logs/" + run.ID + "/00_orchestrator_events.jsonl"
	return analysis.Stream{Ref: ref, Events: events}
}

func dpAggWithInvocation(run domain.RunRef, id domain.AgentInstanceID) domain.Aggregate {
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:      run,
		Activity: domain.RunActivity{Invocations: []domain.Invocation{{Actor: domain.AgentInstance(id)}}},
	}}}
}

func TestDispatchWithoutInvocation_NoInvocationForTheId_OneFindingWithDispatchDetails(t *testing.T) {
	s := dpOrch(testRunRef, dpDispatch("c1", dpMissing, 0))

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{s}, domain.Aggregate{})

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Kind != domain.FindingDispatchWithoutInvocation {
		t.Errorf("kind = %q, want %q", f.Kind, domain.FindingDispatchWithoutInvocation)
	}
	if f.Severity != domain.SeverityWarning {
		t.Errorf("severity = %v, want warning", f.Severity)
	}
	if f.Run != testRunRef {
		t.Errorf("run = %+v, want %+v", f.Run, testRunRef)
	}
	if f.Actor != domain.Orchestrator() {
		t.Errorf("actor = %+v, want the orchestrator", f.Actor)
	}
	if f.Path != s.Ref.Path {
		t.Errorf("path = %q, want %q", f.Path, s.Ref.Path)
	}
	if !strings.Contains(f.Detail, string(dpMissing)) {
		t.Errorf("detail = %q, want it to name %q", f.Detail, dpMissing)
	}
}

func TestDispatchWithoutInvocation_DispatchedIdHasAnAgentFolderInTheRun_NoFinding(t *testing.T) {
	// A second dispatch of a missing id proves the check ran and discriminated.
	orch := dpOrch(testRunRef, dpDispatch("c1", dpExisting, 0), dpDispatch("c2", dpMissing, 5))
	folder := osStream(dpExisting)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch, folder}, domain.Aggregate{})

	if len(got) != 1 || !strings.Contains(got[0].Detail, string(dpMissing)) {
		t.Errorf("findings = %+v, want exactly one, for %q", got, dpMissing)
	}
}

func TestDispatchWithoutInvocation_DispatchedIdIsAnInvocationOfTheRunModel_NoFinding(t *testing.T) {
	orch := dpOrch(testRunRef, dpDispatch("c1", dpExisting, 0), dpDispatch("c2", dpMissing, 5))
	agg := dpAggWithInvocation(testRunRef, dpExisting)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch}, agg)

	if len(got) != 1 || !strings.Contains(got[0].Detail, string(dpMissing)) {
		t.Errorf("findings = %+v, want exactly one, for %q", got, dpMissing)
	}
}

func TestDispatchWithoutInvocation_DispatchedIdIsAnAgentOfTheRunAggregate_NoFinding(t *testing.T) {
	orch := dpOrch(testRunRef, dpDispatch("c1", dpExisting, 0), dpDispatch("c2", dpMissing, 5))
	agg := domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:    testRunRef,
		Agents: []domain.ActorAggregate{{Actor: domain.AgentInstance(dpExisting)}},
	}}}

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch}, agg)

	if len(got) != 1 || !strings.Contains(got[0].Detail, string(dpMissing)) {
		t.Errorf("findings = %+v, want exactly one, for %q", got, dpMissing)
	}
}

func TestDispatchWithoutInvocation_InvocationExistsOnlyInAnotherRun_StillAFinding(t *testing.T) {
	orch := dpOrch(testRunRef, dpDispatch("c1", dpExisting, 0))
	agg := dpAggWithInvocation(dpOtherRun, dpExisting)
	folderOfOtherRun := analysis.Stream{Ref: agentStream(dpOtherRun, dpExisting)}

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch, folderOfOtherRun}, agg)

	if len(got) != 1 || got[0].Run != testRunRef {
		t.Errorf("findings = %+v, want exactly one, for run %+v", got, testRunRef)
	}
}

func TestDispatchWithoutInvocation_RepeatedDispatchOfAMissingId_OneFindingEach(t *testing.T) {
	orch := dpOrch(testRunRef, dpDispatch("c1", dpMissing, 0), dpDispatch("c2", dpMissing, 5), dpDispatch("c3", dpMissing, 9))

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch}, domain.Aggregate{})

	if len(got) != 3 {
		t.Errorf("findings = %+v, want three, one per dispatch", got)
	}
}

func TestDispatchWithoutInvocation_EachRunIsCheckedAgainstItsOwnInvocations(t *testing.T) {
	orchA := dpOrch(testRunRef, dpDispatch("c1", dpExisting, 0))
	orchB := dpOrch(dpOtherRun, dpDispatch("c2", dpExisting, 0))
	agg := dpAggWithInvocation(testRunRef, dpExisting)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orchA, orchB}, agg)

	if len(got) != 1 || got[0].Run != dpOtherRun {
		t.Errorf("findings = %+v, want exactly one, for run %+v", got, dpOtherRun)
	}
}

func TestDispatchWithoutInvocation_EventsWithoutADispatchedId_NoFinding(t *testing.T) {
	read := rmToolStart("c2", "Read", rmAt(5))
	orch := dpOrch(testRunRef, dpDispatch("c1", "", 0), read)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch}, domain.Aggregate{})

	if len(got) != 0 {
		t.Errorf("findings = %+v, want none: input not captured or no id is skipped silently", got)
	}
}

func TestDispatchWithoutInvocation_InvocationWithoutDispatch_NoFinding(t *testing.T) {
	orch := dpOrch(testRunRef, rmToolStart("c1", "Read", rmAt(0)))
	agg := dpAggWithInvocation(testRunRef, dpExisting)

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{orch, osStream(dpExisting)}, agg)

	if len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestDispatchWithoutInvocation_DispatchOnTheUnattributableBucket_NotChecked(t *testing.T) {
	bucket := dpOrch(domain.UnattributableRun(), dpDispatch("c1", dpMissing, 0))
	named := dpOrch(testRunRef, dpDispatch("c2", dpMissing, 0))

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{bucket, named}, domain.Aggregate{})

	if len(got) != 1 || got[0].Run != testRunRef {
		t.Errorf("findings = %+v, want exactly one, for the named run only", got)
	}
}

func TestDispatchWithoutInvocation_DispatchOnAnAgentStream_NotChecked(t *testing.T) {
	// The agent folder id differs from the dispatched id, so no invocation exists for dpMissing.
	agentStreamWithDispatch := osStream("reviewer#5", dpDispatch("c1", dpMissing, 0))
	orch := dpOrch(testRunRef, dpDispatch("c2", dpMissing, 0))

	got := analysis.DispatchWithoutInvocationFindings([]analysis.Stream{agentStreamWithDispatch, orch}, domain.Aggregate{})

	if len(got) != 1 || got[0].Actor != domain.Orchestrator() {
		t.Errorf("findings = %+v, want exactly one, for the orchestrator dispatch", got)
	}
}
