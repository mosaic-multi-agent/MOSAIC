package app_test

// Tests for the dispatch_without_invocation finding as it travels through the
// service: it is a log finding (reported and exported to digests), it never
// makes the report incomplete, and the run total is not affected.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const (
	dpMissingID  = domain.AgentInstanceID("planner#3")
	dpExistingID = domain.AgentInstanceID("coder#7")
)

func dpDispatchEvent(callID string, id domain.AgentInstanceID) domain.Event {
	return domain.Event{
		Type:          domain.EventToolCallStart,
		ToolCallStart: &domain.ToolCallStartFields{CallID: callID, ToolName: "Agent", DispatchedInstance: id},
	}
}

// dispatchLogs scripts one logs run whose orchestrator dispatches an agent with
// no invocation and one that has an agent folder.
func dispatchLogs() *digestEnv {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	orch := e.logRuns[0].OrchestratorFile
	e.reader.addEvents(orch, dpDispatchEvent("c1", dpMissingID), dpDispatchEvent("c2", dpExistingID))
	addLogAgent(e, 0, dpExistingID, igStart(dpExistingID, "coder"), appTestInvEndEvent(dpExistingID, dgModel, igUsage()))
	return e
}

func TestAnalyze_DispatchOfAnAgentWithoutInvocation_ReportsOneFindingForThatDispatch(t *testing.T) {
	res := analyzeDGReport(t, dispatchLogs(), dgLogsPath)

	got := dgFindings(res.Report, domain.FindingDispatchWithoutInvocation)

	if len(got) != 1 {
		t.Fatalf("dispatch_without_invocation findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Run != domain.NamedRun(dgRunA) || f.Actor != domain.Orchestrator() || f.Severity != domain.SeverityWarning {
		t.Errorf("finding = %+v, want a warning for run %s, scoped to the orchestrator", f, dgRunA)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("a dispatch without invocation must not make the report incomplete")
	}
}

func TestAnalyze_DispatchWithUnattributableBucketDispatch_IsNotReported(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.addLogBucket(1_000)
	e.reader.addEvents(e.logBucket.OrchestratorFile, dpDispatchEvent("c1", dpMissingID))

	res := analyzeDGReport(t, e, dgLogsPath)

	if got := dgFindings(res.Report, domain.FindingDispatchWithoutInvocation); len(got) != 0 {
		t.Errorf("findings = %+v, want none: the bucket holds the run's first dispatch away from its invocation", got)
	}
}

func TestExport_FromLogs_CarriesTheDispatchWithoutInvocationFinding(t *testing.T) {
	e := dispatchLogs()

	doExport(t, e, exportReq(""))

	var got []domain.Finding
	for _, f := range onlyWrite(t, e).d.Findings {
		if f.Kind == domain.FindingDispatchWithoutInvocation {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("exported dispatch_without_invocation findings = %+v, want exactly one", got)
	}
	if f := got[0]; f.Run != domain.NamedRun(dgRunA) || f.Actor != domain.Orchestrator() || f.Severity != domain.SeverityWarning {
		t.Errorf("exported finding = %+v, want a warning for run %s, scoped to the orchestrator", f, dgRunA)
	}
}

func TestAnalyze_ExportedDigestOnItsOwn_ShowsTheDispatchWithoutInvocationFinding(t *testing.T) {
	e := dispatchLogs()
	doExport(t, e, exportReq(""))
	d := onlyWrite(t, e).d
	digestOnly := newDigestEnv()
	digestOnly.digestFile(dgFilePath, d)

	res := analyzeDGReport(t, digestOnly, dgFilePath)

	got := dgFindings(res.Report, domain.FindingDispatchWithoutInvocation)
	if len(got) != 1 || got[0].Run != domain.NamedRun(dgRunA) {
		t.Errorf("findings = %+v, want exactly one, for run %s", got, dgRunA)
	}
}

func TestTotalForRun_RunWithAMissingDispatchTarget_TotalIsCompleteAndUnchanged(t *testing.T) {
	withDispatch := dispatchLogs()
	without := dispatchLogs()
	without.reader.events[without.logRuns[0].OrchestratorFile] = without.reader.events[without.logRuns[0].OrchestratorFile][:2]
	req := app.Request{ExplicitPath: dgLogsPath}

	got, outcome, err := withDispatch.service().TotalForRun(context.Background(), req, dgRunA)
	want, _, wantErr := without.service().TotalForRun(context.Background(), req, dgRunA)

	if err != nil || wantErr != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun: err=%v wantErr=%v outcome=%v", err, wantErr, outcome)
	}
	if !got.Complete {
		t.Error("total.Complete = false, want true")
	}
	if !reflect.DeepEqual(got.Tokens, want.Tokens) || got.Money != want.Money || got.Complete != want.Complete {
		t.Errorf("total = %+v / %+v, want it unchanged by the dispatches: %+v / %+v",
			got.Tokens, got.Money, want.Tokens, want.Money)
	}
}
