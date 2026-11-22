package harness_test

// Tests for the delivery half of BuildCodexArgs: instructions and prompt travel
// on stdin, and nothing on the command line carries a newline or instruction
// text (Windows .cmd shims run through cmd /c and the command line is length
// limited).

import (
	"strings"
	"testing"

	"mosaic-common/harness"
)

const fullAgentInstruction = "You review code."

func TestBuildCodexArgs_Argv_HasNoNewlineForAnyAgent(t *testing.T) {
	for _, name := range []string{"agent_full.toml", "agent_collision.toml", "agent_minimal.toml"} {
		t.Run(name, func(t *testing.T) {
			args, _ := buildCodex(t, harness.SpawnRequest{
				WorkingDir:   "/work",
				Prompt:       "line one\nline two",
				SystemPrompt: "system\nprompt",
			}, loadCodexAgent(t, name))

			if len(args) == 0 {
				t.Fatal("empty argv")
			}
			for _, a := range args {
				if strings.ContainsAny(a, "\r\n") {
					t.Errorf("argv element contains a newline: %q", a)
				}
			}
		})
	}
}

func TestBuildCodexArgs_Argv_CarriesNoInstructionOrPromptText(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{
		Prompt:       "the user prompt",
		SystemPrompt: "the system prompt",
	}, loadCodexAgent(t, "agent_full.toml"))

	if len(args) == 0 {
		t.Fatal("empty argv")
	}
	for _, a := range args {
		for _, text := range []string{fullAgentInstruction, "Second instruction line.", "the user prompt", "the system prompt"} {
			if strings.Contains(a, text) {
				t.Errorf("argv element %q carries stdin text %q", a, text)
			}
		}
	}
}

func TestBuildCodexArgs_Stdin_CarriesInstructionsThenSystemPromptThenPrompt(t *testing.T) {
	_, stdin := buildCodex(t, harness.SpawnRequest{
		Prompt:       "the user prompt",
		SystemPrompt: "the system prompt",
	}, loadCodexAgent(t, "agent_full.toml"))

	got := string(stdin)
	instr := strings.Index(got, fullAgentInstruction)
	sys := strings.Index(got, "the system prompt")
	prompt := strings.Index(got, "the user prompt")
	if instr < 0 || sys < 0 || prompt < 0 {
		t.Fatalf("stdin lacks instructions, system prompt or prompt: %q", got)
	}
	if !(instr < sys && sys < prompt) {
		t.Errorf("want order instructions, system prompt, prompt; got %q", got)
	}
	if !strings.Contains(got, "Second instruction line.") {
		t.Errorf("stdin lacks the full instruction body: %q", got)
	}
	if !strings.HasSuffix(got, "the system prompt\nthe user prompt") {
		t.Errorf("want system prompt and prompt joined by a newline at the end, got %q", got)
	}
}

func TestBuildCodexArgs_Stdin_WithoutSystemPrompt_InstructionsThenPrompt(t *testing.T) {
	_, stdin := buildCodex(t, harness.SpawnRequest{Prompt: "the user prompt"}, loadCodexAgent(t, "agent_full.toml"))

	got := string(stdin)
	if !strings.Contains(got, fullAgentInstruction) {
		t.Errorf("stdin lacks the instructions: %q", got)
	}
	if !strings.HasSuffix(got, "\nthe user prompt") {
		t.Errorf("want the prompt after a newline at the end, got %q", got)
	}
}

func TestBuildCodexArgs_Stdin_WithoutPrompt_StillCarriesInstructions(t *testing.T) {
	_, stdin := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_minimal.toml"))

	if !strings.Contains(string(stdin), "You do the minimum.") {
		t.Errorf("want the instructions on stdin even without a prompt, got %q", stdin)
	}
}

func TestBuildCodexArgs_InstructionTextVariesWithAgent(t *testing.T) {
	_, full := buildCodex(t, harness.SpawnRequest{Prompt: "p"}, loadCodexAgent(t, "agent_full.toml"))
	_, minimal := buildCodex(t, harness.SpawnRequest{Prompt: "p"}, loadCodexAgent(t, "agent_minimal.toml"))

	if strings.Contains(string(full), "You do the minimum.") || strings.Contains(string(minimal), fullAgentInstruction) {
		t.Errorf("stdin must carry the loaded agent's own instructions only:\nfull: %q\nminimal: %q", full, minimal)
	}
}
