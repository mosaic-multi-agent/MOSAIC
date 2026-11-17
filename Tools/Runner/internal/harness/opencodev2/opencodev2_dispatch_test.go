package opencodev2_test

// Tests for what one dispatch sends to V2: the command line (always a
// standalone run of the agent by id, no model, no session flags), the prompt
// and environment block on stdin, and the successful results of Invoke and
// InvokeRaw.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/opencodev2"
)

// forbiddenFlags are never passed: the agent file's model applies, the working
// directory is set on the process, and sessions are never continued.
var forbiddenFlags = []string{"--model", "--dir", "--variant", "--session", "--continue", "--fork", "--attach"}

func assertStandaloneRunOf(t *testing.T, args []string, agentID string) {
	t.Helper()
	if len(args) == 0 || args[0] != "run" {
		t.Errorf("want the run subcommand first, got %v", args)
	}
	if !containsSequence(args, "--agent", agentID) {
		t.Errorf("want --agent %q in args, got %v", agentID, args)
	}
	if !containsSequence(args, "--format", "json") {
		t.Errorf("want --format json in args, got %v", args)
	}
	for _, want := range []string{"--standalone", "--auto"} {
		if !containsArg(args, want) {
			t.Errorf("want %s in args, got %v", want, args)
		}
	}
	for _, bad := range forbiddenFlags {
		if containsArg(args, bad) {
			t.Errorf("args must not contain %s, got %v", bad, args)
		}
	}
}

func TestInvoke_Success_ReturnsTheProtocolResponseFromTheFinalText(t *testing.T) {
	setHelperEnv(t, "success")

	resp, err := newAdapter(t, 10*time.Second).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err != nil {
		t.Fatalf("want success on exit 0, got %v", err)
	}
	if resp.StatusCode != domain.StatusSUCCESS || resp.AgentInstanceID != "test-agent#1" {
		t.Errorf("want SUCCESS for test-agent#1, got %+v", resp)
	}
}

func TestInvoke_RunsTheAgentByIdStandalone(t *testing.T) {
	setHelperEnv(t, "success")
	argsFile := captureFile(t, "GO_HELPER_ARGS_FILE", "args.json")

	_, err := newAdapter(t, 10*time.Second).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertStandaloneRunOf(t, readArgs(t, argsFile), "test-agent")
}

func TestInvoke_PromptAndEnvironmentBlockTravelOnStdin(t *testing.T) {
	setHelperEnv(t, "success")
	stdinFile := captureFile(t, "GO_HELPER_STDIN_FILE", "stdin.txt")
	root := t.TempDir()
	adapter := newAdapter(t, 10*time.Second).WithWorkspaceRoot(root)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("Research#3"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdin := string(readFileOrFail(t, stdinFile))
	envAt := strings.Index(stdin, "<env>")
	promptAt := strings.Index(stdin, "Research#3")
	if envAt < 0 || promptAt < 0 || envAt > promptAt {
		t.Errorf("want the <env> block first, then the request carrying the instance id; got %q", stdin)
	}
	if !strings.Contains(stdin, "do the thing") {
		t.Errorf("want the task description in the prompt, got %q", stdin)
	}
	if !strings.Contains(stdin, root) {
		t.Errorf("want the environment block to name the working directory %q, got %q", root, stdin)
	}
}

func TestInvokeRaw_RunsTheOrchestratorAgentByIdStandalone(t *testing.T) {
	setHelperEnv(t, "raw-text")
	argsFile := captureFile(t, "GO_HELPER_ARGS_FILE", "args.json")
	stdinFile := captureFile(t, "GO_HELPER_STDIN_FILE", "stdin.txt")

	_, err := newAdapter(t, 10*time.Second).InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertStandaloneRunOf(t, readArgs(t, argsFile), "orchestrator-agent")
	if stdin := string(readFileOrFail(t, stdinFile)); !strings.Contains(stdin, string(rawPayload)) {
		t.Errorf("want the payload on stdin, got %q", stdin)
	}
}

func TestInvokeRaw_ReturnsTheFinalTextEvenWhenItHoldsNoProtocolJSON(t *testing.T) {
	setHelperEnv(t, "raw-text")

	got, err := newAdapter(t, 10*time.Second).InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

	if err != nil {
		t.Fatalf("a consultation reply without protocol JSON is a success, got %v", err)
	}
	if string(got) != "route to agent-b, no protocol here" {
		t.Errorf("want the final text as the reply, got %q", got)
	}
}

func TestInvoke_DeniedToolCall_DoesNotFailTheDispatch(t *testing.T) {
	setHelperEnv(t, "denied-tool")

	resp, err := newAdapter(t, 10*time.Second).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err != nil {
		t.Fatalf("a permission-denied tool call must not fail the dispatch, got %v", err)
	}
	if resp.StatusCode != domain.StatusSUCCESS {
		t.Errorf("want SUCCESS, got %+v", resp)
	}
}

func TestInvoke_FinalTextWithoutProtocolJSON_ReturnsMalformedOutput(t *testing.T) {
	setHelperEnv(t, "not-protocol")
	logger := &recordingLogger{}
	adapter := opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), 10*time.Second, logger)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if !errors.Is(err, cliexec.ErrMalformedOutput) {
		t.Fatalf("want ErrMalformedOutput, got %v", err)
	}
	if logger.count(domain.EventHarnessParseFailed) == 0 {
		t.Errorf("want a parse-failed event for an unparseable reply")
	}
}

// Agents are run in place by id: nothing is copied, rewritten or backed up.
func TestInvoke_NeverModifiesOrBacksUpTheAgentFile(t *testing.T) {
	setHelperEnv(t, "success")
	dir := t.TempDir()
	defPath := filepath.Join(dir, "test-agent.md")
	const content = "---\nname: test-agent\nmodel: some/model\n---\n\nAgent body.\n"
	if err := os.WriteFile(defPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(defPath)
	agent := domain.AgentReference{Identifier: "test-agent", DefinitionPath: defPath}
	adapter := newAdapter(t, 10*time.Second).WithWorkspaceRoot(dir)

	_, invokeErr := adapter.Invoke(context.Background(), agent, minimalRequest("test-agent#1"))
	_, rawErr := adapter.InvokeRaw(context.Background(), agent, rawPayload)

	if invokeErr != nil || rawErr != nil {
		t.Fatalf("both dispatches must succeed; Invoke=%v InvokeRaw=%v", invokeErr, rawErr)
	}
	after, err := os.Stat(defPath)
	if err != nil {
		t.Fatalf("agent file vanished: %v", err)
	}
	got, _ := os.ReadFile(defPath)
	if string(got) != content || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the agent file was rewritten: %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("want only the agent file in its folder (no copies or backups), got %v", entries)
	}
}

// The composition root applies the per-harness default before construction;
// the adapter reports exactly the path or command name it was given.
func TestAdapter_ExecutablePath_ReturnsTheConstructedPath(t *testing.T) {
	for _, path := range []string{"/usr/local/bin/opencode", "opencode"} {
		adapter := opencodev2.NewOpenCodeV2Adapter(path, time.Second)

		if got := adapter.ExecutablePath(); got != path {
			t.Errorf("ExecutablePath() = %q, want %q", got, path)
		}
	}
}

func TestAdapter_SatisfiesTheRunnerCapabilityInterfaces(t *testing.T) {
	adapter := newAdapter(t, time.Second)

	var _ domain.HarnessAdapter = adapter
	var _ domain.RawInvoker = adapter
	var _ domain.ExecutableRevealer = adapter
}
