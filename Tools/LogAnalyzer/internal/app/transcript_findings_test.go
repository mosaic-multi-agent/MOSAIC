package app_test

// Tests for the transcript integrity findings as they travel through the
// service with a fake transcript port.
//
// Rules under test:
//   - transcript_capture_gap and instance_prompt_mismatch appear in the report
//     for the folder id as actor, never make the report incomplete, and are
//     carried by a digest export (log-only kinds).
//   - They are never raised for a digest-loaded run, and a service without a
//     transcript port raises none.
//   - total never reads transcripts and its figures are unchanged.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// transcriptLogs scripts one logs run with one single-start agent folder whose
// stream holds the usage records m1 only.
func transcriptLogs() *digestEnv {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	tcAddAgent(e, tcAgent, "m1")
	return e
}

func TestAnalyze_TranscriptMessageWithoutUsageRecord_ReportsOneCaptureGapForTheFolderId(t *testing.T) {
	e := transcriptLogs()
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1", "m2"}, ""),
	}}

	res, err := tcAnalyze(e, port)

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	f := requireOneFinding(t, res.Report, domain.FindingTranscriptCaptureGap, domain.NamedRun(dgRunA), tcAgent)
	if !hasNumberToken(f.Detail, 1) {
		t.Errorf("detail %q does not state the count 1", f.Detail)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("a transcript capture gap must not make the report incomplete")
	}
}

func TestAnalyze_AgentWhoseFirstPromptNamesAnotherId_ReportsOneInstancePromptMismatch(t *testing.T) {
	e := transcriptLogs()
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1"}, "agent_instance_id: "+string(tcOther)+"\nDo the work."),
	}}

	res, err := tcAnalyze(e, port)

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	requireOneFinding(t, res.Report, domain.FindingInstancePromptMismatch, domain.NamedRun(dgRunA), tcAgent)
	if got := dgFindings(res.Report, domain.FindingTranscriptCaptureGap); len(got) != 0 {
		t.Errorf("capture gap findings = %+v, want none: the ids match", got)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("an instance prompt mismatch must not make the report incomplete")
	}
}

func TestAnalyze_OrchestratorTranscriptWithMoreMessagesThanUsage_ReportsACaptureGapForTheOrchestrator(t *testing.T) {
	e := transcriptLogs()
	e.reader.addEvents(e.logRuns[0].OrchestratorFile, tcUsageRecord("o1"))
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{tcOrchFacts(dgRunA, "o1", "o2", "o3")}}

	res, err := tcAnalyze(e, port)

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	got := dgFindings(res.Report, domain.FindingTranscriptCaptureGap)
	if len(got) != 1 || got[0].Actor != domain.Orchestrator() || got[0].Run != domain.NamedRun(dgRunA) {
		t.Fatalf("findings = %+v, want one for the orchestrator of %s", got, dgRunA)
	}
	if !hasNumberToken(got[0].Detail, 2) {
		t.Errorf("detail %q does not state the count 2", got[0].Detail)
	}
}

func TestAnalyze_AgentStreamWithSeveralStarts_ReportsNoCaptureGap(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	addLogAgent(e, 0, tcAgent,
		igStart(tcAgent, "planner"), tcUsageRecord("m1"), appTestInvEndEvent(tcAgent, dgModel, igUsage()),
		igStart(tcAgent, "planner"), appTestInvEndEvent(tcAgent, dgModel, igUsage()))
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1", "m2"}, ""),
	}}

	res, err := tcAnalyze(e, port)

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := dgFindings(res.Report, domain.FindingTranscriptCaptureGap); len(got) != 0 {
		t.Errorf("findings = %+v, want none for a stream with two starts", got)
	}
	if len(port.calls) == 0 {
		t.Error("the transcript port was not consulted at all")
	}
}

func TestAnalyze_TranscriptPortIsAskedAboutTheLogRuns(t *testing.T) {
	e := transcriptLogs()
	port := &MockTranscriptSource{}

	if _, err := tcAnalyze(e, port); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(port.calls) != 1 || len(port.calls[0].Runs) != 1 || port.calls[0].Runs[0].Run != domain.NamedRun(dgRunA) {
		t.Errorf("port calls = %+v, want one call with the inventory of run %s", port.calls, dgRunA)
	}
}

func TestAnalyze_WithoutATranscriptPort_RaisesNoTranscriptFindings(t *testing.T) {
	res := analyzeDGReport(t, transcriptLogs(), dgLogsPath)

	for _, kind := range []domain.FindingKind{domain.FindingTranscriptCaptureGap, domain.FindingInstancePromptMismatch} {
		if got := dgFindings(res.Report, kind); len(got) != 0 {
			t.Errorf("%s findings = %+v, want none", kind, got)
		}
	}
}

func TestExport_FromLogs_CarriesBothTranscriptFindingKinds(t *testing.T) {
	e := transcriptLogs()
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1", "m2"}, "agent_instance_id: "+string(tcOther)),
	}}

	if _, err := tcService(e, port).Export(context.Background(), exportReq("")); err != nil {
		t.Fatalf("Export: %v", err)
	}

	count := map[domain.FindingKind]int{}
	for _, f := range onlyWrite(t, e).d.Findings {
		if f.Kind != domain.FindingTranscriptCaptureGap && f.Kind != domain.FindingInstancePromptMismatch {
			continue
		}
		count[f.Kind]++
		if f.Run != domain.NamedRun(dgRunA) || f.Actor != domain.AgentInstance(tcAgent) || f.Severity != domain.SeverityWarning {
			t.Errorf("exported %s finding = %+v, want a warning for run %s and the folder id", f.Kind, f, dgRunA)
		}
	}
	want := map[domain.FindingKind]int{domain.FindingTranscriptCaptureGap: 1, domain.FindingInstancePromptMismatch: 1}
	if !reflect.DeepEqual(count, want) {
		t.Errorf("exported transcript findings = %v, want %v", count, want)
	}
}

func TestAnalyze_DigestLoadedRun_GetsNoTranscriptFindingFromThePort(t *testing.T) {
	// Logs hold run A; run B comes from a digest. The port knows facts for both.
	e := transcriptLogs()
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1"}, "agent_instance_id: "+string(tcOther)),
		tcAgentFacts(dgRunB, tcAgent, []string{"m1"}, "agent_instance_id: "+string(tcOther)),
	}}

	res, err := tcAnalyze(e, port)

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	got := dgFindings(res.Report, domain.FindingInstancePromptMismatch)
	if len(got) != 1 || got[0].Run != domain.NamedRun(dgRunA) {
		t.Errorf("findings = %+v, want exactly one, for the log run %s only", got, dgRunA)
	}
}

func TestAnalyze_DigestOnlySource_GetsNoTranscriptFindings(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1"}, "agent_instance_id: "+string(tcOther)),
	}}

	res, err := tcService(e, port).Analyze(context.Background(), app.Request{ExplicitPath: dgFilePath})

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	for _, kind := range []domain.FindingKind{domain.FindingTranscriptCaptureGap, domain.FindingInstancePromptMismatch} {
		if got := dgFindings(res.Report, kind); len(got) != 0 {
			t.Errorf("%s findings = %+v, want none for a digest source", kind, got)
		}
	}
}

func TestTotalForRun_WithATranscriptPort_NeverReadsTranscriptsAndTotalIsUnchanged(t *testing.T) {
	port := &MockTranscriptSource{facts: []domain.TranscriptFacts{
		tcAgentFacts(dgRunA, tcAgent, []string{"m1", "m2"}, "agent_instance_id: "+string(tcOther)),
	}}
	req := app.Request{ExplicitPath: dgLogsPath}

	got, outcome, err := tcService(transcriptLogs(), port).TotalForRun(context.Background(), req, dgRunA)
	want, _, wantErr := tcService(transcriptLogs(), nil).TotalForRun(context.Background(), req, dgRunA)

	if err != nil || wantErr != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun: err=%v wantErr=%v outcome=%v", err, wantErr, outcome)
	}
	if !got.Complete || !reflect.DeepEqual(got.Tokens, want.Tokens) || got.Money != want.Money {
		t.Errorf("total = %+v, want it unchanged: %+v", got, want)
	}
	if len(port.calls) != 0 {
		t.Errorf("the transcript port was called %d times by total, want 0", len(port.calls))
	}
}
