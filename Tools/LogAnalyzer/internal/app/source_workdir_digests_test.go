package app_test

// Tests for the default start (no path) when digest files lie directly in the
// working directory: they are loaded in addition to OrchestrationLogs/ and the
// digests inside it, under the existing merge precedence; a working directory
// holding only digests opens without a source prompt; the prompt appears only
// when neither logs nor digests exist; explicit paths and the logs-only
// resolution used by total are not affected.

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const wdDir = "/workdir"

// newWorkdirEnv builds a digest environment whose default location resolves to
// the scripted logs root when logs were added, and to nothing otherwise.
func newWorkdirEnv() *digestEnv {
	e := newDigestEnv()
	e.logSource.defaultFunc = func(_ string) domain.Source {
		if e.hasLogs {
			return domain.Source{Kind: domain.SourceLogsRoot, Path: e.logsPath}
		}
		return domain.Source{Kind: domain.SourceNotFound}
	}
	return e
}

// workdirDigests scripts the digests lying directly in the working directory.
func (e *digestEnv) workdirDigests(items ...digestItem) {
	e.digestDir(wdDir, items...)
}

func analyzeDefault(t *testing.T, e *digestEnv) app.Result {
	t.Helper()
	res, err := e.service().Analyze(context.Background(), app.Request{})
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}
	return res
}

func sortedRunIDs(r domain.Report) []string {
	ids := dgRunIDs(r)
	sort.Strings(ids)
	return ids
}

func TestAnalyze_DefaultStart_LogsAndWorkdirDigests_BothLoaded(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := analyzeDefault(t, e)

	if res.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport", res.Outcome)
	}
	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireInput(t, res.Report, dgRunB, 2_000_000)
	// Decision: with logs present the combined source stays the logs root
	// (transcripts and alarms are keyed on it).
	if res.Source.Kind != domain.SourceLogsRoot || res.Source.Path != e.logsPath {
		t.Errorf("Source = %+v, want the logs root %s", res.Source, e.logsPath)
	}
}

func TestAnalyze_DefaultStart_RunInLogsAndWorkdirDigest_LogWinsCountedOnce(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 9_000_000))})

	res := analyzeDefault(t, e)

	if got := dgRunIDs(res.Report); !reflect.DeepEqual(got, []string{dgRunA}) {
		t.Fatalf("run ids = %v, want [%s] counted once", got, dgRunA)
	}
	requireInput(t, res.Report, dgRunA, 1_000_000)
	if n := len(dgFindings(res.Report, domain.FindingDuplicateRun)); n != 1 {
		t.Errorf("duplicate_run findings = %d, want 1", n)
	}
}

func TestAnalyze_DefaultStart_RunInTwoWorkdirDigests_NewestWritten(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(
		digestItem{name: "a.digest.json", d: dgDigest(dgT2, dgRun(dgRunB, 5_000_000))},
		digestItem{name: "b.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 7_000_000))},
	)

	res := analyzeDefault(t, e)

	requireInput(t, res.Report, dgRunB, 5_000_000)
}

func TestAnalyze_DefaultStart_EqualWrittenAt_LogsFolderDigestWinsOverWorkdirDigest(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "l.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 3_000_000))})
	e.workdirDigests(digestItem{name: "w.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 4_000_000), dgRun(dgRunC, 5_000_000))})

	res := analyzeDefault(t, e)

	// The workdir-only run proves the workdir digest was read at all; the
	// tied run proves it loses against the digest beside the logs.
	requireInput(t, res.Report, dgRunC, 5_000_000)
	requireInput(t, res.Report, dgRunB, 3_000_000)
}

// Optional guard: a file listed by both discoveries (only possible when the
// working directory is the logs folder itself) must not be read twice nor
// reported as its own duplicate. The workdir-only file makes the test fail
// unless the workdir listing is consulted at all.
func TestAnalyze_DefaultStart_FileListedForBothLocations_ReadOnceWithoutDuplicateFinding(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	shared := digestItem{name: "s.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))}
	e.digestDir(e.logsPath, shared)
	sharedPath := e.digests.discover[e.logsPath][0]
	e.workdirDigests(digestItem{name: "w.digest.json", d: dgDigest(dgT1, dgRun(dgRunC, 5_000_000))})
	e.digests.discover[wdDir] = append([]string{sharedPath}, e.digests.discover[wdDir]...)

	res := analyzeDefault(t, e)

	requireInput(t, res.Report, dgRunC, 5_000_000)
	requireInput(t, res.Report, dgRunB, 2_000_000)
	if n := len(dgFindings(res.Report, domain.FindingDuplicateRun)); n != 0 {
		t.Errorf("duplicate_run findings = %d, want 0 for one file seen twice", n)
	}
	reads := 0
	for _, p := range e.digests.readCalls {
		if p == sharedPath {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("shared file reads = %d (%v), want 1", reads, e.digests.readCalls)
	}
}

// Decision: a digest in the working directory (for example a TUI export saved
// there) whose runs all exist in the logs keeps the existing duplicate rule:
// every run counted once, log copy wins, one duplicate_run info finding per
// duplicated run. Nothing is suppressed.
func TestAnalyze_DefaultStart_WorkdirExportOfLogRuns_CountedOnceWithOneFindingPerRun(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.addLogRun(dgRunB, 2_000_000)
	e.workdirDigests(digestItem{name: "export.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 9_000_000), dgRun(dgRunB, 9_000_000))})

	res := analyzeDefault(t, e)

	if got := sortedRunIDs(res.Report); !reflect.DeepEqual(got, []string{dgRunA, dgRunB}) {
		t.Fatalf("run ids = %v, want [%s %s] each once", got, dgRunA, dgRunB)
	}
	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireInput(t, res.Report, dgRunB, 2_000_000)
	if n := len(dgFindings(res.Report, domain.FindingDuplicateRun)); n != 2 {
		t.Errorf("duplicate_run findings = %d, want 2 (one per duplicated run)", n)
	}
}

// Regression coverage for the combined source: transcripts and alarms are
// still read from the logs source, not from the working directory.
func TestAnalyze_DefaultStart_LogsAndWorkdirDigests_TranscriptsAndAlarmsReadFromLogsSource(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})
	alarms := &fakeAlarmSource{alarms: []domain.TranscriptAlarm{alarmFor(dgRunA, 4)}}
	transcripts := &MockTranscriptSource{}
	svc := app.New(app.Deps{
		Source: e.logSource, Reader: e.reader, Pricing: e.pricing, Digests: e.digests,
		Clock: e.clock, Interaction: e.interaction, WorkDir: "/workdir",
		Alarms: alarms, Transcripts: transcripts,
	})

	res, err := svc.Analyze(context.Background(), app.Request{})

	if err != nil || res.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze: err=%v outcome=%v", err, res.Outcome)
	}
	requireInput(t, res.Report, dgRunB, 2_000_000)
	if len(alarms.sources) == 0 || alarms.sources[0].Kind != domain.SourceLogsRoot || alarms.sources[0].Path != e.logsPath {
		t.Errorf("alarm source asked about %+v, want the logs root %s", alarms.sources, e.logsPath)
	}
	if got := alarmFindings(res.Report.Quality.Findings); len(got) != 1 || got[0].Run != domain.NamedRun(dgRunA) {
		t.Errorf("alarm findings = %+v, want one for run %s", got, dgRunA)
	}
	if len(transcripts.calls) == 0 || transcripts.calls[0].Source.Kind != domain.SourceLogsRoot || transcripts.calls[0].Source.Path != e.logsPath {
		t.Errorf("transcripts asked about %+v, want an inventory of the logs root %s", transcripts.calls, e.logsPath)
	}
}

func TestAnalyze_DefaultStart_NoLogsButWorkdirDigests_OpensWithoutPrompt(t *testing.T) {
	e := newWorkdirEnv()
	ia := &scriptedInteraction{}
	e.withInteraction(ia)
	e.workdirDigests(
		digestItem{name: "a.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))},
		digestItem{name: "b.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))},
	)

	res := analyzeDefault(t, e)

	if res.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport", res.Outcome)
	}
	if len(ia.asked) != 0 {
		t.Errorf("questions asked = %v, want none", ia.asked)
	}
	if got := sortedRunIDs(res.Report); !reflect.DeepEqual(got, []string{dgRunA, dgRunB}) {
		t.Errorf("run ids = %v, want [%s %s]", got, dgRunA, dgRunB)
	}
	// Decision: a workdir holding only digests reports itself as a digest directory.
	if res.Source.Kind != domain.SourceDigestDir || res.Source.Path != wdDir {
		t.Errorf("Source = %+v, want a digest directory at %s", res.Source, wdDir)
	}
}

func TestAnalyze_DefaultStart_NeitherLogsNorDigests_PromptsForSource(t *testing.T) {
	e := newWorkdirEnv()
	ia := &scriptedInteraction{}
	e.withInteraction(ia)

	res := analyzeDefault(t, e)

	if !ia.askTextCalledWith(app.QuestionLogSourcePath) {
		t.Errorf("source prompt not asked; asked = %v", ia.asked)
	}
	if res.Outcome != app.OutcomeSourceNotFound {
		t.Errorf("Outcome = %v, want OutcomeSourceNotFound when the prompt is skipped", res.Outcome)
	}
}

func TestAnalyze_DefaultStart_LogsPresent_NoPrompt(t *testing.T) {
	e := newWorkdirEnv()
	ia := &scriptedInteraction{}
	e.withInteraction(ia)
	e.addLogRun(dgRunA, 1_000_000)

	analyzeDefault(t, e)

	if len(ia.asked) != 0 {
		t.Errorf("questions asked = %v, want none when logs exist", ia.asked)
	}
}

func TestAnalyze_ExplicitPath_IgnoresWorkdirDigests(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := analyzeDGReport(t, e, e.logsPath)

	if _, ok := res.Report.FindRun(dgRunB); ok {
		t.Errorf("workdir digest run %s must not load for an explicit path; runs = %v", dgRunB, dgRunIDs(res.Report))
	}
}

func TestTotalForRun_DefaultStart_IgnoresWorkdirDigests(t *testing.T) {
	e := newWorkdirEnv()
	ia := &scriptedInteraction{}
	e.withInteraction(ia)
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))})

	_, outcome, err := e.service().TotalForRun(context.Background(), app.Request{}, dgRunA)

	if err != nil {
		t.Fatalf("TotalForRun returned unexpected error: %v", err)
	}
	if outcome != app.OutcomeSourceNotFound {
		t.Errorf("Outcome = %v, want OutcomeSourceNotFound (the logs-only resolution never looks at digests)", outcome)
	}
	if !ia.askTextCalledWith(app.QuestionLogSourcePath) {
		t.Errorf("source prompt not asked; asked = %v", ia.asked)
	}
}
