package codex_test

// Fake Codex CLI shared by the adapter tests in this package.
//
// This test binary doubles as the fake `codex` executable when the
// environment variable GO_WANT_HELPER_PROCESS=1 is set. GO_HELPER_CMD selects
// the scenario, GO_HELPER_ARGS_FILE receives the serialised argv,
// GO_HELPER_STDIN_FILE (optional) receives stdin and GO_HELPER_ENV_FILE
// (optional) receives the environment.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-run/internal/testutil"
)

const (
	protocolReply = `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`
	rawReply      = `{"action":"route","agent":"agent-b"}`

	threadStartedLine = `{"type":"thread.started","thread_id":"t1"}`
	turnStartedLine   = `{"type":"turn.started"}`
	usageLine         = `{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":5}}`
	turnFailedLine    = `{"type":"turn.failed","error":{"message":"quota exceeded"}}`
	errorEventLine    = `{"type":"error","message":"stream disconnected"}`
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runHelperProcess()
	}
	os.Exit(m.Run())
}

func runHelperProcess() {
	if f := os.Getenv("GO_HELPER_ARGS_FILE"); f != "" {
		data, _ := json.Marshal(os.Args[1:])
		os.WriteFile(f, data, 0o644) //nolint:errcheck
	}
	if f := os.Getenv("GO_HELPER_ENV_FILE"); f != "" {
		data, _ := json.Marshal(os.Environ())
		os.WriteFile(f, data, 0o644) //nolint:errcheck
	}
	if f := os.Getenv("GO_HELPER_STDIN_FILE"); f != "" {
		in, _ := io.ReadAll(os.Stdin)
		os.WriteFile(f, in, 0o644) //nolint:errcheck
	}
	scenario, ok := helperScenarios[os.Getenv("GO_HELPER_CMD")]
	if !ok {
		os.Stderr.WriteString("unknown GO_HELPER_CMD: " + os.Getenv("GO_HELPER_CMD")) //nolint:errcheck
		os.Exit(2)
	}
	scenario()
	os.Exit(0)
}

var helperScenarios = map[string]func(){
	"codex-success":        func() { emitReply(protocolReply) },
	"codex-raw-success":    func() { emitReply(rawReply) },
	"codex-unparseable":    func() { emitReply("I finished, but this is not protocol JSON.") },
	"codex-turn-failed":    func() { testutil.EmitLines(threadStartedLine, turnStartedLine, turnFailedLine) },
	"codex-error-event":    func() { testutil.EmitLines(threadStartedLine, turnStartedLine, errorEventLine) },
	"codex-incomplete":     func() { testutil.EmitLines(threadStartedLine, turnStartedLine) },
	"codex-empty":          func() {},
	"codex-exit1":          helperExit1,
	"codex-exit1-failed":   helperExit1WithTurnFailed,
	"codex-no-message":     func() { testutil.EmitLines(threadStartedLine, turnStartedLine, usageLine) },
	"hang":                 func() { time.Sleep(24 * time.Hour) },
	"codex-activity-paced": testutil.PacedScenario(activityFirstLines, activityRestLines),
}

func emitReply(text string) {
	msg, _ := json.Marshal(text)
	testutil.EmitLines(
		threadStartedLine,
		turnStartedLine,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":`+string(msg)+`}}`,
		usageLine,
	)
}

func helperExit1() {
	os.Stderr.WriteString("sandbox setup failed: simulated") //nolint:errcheck
	os.Exit(1)
}

func helperExit1WithTurnFailed() {
	testutil.EmitLines(threadStartedLine, turnStartedLine, turnFailedLine)
	os.Stderr.WriteString("codex exited abnormally") //nolint:errcheck
	os.Exit(1)
}

// helperExe returns the path of this test binary, which acts as the fake
// `codex` executable when GO_WANT_HELPER_PROCESS=1 is set.
func helperExe(t *testing.T) string {
	t.Helper()
	return os.Args[0]
}

// setHelperEnv makes any subprocess spawned from now on run the given
// scenario and record its argv; it returns the argv file path.
func setHelperEnv(t *testing.T, scenario string) (argsFile string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GO_HELPER_CMD", scenario)
	argsFile = filepath.Join(t.TempDir(), "args.json")
	t.Setenv("GO_HELPER_ARGS_FILE", argsFile)
	return argsFile
}

// captureStdin makes the fake CLI record its stdin and returns the file path.
func captureStdin(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "stdin.bin")
	t.Setenv("GO_HELPER_STDIN_FILE", f)
	return f
}

func readArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatalf("decode args: %v", err)
	}
	return args
}

func containsArg(args []string, arg string) bool {
	for _, a := range args {
		if a == arg {
			return true
		}
	}
	return false
}

func containsSequence(args []string, a, b string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == a && args[i+1] == b {
			return true
		}
	}
	return false
}
