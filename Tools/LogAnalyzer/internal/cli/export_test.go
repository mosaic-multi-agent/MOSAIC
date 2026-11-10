package cli_test

// Tests for the export command: flag validation, run selection, exit codes,
// overwrite handling and the success message. All run on fakes.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
)

func exportRunIDs(w digestWrite) []string {
	var ids []string
	for _, r := range w.d.Aggregate.Runs {
		ids = append(ids, r.Run.ID)
	}
	return ids
}

func TestExport_MissingOut_IsUsageErrorNamingTheFlag(t *testing.T) {
	env := newExportEnv(exportRunOne)

	code, _, stderr := env.execute("export", "--path", exportLogsPath)

	if code != cli.ExitUsage {
		t.Errorf("exit = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, "--out") {
		t.Errorf("stderr must name the missing --out flag; got %q", stderr)
	}
	if len(env.digests.writes) != 0 || len(env.classified) != 0 {
		t.Errorf("nothing may be resolved or written without --out; writes=%d classified=%v", len(env.digests.writes), env.classified)
	}
}

func TestExport_OutWithoutDigestSuffix_IsUsageError(t *testing.T) {
	env := newExportEnv(exportRunOne)

	code, _, stderr := env.execute("export", "--path", exportLogsPath, "--out", "runs.json")

	if code != cli.ExitUsage {
		t.Errorf("exit = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, ".digest.json") {
		t.Errorf("stderr must name the required suffix; got %q", stderr)
	}
	if len(env.digests.writes) != 0 {
		t.Errorf("no digest may be written for an invalid name; writes=%d", len(env.digests.writes))
	}
}

func TestExport_AllRuns_WritesEveryRunAndReportsCount(t *testing.T) {
	env := newExportEnv(exportRunOne, exportRunTwo)

	code, stdout, _ := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath)

	if code != cli.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess", code)
	}
	if len(env.digests.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(env.digests.writes))
	}
	w := env.digests.writes[0]
	if w.path != exportOutPath || w.overwrite {
		t.Errorf("write path=%q overwrite=%v, want %q without overwrite", w.path, w.overwrite, exportOutPath)
	}
	if got := exportRunIDs(w); len(got) != 2 || got[0] != exportRunOne || got[1] != exportRunTwo {
		t.Errorf("written runs = %v, want both runs in id order", got)
	}
	if !strings.Contains(stdout, "2 run") || !strings.Contains(stdout, exportOutPath) {
		t.Errorf("success message must name the count and path; got %q", stdout)
	}
	if strings.Contains(stdout, "unattributable") {
		t.Errorf("message must not mention a bucket that was not written; got %q", stdout)
	}
}

func TestExport_AllRunsWithBucket_WritesAndMentionsBucket(t *testing.T) {
	env := newExportEnv(exportRunOne).withBucket()

	code, stdout, _ := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath)

	if code != cli.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess", code)
	}
	if len(env.digests.writes) != 1 || env.digests.writes[0].d.Aggregate.Unattributable == nil {
		t.Fatalf("the digest must carry the unattributable bucket; writes=%+v", env.digests.writes)
	}
	if !strings.Contains(stdout, "1 run") || !strings.Contains(stdout, "unattributable") {
		t.Errorf("message must name the run and the bucket; got %q", stdout)
	}
}

func TestExport_RunFilter_WritesOnlyThatRunWithoutBucket(t *testing.T) {
	env := newExportEnv(exportRunOne, exportRunTwo).withBucket()

	code, stdout, _ := env.execute("export", "--path", exportLogsPath, "--run", exportRunTwo, "--out", exportOutPath)

	if code != cli.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess", code)
	}
	if len(env.digests.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(env.digests.writes))
	}
	w := env.digests.writes[0]
	if got := exportRunIDs(w); len(got) != 1 || got[0] != exportRunTwo {
		t.Errorf("written runs = %v, want only %s", got, exportRunTwo)
	}
	if w.d.Aggregate.Unattributable != nil {
		t.Error("a --run export must not carry the unattributable bucket")
	}
	if !strings.Contains(stdout, "1 run") || strings.Contains(stdout, "unattributable") {
		t.Errorf("message = %q, want one run and no bucket", stdout)
	}
}

func TestExport_RunNotInSource_IsNoDataAndWritesNothing(t *testing.T) {
	env := newExportEnv(exportRunOne)
	missing := "20269999T000000Z-ffff"

	code, stdout, stderr := env.execute("export", "--path", exportLogsPath, "--run", missing, "--out", exportOutPath)

	if code != cli.ExitNoData {
		t.Errorf("exit = %d, want ExitNoData (%d)", code, cli.ExitNoData)
	}
	if len(env.digests.writes) != 0 {
		t.Errorf("no file may be written for an unknown run; writes=%d", len(env.digests.writes))
	}
	if !strings.Contains(stderr, missing) {
		t.Errorf("stderr must say which run was not found; got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty; got %q", stdout)
	}
}

func TestExport_SourceNotFound_IsUsageAfterResolving(t *testing.T) {
	env := newExportEnv(exportRunOne)

	code, _, _ := env.execute("export", "--path", "/does-not-exist", "--out", exportOutPath)

	if code != cli.ExitUsage {
		t.Errorf("exit = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
	if len(env.classified) == 0 || env.classified[0] != "/does-not-exist" {
		t.Errorf("the explicit path must be resolved; classified=%v", env.classified)
	}
	if len(env.digests.writes) != 0 {
		t.Errorf("writes = %d, want none", len(env.digests.writes))
	}
}

func TestExport_SourceUnusable_IsUnusableExit(t *testing.T) {
	env := newExportEnv(exportRunOne)

	code, _, _ := env.execute("export", "--path", exportUnusablePath, "--out", exportOutPath)

	if code != cli.ExitUnusable {
		t.Errorf("exit = %d, want ExitUnusable (%d)", code, cli.ExitUnusable)
	}
	if len(env.digests.writes) != 0 {
		t.Errorf("writes = %d, want none", len(env.digests.writes))
	}
}

func TestExport_SourceWithoutRuns_IsNoDataAndWritesNothing(t *testing.T) {
	env := newExportEnv()

	code, _, _ := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath)

	if code != cli.ExitNoData {
		t.Errorf("exit = %d, want ExitNoData (%d)", code, cli.ExitNoData)
	}
	if len(env.digests.writes) != 0 {
		t.Errorf("writes = %d, want none", len(env.digests.writes))
	}
}

func TestExport_ExistingFile_RefusedWithoutForce(t *testing.T) {
	env := newExportEnv(exportRunOne)
	env.digests.existing[exportOutPath] = true

	code, stdout, stderr := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath)

	if code != cli.ExitUsage {
		t.Errorf("exit = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr must tell the user about --force; got %q", stderr)
	}
	if len(env.digests.writes) != 0 || stdout != "" {
		t.Errorf("nothing may be written or reported as exported; writes=%d stdout=%q", len(env.digests.writes), stdout)
	}
}

func TestExport_ExistingFile_ReplacedWithForce(t *testing.T) {
	env := newExportEnv(exportRunOne)
	env.digests.existing[exportOutPath] = true

	code, stdout, _ := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath, "--force")

	if code != cli.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess", code)
	}
	if len(env.digests.writes) != 1 || !env.digests.writes[0].overwrite {
		t.Errorf("the write must request overwrite; writes=%+v", env.digests.writes)
	}
	if !strings.Contains(stdout, "1 run") {
		t.Errorf("message = %q, want the count of runs written", stdout)
	}
}

func TestExport_WriteFailure_IsFailureExit(t *testing.T) {
	env := newExportEnv(exportRunOne)
	env.digests.writeErr = errors.New("disk full")

	code, _, stderr := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath)

	if code != cli.ExitFailure {
		t.Errorf("exit = %d, want ExitFailure (%d)", code, cli.ExitFailure)
	}
	if !strings.Contains(stderr, "disk full") {
		t.Errorf("stderr must carry the error; got %q", stderr)
	}
}

func TestExport_FormatFlagIsAcceptedAndIgnored(t *testing.T) {
	env := newExportEnv(exportRunOne)

	code, stdout, _ := env.execute("export", "--path", exportLogsPath, "--out", exportOutPath, "--format", "json")

	if code != cli.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess", code)
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("--format must not change the export message; got %q", stdout)
	}
	if len(env.digests.writes) != 1 {
		t.Errorf("writes = %d, want 1", len(env.digests.writes))
	}
}

func TestExport_NeverPrompts(t *testing.T) {
	cases := []struct {
		name string
		args []string
		pre  func(*exportEnv)
		want int
	}{
		{"success", []string{"--out", exportOutPath}, nil, cli.ExitSuccess},
		{"existing file", []string{"--out", exportOutPath}, func(e *exportEnv) { e.digests.existing[exportOutPath] = true }, cli.ExitUsage},
		{"run not found", []string{"--out", exportOutPath, "--run", "20269999T000000Z-ffff"}, nil, cli.ExitNoData},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newExportEnv(exportRunOne)
			if c.pre != nil {
				c.pre(env)
			}

			code, _, _ := env.execute(append([]string{"export", "--path", exportLogsPath}, c.args...)...)

			if code != c.want {
				t.Errorf("exit = %d, want %d", code, c.want)
			}
			if env.prompts.asked != 0 {
				t.Errorf("export asked %d question(s); it must never prompt", env.prompts.asked)
			}
		})
	}
}
