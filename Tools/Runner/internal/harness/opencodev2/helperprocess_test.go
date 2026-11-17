package opencodev2_test

// Fake-CLI infrastructure shared by the adapter tests. This test binary doubles
// as the fake opencode executable when GO_WANT_HELPER_PROCESS=1 is set;
// TestMain intercepts the subprocess entry point before Go's test framework
// parses the CLI flags. GO_HELPER_CMD selects the scenario. The optional files
// GO_HELPER_ARGS_FILE, GO_HELPER_ENV_FILE, GO_HELPER_CWD_FILE and
// GO_HELPER_STDIN_FILE receive argv, environment, working directory and stdin.

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"mosaic-run/internal/testutil"
)

const (
	helperProtocol = `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`
	helperStart    = `{"type":"step_start","sessionID":"ses_h","part":{"type":"step-start"}}`
	// helperFinishStop carries a non-zero cost: the adapter must never surface it.
	helperFinishStop = `{"type":"step_finish","sessionID":"ses_h","part":{"reason":"stop","cost":0.5,"tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}}`
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runHelperProcess()
	}
	os.Exit(m.Run())
}

func runHelperProcess() {
	recordHelperState()
	scenario, ok := helperScenarios[os.Getenv("GO_HELPER_CMD")]
	if !ok {
		os.Stderr.WriteString("unknown GO_HELPER_CMD: " + os.Getenv("GO_HELPER_CMD")) //nolint:errcheck
		os.Exit(2)
	}
	scenario()
	os.Exit(0)
}

// recordHelperState persists argv, environment, working directory and stdin to
// the files the test asked for, before any output is produced.
func recordHelperState() {
	writeIfSet("GO_HELPER_ARGS_FILE", mustJSON(os.Args[1:]))
	writeIfSet("GO_HELPER_ENV_FILE", mustJSON(os.Environ()))
	cwd, _ := os.Getwd()
	writeIfSet("GO_HELPER_CWD_FILE", []byte(cwd))
	if os.Getenv("GO_HELPER_STDIN_FILE") != "" {
		data, _ := io.ReadAll(os.Stdin)
		writeIfSet("GO_HELPER_STDIN_FILE", data)
	}
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

func writeIfSet(envName string, data []byte) {
	if path := os.Getenv(envName); path != "" {
		os.WriteFile(path, data, 0o644) //nolint:errcheck
	}
}

var helperScenarios = map[string]func(){
	"success":       scenarioSuccess,
	"raw-text":      scenarioRawText,
	"denied-tool":   scenarioDeniedTool,
	"empty":         func() {},
	"not-protocol":  scenarioNotProtocol,
	"error-agent":   func() { emitErrorAndExit("unknown", `Agent not found: "no-such-agent"`, helperStart) },
	"error-model":   func() { emitErrorAndExit("provider.no-route", "Model unavailable: openai/no-such-model", helperStart) },
	"error-other":   func() { emitErrorAndExit("api.failure", "upstream exploded", helperStart) },
	"usage":         scenarioUsage,
	"exit1-stream":  func() { emit(helperStart); os.Exit(1) },
	"exit130":       func() { emit(helperStart); os.Exit(130) },
	"exit143":       func() { emit(helperStart); os.Exit(143) },
	"incomplete":    func() { emit(helperStart) },
	"hang":          func() { time.Sleep(24 * time.Hour) },
	"start-hang":    func() { emit(helperStart); time.Sleep(24 * time.Hour) },
	"tree-agent":    scenarioTreeAgent,
	"tree-child":    func() { time.Sleep(20 * time.Second) },
	"activity-pace": nil, // set in init below
}

func init() {
	helperScenarios["activity-pace"] = testutil.PacedScenario(
		[]string{helperStart, activityTextLine, activityToolLine, activitySubagentLine, activityErrorToolLine},
		[]string{activityFinishLine, textLine(helperProtocol), helperFinishStop},
	)
}

func emit(lines ...string) { testutil.EmitLines(lines...) }

func textLine(text string) string {
	return `{"type":"text","sessionID":"ses_h","part":{"type":"text","text":` + string(mustJSON(text)) + `}}`
}

func emitErrorAndExit(typ, msg string, before ...string) {
	emit(before...)
	emit(`{"type":"error","sessionID":"ses_h","error":{"type":"` + typ + `","message":` + string(mustJSON(msg)) + `}}`)
	os.Exit(1)
}

func scenarioSuccess() { emit(helperStart, textLine(helperProtocol), helperFinishStop) }

func scenarioRawText() {
	emit(helperStart, textLine("route to agent-b, no protocol here"), helperFinishStop)
}

func scenarioNotProtocol() { emit(helperStart, textLine("just chatting, no JSON"), helperFinishStop) }

// scenarioDeniedTool: a tool call is denied, the agent carries on and answers.
func scenarioDeniedTool() {
	emit(helperStart,
		`{"type":"tool_use","sessionID":"ses_h","part":{"type":"tool","id":"c1","tool":"shell","state":{"status":"error","input":{"command":"git push"},"error":"Permission denied: shell"}}}`,
		`{"type":"step_finish","sessionID":"ses_h","part":{"reason":"tool-calls","cost":0,"tokens":{"input":5,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}}`,
		helperStart, textLine(helperProtocol), helperFinishStop)
}

func scenarioUsage() {
	emit("", "opencode run [message..]", "", "Options:", "  --agent  agent to use")
	os.Stderr.WriteString("Unknown argument: bogus-flag\n") //nolint:errcheck
	os.Exit(1)
}

// scenarioTreeAgent starts a long-lived grandchild, publishes its pid and then
// hangs, so a test can check that ending the invocation ends the grandchild.
func scenarioTreeAgent() {
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), "GO_HELPER_CMD=tree-child")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	if path := os.Getenv("GO_HELPER_PID_FILE"); path != "" {
		tmp := path + ".tmp"
		os.WriteFile(tmp, []byte(strconv.Itoa(child.Process.Pid)), 0o644) //nolint:errcheck
		os.Rename(tmp, path)                                              //nolint:errcheck
	}
	time.Sleep(24 * time.Hour)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func helperExe(t *testing.T) string {
	t.Helper()
	return os.Args[0]
}

// setHelperEnv puts the test process into helper mode for the given scenario so
// every subprocess the adapter spawns runs it.
func setHelperEnv(t *testing.T, cmd string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GO_HELPER_CMD", cmd)
}

// captureFile makes the helper record into a fresh file named by envName and
// returns the file's path.
func captureFile(t *testing.T, envName, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	t.Setenv(envName, path)
	return path
}

func readFileOrFail(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the spawned process did not record %s: %v", filepath.Base(path), err)
	}
	return data
}

func readArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	var args []string
	if err := json.Unmarshal(readFileOrFail(t, argsFile), &args); err != nil {
		t.Fatalf("could not decode recorded args: %v", err)
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
