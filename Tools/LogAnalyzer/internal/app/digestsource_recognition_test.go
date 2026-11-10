package app_test

// Tests for how a digest file is recognised, run against real temp directories
// with the real scanner and digest store: the file name must end in
// .digest.json (case-insensitive); other JSON files and non-digest files are
// ignored without a finding; the digest store is not searched recursively.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logscan"
)

func realService(reader *fakeEventReader) *app.Service {
	return app.New(app.Deps{
		Source:      logscan.NewScanner(),
		Reader:      reader,
		Pricing:     newFakePricingStore(),
		Digests:     digest.NewStore(),
		Clock:       newFakeClock(),
		Interaction: &alwaysSkippingInteraction{},
		WorkDir:     "/workdir",
	})
}

func writeRealDigest(t *testing.T, p string, d domain.Digest) {
	t.Helper()
	if err := digest.NewStore().Write(context.Background(), p, d, false); err != nil {
		t.Fatalf("writing digest %s: %v", p, err)
	}
}

func writeOtherFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func analyzeReal(t *testing.T, path string) app.Result {
	t.Helper()
	res, err := realService(newFakeEventReader()).Analyze(context.Background(), app.Request{ExplicitPath: path})
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}
	return res
}

func TestAnalyze_RealDigestFile_IsAcceptedAsDigestFileSource(t *testing.T) {
	p := filepath.Join(t.TempDir(), "my.digest.json")
	writeRealDigest(t, p, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeReal(t, p)

	if res.Outcome != app.OutcomeReport || res.Source.Kind != domain.SourceDigestFile {
		t.Fatalf("Outcome/Kind = %v/%v, want report from a digest file", res.Outcome, res.Source.Kind)
	}
	requireInput(t, res.Report, dgRunA, 1_000_000)
}

func TestAnalyze_RealDigestFileWithUpperCaseSuffix_IsRecognised(t *testing.T) {
	p := filepath.Join(t.TempDir(), "MY.DIGEST.JSON")
	writeRealDigest(t, p, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeReal(t, p)

	if res.Outcome != app.OutcomeReport {
		t.Errorf("Outcome = %v, want OutcomeReport (suffix match is case-insensitive)", res.Outcome)
	}
}

func TestAnalyze_RealDigestContentUnderOtherFileName_IsNotRecognised(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "my.json")
	writeRealDigest(t, filepath.Join(dir, "tmp.digest.json"), dgDigest(dgT1, dgRun(dgRunA, 1)))
	raw, err := os.ReadFile(filepath.Join(dir, "tmp.digest.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeOtherFile(t, p, string(raw))

	res := analyzeReal(t, p)

	if res.Outcome != app.OutcomeSourceUnusable {
		t.Errorf("Outcome = %v, want OutcomeSourceUnusable (name does not end in .digest.json)", res.Outcome)
	}
}

func TestAnalyze_RealDirectoryOfDigestsAndOtherFiles_LoadsDigestsAndIgnoresTheRest(t *testing.T) {
	dir := t.TempDir()
	writeRealDigest(t, filepath.Join(dir, "a.digest.json"), dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))
	writeRealDigest(t, filepath.Join(dir, "b.DIGEST.JSON"), dgDigest(dgT1, dgRun(dgRunB, 2_000_000)))
	writeOtherFile(t, filepath.Join(dir, "settings.json"), `{"not":"a digest"}`)
	writeOtherFile(t, filepath.Join(dir, "notes.txt"), "hello")
	writeOtherFile(t, filepath.Join(dir, "digest.json"), `{}`)
	writeOtherFile(t, filepath.Join(dir, ".hidden.digest.json"), `{}`)
	writeRealDigest(t, filepath.Join(dir, "nested", "c.digest.json"), dgDigest(dgT1, dgRun(dgRunC, 3_000_000)))

	res := analyzeReal(t, dir)

	if res.Outcome != app.OutcomeReport || res.Source.Kind != domain.SourceDigestDir {
		t.Fatalf("Outcome/Kind = %v/%v, want report from a digest directory", res.Outcome, res.Source.Kind)
	}
	if got := dgRunIDs(res.Report); len(got) != 2 || got[0] != dgRunA || got[1] != dgRunB {
		t.Errorf("run ids = %v, want [%s %s] (other files and nested directories ignored)", got, dgRunA, dgRunB)
	}
	if !res.Report.Quality.IsClean() {
		t.Errorf("ignored files must raise no findings: %+v", res.Report.Quality.Findings)
	}
}

func TestAnalyze_RealDirectoryWithOnlyOtherJSON_IsUnusable(t *testing.T) {
	dir := t.TempDir()
	writeOtherFile(t, filepath.Join(dir, "settings.json"), `{"not":"a digest"}`)

	res := analyzeReal(t, dir)

	if res.Outcome != app.OutcomeSourceUnusable {
		t.Errorf("Outcome = %v, want OutcomeSourceUnusable", res.Outcome)
	}
}

func TestAnalyze_RealLogsRootHoldingDigest_KeepsLogsRootKindAndLoadsBoth(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, dgRunA)
	orch := filepath.Join(runDir, logscan.OrchestratorEventFile)
	writeOtherFile(t, orch, "")
	writeRealDigest(t, filepath.Join(root, "other.digest.json"), dgDigest(dgT1, dgRun(dgRunB, 2_000_000)))
	reader := newFakeEventReader()
	reader.addEvents(orch, appTestTurnEvent(dgModel, domain.TokenUsage{Input: domain.Tokens(1_000_000)}), appTestRunEndEvent())

	res, err := realService(reader).Analyze(context.Background(), app.Request{ExplicitPath: root})

	if err != nil || res.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze = (%v, %v), want OutcomeReport", res.Outcome, err)
	}
	if res.Source.Kind != domain.SourceLogsRoot {
		t.Errorf("Source.Kind = %v, want SourceLogsRoot", res.Source.Kind)
	}
	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireInput(t, res.Report, dgRunB, 2_000_000)
}
