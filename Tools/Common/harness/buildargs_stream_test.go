package harness_test

// Tests for BuildArgs output-format handling: the stream-json form needs
// --verbose, the json form must stay exactly as it was.

import (
	"reflect"
	"testing"

	"mosaic-common/harness"
)

func TestBuildArgs_StreamJSON_Ordinary_EmitsVerbose(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "p", OutputFormat: "stream-json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSequence(args, "--output-format", "stream-json") {
		t.Errorf("want --output-format stream-json in args, got %v", args)
	}
	if !containsArg(args, "--verbose") {
		t.Errorf("want --verbose in args for stream-json, got %v", args)
	}
}

func TestBuildArgs_StreamJSON_Orchestrator_EmitsVerbose(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{Agent: orchestratorAgent(), Prompt: "p", OutputFormat: "stream-json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSequence(args, "--output-format", "stream-json") {
		t.Errorf("want --output-format stream-json in args, got %v", args)
	}
	if !containsArg(args, "--verbose") {
		t.Errorf("want --verbose in args for stream-json, got %v", args)
	}
}

func TestBuildArgs_StreamJSON_VerboseEmittedExactlyOnce(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: ordinaryAgent(), Prompt: "p", OutputFormat: "stream-json",
		DerivedTools: []string{"Read"}, ToolsDerived: true, SessionPersistence: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := 0
	for _, a := range args {
		if a == "--verbose" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want --verbose exactly once, got %d in %v", n, args)
	}
}

func TestBuildArgs_StreamJSON_OtherFlagsUnchanged(t *testing.T) {
	jsonArgs, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: ordinaryAgent(), Prompt: "p", OutputFormat: "json",
		DerivedTools: []string{"Read"}, ToolsDerived: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	streamArgs, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: ordinaryAgent(), Prompt: "p", OutputFormat: "stream-json",
		DerivedTools: []string{"Read"}, ToolsDerived: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, 0, len(streamArgs))
	for i := 0; i < len(streamArgs); i++ {
		switch {
		case streamArgs[i] == "--verbose":
		case streamArgs[i] == "--output-format" && i+1 < len(streamArgs):
			got = append(got, "--output-format", "json")
			i++
		default:
			got = append(got, streamArgs[i])
		}
	}
	if !reflect.DeepEqual(got, jsonArgs) {
		t.Errorf("want stream args to differ from json args only by --verbose and the format value\n json:   %v\n stream: %v", jsonArgs, streamArgs)
	}
}

func TestBuildArgs_JSON_Ordinary_ExactArgsWithoutVerbose(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "p", OutputFormat: "json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"-p",
		"--agent", ordinaryAgent().Identifier,
		"--output-format", "json",
		"--permission-mode", "auto",
		"--no-session-persistence",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("want exactly %v, got %v", want, args)
	}
}

func TestBuildArgs_JSON_Orchestrator_ExactArgsWithoutVerbose(t *testing.T) {
	args, _, err := harness.BuildArgs(harness.SpawnRequest{
		Agent: orchestratorAgent(), Prompt: "p", OutputFormat: "json",
		DerivedTools: []string{"Read", "Bash"}, ToolsDerived: true, SessionPersistence: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"-p",
		"--agent", orchestratorAgent().Identifier,
		"--output-format", "json",
		"--permission-mode", "dontAsk",
		"--allowedTools", "Read",
		"--allowedTools", "Bash",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("want exactly %v, got %v", want, args)
	}
}
