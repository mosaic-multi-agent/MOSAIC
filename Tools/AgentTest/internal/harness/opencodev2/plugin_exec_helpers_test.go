package opencodev2_test

// Helpers that run the generated interceptor plugin under node. The plugin is
// provisioned for real, copied next to a driver script, and set up with a fake
// host context. The interceptor executable in its bridges is a fake script run
// by node that records the arguments and stdin it receives and prints a
// scripted reply, so every assertion is about what the plugin actually does.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type pluginStep struct {
	Kind     string `json:"kind"`
	Event    any    `json:"event,omitempty"`
	Forwards int    `json:"forwards"`
}

type pluginScenario struct {
	LogPath     string       `json:"logPath"`
	NoSubscribe bool         `json:"noSubscribe,omitempty"`
	NoToolHook  bool         `json:"noToolHook,omitempty"`
	FailHooks   []string     `json:"failHooks,omitempty"`
	Steps       []pluginStep `json:"steps"`
}

type stepResult struct {
	Kind           string         `json:"kind"`
	Threw          *string        `json:"threw"`
	Event          map[string]any `json:"event"`
	Forwarded      int            `json:"forwarded"`
	SubscribeCount int            `json:"subscribeCount"`
}

type driverResult struct {
	ID             string       `json:"id"`
	Hooks          []string     `json:"hooks"`
	SetupError     string       `json:"setupError"`
	ImportError    string       `json:"importError"`
	LogAfterSetup  int          `json:"logAfterSetup"`
	SubscribeCount int          `json:"subscribeCount"`
	Steps          []stepResult `json:"steps"`
	Writes         []string     `json:"writes"`
}

type forwardedCall struct {
	Args    []string       `json:"args"`
	Phase   string         `json:"phase"`
	Stdin   string         `json:"stdin"`
	Payload map[string]any `json:"-"`
}

// pluginRun configures one execution: scripted replies and exit codes per
// phase, and the scenario to replay.
type pluginRun struct {
	Replies  map[string]string
	Exits    map[string]int
	Scenario pluginScenario
}

type pluginOutcome struct {
	driverResult
	Calls      []forwardedCall
	ControlDir string
	ScriptDir  string
}

func requireNode(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; the generated plugin cannot be executed")
	}
	return node
}

// runPlugin executes the scenario against the generated plugin and returns
// what the driver observed plus every call the fake interceptor received.
func runPlugin(t *testing.T, run pluginRun) pluginOutcome {
	t.Helper()
	node := requireNode(t)
	scriptDir := t.TempDir()
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.InterceptorPath = node
	req.InterceptorArgs = []string{testdataPath(t, "plugin_driver", "fake_interceptor.mjs"), scriptDir}
	pluginSrc := provisionInterceptor(t, sb, req)

	pluginPath := filepath.Join(scriptDir, "plugin.mts")
	writeTestFile(t, pluginPath, pluginSrc)
	for phase, reply := range run.Replies {
		writeTestFile(t, filepath.Join(scriptDir, "reply-"+phase), reply)
	}
	for phase, code := range run.Exits {
		writeTestFile(t, filepath.Join(scriptDir, "exit-"+phase), itoa(code))
	}
	run.Scenario.LogPath = filepath.Join(scriptDir, "log.jsonl")
	scenarioJSON, err := json.Marshal(run.Scenario)
	if err != nil {
		t.Fatalf("marshal scenario: %v", err)
	}
	scenarioPath := filepath.Join(scriptDir, "scenario.json")
	writeTestFile(t, scenarioPath, string(scenarioJSON))
	resultPath := filepath.Join(scriptDir, "result.json")

	cmd := exec.Command(node, testdataPath(t, "plugin_driver", "drive.mjs"), pluginPath, scenarioPath, resultPath)
	cmd.Env = append(os.Environ(), "NODE_NO_WARNINGS=1")
	timer := time.AfterFunc(2*time.Minute, func() { _ = cmd.Process.Kill() })
	out, runErr := cmd.CombinedOutput()
	timer.Stop()
	if runErr != nil {
		t.Fatalf("driver failed: %v\n%s", runErr, out)
	}
	return readOutcome(t, resultPath, scriptDir, sb.ControlDir)
}

func readOutcome(t *testing.T, resultPath, scriptDir, controlDir string) pluginOutcome {
	t.Helper()
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("driver wrote no result: %v", err)
	}
	var res driverResult
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatalf("parsing driver result: %v\n%s", err, data)
	}
	if strings.Contains(res.ImportError, "ERR_UNKNOWN_FILE_EXTENSION") {
		t.Skipf("this node cannot import TypeScript directly: %s", res.ImportError)
	}
	if res.ImportError != "" {
		t.Fatalf("generated plugin does not load under node: %s", res.ImportError)
	}
	out := pluginOutcome{driverResult: res, ControlDir: controlDir, ScriptDir: scriptDir}
	out.Calls = readCalls(t, filepath.Join(scriptDir, "log.jsonl"))
	if len(res.Writes) != 0 {
		t.Errorf("plugin wrote to stdout/stderr: %q", res.Writes)
	}
	return out
}

func readCalls(t *testing.T, logPath string) []forwardedCall {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil
	}
	var calls []forwardedCall
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var c forwardedCall
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatalf("bad interceptor log line %q: %v", line, err)
		}
		if err := json.Unmarshal([]byte(c.Stdin), &c.Payload); err != nil {
			t.Fatalf("interceptor stdin is not a JSON object: %q: %v", c.Stdin, err)
		}
		calls = append(calls, c)
	}
	return calls
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// callsForPhase returns the forwarded calls for one phase, in order.
func (o pluginOutcome) callsForPhase(phase string) []forwardedCall {
	var out []forwardedCall
	for _, c := range o.Calls {
		if c.Phase == phase {
			out = append(out, c)
		}
	}
	return out
}
