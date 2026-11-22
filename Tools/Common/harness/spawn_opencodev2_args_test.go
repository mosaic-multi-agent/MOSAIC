package harness_test

// Tests for BuildOpenCodeV2Args: one headless OpenCode V2 dispatch. Reuses
// ordinaryAgent and containsArg/containsSequence from the shared test helpers.

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/harness"
)

func v2Args(t *testing.T, req harness.SpawnRequest) ([]string, []byte) {
	t.Helper()
	args, stdin, err := harness.BuildOpenCodeV2Args(req)
	if err != nil {
		t.Fatalf("BuildOpenCodeV2Args: unexpected error: %v", err)
	}
	return args, stdin
}

func TestBuildOpenCodeV2Args_MinimalRequestIsExactlyTheFixedFlags(t *testing.T) {
	args, _ := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x"})

	want := []string{"run", "--agent", "test-agent", "--format", "json", "--auto", "--standalone"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("want args %v, got %v", want, args)
	}
}

func TestBuildOpenCodeV2Args_AlwaysStandalone(t *testing.T) {
	reqs := map[string]harness.SpawnRequest{
		"minimal":        {Agent: ordinaryAgent(), Prompt: "x"},
		"with model":     {Agent: ordinaryAgent(), Prompt: "x", Model: "openai/gpt-5"},
		"with extra":     {Agent: ordinaryAgent(), Prompt: "x", ExtraArgs: []string{"--print-logs"}},
		"json format":    {Agent: ordinaryAgent(), Prompt: "x", OutputFormat: "json"},
		"with workdir":   {Agent: ordinaryAgent(), Prompt: "x", WorkingDir: "/work"},
		"empty prompt":   {Agent: ordinaryAgent()},
		"system prompt":  {Agent: ordinaryAgent(), Prompt: "x", SystemPrompt: "sys"},
		"orchestrator":   {Agent: orchestratorAgent(), Prompt: "x"},
		"max turns/tool": {Agent: ordinaryAgent(), Prompt: "x", MaxTurns: 3, AllowedTools: []string{"Read"}},
	}
	for name, req := range reqs {
		t.Run(name, func(t *testing.T) {
			args, _ := v2Args(t, req)
			if !containsArg(args, "--standalone") {
				t.Errorf("want --standalone in args, got %v", args)
			}
		})
	}
}

func TestBuildOpenCodeV2Args_StartsWithRunAndCarriesAgentFormatAuto(t *testing.T) {
	args, _ := v2Args(t, harness.SpawnRequest{Agent: orchestratorAgent(), Prompt: "x"})

	if len(args) == 0 || args[0] != "run" {
		t.Errorf("want args to start with run, got %v", args)
	}
	if !containsSequence(args, "--agent", "orchestrator-agent") {
		t.Errorf("want --agent orchestrator-agent, got %v", args)
	}
	if !containsSequence(args, "--format", "json") {
		t.Errorf("want --format json, got %v", args)
	}
	if !containsArg(args, "--auto") {
		t.Errorf("want --auto, got %v", args)
	}
}

func TestBuildOpenCodeV2Args_NoModelFlagWhenModelEmpty(t *testing.T) {
	args, _ := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x"})

	if containsArg(args, "--model") {
		t.Errorf("want no --model without a model, got %v", args)
	}
}

func TestBuildOpenCodeV2Args_ModelPassedVerbatim(t *testing.T) {
	for _, model := range []string{"openai/gpt-5", "openai/gpt-5#high", "anthropic/claude-sonnet-4-5#max"} {
		args, _ := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x", Model: model})

		if !containsSequence(args, "--model", model) {
			t.Errorf("want --model %q verbatim, got %v", model, args)
		}
	}
}

func TestBuildOpenCodeV2Args_ModelFollowsStandalone(t *testing.T) {
	args, _ := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x", Model: "openai/gpt-5#high"})

	want := []string{"run", "--agent", "test-agent", "--format", "json", "--auto", "--standalone", "--model", "openai/gpt-5#high"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("want args %v, got %v", want, args)
	}
}

func TestBuildOpenCodeV2Args_ExtraArgsComeLast(t *testing.T) {
	args, _ := v2Args(t, harness.SpawnRequest{
		Agent: ordinaryAgent(), Prompt: "x", Model: "openai/gpt-5", ExtraArgs: []string{"--print-logs", "--log-level", "DEBUG"},
	})

	want := []string{
		"run", "--agent", "test-agent", "--format", "json", "--auto", "--standalone",
		"--model", "openai/gpt-5", "--print-logs", "--log-level", "DEBUG",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("want args %v, got %v", want, args)
	}
}

func TestBuildOpenCodeV2Args_NeverEmitsV1OnlyOrSessionFlags(t *testing.T) {
	// Every field the V2 builder ignores is populated, plus a model with a
	// variant: none of it may surface as --dir, --variant or a session flag.
	req := harness.SpawnRequest{
		Agent: ordinaryAgent(), Prompt: "x", SystemPrompt: "sys", Model: "openai/gpt-5#high",
		WorkingDir: "/some/dir", MaxTurns: 5, AllowedTools: []string{"Read"}, DerivedTools: []string{"Read"},
		ToolsDerived: true, SessionPersistence: true, Env: []string{"A=b"},
	}
	args, _ := v2Args(t, req)

	for _, forbidden := range []string{"--dir", "--variant", "--session", "--continue", "--fork", "--max-turns", "--allowedTools", "-p"} {
		if containsArg(args, forbidden) {
			t.Errorf("%s must never appear, got %v", forbidden, args)
		}
	}
	for _, a := range args {
		if strings.Contains(a, "/some/dir") {
			t.Errorf("want the working directory kept out of argv, got %q in %v", a, args)
		}
	}
}

func TestBuildOpenCodeV2Args_NoPositionalMessageAndNoPromptInArgv(t *testing.T) {
	prompt := "line one\nline two with {\"json\": true}"
	args, _ := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: prompt, SystemPrompt: "<env>\nx\n</env>"})

	assertNoPromptInArgv(t, args, "line one", "line two", "<env>")
	if len(args) != 7 {
		t.Errorf("want exactly 7 args (no positional message), got %d: %v", len(args), args)
	}
}

func TestBuildOpenCodeV2Args_EmptyAgentIdentifierRejected(t *testing.T) {
	_, _, err := harness.BuildOpenCodeV2Args(harness.SpawnRequest{Prompt: "x"})

	if !errors.Is(err, harness.ErrOpenCodeV2EmptyAgentIdentifier) {
		t.Errorf("want ErrOpenCodeV2EmptyAgentIdentifier, got %v", err)
	}
}

func TestBuildOpenCodeV2Args_UnsupportedOutputFormatRejected(t *testing.T) {
	_, _, err := harness.BuildOpenCodeV2Args(harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x", OutputFormat: "stream-json"})

	if !errors.Is(err, harness.ErrOpenCodeV2UnsupportedOutputFormat) {
		t.Errorf("want ErrOpenCodeV2UnsupportedOutputFormat, got %v", err)
	}
}

func TestBuildOpenCodeV2Args_EmptyAndJSONOutputFormatAccepted(t *testing.T) {
	for _, format := range []string{"", "json"} {
		args, _ := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x", OutputFormat: format})

		if !containsSequence(args, "--format", "json") {
			t.Errorf("OutputFormat %q: want --format json, got %v", format, args)
		}
	}
}

func TestBuildOpenCodeV2Args_StdinIsPromptWhenNoSystemPrompt(t *testing.T) {
	_, stdin := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "the request"})

	if string(stdin) != "the request" {
		t.Errorf("want stdin %q, got %q", "the request", stdin)
	}
}

func TestBuildOpenCodeV2Args_StdinIsSystemPromptNewlinePrompt(t *testing.T) {
	_, stdin := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent(), SystemPrompt: "<env>\nWorking directory: /w\n</env>", Prompt: "the request"})

	if want := "<env>\nWorking directory: /w\n</env>\nthe request"; string(stdin) != want {
		t.Errorf("want stdin %q, got %q", want, stdin)
	}
}

func TestBuildOpenCodeV2Args_StdinNilWhenContentEmpty(t *testing.T) {
	_, stdin := v2Args(t, harness.SpawnRequest{Agent: ordinaryAgent()})

	if stdin != nil {
		t.Errorf("want nil stdin for empty content, got %q", stdin)
	}
}

func TestBuildOpenCodeV2Args_IsPure(t *testing.T) {
	req := harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "x", Model: "openai/gpt-5", ExtraArgs: []string{"--print-logs"}}

	a1, s1 := v2Args(t, req)
	a2, s2 := v2Args(t, req)

	if !reflect.DeepEqual(a1, a2) || string(s1) != string(s2) {
		t.Errorf("want identical output for identical input, got %v/%q then %v/%q", a1, s1, a2, s2)
	}
	if !reflect.DeepEqual(req.ExtraArgs, []string{"--print-logs"}) {
		t.Errorf("want ExtraArgs untouched, got %v", req.ExtraArgs)
	}
}
