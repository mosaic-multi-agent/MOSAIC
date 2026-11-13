package app_test

// Tests for the split of the integrity findings between the run model and the
// digest: instance_type_mismatch and unresolved_invocation are recomputed from
// the run model on every load and never exported; overlapping_invocation_start
// is a log-only finding that a digest export carries.

import (
	"context"
	"regexp"
	"strconv"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const (
	igMismatchID   = domain.AgentInstanceID("implementation-review#82")
	igMismatchType = "test-writer-tdd"
	igOverlapID    = domain.AgentInstanceID("planner#1")
	igUnresolvedID = domain.AgentInstanceID("unknown-agent")
)

func igUsage() domain.TokenUsage { return domain.TokenUsage{Input: domain.Tokens(10)} }

func igStart(id domain.AgentInstanceID, agentType string) domain.Event {
	return domain.Event{
		Type:            domain.EventInvocationStart,
		InvocationStart: &domain.InvocationStartFields{AgentInstanceID: id, AgentType: agentType},
	}
}

// addLogAgent adds an agent folder with the given events to the logs run at index run.
func addLogAgent(e *digestEnv, runIdx int, folder domain.AgentInstanceID, events ...domain.Event) {
	r := &e.logRuns[runIdx]
	dir := r.Dir + "/" + string(folder)
	file := dir + "/03_events.jsonl"
	r.Agents = append(r.Agents, domain.AgentEntry{Dir: dir, InstanceHint: folder, EventFile: file})
	e.reader.addEvents(file, events...)
}

// integrityLogs scripts one logs run with three agent folders: an instance holding
// another type's id, an instance with no proper id, and one with two starts and no
// end between them.
func integrityLogs() *digestEnv {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	addLogAgent(e, 0, igMismatchID,
		igStart(igMismatchID, igMismatchType), appTestInvEndEvent(igMismatchID, dgModel, igUsage()))
	addLogAgent(e, 0, igUnresolvedID,
		igStart(igUnresolvedID, ""), appTestInvEndEvent(igUnresolvedID, dgModel, igUsage()))
	addLogAgent(e, 0, igOverlapID,
		igStart(igOverlapID, "planner"), igStart(igOverlapID, "planner"),
		appTestInvEndEvent(igOverlapID, dgModel, igUsage()))
	return e
}

// integrityDigestRun is a digest run whose agents hold a mismatching and an unresolved id.
func integrityDigestRun(id string) domain.RunAggregate {
	run := dgRun(id, 1_000_000)
	mismatch := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: igMismatchID, Type: igMismatchType}
	unresolved := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: igUnresolvedID}
	run.Agents = igAgents(mismatch, unresolved)
	run.Activity.Invocations = []domain.Invocation{
		{Actor: mismatch, State: domain.InvocationClosed, Usage: igUsageRecords(1)},
		{Actor: unresolved, State: domain.InvocationClosed, Usage: igUsageRecords(igUnresolvedCount)},
	}
	return run
}

// igUnresolvedCount is the number of usage records of the unresolved digest instance.
const igUnresolvedCount = 17

func igUsageRecords(n int) []domain.InvocationUsage {
	out := make([]domain.InvocationUsage, n)
	for i := range out {
		out[i] = domain.InvocationUsage{Model: dgModel, Usage: igUsage()}
	}
	return out
}

func igAgents(mismatch, unresolved domain.ActorRef) []domain.ActorAggregate {
	return []domain.ActorAggregate{
		{Actor: mismatch, Invocations: igUsageRecords(1)},
		{Actor: unresolved, Invocations: igUsageRecords(igUnresolvedCount)},
	}
}

// integrityAgentsOnlyDigestRun is a digest run as decoded without an invocations
// section: the agents carry the instances, Activity.Invocations is empty.
func integrityAgentsOnlyDigestRun(id string) domain.RunAggregate {
	run := dgRun(id, 1_000_000)
	run.Agents = igAgents(
		domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: igMismatchID, Type: igMismatchType},
		domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: igUnresolvedID})
	return run
}

// hasNumberToken reports whether text contains n as a whole number.
func hasNumberToken(text string, n int) bool {
	return regexp.MustCompile(`(^|[^0-9])` + strconv.Itoa(n) + `([^0-9]|$)`).MatchString(text)
}

func requireOneFinding(t *testing.T, r domain.Report, kind domain.FindingKind, run domain.RunRef, instance domain.AgentInstanceID) domain.Finding {
	t.Helper()
	got := dgFindings(r, kind)
	if len(got) != 1 {
		t.Fatalf("%s findings = %+v, want exactly one", kind, got)
	}
	f := got[0]
	if f.Run != run || f.Actor.Instance != instance || f.Severity != domain.SeverityWarning {
		t.Errorf("%s finding = %+v, want a warning for run %+v, instance %q", kind, f, run, instance)
	}
	return f
}

func TestAnalyze_DigestLoadedRun_ShowsTheRunModelFindings(t *testing.T) {
	// The digest carries no integrity findings, as one written before this check existed.
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, integrityDigestRun(dgRunA)))

	res := analyzeDGReport(t, e, dgFilePath)

	run := domain.NamedRun(dgRunA)
	requireOneFinding(t, res.Report, domain.FindingInstanceTypeMismatch, run, igMismatchID)
	f := requireOneFinding(t, res.Report, domain.FindingUnresolvedInvocation, run, igUnresolvedID)
	if !hasNumberToken(f.Detail, igUnresolvedCount) {
		t.Errorf("unresolved detail = %q, want it to state the usage record count %d", f.Detail, igUnresolvedCount)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("integrity findings must not make the report incomplete")
	}
}

func TestAnalyze_DigestRunWithAgentsOnly_ShowsTheRunModelFindings(t *testing.T) {
	// A digest without an invocations section still carries every instance under agents.
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, integrityAgentsOnlyDigestRun(dgRunA)))

	res := analyzeDGReport(t, e, dgFilePath)

	run := domain.NamedRun(dgRunA)
	requireOneFinding(t, res.Report, domain.FindingInstanceTypeMismatch, run, igMismatchID)
	f := requireOneFinding(t, res.Report, domain.FindingUnresolvedInvocation, run, igUnresolvedID)
	if !hasNumberToken(f.Detail, igUnresolvedCount) {
		t.Errorf("unresolved detail = %q, want it to state the usage record count %d", f.Detail, igUnresolvedCount)
	}
}

func TestAnalyze_DigestWithEqualOrUnknownTypes_ShowsNoMismatch(t *testing.T) {
	run := dgRun(dgRunA, 1)
	usage := []domain.InvocationUsage{{Model: dgModel, Usage: igUsage()}}
	for _, a := range []domain.ActorRef{
		{Kind: domain.ActorAgentInstance, Instance: "planner#1", Type: "planner"},
		{Kind: domain.ActorAgentInstance, Instance: "planner#2"},
	} {
		run.Agents = append(run.Agents, domain.ActorAggregate{Actor: a, Invocations: usage})
		run.Activity.Invocations = append(run.Activity.Invocations,
			domain.Invocation{Actor: a, State: domain.InvocationClosed, Usage: usage})
	}
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, run))

	res := analyzeDGReport(t, e, dgFilePath)

	for _, k := range []domain.FindingKind{domain.FindingInstanceTypeMismatch, domain.FindingUnresolvedInvocation} {
		if got := dgFindings(res.Report, k); len(got) != 0 {
			t.Errorf("%s findings = %+v, want none", k, got)
		}
	}
}

func TestAnalyze_LogRun_ShowsTheRunModelAndOverlapFindings(t *testing.T) {
	res := analyzeDGReport(t, integrityLogs(), dgLogsPath)

	run := domain.NamedRun(dgRunA)
	requireOneFinding(t, res.Report, domain.FindingInstanceTypeMismatch, run, igMismatchID)
	requireOneFinding(t, res.Report, domain.FindingUnresolvedInvocation, run, igUnresolvedID)
	requireOneFinding(t, res.Report, domain.FindingOverlappingInvocationStart, run, igOverlapID)
}

func TestExport_FromLogs_CarriesOverlapButNeitherRunModelKind(t *testing.T) {
	e := integrityLogs()

	doExport(t, e, exportReq(""))

	got := onlyWrite(t, e).d.Findings
	var overlap []domain.Finding
	for _, f := range got {
		switch f.Kind {
		case domain.FindingInstanceTypeMismatch, domain.FindingUnresolvedInvocation:
			t.Errorf("exported digest carries %s: run-model kinds are recomputed on load, never exported", f.Kind)
		case domain.FindingOverlappingInvocationStart:
			overlap = append(overlap, f)
		}
	}
	if len(overlap) != 1 {
		t.Fatalf("exported overlapping_invocation_start findings = %+v, want exactly one", overlap)
	}
	if f := overlap[0]; f.Run != domain.NamedRun(dgRunA) || f.Actor.Instance != igOverlapID || f.Severity != domain.SeverityWarning {
		t.Errorf("exported finding = %+v, want a warning for run %s, instance %q", f, dgRunA, igOverlapID)
	}
}

// exportedDigestOfIntegrityLogs exports the integrity logs and returns the digest written.
func exportedDigestOfIntegrityLogs(t *testing.T) (domain.Digest, []domain.Finding) {
	t.Helper()
	e := integrityLogs()
	doExport(t, e, exportReq(""))
	w := onlyWrite(t, e)
	return w.d, w.d.Findings
}

func TestAnalyze_ExportedDigestOnItsOwn_ShowsEachKindOnce(t *testing.T) {
	d, _ := exportedDigestOfIntegrityLogs(t)
	e := newDigestEnv()
	// The digest store hands back only load-time findings, so none are carried here.
	e.digestFile(dgFilePath, d)

	res := analyzeDGReport(t, e, dgFilePath)

	run := domain.NamedRun(dgRunA)
	requireOneFinding(t, res.Report, domain.FindingInstanceTypeMismatch, run, igMismatchID)
	requireOneFinding(t, res.Report, domain.FindingUnresolvedInvocation, run, igUnresolvedID)
	requireOneFinding(t, res.Report, domain.FindingOverlappingInvocationStart, run, igOverlapID)
}

func TestAnalyze_LogRunPlusItsExportedDigest_DoesNotDoubleTheRunModelKinds(t *testing.T) {
	d, _ := exportedDigestOfIntegrityLogs(t)
	e := integrityLogs()
	e.digestDir(e.logsPath, digestItem{name: "run.digest.json", d: d})

	res := analyzeDGReport(t, e, e.logsPath)

	run := domain.NamedRun(dgRunA)
	requireOneFinding(t, res.Report, domain.FindingInstanceTypeMismatch, run, igMismatchID)
	requireOneFinding(t, res.Report, domain.FindingUnresolvedInvocation, run, igUnresolvedID)
}

func TestAnalyze_LogRunAndDigestOnlyRun_EachShowsItsRunModelFindingsOnce(t *testing.T) {
	e := integrityLogs()
	e.digestDir(e.logsPath, digestItem{name: "other.digest.json", d: dgDigest(dgT1, integrityDigestRun(dgRunB))})

	res := analyzeDGReport(t, e, e.logsPath)

	for _, run := range []string{dgRunA, dgRunB} {
		var n int
		for _, f := range dgFindings(res.Report, domain.FindingInstanceTypeMismatch) {
			if f.Run == domain.NamedRun(run) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("run %s instance_type_mismatch findings = %d, want 1", run, n)
		}
	}
}

func TestTotalForRun_RunWithIntegrityProblems_TotalIsComplete(t *testing.T) {
	e := integrityLogs()

	total, outcome, err := e.service().TotalForRun(context.Background(), app.Request{ExplicitPath: dgLogsPath}, dgRunA)

	if err != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun: err=%v outcome=%v", err, outcome)
	}
	if !total.Complete {
		t.Error("total.Complete = false, want true: integrity findings never touch the total")
	}
}
