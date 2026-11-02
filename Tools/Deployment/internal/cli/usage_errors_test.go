package cli_test

// usage_errors_test.go verifies that every parse or usage failure is reported: the error is
// printed to stderr, standard output stays empty, and the exit code is the usage code. It
// covers every subcommand and the root command.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
)

// usageSubcommands lists every subcommand registered on the root command.
var usageSubcommands = []string{
	"agents", "check-index", "deploy", "hooks", "promote", "render", "transform", "update", "workflows",
}

// runUsage runs the CLI with args against a fresh spy service and returns the exit code,
// standard output and standard error.
func runUsage(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Run(context.Background(), args, &spyService{}, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun_UnknownFlagOnSubcommand_PrintsErrorToStderrAndExitsUsage(t *testing.T) {
	for _, sub := range usageSubcommands {
		t.Run(sub, func(t *testing.T) {
			code, stdout, stderr := runUsage(t, sub, "--no-such-flag")

			if code != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
			}
			if !strings.HasPrefix(stderr, "error: ") || !strings.Contains(stderr, "no-such-flag") {
				t.Errorf("stderr = %q, want an error line naming the unknown flag", stderr)
			}
			if strings.Count(stderr, "no-such-flag") != 1 {
				t.Errorf("stderr = %q, want the error printed exactly once", stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestRun_UnknownFlagWithJSONOutput_KeepsStdoutEmpty(t *testing.T) {
	code, stdout, stderr := runUsage(t, "deploy", "--output", "json", "--no-such-flag")

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty under --output json", stdout)
	}
	if !strings.Contains(stderr, "no-such-flag") {
		t.Errorf("stderr = %q, want the unknown flag named", stderr)
	}
}

func TestRun_BadFlagValue_PrintsErrorToStderrAndExitsUsage(t *testing.T) {
	code, stdout, stderr := runUsage(t, "deploy", "--dry-run=not-a-bool")

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.HasPrefix(stderr, "error: ") || !strings.Contains(stderr, "dry-run") {
		t.Errorf("stderr = %q, want an error line naming the flag", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestRun_FlagMissingItsValue_PrintsErrorToStderrAndExitsUsage(t *testing.T) {
	code, _, stderr := runUsage(t, "render", "--harness")

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.HasPrefix(stderr, "error: ") || !strings.Contains(stderr, "harness") {
		t.Errorf("stderr = %q, want an error line naming the flag", stderr)
	}
}

func TestRun_UnknownSubcommand_PrintsErrorToStderrAndExitsUsage(t *testing.T) {
	code, stdout, stderr := runUsage(t, "no-such-command")

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.HasPrefix(stderr, "error: ") || !strings.Contains(stderr, "no-such-command") {
		t.Errorf("stderr = %q, want an error line naming the unknown subcommand", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestRun_UnknownGlobalFlag_PrintsErrorToStderrAndExitsUsage(t *testing.T) {
	code, _, stderr := runUsage(t, "--no-such-flag", "deploy")

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, "no-such-flag") {
		t.Errorf("stderr = %q, want the unknown flag named", stderr)
	}
}

func TestRun_NoSubcommand_PrintsExactMessageNamingAllSubcommands(t *testing.T) {
	code, stdout, stderr := runUsage(t)

	want := "error: a subcommand is required: agents, check-index, deploy, hooks, promote, render, transform, update, workflows\n"
	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestRun_SubcommandHandlerError_IsNotPrintedTwice(t *testing.T) {
	// An invalid --conflict value is reported by the handler itself and exits 3.
	code, _, stderr := runUsage(t, "deploy", "--conflict", "bogus")

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Fatalf("stderr is empty, want the handler's own error")
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	seen := map[string]bool{}
	for _, l := range lines {
		if seen[l] {
			t.Errorf("stderr line %q printed twice in %q", l, stderr)
		}
		seen[l] = true
	}
}
