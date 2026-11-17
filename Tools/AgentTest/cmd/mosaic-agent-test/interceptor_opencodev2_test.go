package main

// Tests for the interceptor route with --harness opencode-v2: the route
// resolves the V2 adapter, always exits 0 and always writes a JSON reply on
// stdout, because the plugin that spawned it treats anything else as a failed
// hook. Also checks AgentTest's own catalog copy carries the OpenCode V2
// harness-injections files, which agent rendering for that harness needs.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInterceptWith drives runIntercept with stdin content and returns the exit
// code, the stdout bytes and the stderr bytes.
func runInterceptWith(t *testing.T, args []string, stdinContent string) (int, []byte, []byte) {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	defer inR.Close()
	if _, err := inW.WriteString(stdinContent); err != nil {
		t.Fatalf("writing stdin: %v", err)
	}
	inW.Close()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	defer outR.Close()
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	defer errR.Close()

	code := runIntercept(args, inR, outW, errW)
	outW.Close()
	errW.Close()

	stdout, _ := io.ReadAll(outR)
	stderr, _ := io.ReadAll(errR)
	return code, stdout, stderr
}

func interceptArgs(t *testing.T, harnessID, phase string) []string {
	t.Helper()
	root := t.TempDir()
	control := filepath.Join(root, "control")
	subject := filepath.Join(root, "subject")
	for _, d := range []string{control, subject} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", d, err)
		}
	}
	return []string{"--workspace", subject, "--control-dir", control, "--harness", harnessID, "--phase", phase}
}

const v2PrePayload = `{"hook_event_name":"execute.before","tool":"subagent","sessionID":"ses_p","agent":"orchestrator","messageID":"msg_1","id":"call_1","input":{"agent":"worker","description":"d","prompt":"do the work"}}`

const v2PostPayload = `{"hook_event_name":"execute.after","tool":"subagent","sessionID":"ses_p","agent":"orchestrator","messageID":"msg_1","id":"call_1","input":{"agent":"worker"},"status":"completed","result":{"output":{"sessionID":"ses_c","status":"running","output":"started"}}}`

const v2CompletionPayload = `{"hook_event_name":"subagent.completed","sessionID":"ses_p","id":"call_1","agent":"worker","childSessionID":"ses_c","state":"completed","text":"<subagent state=\"completed\">\nthe reply\n</subagent>"}`

func TestInterceptRoute_OpenCodeV2_EveryPhaseExitsZeroWithAJSONReply(t *testing.T) {
	cases := map[string]string{
		"pre":        v2PrePayload,
		"post":       v2PostPayload,
		"completion": v2CompletionPayload,
	}
	for phase, payload := range cases {
		t.Run(phase, func(t *testing.T) {
			code, stdout, stderr := runInterceptWith(t, interceptArgs(t, "opencode-v2", phase), payload)

			if code != 0 {
				t.Fatalf("runIntercept exit code = %d, want 0; stderr=%s", code, stderr)
			}
			var reply map[string]any
			if err := json.Unmarshal(stdout, &reply); err != nil {
				t.Errorf("stdout = %q, want a JSON object reply: %v", stdout, err)
			}
		})
	}
}

func TestInterceptRoute_OpenCodeV2_MalformedPayloadStillExitsZeroWithAJSONReply(t *testing.T) {
	code, stdout, stderr := runInterceptWith(t, interceptArgs(t, "opencode-v2", "pre"), "not json {{{")

	if code != 0 {
		t.Fatalf("runIntercept exit code = %d, want 0; stderr=%s", code, stderr)
	}
	var reply map[string]any
	if err := json.Unmarshal(stdout, &reply); err != nil {
		t.Errorf("stdout = %q, want a JSON object reply even for a malformed payload: %v", stdout, err)
	}
}

func TestCatalogCopy_HasTheOpenCodeV2HarnessInjectionFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "catalog", "HarnessInjections", "OpenCode V2")
	for _, name := range []string{"HarnessInjections.md", "HarnessInjectionsOrchestrator.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("AgentTest catalog copy is missing %s for OpenCode V2: %v", name, err)
			continue
		}
		if !strings.HasPrefix(string(data), "---") || !strings.Contains(string(data), "harness: opencode-v2") {
			t.Errorf("AgentTest catalog copy %s is not an opencode-v2 harness-injections file (want front matter naming harness: opencode-v2)", name)
		}
	}
}
