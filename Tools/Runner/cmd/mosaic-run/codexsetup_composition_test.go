package main

// codexsetup_composition_test.go exercises the Codex setup preflight as the
// composition roots build it: inside the run-lifecycle decorator, behind the
// production checker, in the TUI session factory and in the CLI composition.
// Test-mode runs shell out to `mosaic-run run`, so they reach the CLI
// composition covered here.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-common/interaction"

	"mosaic-run/internal/codexsetup"
	"mosaic-run/internal/domain"
)

// composeCodexSetup nests lifecycle (outermost) -> preflight -> inner.
func composeCodexSetup(runFolder string, inner *mlSession, checker codexSetupChecker, in domain.Interaction, interactive bool) func(context.Context) (domain.RunOutcome, error) {
	pre := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, checker, in, interactive))
	sess := newRunLifecycleSession(pre, mlLogConfig(runFolder, commonharness.HarnessIDCodex))
	return func(ctx context.Context) (domain.RunOutcome, error) {
		return sess.Start(ctx, domain.RunConfig{})
	}
}

func TestCodexSetupComposition_TUIRefusal_YieldsOneAbortedPair(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	in := &MockInteraction{ConfirmReply: interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: false}}
	inner := &mlSession{}

	out, err := composeCodexSetup(runFolder, inner, &MockCodexSetupChecker{Result: codexUntrusted}, in, true)(context.Background())

	if err != nil || out.Status != domain.RunRefused {
		t.Fatalf("got (%+v, %v), want RunRefused and a nil error", out, err)
	}
	if inner.calls != 0 {
		t.Errorf("wrapped session started %d time(s) after a refusal", inner.calls)
	}
	events := mlReadEvents(t, mlRunLogPath(ws))
	mlAssertPairs(t, events, 1)
	if events[1]["outcome"] != "aborted" {
		t.Errorf("run_end.outcome = %v, want aborted", events[1]["outcome"])
	}
}

func TestCodexSetupComposition_OrderIsRunStartThenCheckThenStart(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	var atCheck, atStart []string
	checker := &orderRecordingChecker{result: codexUntrusted, onCheck: func() {
		atCheck = mlEventNames(mlReadEvents(t, mlRunLogPath(ws)))
	}}
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		atStart = mlEventNames(mlReadEvents(t, mlRunLogPath(ws)))
		return domain.RunOutcome{Status: domain.RunCompleted}, nil
	}}

	_, err := composeCodexSetup(runFolder, inner, checker, &MockInteraction{}, false)(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if len(atCheck) != 1 || atCheck[0] != "run_start" {
		t.Errorf("events when the setup check ran = %v, want only run_start", atCheck)
	}
	if len(atStart) != 1 || atStart[0] != "run_start" {
		t.Errorf("events when the wrapped Start ran = %v, want only run_start", atStart)
	}
	if checker.calls != 1 || inner.calls != 1 {
		t.Errorf("checks=%d starts=%d, want 1 each", checker.calls, inner.calls)
	}
	mlAssertPairs(t, mlReadEvents(t, mlRunLogPath(ws)), 1)
}

func TestCodexSetupComposition_ResumeAfterRefusal_RechecksAndWritesSecondPair(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	in := &MockInteraction{ConfirmReply: interaction.ConfirmAnswer{Status: interaction.Cancelled}}
	inner := &mlSession{}
	checker := &MockCodexSetupChecker{Result: codexUntrusted}
	start := composeCodexSetup(runFolder, inner, checker, in, true)

	_, _ = start(context.Background())
	in.ConfirmReply = interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: true}
	out, err := start(context.Background())

	if err != nil || out.Status != domain.RunCompleted {
		t.Fatalf("second start got (%+v, %v), want proceed", out, err)
	}
	if len(checker.Checked) != 2 || len(in.Questions) != 2 || inner.calls != 1 {
		t.Errorf("checks=%d questions=%d starts=%d, want 2, 2, 1", len(checker.Checked), len(in.Questions), inner.calls)
	}
	events := mlReadEvents(t, mlRunLogPath(ws))
	mlAssertPairs(t, events, 2)
	if events[1]["outcome"] != "aborted" || events[3]["outcome"] != "completed" {
		t.Errorf("outcomes = %v, %v, want aborted then completed", events[1]["outcome"], events[3]["outcome"])
	}
}

// orderRecordingChecker runs a hook when the check happens.
type orderRecordingChecker struct {
	result  codexsetup.Result
	onCheck func()
	calls   int
}

func (c *orderRecordingChecker) Check(string) codexsetup.Result {
	c.calls++
	c.onCheck()
	return c.result
}

// writeCodexHome points CODEX_HOME at a fresh directory holding config (when
// not empty) as config.toml.
func writeCodexHome(t *testing.T, config string) {
	t.Helper()
	home := t.TempDir()
	if config != "" {
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEX_HOME", home)
}

// goodCodexConfig trusts workDir and configures the contact-user server.
func goodCodexConfig(workDir string) string {
	return fmt.Sprintf("[projects.'%s']\ntrust_level = \"trusted\"\n\n[mcp_servers.contact-user]\ncommand = \"contact-user-server\"\n", workDir)
}

func TestWithCodexSetupPreflight_RealCheckerWarnsAboutAnUntrustedWorkspace(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	writeCodexHome(t, "")
	in := &MockInteraction{}
	inner := &mlSession{}

	_, err := withCodexSetupPreflight(inner, runFolder, commonharness.HarnessIDCodex, in, false, domain.NopDebugLogger{}).
		Start(context.Background(), domain.RunConfig{})

	if err != nil || inner.calls != 1 {
		t.Fatalf("err=%v calls=%d, want the run to proceed", err, inner.calls)
	}
	if len(noticesMentioning(in.Notices, "trust")) != 1 {
		t.Errorf("notices %+v, want one workspace-trust warning from the production checker", in.Notices)
	}
	if len(noticesMentioning(in.Notices, commonharness.CodexContactUserServer)) != 1 {
		t.Errorf("notices %+v, want one contact-user warning from the production checker", in.Notices)
	}
}

func TestWithCodexSetupPreflight_RealCheckerStaysSilentForAGoodSetup(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	writeCodexHome(t, goodCodexConfig(ws))
	in := &MockInteraction{}
	inner := &mlSession{}

	_, err := withCodexSetupPreflight(inner, runFolder, commonharness.HarnessIDCodex, in, true, domain.NopDebugLogger{}).
		Start(context.Background(), domain.RunConfig{})

	if err != nil || inner.calls != 1 {
		t.Fatalf("err=%v calls=%d, want the run to proceed", err, inner.calls)
	}
	if len(in.Notices) != 0 || len(in.Questions) != 0 {
		t.Errorf("notices=%d questions=%d, want none for a trusted workspace with contact-user", len(in.Notices), len(in.Questions))
	}
}

func TestWithCodexSetupPreflight_InteractiveFlagReachesTheWrapper(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		runFolder, _ := mlRunFolder(t)
		writeCodexHome(t, "")
		in := &MockInteraction{ConfirmReply: interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: false}}
		inner := &mlSession{}

		out, _ := withCodexSetupPreflight(inner, runFolder, commonharness.HarnessIDCodex, in, interactive, domain.NopDebugLogger{}).
			Start(context.Background(), domain.RunConfig{})

		if interactive && (out.Status != domain.RunRefused || inner.calls != 0 || len(in.Questions) != 1) {
			t.Errorf("interactive: got %+v, %d call(s), %d question(s), want a refusal after one question", out, inner.calls, len(in.Questions))
		}
		if !interactive && (inner.calls != 1 || len(in.Questions) != 0) {
			t.Errorf("CLI: %d call(s), %d question(s), want the run to proceed without a question", inner.calls, len(in.Questions))
		}
	}
}

func TestTUIWiring_CodexSetupPreflightIsComposedIntoTheSessionFactory(t *testing.T) {
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	w := buildInteractiveWiring(in)
	runFolder, ws := mlRunFolder(t)
	writeCodexHome(t, "") // untrusted workspace; the detached program answers No
	cfg := mlTUIConfig()
	cfg.Harness = commonharness.HarnessIDCodex
	sess := w.Options.SessionFactory(runFolder, true, "", cfg)

	out, _ := sess.Start(context.Background(), refusedRunConfig(runFolder, mlRunID, commonharness.HarnessIDCodex))

	if out.Status != domain.RunRefused {
		t.Fatalf("outcome = %+v, want RunRefused", out)
	}
	if !strings.Contains(strings.ToLower(out.Message), "trust") || strings.Contains(out.Message, "no-such-orchestrator") {
		t.Errorf("refusal message %q must come from the Codex setup check (workspace trust), not from the wrapped session", out.Message)
	}
	events := mlReadEvents(t, mlRunLogPath(ws))
	mlAssertPairs(t, events, 1)
	if events[1]["outcome"] != "aborted" {
		t.Errorf("run_end.outcome = %v, want aborted", events[1]["outcome"])
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() { os.Stdout = orig }()
		fn()
	}()
	_ = w.Close()
	return <-done
}

// firstLineIndex returns the index of the first output line containing every
// substring (case-insensitive), or -1.
func firstLineIndex(lines []string, substrs ...string) int {
	for i, l := range lines {
		low := strings.ToLower(l)
		all := true
		for _, s := range substrs {
			all = all && strings.Contains(low, strings.ToLower(s))
		}
		if all {
			return i
		}
	}
	return -1
}

func TestCLIComposition_CodexRunWarnsAboutSetupAfterTheExperimentalNotice(t *testing.T) {
	ws := newCodexWorkspace(t)
	writeCodexHome(t, "") // untrusted workspace, no contact-user server
	var code int

	output := captureStdout(t, func() { code = ws.run(t) })

	if code != 0 {
		t.Fatalf("exit code = %d, want 0: the CLI warns and proceeds\n%s", code, output)
	}
	lines := strings.Split(output, "\n")
	experimental := firstLineIndex(lines, "[warning]", "experimental")
	trust := firstLineIndex(lines, "[warning]", "trust")
	contact := firstLineIndex(lines, "[warning]", commonharness.CodexContactUserServer)
	if experimental < 0 || trust < 0 || contact < 0 {
		t.Fatalf("missing warnings (experimental=%d trust=%d contact-user=%d) in:\n%s", experimental, trust, contact, output)
	}
	if experimental > trust || experimental > contact {
		t.Errorf("the experimental notice (line %d) must precede the setup warnings (trust %d, contact-user %d)", experimental, trust, contact)
	}
}

func TestCLIComposition_CodexRunWithAGoodSetupShowsNoSetupWarning(t *testing.T) {
	ws := newCodexWorkspace(t)
	writeCodexHome(t, goodCodexConfig(ws.root))
	var code int

	output := captureStdout(t, func() { code = ws.run(t) })

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, output)
	}
	if i := firstLineIndex(strings.Split(output, "\n"), "[warning]", "trust"); i >= 0 {
		t.Errorf("unexpected workspace-trust warning for a trusted workspace:\n%s", output)
	}
}
