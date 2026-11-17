package harness_test

// Tests for the OpenCode V2 spawner, driven through the fake-subprocess helper
// (helper_opencodev2_test.go): argument and stdin delivery, success, the
// distinct failure causes on non-zero exits, and preservation of the failure
// detail V2 writes to stdout.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

func spawnV2(t *testing.T, mode string, req harness.SpawnRequest) (harness.Response, []string, error) {
	t.Helper()
	argsFile := setHelperEnv(t, mode)
	if req.Agent.Identifier == "" {
		req.Agent = ordinaryAgent()
	}
	spawner := harness.NewOpenCodeV2(helperExe(t), harness.WithTimeout(20*time.Second))
	resp, err := spawner.Spawn(context.Background(), req)
	return resp, readArgsIfPresent(t, argsFile), err
}

func readArgsIfPresent(t *testing.T, file string) []string {
	t.Helper()
	if _, err := os.Stat(file); err != nil {
		return nil
	}
	return readArgs(t, file)
}

// ---------------------------------------------------------------------------
// Success and delivery
// ---------------------------------------------------------------------------

func TestOpenCodeV2Spawn_SuccessReturnsTextAndProtocol(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-success", harness.SpawnRequest{Prompt: "go"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != v2HelperProtocol {
		t.Errorf("want Text %q, got %q", v2HelperProtocol, resp.Text)
	}
	if string(resp.Protocol) != v2HelperProtocol {
		t.Errorf("want Protocol %q, got %q", v2HelperProtocol, resp.Protocol)
	}
	if resp.ExitCode != 0 {
		t.Errorf("want ExitCode 0, got %d", resp.ExitCode)
	}
}

func TestOpenCodeV2Spawn_PassesV2ArgumentsToTheProcess(t *testing.T) {
	_, argv, _ := spawnV2(t, "v2-success", harness.SpawnRequest{Agent: orchestratorAgent(), Prompt: "go", Model: "openai/gpt-5#high"})

	want := []string{"run", "--agent", "orchestrator-agent", "--format", "json", "--auto", "--standalone", "--model", "openai/gpt-5#high"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Errorf("want argv %v, got %v", want, argv)
	}
}

func TestOpenCodeV2Spawn_NeverPassesDirOrVariantEvenWithWorkingDir(t *testing.T) {
	_, argv, _ := spawnV2(t, "v2-success", harness.SpawnRequest{Prompt: "go", WorkingDir: t.TempDir(), Model: "a/b#c"})

	if len(argv) == 0 {
		t.Fatal("want the process to have been started with arguments")
	}
	for _, f := range []string{"--dir", "--variant"} {
		if containsArg(argv, f) {
			t.Errorf("%s must never be passed, got %v", f, argv)
		}
	}
}

func TestOpenCodeV2Spawn_PromptReachesStdin(t *testing.T) {
	resp, argv, _ := spawnV2(t, "v2-echo", harness.SpawnRequest{SystemPrompt: "<env>\nsys\n</env>", Prompt: specialPayload})

	got := echoField(t, resp.Text, "stdin")
	if want := "<env>\nsys\n</env>\n" + specialPayload; got != want {
		t.Errorf("want stdin %q, got %q", want, got)
	}
	assertNoPromptInArgv(t, argv, "line1 with spaces", "<env>")
}

func TestOpenCodeV2Spawn_NonProtocolFinalTextIsKeptInResponseText(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-echo", harness.SpawnRequest{Prompt: "go"})

	if !errors.Is(err, harness.ErrProtocolNotExtractable) {
		t.Fatalf("want ErrProtocolNotExtractable, got %v", err)
	}
	if !strings.HasPrefix(resp.Text, "pwd=") {
		t.Errorf("want Response.Text to hold the final text, got %q", resp.Text)
	}
}

// ---------------------------------------------------------------------------
// Child PWD agrees with the child's working directory
// ---------------------------------------------------------------------------

// echoField extracts a field from the v2-echo helper's final text.
func echoField(t *testing.T, text, field string) string {
	t.Helper()
	const stdinKey = "\nstdin="
	if field == "stdin" {
		i := strings.Index(text, stdinKey)
		if i < 0 {
			t.Fatalf("no stdin field in %q", text)
		}
		return text[i+len(stdinKey):]
	}
	for _, line := range strings.Split(text, "\n") {
		if v, ok := strings.CutPrefix(line, field+"="); ok {
			return v
		}
	}
	t.Fatalf("no %s field in %q", field, text)
	return ""
}

func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

func TestOpenCodeV2Spawn_ChildPWDEqualsWorkingDirDespiteStaleParentPWD(t *testing.T) {
	stale := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("PWD", stale)

	resp, _, _ := spawnV2(t, "v2-echo", harness.SpawnRequest{Prompt: "go", WorkingDir: workDir})

	pwd, cwd := echoField(t, resp.Text, "pwd"), echoField(t, resp.Text, "cwd")
	if !samePath(t, pwd, workDir) {
		t.Errorf("want child PWD %q to equal the working directory %q (stale parent PWD %q)", pwd, workDir, stale)
	}
	if !samePath(t, cwd, workDir) {
		t.Errorf("want child cwd %q to equal the working directory %q", cwd, workDir)
	}
}

func TestOpenCodeV2Spawn_ChildPWDEqualsWorkingDirWhenParentHasNoPWD(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("PWD", "")
	os.Unsetenv("PWD") //nolint:errcheck

	resp, _, _ := spawnV2(t, "v2-echo", harness.SpawnRequest{Prompt: "go", WorkingDir: workDir})

	if pwd := echoField(t, resp.Text, "pwd"); !samePath(t, pwd, workDir) {
		t.Errorf("want child PWD %q to equal the working directory %q", pwd, workDir)
	}
}

func TestOpenCodeV2Spawn_WithoutWorkingDirChildPWDEqualsItsDirectory(t *testing.T) {
	t.Setenv("PWD", t.TempDir()) // stale: not the directory the child starts in

	resp, _, _ := spawnV2(t, "v2-echo", harness.SpawnRequest{Prompt: "go"})

	pwd, cwd := echoField(t, resp.Text, "pwd"), echoField(t, resp.Text, "cwd")
	if !samePath(t, pwd, cwd) {
		t.Errorf("want child PWD %q to equal its working directory %q", pwd, cwd)
	}
}

func TestOpenCodeV2Spawn_CallerEnvIsStillPassedToTheChild(t *testing.T) {
	setHelperEnv(t, "echo-env")
	t.Setenv("GO_HELPER_ECHO_VAR", "MOSAIC_V2_PROBE")
	spawner := harness.NewOpenCodeV2(helperExe(t))

	resp, _ := spawner.Spawn(context.Background(), harness.SpawnRequest{
		Agent: ordinaryAgent(), Prompt: "go", Env: []string{"MOSAIC_V2_PROBE=probe-value"},
	})

	if string(resp.Stdout) != "probe-value" {
		t.Errorf("want the caller's Env entry visible to the child, got stdout %q", resp.Stdout)
	}
}

// ---------------------------------------------------------------------------
// Non-zero exits: each cause is distinct and the stdout detail is preserved
// ---------------------------------------------------------------------------

func TestOpenCodeV2Spawn_UnknownAgentExitOnePreservesStdoutDetail(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-error-agent", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownAgent)
	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want error to wrap ErrNonZeroExit, got %v", err)
	}
	if !strings.Contains(err.Error(), `Agent not found: "no-such-agent"`) {
		t.Errorf("want V2's message in the error, got %q", err.Error())
	}
	assertV2DetailKept(t, resp, 1, `Agent not found`)
}

func TestOpenCodeV2Spawn_UnknownModelExitOnePreservesStdoutDetail(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-error-model", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownModel)
	if !strings.Contains(err.Error(), "Model unavailable: openai/no-such-model") {
		t.Errorf("want V2's message in the error, got %q", err.Error())
	}
	assertV2DetailKept(t, resp, 1, "provider.no-route")
}

func TestOpenCodeV2Spawn_OtherErrorEventExitOneIsRunFailed(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-error-other", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2RunFailed)
	if !strings.Contains(err.Error(), "upstream exploded") {
		t.Errorf("want V2's message in the error, got %q", err.Error())
	}
	assertV2DetailKept(t, resp, 1, "upstream exploded")
}

func TestOpenCodeV2Spawn_UnknownFlagHelpTextIsUsageErrorWithStderrReason(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-usage", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UsageError)
	if !strings.Contains(err.Error(), "Unknown argument: bogus-flag") {
		t.Errorf("want the stderr reason in the error, got %q", err.Error())
	}
	assertV2DetailKept(t, resp, 1, "opencode run [message..]")
	if !strings.Contains(resp.Stderr, "Unknown argument") {
		t.Errorf("want Stderr kept, got %q", resp.Stderr)
	}
}

func TestOpenCodeV2Spawn_ExitOneWithoutErrorEventIsRunFailed(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-exit1-stream", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2RunFailed)
	assertV2DetailKept(t, resp, 1, "partial")
}

func TestOpenCodeV2Spawn_Exit130IsInterrupted(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-exit130", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2Interrupted)
	assertV2DetailKept(t, resp, 130, "step_start")
}

func TestOpenCodeV2Spawn_Exit143IsTerminated(t *testing.T) {
	resp, _, err := spawnV2(t, "v2-exit143", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2Terminated)
	assertV2DetailKept(t, resp, 143, "step_start")
}

func TestOpenCodeV2Spawn_ErrorEventWithExitZeroStillFails(t *testing.T) {
	_, _, err := spawnV2(t, "v2-error-exit0", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownAgent)
}

func TestOpenCodeV2Spawn_IncompleteStreamAtExitZeroIsStreamIncomplete(t *testing.T) {
	_, _, err := spawnV2(t, "v2-incomplete-wait", harness.SpawnRequest{Prompt: "go"})

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2StreamIncomplete)
}

// assertV2DetailKept checks a failed Response still carries the exit code and
// the stdout the V1 spawner would have dropped.
func assertV2DetailKept(t *testing.T, resp harness.Response, exit int, stdoutFragment string) {
	t.Helper()
	if resp.ExitCode != exit {
		t.Errorf("want ExitCode %d kept on failure, got %d", exit, resp.ExitCode)
	}
	if !strings.Contains(string(resp.Stdout), stdoutFragment) {
		t.Errorf("want Stdout kept on failure and containing %q, got %q", stdoutFragment, resp.Stdout)
	}
	if resp.Duration <= 0 {
		t.Errorf("want Duration kept on failure, got %v", resp.Duration)
	}
}

// ---------------------------------------------------------------------------
// Failures before an exit
// ---------------------------------------------------------------------------

func TestOpenCodeV2Spawn_MissingExecutableIsExecutableNotFound(t *testing.T) {
	spawner := harness.NewOpenCodeV2(filepath.Join(t.TempDir(), "no-such-opencode"))

	_, err := spawner.Spawn(context.Background(), harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "go"})

	if !errors.Is(err, harness.ErrExecutableNotFound) {
		t.Errorf("want ErrExecutableNotFound, got %v", err)
	}
}

func TestOpenCodeV2Spawn_EmptyAgentIdentifierIsRejectedBeforeSpawning(t *testing.T) {
	argsFile := setHelperEnv(t, "v2-success")
	spawner := harness.NewOpenCodeV2(helperExe(t))

	_, err := spawner.Spawn(context.Background(), harness.SpawnRequest{Prompt: "go"})

	if !errors.Is(err, harness.ErrOpenCodeV2EmptyAgentIdentifier) {
		t.Errorf("want ErrOpenCodeV2EmptyAgentIdentifier, got %v", err)
	}
	if _, statErr := os.Stat(argsFile); statErr == nil {
		t.Error("want no process started for an invalid request")
	}
}

func TestOpenCodeV2Spawn_TimeoutIsTimeoutNotAV2Failure(t *testing.T) {
	setHelperEnv(t, "hang")
	spawner := harness.NewOpenCodeV2(helperExe(t), harness.WithTimeout(300*time.Millisecond))

	_, err := spawner.Spawn(context.Background(), harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "go"})

	if !errors.Is(err, harness.ErrTimeout) {
		t.Errorf("want ErrTimeout, got %v", err)
	}
	for _, s := range allV2Sentinels {
		if errors.Is(err, s) {
			t.Errorf("want no V2 failure cause for a timeout, got %v", err)
		}
	}
}
