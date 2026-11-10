package app_test

// Tests that Analyze turns adapter format-change alarms into a data-loss
// finding on the alarmed run.
//
// Rules under test:
//   - A loaded named run with at least one alarm gets exactly one warning
//     finding carrying the run and the path and line of its first alarm; the
//     report quality is incomplete, so the run's cost is not shown as a real zero.
//   - Runs without an alarm, alarms for runs that are not loaded and a service
//     without an alarm source change nothing.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// fakeAlarmSource returns scripted alarms and records the sources it was asked about.
type fakeAlarmSource struct {
	alarms  []domain.TranscriptAlarm
	sources []domain.Source
}

func (f *fakeAlarmSource) TranscriptAlarms(_ context.Context, src domain.Source) []domain.TranscriptAlarm {
	f.sources = append(f.sources, src)
	return f.alarms
}

// newAlarmTestService builds a service over a one-run source whose run has no
// usage. alarms may be nil to leave the port unwired.
func newAlarmTestService(alarms domain.AlarmSource) *app.Service {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			inv := oneRunTwoAgentInventory(s)
			inv.Runs[0].Agents = nil
			return inv, nil
		},
	}
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile, appTestRunEndEvent())
	deps := app.Deps{
		Source:      logSource,
		Reader:      reader,
		Pricing:     newFakePricingStore(),
		Clock:       newFakeClock(),
		Interaction: &alwaysSkippingInteraction{},
		WorkDir:     "/workdir",
	}
	if alarms != nil {
		deps.Alarms = alarms
	}
	return app.New(deps)
}

func alarmFor(runID string, line int) domain.TranscriptAlarm {
	return domain.TranscriptAlarm{
		Run: domain.NamedRun(runID), Harness: domain.HarnessGHCPCLI, SessionID: "s1",
		Path: "/test-logs/mosaic-logger-diagnostics.jsonl", Line: line,
	}
}

func alarmFindings(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Kind == domain.FindingTranscriptFormatChanged {
			out = append(out, f)
		}
	}
	return out
}

func analyzeWithAlarms(t *testing.T, alarms domain.AlarmSource) domain.Report {
	t.Helper()
	result, err := newAlarmTestService(alarms).Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
	if err != nil || result.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze: err=%v outcome=%v", err, result.Outcome)
	}
	return result.Report
}

func TestAnalyze_RunWithAlarm_GetsOneDataLossWarningFindingForThatRun(t *testing.T) {
	src := &fakeAlarmSource{alarms: []domain.TranscriptAlarm{alarmFor(appTestRunID, 3), alarmFor(appTestRunID, 9)}}

	report := analyzeWithAlarms(t, src)

	got := alarmFindings(report.Quality.Findings)
	if len(got) != 1 {
		t.Fatalf("alarm findings = %+v, want exactly one for two alarms of the same run", got)
	}
	f := got[0]
	if f.Run != domain.NamedRun(appTestRunID) || f.Severity != domain.SeverityWarning {
		t.Errorf("finding run %+v severity %v, want the alarmed run and warning", f.Run, f.Severity)
	}
	if f.Path != "/test-logs/mosaic-logger-diagnostics.jsonl" || f.Line != 3 {
		t.Errorf("finding location = %q:%d, want the first alarm's", f.Path, f.Line)
	}
	if f.Detail == "" {
		t.Error("finding has no detail text")
	}
	if !report.Quality.Incomplete() {
		t.Error("quality is complete, want incomplete so the run's cost is not presented as a real zero")
	}
	if len(src.sources) == 0 || src.sources[0].Path != "/test-logs" {
		t.Errorf("alarm source asked about %+v, want the analysed source", src.sources)
	}
}

func TestAnalyze_RunWithoutAlarmEntry_IsUnchanged(t *testing.T) {
	report := analyzeWithAlarms(t, &fakeAlarmSource{})

	if got := alarmFindings(report.Quality.Findings); len(got) != 0 {
		t.Errorf("alarm findings = %+v, want none", got)
	}
	if report.Quality.Incomplete() {
		t.Error("quality is incomplete without any alarm")
	}
}

func TestAnalyze_AlarmForAnotherRun_ChangesNothing(t *testing.T) {
	report := analyzeWithAlarms(t, &fakeAlarmSource{alarms: []domain.TranscriptAlarm{alarmFor("20261031T100000Z-ffff", 1)}})

	if got := alarmFindings(report.Quality.Findings); len(got) != 0 {
		t.Errorf("alarm findings = %+v, want none for a run that is not loaded", got)
	}
	if report.Quality.Incomplete() {
		t.Error("quality is incomplete for an alarm of a run that is not loaded")
	}
}

func TestAnalyze_WithoutAlarmSource_ReadsNoAlarms(t *testing.T) {
	report := analyzeWithAlarms(t, nil)

	if got := alarmFindings(report.Quality.Findings); len(got) != 0 {
		t.Errorf("alarm findings = %+v, want none", got)
	}
}
