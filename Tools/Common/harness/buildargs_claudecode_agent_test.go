package harness_test

// Tests for the Claude Code argument shape built by BuildArgs: every
// invocation launches `-p --agent <identifier>`, the definition path is never
// passed, and stdin carries exactly the prompt.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-common/harness"
)

func someAgent() harness.AgentRef {
	return harness.AgentRef{
		Identifier:     "some-agent",
		DefinitionPath: "/deployed/agents/some-agent.md",
	}
}

func TestBuildArgs_ClaudeCode_LaunchesWithAgentFlag(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: someAgent(), Prompt: "x", OutputFormat: "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) < 3 || args[0] != "-p" || args[1] != "--agent" || args[2] != "some-agent" {
		t.Errorf("want args to start with -p --agent some-agent, got %v", args)
	}
}

func TestBuildArgs_ClaudeCode_NeverPassesDefinitionPath(t *testing.T) {
	agent := someAgent()
	args, _, err := harness.BuildArgs(harness.SpawnRequest{Agent: agent, Prompt: "x", OutputFormat: "json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if containsArg(args, "--append-system-prompt-file") {
		t.Errorf("want no --append-system-prompt-file, got %v", args)
	}
	if containsArg(args, agent.DefinitionPath) {
		t.Errorf("want definition path absent from args, got %v", args)
	}
}

func TestBuildArgs_ClaudeCode_StdinIsExactlyThePrompt(t *testing.T) {
	const prompt = "line one\nline two"
	_, stdin, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: someAgent(), Prompt: prompt, OutputFormat: "json", WorkingDir: "/some/dir",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdin) != prompt {
		t.Errorf("want stdin exactly %q, got %q", prompt, stdin)
	}
	if strings.Contains(string(stdin), "<env>") {
		t.Errorf("want no <env> block in stdin, got %q", stdin)
	}
}

func TestBuildArgs_ClaudeCode_EmptyPromptYieldsNilStdin(t *testing.T) {
	_, stdin, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: someAgent(), Prompt: "", OutputFormat: "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdin != nil {
		t.Errorf("want nil stdin for empty prompt, got %q", stdin)
	}
}

func TestBuildArgs_ClaudeCode_FullArgumentOrder(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent:        someAgent(),
		Prompt:       "p",
		OutputFormat: "stream-json",
		DerivedTools: []string{"Read", "Bash"},
		ExtraArgs:    []string{"--model", "m"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"-p", "--agent", "some-agent",
		"--output-format", "stream-json",
		"--permission-mode", "dontAsk",
		"--verbose",
		"--no-session-persistence",
		"--allowedTools", "Read",
		"--allowedTools", "Bash",
		"--model", "m",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("want %v, got %v", want, args)
	}
}

func TestBuildArgs_ClaudeCode_SessionPersistenceOmitsNoSessionFlag(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: someAgent(), Prompt: "p", OutputFormat: "json", SessionPersistence: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if containsArg(args, "--no-session-persistence") {
		t.Errorf("want --no-session-persistence absent, got %v", args)
	}
	if !containsSequence(args, "--agent", "some-agent") {
		t.Errorf("want --agent some-agent, got %v", args)
	}
}

func TestBuildArgs_ClaudeCode_PermissionModeSelection(t *testing.T) {
	cases := []struct {
		name string
		req  harness.SpawnRequest
		want string
	}{
		{"no tools", harness.SpawnRequest{}, "auto"},
		{"derived tools", harness.SpawnRequest{DerivedTools: []string{"Read"}}, "dontAsk"},
		{"tools derived but empty", harness.SpawnRequest{ToolsDerived: true}, "dontAsk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.Agent = someAgent()
			tc.req.OutputFormat = "json"
			args, _, err := harness.BuildArgs(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !containsSequence(args, "--permission-mode", tc.want) {
				t.Errorf("want --permission-mode %s, got %v", tc.want, args)
			}
			if !containsSequence(args, "--agent", "some-agent") {
				t.Errorf("want --agent some-agent, got %v", args)
			}
		})
	}
}
