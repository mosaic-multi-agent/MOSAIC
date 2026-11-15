package codex_test

// Tests that Invoke and InvokeRaw run the Codex CLI with the agent's own
// instructions, model and declared settings, return the protocol response or
// the raw payload, and leave the agent file untouched.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/codex"
)

const adapterTimeout = 10 * time.Second

func newAdapter(t *testing.T) *codex.CodexAdapter {
	t.Helper()
	return codex.NewCodexAdapter(helperExe(t), adapterTimeout)
}

func TestCodexAdapter_Invoke_ReturnsTheProtocolResponse(t *testing.T) {
	setHelperEnv(t, "codex-success")

	resp, err := newAdapter(t).Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != domain.StatusSUCCESS {
		t.Errorf("StatusCode = %q, want SUCCESS", resp.StatusCode)
	}
	if resp.AgentInstanceID != "test-agent#1" || resp.StatusMessage != "ok" {
		t.Errorf("response not decoded from the final agent message: %+v", resp)
	}
}

func TestCodexAdapter_Invoke_RunsCodexExecWithJSONOutput(t *testing.T) {
	argsFile := setHelperEnv(t, "codex-success")

	if _, err := newAdapter(t).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if len(args) == 0 || args[0] != "exec" {
		t.Errorf("want the exec subcommand first, got %v", args)
	}
	if !containsArg(args, "--json") {
		t.Errorf("want --json in argv, got %v", args)
	}
}

func TestCodexAdapter_Invoke_AppliesTheAgentsModelAndSandbox(t *testing.T) {
	argsFile := setHelperEnv(t, "codex-success")

	if _, err := newAdapter(t).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "-m", "gpt-5") {
		t.Errorf("want the agent's model (-m gpt-5), got %v", args)
	}
	if !containsSequence(args, "-s", "workspace-write") {
		t.Errorf("want the agent's sandbox (-s workspace-write), got %v", args)
	}
}

func TestCodexAdapter_Invoke_AppliesTheAgentsDeclaredSettings(t *testing.T) {
	argsFile := setHelperEnv(t, "codex-success")

	if _, err := newAdapter(t).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	for _, want := range []string{`web_search="live"`, `retries=3`, `sandbox_workspace_write.network_access=true`} {
		if !containsSequence(args, "-c", want) {
			t.Errorf("want -c %s in argv, got %v", want, args)
		}
	}
}

func TestCodexAdapter_Invoke_DeliversInstructionsAndRequestOnStdinNotArgv(t *testing.T) {
	argsFile := setHelperEnv(t, "codex-success")
	stdinFile := captureStdin(t)

	if _, err := newAdapter(t).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdin := readFile(t, stdinFile)
	if !strings.Contains(stdin, "You are the reviewer") {
		t.Errorf("stdin lacks the agent's instructions: %q", stdin)
	}
	if !strings.Contains(stdin, "review the change") || !strings.Contains(stdin, "test-agent#1") {
		t.Errorf("stdin lacks the marshalled request: %q", stdin)
	}
	if strings.Index(stdin, "You are the reviewer") > strings.Index(stdin, "review the change") {
		t.Errorf("instructions must precede the request on stdin: %q", stdin)
	}
	for _, a := range readArgs(t, argsFile) {
		if strings.Contains(a, "You are the reviewer") || strings.Contains(a, "review the change") || strings.Contains(a, "\n") {
			t.Errorf("argv must carry neither prompt text nor newlines, got %q", a)
		}
	}
}

func TestCodexAdapter_Invoke_LeavesTheAgentFileByteIdentical(t *testing.T) {
	setHelperEnv(t, "codex-success")
	ref, original := agentCopy(t)

	if _, err := newAdapter(t).Invoke(context.Background(), ref, request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileUnchanged(t, ref.DefinitionPath, original)
}

func TestCodexAdapter_Invoke_ReloadsTheAgentOnEveryInvocation(t *testing.T) {
	stdinFile := captureStdin(t)
	setHelperEnv(t, "codex-success")
	ref, original := agentCopy(t)
	adapter := newAdapter(t)
	if _, err := adapter.Invoke(context.Background(), ref, request("test-agent#1")); err != nil {
		t.Fatalf("first invocation: %v", err)
	}

	edited := strings.Replace(string(original), "You are the reviewer.", "You are the REPLACED reviewer.", 1)
	writeFile(t, ref.DefinitionPath, edited)
	if _, err := adapter.Invoke(context.Background(), ref, request("test-agent#2")); err != nil {
		t.Fatalf("second invocation: %v", err)
	}

	if got := readFile(t, stdinFile); !strings.Contains(got, "REPLACED reviewer") {
		t.Errorf("the second invocation must use the edited file, stdin = %q", got)
	}
}

func TestCodexAdapter_InvokeRaw_ReturnsTheRawFinalMessage(t *testing.T) {
	setHelperEnv(t, "codex-raw-success")

	out, err := newAdapter(t).InvokeRaw(context.Background(), agentRef(t), rawPayload)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != rawReply {
		t.Errorf("output = %q, want the raw final agent message %q", out, rawReply)
	}
}

func TestCodexAdapter_InvokeRaw_DeliversPayloadAndAgentSettings(t *testing.T) {
	argsFile := setHelperEnv(t, "codex-raw-success")
	stdinFile := captureStdin(t)

	if _, err := newAdapter(t).InvokeRaw(context.Background(), agentRef(t), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdin := readFile(t, stdinFile)
	if !strings.Contains(stdin, string(rawPayload)) || !strings.Contains(stdin, "You are the reviewer") {
		t.Errorf("stdin must carry the instructions and the payload, got %q", stdin)
	}
	if !containsSequence(readArgs(t, argsFile), "-m", "gpt-5") {
		t.Errorf("want the agent's model applied to a consultation too")
	}
}

func TestCodexAdapter_InvokeRaw_AcceptsNonProtocolReply(t *testing.T) {
	setHelperEnv(t, "codex-unparseable")

	out, err := newAdapter(t).InvokeRaw(context.Background(), agentRef(t), rawPayload)

	if err != nil {
		t.Fatalf("a consultation reply need not be protocol JSON, got %v", err)
	}
	if !strings.Contains(string(out), "not protocol JSON") {
		t.Errorf("want the final message returned verbatim, got %q", out)
	}
}

func TestCodexAdapter_InvokeRaw_LeavesTheAgentFileByteIdentical(t *testing.T) {
	setHelperEnv(t, "codex-raw-success")
	ref, original := agentCopy(t)

	if _, err := newAdapter(t).InvokeRaw(context.Background(), ref, rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileUnchanged(t, ref.DefinitionPath, original)
}

func TestCodexAdapter_SatisfiesTheRunnerPorts(t *testing.T) {
	adapter := newAdapter(t)

	var _ domain.HarnessAdapter = adapter
	var _ domain.RawInvoker = adapter
	revealer, ok := any(adapter).(domain.ExecutableRevealer)
	if !ok || revealer.ExecutablePath() != helperExe(t) {
		t.Errorf("want an ExecutableRevealer returning the constructed path")
	}
}
