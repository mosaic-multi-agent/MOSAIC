package claudecode_test

// Tests for ClaudeCodeAdapter CLI argument construction.
// Uses the fake-CLI helper process defined in helperprocess_test.go.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/claudecode"
)

// ---------------------------------------------------------------------------
// CLI argument construction
// ---------------------------------------------------------------------------

// TestClaudeCodeAdapter_OrdinaryInvocation_LaunchesWithAgentFlag verifies that
// the adapter launches with --agent <Identifier> and never passes the
// definition path or --append-system-prompt-file.
func TestClaudeCodeAdapter_OrdinaryInvocation_LaunchesWithAgentFlag(t *testing.T) {
	argsFile := setHelperEnv(t, "success")
	agent := ordinaryAgentRefCC(t)

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), agent, minimalClaudeRequest("test-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "--agent", agent.Identifier) {
		t.Errorf("want --agent %q in args, got %v", agent.Identifier, args)
	}
	if containsArg(args, "--append-system-prompt-file") || containsArg(args, agent.DefinitionPath) {
		t.Errorf("want no --append-system-prompt-file and no definition path in args, got %v", args)
	}
}

// TestClaudeCodeAdapter_OrdinaryInvocation_IncludesPromptFlag verifies that
// the adapter passes -p as a bare flag and delivers the marshalled
// request JSON on stdin. The prompt travels on stdin rather than as the value
// following -p, so that multi-line prompts are not truncated by cmd.exe on Windows.
func TestClaudeCodeAdapter_OrdinaryInvocation_IncludesPromptFlag(t *testing.T) {
	argsFile := setHelperEnv(t, "success")
	stdinFile := newTestStdinCapture(t)

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	// -p must be present as a bare flag — the prompt travels on stdin, not as
	// the following argv element.
	if !containsArg(args, "-p") {
		t.Fatalf("want bare -p flag in args, got %v", args)
	}
	// The prompt content (agent_instance_id is a reliable field) must be on stdin.
	stdin := readHelperStdin(t, stdinFile)
	if !strings.Contains(string(stdin), "test-agent#1") {
		t.Errorf("want agent_instance_id %q in stdin payload, got %q", "test-agent#1", stdin)
	}
}

// TestClaudeCodeAdapter_OrdinaryInvocation_IncludesOutputFormatStreamJSON verifies
// that the adapter passes --output-format stream-json --verbose.
func TestClaudeCodeAdapter_OrdinaryInvocation_IncludesOutputFormatStreamJSON(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "--output-format", "stream-json") || !containsArg(args, "--verbose") {
		t.Errorf("want --output-format stream-json and --verbose in args, got %v", args)
	}
}

// TestClaudeCodeAdapter_OrdinaryInvocation_IncludesPermissionModeDontAsk verifies
// that the adapter uses --permission-mode dontAsk when the agent's
// definition file provides a valid tools frontmatter field.
func TestClaudeCodeAdapter_OrdinaryInvocation_IncludesPermissionModeDontAsk(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "--permission-mode", "dontAsk") {
		t.Errorf("want --permission-mode dontAsk in args (derived from agent tools frontmatter), got %v", args)
	}
}

// TestClaudeCodeAdapter_OrdinaryInvocation_NeverDangerouslySkipPermissions
// verifies that --dangerously-skip-permissions is never present in the
// adapter arguments.
func TestClaudeCodeAdapter_OrdinaryInvocation_NeverDangerouslySkipPermissions(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if containsArg(args, "--dangerously-skip-permissions") {
		t.Errorf("--dangerously-skip-permissions must never appear in args, got %v", args)
	}
}

// TestClaudeCodeAdapter_Invocation_NoEnvBlockInPrompt verifies that no
// agent gets a synthesized <env> block in the stdin prompt: the
// prompt is exactly the marshalled request.
func TestClaudeCodeAdapter_Invocation_NoEnvBlockInPrompt(t *testing.T) {
	kinds := map[string]func(*testing.T) domain.AgentReference{
		"ordinary":     ordinaryAgentRefCC,
		"orchestrator": orchestratorAgentRefCC,
	}
	for name, mk := range kinds {
		t.Run(name, func(t *testing.T) {
			setHelperEnv(t, "success")
			stdinFile := newTestStdinCapture(t)

			adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
			_, err := adapter.Invoke(context.Background(), mk(t), minimalClaudeRequest("some-agent#1"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			stdin := string(readHelperStdin(t, stdinFile))
			if strings.Contains(stdin, "<env>") || strings.Contains(stdin, "Working directory:") {
				t.Errorf("want no <env> block in stdin prompt, got %q", stdin)
			}
			if !strings.HasPrefix(strings.TrimSpace(stdin), "{") {
				t.Errorf("want stdin to start with the marshalled request JSON, got %q", stdin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CLI argument construction — orchestrator agents
// ---------------------------------------------------------------------------

// TestClaudeCodeAdapter_OrchestratorInvocation_IncludesAgentFlag verifies that
// orchestrator agents include --agent <Identifier>.
func TestClaudeCodeAdapter_OrchestratorInvocation_IncludesAgentFlag(t *testing.T) {
	argsFile := setHelperEnv(t, "success")
	agent := orchestratorAgentRefCC(t)

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), agent, minimalClaudeRequest("orchestrator-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "--agent", agent.Identifier) {
		t.Errorf("want --agent %q in args, got %v", agent.Identifier, args)
	}
}

// TestClaudeCodeAdapter_OrchestratorInvocation_NoAppendSystemPromptFile
// verifies that orchestrator agents do NOT include
// --append-system-prompt-file.
func TestClaudeCodeAdapter_OrchestratorInvocation_NoAppendSystemPromptFile(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), orchestratorAgentRefCC(t), minimalClaudeRequest("orchestrator-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if containsArg(args, "--append-system-prompt-file") {
		t.Errorf("want --append-system-prompt-file absent from orchestrator args, got %v", args)
	}
}

// TestClaudeCodeAdapter_OrchestratorInvocation_IncludesOutputFormatStreamJSON
// verifies that orchestrator agents include --output-format stream-json --verbose.
func TestClaudeCodeAdapter_OrchestratorInvocation_IncludesOutputFormatStreamJSON(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), orchestratorAgentRefCC(t), minimalClaudeRequest("orchestrator-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "--output-format", "stream-json") || !containsArg(args, "--verbose") {
		t.Errorf("want --output-format stream-json and --verbose in orchestrator args, got %v", args)
	}
}

// TestClaudeCodeAdapter_OrchestratorInvocation_IncludesPermissionModeDontAsk
// verifies that orchestrator agents use --permission-mode dontAsk when
// the agent's definition file provides a valid tools frontmatter field.
func TestClaudeCodeAdapter_OrchestratorInvocation_IncludesPermissionModeDontAsk(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), orchestratorAgentRefCC(t), minimalClaudeRequest("orchestrator-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if !containsSequence(args, "--permission-mode", "dontAsk") {
		t.Errorf("want --permission-mode dontAsk in orchestrator args (derived from agent tools frontmatter), got %v", args)
	}
}

// TestClaudeCodeAdapter_OrchestratorInvocation_NeverDangerouslySkipPermissions
// verifies that --dangerously-skip-permissions is never present in orchestrator
// invocation arguments.
func TestClaudeCodeAdapter_OrchestratorInvocation_NeverDangerouslySkipPermissions(t *testing.T) {
	argsFile := setHelperEnv(t, "success")

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), orchestratorAgentRefCC(t), minimalClaudeRequest("orchestrator-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if containsArg(args, "--dangerously-skip-permissions") {
		t.Errorf("--dangerously-skip-permissions must never appear in orchestrator agent args, got %v", args)
	}
}

// TestClaudeCodeAdapter_OrchestratorInvocation_IncludesRequestInPrompt verifies
// that the marshalled request JSON is present in the orchestrator stdin payload.
func TestClaudeCodeAdapter_OrchestratorInvocation_IncludesRequestInPrompt(t *testing.T) {
	setHelperEnv(t, "success")
	stdinFile := newTestStdinCapture(t)

	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	_, err := adapter.Invoke(context.Background(), orchestratorAgentRefCC(t), minimalClaudeRequest("orchestrator-agent#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// agent_instance_id is a reliable field from the marshalled request JSON.
	stdin := string(readHelperStdin(t, stdinFile))
	if !strings.Contains(stdin, "orchestrator-agent#1") {
		t.Errorf("want marshalled request JSON (containing agent_instance_id) in orchestrator stdin payload, got %q", stdin)
	}
}
