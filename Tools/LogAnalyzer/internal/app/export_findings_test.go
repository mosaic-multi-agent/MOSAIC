package app_test

// Tests for the findings an export carries: original log-reading findings and
// findings carried inside digests, never load-time or pricing findings, with
// run attribution of run-less findings by the folder they came from.

import (
	"context"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

var (
	exRunDirA   = dgLogsPath + "/" + dgRunA
	exRunDirB   = dgLogsPath + "/" + dgRunB
	exBucketDir = dgLogsPath + "/unknown-run"
)

// findingKey is the part of a finding a digest preserves.
type findingKey struct {
	Kind  domain.FindingKind
	Sev   domain.FindingSeverity
	Run   domain.RunRef
	Actor domain.ActorRef
}

func keysOf(fs []domain.Finding) []findingKey {
	var out []findingKey
	for _, f := range fs {
		out = append(out, findingKey{f.Kind, f.Severity, f.Run, f.Actor})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Run.ID < b.Run.ID || (a.Run.ID == b.Run.ID && a.Run.Kind < b.Run.Kind)
	})
	return out
}

// requireFindings asserts the digest carries exactly the wanted (kind, run) pairs.
func requireFindings(t *testing.T, got []domain.Finding, want ...domain.Finding) {
	t.Helper()
	g, w := keysOf(got), keysOf(want)
	if len(g) != len(w) {
		t.Fatalf("digest findings = %+v, want %+v", g, w)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("digest findings = %+v, want %+v", g, w)
		}
	}
}

// runless builds a log-reading finding that names no run, raised for path.
func runless(kind domain.FindingKind, path string) domain.Finding {
	return domain.Finding{Kind: kind, Severity: domain.SeverityWarning, Path: path}
}

// exported builds the finding an export is expected to carry.
func exported(kind domain.FindingKind, run domain.RunRef) domain.Finding {
	return domain.Finding{Kind: kind, Severity: domain.SeverityWarning, Run: run}
}

// findingsEnv scripts logs runs A and B and a bucket with the given enumeration findings.
func findingsEnv(enum ...domain.Finding) *digestEnv {
	e := logsWithTwoRunsAndBucket()
	e.enumFindings = enum
	return e
}

func TestExport_LogReadingFindings_AreCarried(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	orch := exRunDirA + "/00_orchestrator_events.jsonl"
	e.reader.addFindings(orch, domain.Finding{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning,
		Path: orch, Line: 3, Run: domain.NamedRun(dgRunA), Detail: "bad json",
	})

	doExport(t, e, exportReq(""))

	requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingMalformedLine, domain.NamedRun(dgRunA)))
}

func TestExport_FindingsCarriedInsideDigests_AreCarriedWithTheirScope(t *testing.T) {
	e := newDigestEnv()
	general := dgFinding(domain.FindingUnknownSchema, domain.RunRef{})
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1), dgRun(dgRunB, 2)),
		dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunB)), general)

	doExport(t, e, app.ExportRequest{ExplicitPath: dgFilePath, OutPath: exOut})

	requireFindings(t, onlyWrite(t, e).d.Findings,
		exported(domain.FindingMalformedLine, domain.NamedRun(dgRunB)),
		exported(domain.FindingUnknownSchema, domain.RunRef{}))
}

func TestExport_LoadTimeAndPricingFindings_AreNeverCarried(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.pricing.findings = []domain.Finding{{Kind: domain.FindingInvalidPricingEntry, Severity: domain.SeverityWarning}}
	e.digestDir(e.logsPath,
		digestItem{name: "dup.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 9)), // loses to logs: duplicate_run
			findings: []domain.Finding{dgFinding(domain.FindingUnsupportedDigestVersion, domain.RunRef{})}},
		digestItem{name: "bad.digest.json", findings: []domain.Finding{
			dgFinding(domain.FindingInvalidDigest, domain.RunRef{}),
		}},
		digestItem{name: "ok.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2)),
			findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunB))}},
	)

	doExport(t, e, exportReq(""))

	requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingMalformedLine, domain.NamedRun(dgRunB)))
}

func TestExport_RunlessFindingFromRunFolder_IsRecordedAgainstThatRun(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"run folder itself", exRunDirA},
		{"agent folder inside the run", exRunDirA + "/TestAgent#1"},
		{"event file inside the run", exRunDirA + "/00_orchestrator_events.jsonl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := findingsEnv(runless(domain.FindingMissingEventFile, tc.path))

			doExport(t, e, exportReq(""))

			requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingMissingEventFile, domain.NamedRun(dgRunA)))
		})
	}
}

func TestExport_RunlessFindingFromRunFolder_IsKeptInRunIDExportOfThatRunOnly(t *testing.T) {
	e := findingsEnv(runless(domain.FindingMissingEventFile, exRunDirA+"/TestAgent#1"))
	doExport(t, e, exportReq(dgRunA))
	requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingMissingEventFile, domain.NamedRun(dgRunA)))

	other := findingsEnv(runless(domain.FindingMissingEventFile, exRunDirA+"/TestAgent#1"))
	doExport(t, other, exportReq(dgRunB))
	requireFindings(t, onlyWrite(t, other).d.Findings)
}

func TestExport_RunlessFindingFromBucketFolder_IsRecordedAgainstTheBucket(t *testing.T) {
	e := findingsEnv(runless(domain.FindingMissingEventFile, exBucketDir+"/Agent#1"))

	doExport(t, e, exportReq(""))

	requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingMissingEventFile, domain.UnattributableRun()))
}

func TestExport_FindingFromNoRunFolder_IsGeneralOnlyWithoutRunID(t *testing.T) {
	junk := dgLogsPath + "/junk"
	lookalike := exRunDirA + "-extra" // shares the run folder's name as a prefix, not a parent
	for _, path := range []string{junk, dgLogsPath, lookalike} {
		t.Run(path, func(t *testing.T) {
			e := findingsEnv(runless(domain.FindingUnrecognisedFolder, path))
			doExport(t, e, exportReq(""))
			requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingUnrecognisedFolder, domain.RunRef{}))

			byRun := findingsEnv(runless(domain.FindingUnrecognisedFolder, path))
			doExport(t, byRun, exportReq(dgRunA))
			requireFindings(t, onlyWrite(t, byRun).d.Findings)
		})
	}
}

func TestExport_SingleRunSource_AttributesFindingsUnderSourcePathToThatRun(t *testing.T) {
	e := newDigestEnv()
	orch := exRunDirA + "/00_orchestrator_events.jsonl"
	e.reader.addEvents(orch, appTestTurnEvent(dgModel, domain.TokenUsage{Input: domain.Tokens(1)}), appTestRunEndEvent())
	e.logSource.classifyFunc = func(p string) domain.Source {
		if p == exRunDirA {
			return domain.Source{Kind: domain.SourceSingleRun, Path: p}
		}
		return domain.Source{Kind: domain.SourceNotFound, Path: p}
	}
	e.logSource.enumerateFunc = func(src domain.Source) (domain.Inventory, []domain.Finding) {
		return domain.Inventory{
			Source: src,
			Runs:   []domain.RunEntry{{Run: domain.NamedRun(dgRunA), Dir: exRunDirA, OrchestratorFile: orch}},
		}, []domain.Finding{runless(domain.FindingMissingEventFile, exRunDirA+"/TestAgent#1")}
	}

	doExport(t, e, app.ExportRequest{ExplicitPath: exRunDirA, OutPath: exOut})

	requireFindings(t, onlyWrite(t, e).d.Findings, exported(domain.FindingMissingEventFile, domain.NamedRun(dgRunA)))
}

func TestExport_RunIDExportOfCleanRun_NextToLossyRun_IsComplete(t *testing.T) {
	e := findingsEnv(runless(domain.FindingUnrecognisedFolder, dgLogsPath+"/junk"))
	orchB := exRunDirB + "/00_orchestrator_events.jsonl"
	e.reader.addFindings(orchB, domain.Finding{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning,
		Path: orchB, Line: 2, Run: domain.NamedRun(dgRunB),
	})

	doExport(t, e, exportReq(dgRunA))

	d := onlyWrite(t, e).d
	if len(d.Findings) != 0 || domain.NewQualitySummary(d.Findings).Incomplete() {
		t.Errorf("clean run's digest findings = %+v, want none (complete)", d.Findings)
	}
}

func TestExport_NoRunID_CarriesEveryScopeOfFinding(t *testing.T) {
	e := findingsEnv(
		runless(domain.FindingMissingEventFile, exRunDirB+"/Agent#1"),
		runless(domain.FindingUnrecognisedFolder, dgLogsPath+"/junk"),
		runless(domain.FindingUnreadableFile, exBucketDir),
	)
	orchA := exRunDirA + "/00_orchestrator_events.jsonl"
	e.reader.addFindings(orchA, domain.Finding{
		Kind: domain.FindingTruncatedLine, Severity: domain.SeverityWarning, Path: orchA, Run: domain.NamedRun(dgRunA),
	})

	doExport(t, e, exportReq(""))

	requireFindings(t, onlyWrite(t, e).d.Findings,
		exported(domain.FindingTruncatedLine, domain.NamedRun(dgRunA)),
		exported(domain.FindingMissingEventFile, domain.NamedRun(dgRunB)),
		exported(domain.FindingUnrecognisedFolder, domain.RunRef{}),
		exported(domain.FindingUnreadableFile, domain.UnattributableRun()),
	)
}

func TestExport_DoesNotChangeRunIDOfFindingsInTheLogReport(t *testing.T) {
	e := findingsEnv(runless(domain.FindingMissingEventFile, exRunDirA+"/TestAgent#1"))
	svc := e.service()

	if _, err := svc.Export(context.Background(), exportReq("")); err != nil {
		t.Fatalf("Export returned unexpected error: %v", err)
	}
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: dgLogsPath})
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}

	got := dgFindings(res.Report, domain.FindingMissingEventFile)
	if len(got) != 1 || got[0].Run != (domain.RunRef{}) {
		t.Errorf("report findings = %+v, want one finding with no run (R-15)", got)
	}
	if len(onlyWrite(t, e).d.Findings) != 1 {
		t.Error("the export must still have written the finding")
	}
}
