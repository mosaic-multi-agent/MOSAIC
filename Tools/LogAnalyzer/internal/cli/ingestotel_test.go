package cli_test

// Tests for the ingest-otel command against a scripted ingester.
//
// Rules under test:
//   - The export file comes from the positional argument, else from the
//     LogAnalyzer environment variable; the argument wins.
//   - The ingester gets the run's folder; success prints one summary line on
//     stdout and each diagnostic as "<kind>: <message>" on stderr.
//   - Every refusal has its own documented exit code and a message on stderr
//     that is not the generic "unknown command" error.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const (
	ingestEnvVar = "COPILOT_OTEL_FILE_EXPORTER_PATH"
	ingestRunDir = "/logs/" + cliTestRunID
)

type fakeUsageIngester struct {
	result domain.UsageIngestResult
	err    error
	calls  []domain.UsageIngestRequest
}

func (f *fakeUsageIngester) IngestRun(_ context.Context, req domain.UsageIngestRequest) (domain.UsageIngestResult, error) {
	f.calls = append(f.calls, req)
	return f.result, f.err
}

// runIngestOtel runs the CLI with one named run in a logs root classified as src.
// env is LogAnalyzer's own environment; a nil env.Getenv is covered separately.
func runIngestOtel(t *testing.T, src domain.Source, ing *fakeUsageIngester, env map[string]string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			return domain.Inventory{Source: s, Runs: []domain.RunEntry{{
				Run: cliTestRunRef, Dir: ingestRunDir, OrchestratorFile: cliTestOrchFile,
			}}}, nil
		},
	}
	svc := app.New(app.Deps{
		Source: logSource, Reader: newFakeEventReader(), Pricing: newFakePricingStore(),
		Clock: newFakeClock(), Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
		Ingester: ing,
	})
	var out, errOut bytes.Buffer
	code = cli.Execute(cli.Options{
		Service: svc, Stdout: &out, Stderr: &errOut,
		Getenv: func(k string) string { return env[k] },
	}, args)
	return code, out.String(), errOut.String()
}

func logsRoot() domain.Source { return domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"} }

func doneResult() domain.UsageIngestResult {
	return domain.UsageIngestResult{Status: domain.IngestDone, RecordsWritten: 2, Finalized: 1, Pending: 0}
}

func assertRefusedWithMessage(t *testing.T, code, want int, stderr string, ing *fakeUsageIngester) {
	t.Helper()
	if code != want {
		t.Errorf("exit code = %d, want %d (stderr: %q)", code, want, stderr)
	}
	if strings.TrimSpace(stderr) == "" || strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q, want a message about the refusal", stderr)
	}
	if len(ing.calls) != 0 {
		t.Errorf("ingester called %d times for a refused request", len(ing.calls))
	}
}

func TestIngestOtel_PathArgument_IngestsThatFileIntoTheRunFolder(t *testing.T) {
	ing := &fakeUsageIngester{result: doneResult()}

	code, stdout, stderr := runIngestOtel(t, logsRoot(), ing, nil,
		"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/exports/otel.jsonl")

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr: %q)", code, cli.ExitSuccess, stderr)
	}
	if len(ing.calls) != 1 || ing.calls[0].RunDir != ingestRunDir || ing.calls[0].ExportPath != "/exports/otel.jsonl" {
		t.Errorf("ingester calls = %+v, want one for %q with /exports/otel.jsonl", ing.calls, ingestRunDir)
	}
	want := "run " + cliTestRunID + ": 2 records written, 1 finalised as unattributed, 0 pending"
	if strings.TrimSpace(stdout) != want {
		t.Errorf("stdout = %q, want the summary line %q", stdout, want)
	}
}

func TestIngestOtel_NoArgument_TakesTheFileFromTheEnvironmentVariable(t *testing.T) {
	ing := &fakeUsageIngester{result: doneResult()}
	env := map[string]string{ingestEnvVar: "/env/otel.jsonl"}

	code, _, stderr := runIngestOtel(t, logsRoot(), ing, env, "ingest-otel", "--path", "/logs", "--run", cliTestRunID)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr: %q)", code, cli.ExitSuccess, stderr)
	}
	if len(ing.calls) != 1 || ing.calls[0].ExportPath != "/env/otel.jsonl" {
		t.Errorf("ingester calls = %+v, want one for /env/otel.jsonl", ing.calls)
	}
}

func TestIngestOtel_ArgumentAndVariable_ArgumentWins(t *testing.T) {
	ing := &fakeUsageIngester{result: doneResult()}
	env := map[string]string{ingestEnvVar: "/env/otel.jsonl"}

	code, _, _ := runIngestOtel(t, logsRoot(), ing, env,
		"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/arg/otel.jsonl")

	if code != cli.ExitSuccess || len(ing.calls) != 1 || ing.calls[0].ExportPath != "/arg/otel.jsonl" {
		t.Errorf("exit = %d, calls = %+v, want success with /arg/otel.jsonl", code, ing.calls)
	}
}

func TestIngestOtel_FormatFlagIsIgnored(t *testing.T) {
	ing := &fakeUsageIngester{result: doneResult()}

	code, stdout, _ := runIngestOtel(t, logsRoot(), ing, nil,
		"ingest-otel", "--format", "json", "--path", "/logs", "--run", cliTestRunID, "/x.jsonl")

	if code != cli.ExitSuccess || !strings.HasPrefix(stdout, "run "+cliTestRunID+": ") {
		t.Errorf("exit = %d, stdout = %q, want the plain summary line", code, stdout)
	}
}

func TestIngestOtel_Diagnostics_GoToStderrAsKindAndMessage(t *testing.T) {
	res := doneResult()
	res.Diagnostics = []domain.UsageIngestDiagnostic{
		{Kind: "lines_skipped", Message: "2 of 40 lines were not valid JSON"},
		{Kind: "conversation_not_tied", Message: "usage spans belong to other sessions"},
	}
	ing := &fakeUsageIngester{result: res}

	code, stdout, stderr := runIngestOtel(t, logsRoot(), ing, nil,
		"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/x.jsonl")

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	for _, want := range []string{"lines_skipped: 2 of 40 lines were not valid JSON", "conversation_not_tied: usage spans belong to other sessions"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stdout, "lines_skipped") {
		t.Errorf("stdout carries a diagnostic:\n%s", stdout)
	}
}

func TestIngestOtel_NoArgumentAndVariableUnset_ExitsUsageNamingTheVariable(t *testing.T) {
	ing := &fakeUsageIngester{}

	code, _, stderr := runIngestOtel(t, logsRoot(), ing, nil, "ingest-otel", "--path", "/logs", "--run", cliTestRunID)

	assertRefusedWithMessage(t, code, cli.ExitUsage, stderr, ing)
	if !strings.Contains(stderr, ingestEnvVar) {
		t.Errorf("stderr = %q, want it to name %s", stderr, ingestEnvVar)
	}
}

func TestIngestOtel_EmptyVariable_CountsAsUnset(t *testing.T) {
	ing := &fakeUsageIngester{}
	env := map[string]string{ingestEnvVar: ""}

	code, _, stderr := runIngestOtel(t, logsRoot(), ing, env, "ingest-otel", "--path", "/logs", "--run", cliTestRunID)

	assertRefusedWithMessage(t, code, cli.ExitUsage, stderr, ing)
}

func TestIngestOtel_NoGetenvInOptions_CountsAsUnset(t *testing.T) {
	ing := &fakeUsageIngester{}
	svc := app.New(app.Deps{
		Source: &fakeLogSource{classifyFunc: func(string) domain.Source { return logsRoot() }},
		Reader: newFakeEventReader(), Pricing: newFakePricingStore(), Clock: newFakeClock(),
		Interaction: &alwaysSkippingInteraction{}, Ingester: ing,
	})
	var out, errOut bytes.Buffer

	code := cli.Execute(cli.Options{Service: svc, Stdout: &out, Stderr: &errOut},
		[]string{"ingest-otel", "--path", "/logs", "--run", cliTestRunID})

	assertRefusedWithMessage(t, code, cli.ExitUsage, errOut.String(), ing)
}

func TestIngestOtel_MissingRunFlag_ExitsUsageNamingTheFlag(t *testing.T) {
	ing := &fakeUsageIngester{}

	code, _, stderr := runIngestOtel(t, logsRoot(), ing, nil, "ingest-otel", "--path", "/logs", "/x.jsonl")

	assertRefusedWithMessage(t, code, cli.ExitUsage, stderr, ing)
	if !strings.Contains(stderr, "--run") {
		t.Errorf("stderr = %q, want it to name --run", stderr)
	}
}

func TestIngestOtel_UnknownRun_ExitsNoDataNamingTheRun(t *testing.T) {
	ing := &fakeUsageIngester{}

	code, _, stderr := runIngestOtel(t, logsRoot(), ing, nil,
		"ingest-otel", "--path", "/logs", "--run", "20990101T000000Z-zzzz", "/x.jsonl")

	assertRefusedWithMessage(t, code, cli.ExitNoData, stderr, ing)
	if !strings.Contains(stderr, "20990101T000000Z-zzzz") {
		t.Errorf("stderr = %q, want it to name the unknown run", stderr)
	}
}

func TestIngestOtel_SourceNotFound_ExitsUsage(t *testing.T) {
	ing := &fakeUsageIngester{}

	code, _, stderr := runIngestOtel(t, domain.Source{Kind: domain.SourceNotFound, Path: "/logs"}, ing, nil,
		"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/x.jsonl")

	assertRefusedWithMessage(t, code, cli.ExitUsage, stderr, ing)
}

func TestIngestOtel_UnusablePath_ExitsUnusable(t *testing.T) {
	ing := &fakeUsageIngester{}
	src := domain.Source{Kind: domain.SourceUnusable, Path: "/logs", Reason: "not a logs tree"}

	code, _, stderr := runIngestOtel(t, src, ing, nil, "ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/x.jsonl")

	assertRefusedWithMessage(t, code, cli.ExitUnusable, stderr, ing)
}

func TestIngestOtel_DigestSource_ExitsUnusable(t *testing.T) {
	ing := &fakeUsageIngester{}
	src := domain.Source{Kind: domain.SourceDigestFile, Path: "/logs/a.digest.json"}

	code, _, stderr := runIngestOtel(t, src, ing, nil,
		"ingest-otel", "--path", "/logs/a.digest.json", "--run", cliTestRunID, "/x.jsonl")

	assertRefusedWithMessage(t, code, cli.ExitUnusable, stderr, ing)
}

func TestIngestOtel_IngestStatuses_MapToTheirExitCodesAndNameTheirDiagnostic(t *testing.T) {
	cases := []struct {
		name   string
		status domain.UsageIngestStatus
		kind   string
		want   int
	}{
		{"file missing", domain.IngestFileMissing, "file_missing", cli.ExitUsage},
		{"lock held", domain.IngestLockHeld, "lock_held", cli.ExitIngestSkipped},
		{"unknown state", domain.IngestUnknownState, "unknown_state_version", cli.ExitIngestSkipped},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ing := &fakeUsageIngester{result: domain.UsageIngestResult{
				Status:      c.status,
				Diagnostics: []domain.UsageIngestDiagnostic{{Kind: c.kind, Message: "details of " + c.kind}},
			}}

			code, _, stderr := runIngestOtel(t, logsRoot(), ing, nil,
				"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/x.jsonl")

			if code != c.want {
				t.Errorf("exit code = %d, want %d", code, c.want)
			}
			if !strings.Contains(stderr, c.kind+": details of "+c.kind) {
				t.Errorf("stderr lacks the diagnostic line:\n%s", stderr)
			}
			if len(ing.calls) != 1 {
				t.Errorf("ingester called %d times, want 1", len(ing.calls))
			}
		})
	}
}

func TestIngestOtel_IngestFailure_ExitsFailureWithTheErrorOnStderr(t *testing.T) {
	ing := &fakeUsageIngester{
		result: domain.UsageIngestResult{Status: domain.IngestFailed},
		err:    errors.New("cannot write run folder"),
	}

	code, _, stderr := runIngestOtel(t, logsRoot(), ing, nil,
		"ingest-otel", "--path", "/logs", "--run", cliTestRunID, "/x.jsonl")

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d", code, cli.ExitFailure)
	}
	if !strings.Contains(stderr, "cannot write run folder") {
		t.Errorf("stderr = %q, want the error", stderr)
	}
}
