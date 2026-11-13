package domain_test

// Tests for the structured agent_instance_id extraction rule and the
// subagent-launch tool set.
//
// Rules under test:
//   - Only the structured form `agent_instance_id <sep> id` is recognised, where
//     <sep> is ':' or '=' with optional ASCII whitespace and quotes around it.
//   - The token must not be preceded by [A-Za-z0-9_].
//   - The id has the {type}#{N} form; a bare Name#N never matches.
//   - The first valid occurrence wins.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestExtractInstanceID_StructuredForms_ReturnTheId(t *testing.T) {
	cases := []struct {
		name string
		text string
		want domain.AgentInstanceID
	}{
		{"colon and space", "agent_instance_id: planner#3", "planner#3"},
		{"colon without space", "agent_instance_id:planner#3", "planner#3"},
		{"equals sign", "agent_instance_id=planner#3", "planner#3"},
		{"spaces around equals", "agent_instance_id  =  planner#3", "planner#3"},
		{"space before colon", "agent_instance_id : planner#3", "planner#3"},
		{"json style double quotes", `"agent_instance_id": "planner#3"`, "planner#3"},
		{"single quotes", `'agent_instance_id' = 'planner#3'`, "planner#3"},
		{"quoted value only", `agent_instance_id: "planner#3"`, "planner#3"},
		{"space after opening quote", `agent_instance_id: " planner#3"`, "planner#3"},
		{"newline between separator and id", "agent_instance_id:\n  planner#3", "planner#3"},
		{"tab and carriage return whitespace", "agent_instance_id\t:\r\n\tplanner#3", "planner#3"},
		{"type with dot underscore hyphen", "agent_instance_id: a.b_c-d#12", "a.b_c-d#12"},
		{"multi digit number", "agent_instance_id: planner#1234", "planner#1234"},
		{"surrounded by prose", "You are agent. Run: agent_instance_id: planner#3. Go.", "planner#3"},
		{"at the very start of the text", "agent_instance_id=a#1 rest", "a#1"},
		{"preceded by a hyphen", "-agent_instance_id: a#1", "a#1"},
		{"preceded by a dot", ".agent_instance_id: a#1", "a#1"},
		{"preceded by a space", "id agent_instance_id: a#1", "a#1"},
		{"preceded by a non-ASCII letter", "\u00e9agent_instance_id: a#1", "a#1"},
		{"id is the longest match, trailing letters are not part of it", "agent_instance_id: planner#12x", "planner#12"},
		{"id is the longest match, trailing id characters stop at the number", "agent_instance_id: planner#12.5", "planner#12"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := domain.ExtractInstanceID(tc.text)

			if !ok || got != tc.want {
				t.Errorf("ExtractInstanceID(%q) = (%q, %v), want (%q, true)", tc.text, got, ok, tc.want)
			}
		})
	}
}

func TestExtractInstanceID_NoStructuredForm_ReportsNotFound(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty text", ""},
		{"bare Name#N", "Please run planner#3 now"},
		{"bare Name#N after unrelated label", "agent: planner#3"},
		{"token without value", "the agent_instance_id is required"},
		{"token followed by non-id", "agent_instance_id: unknown-agent"},
		{"missing number", "agent_instance_id: planner#"},
		{"missing type", "agent_instance_id: #3"},
		{"type starting with a digit", "agent_instance_id: 1planner#3"},
		{"type starting with a hyphen", "agent_instance_id: -planner#3"},
		{"two separators", "agent_instance_id::planner#3"},
		{"no separator", "agent_instance_id planner#3"},
		{"preceded by a letter", "my_agent_instance_id: planner#3"},
		{"preceded by an underscore", "_agent_instance_id: planner#3"},
		{"preceded by a digit", "1agent_instance_id: planner#3"},
		{"preceded by an uppercase letter", "Xagent_instance_id: planner#3"},
		{"different token case", "AGENT_INSTANCE_ID: planner#3"},
		{"non-ASCII space before the id", "agent_instance_id:\u00a0planner#3"},
		{"non-ASCII space before the separator", "agent_instance_id\u00a0: planner#3"},
		{"id on a different label", "agent_id: planner#3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := domain.ExtractInstanceID(tc.text)

			if ok || got != "" {
				t.Errorf("ExtractInstanceID(%q) = (%q, %v), want (\"\", false)", tc.text, got, ok)
			}
		})
	}
}

func TestExtractInstanceID_SeveralOccurrences_FirstValidOneWins(t *testing.T) {
	cases := []struct {
		name string
		text string
		want domain.AgentInstanceID
	}{
		{"two valid", "agent_instance_id: first#1 and agent_instance_id: second#2", "first#1"},
		{"invalid occurrence is skipped", "agent_instance_id: unknown-agent then agent_instance_id: second#2", "second#2"},
		{"preceded occurrence is skipped", "x_agent_instance_id: first#1 agent_instance_id: second#2", "second#2"},
		{"bare id before the structured form is ignored", "planner#9 agent_instance_id: second#2", "second#2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := domain.ExtractInstanceID(tc.text)

			if !ok || got != tc.want {
				t.Errorf("ExtractInstanceID(%q) = (%q, %v), want (%q, true)", tc.text, got, ok, tc.want)
			}
		})
	}
}

func TestIsSubagentLaunchTool_KnownLaunchTools_AnyAsciiCase(t *testing.T) {
	for _, name := range []string{
		"Agent", "agent", "AGENT", "Task", "task", "TASK",
		"runSubagent", "runsubagent", "RunSubagent", "execution_subagent", "Execution_Subagent",
	} {
		if !domain.IsSubagentLaunchTool(name) {
			t.Errorf("IsSubagentLaunchTool(%q) = false, want true", name)
		}
	}
}

func TestIsSubagentLaunchTool_OtherTools_False(t *testing.T) {
	for _, name := range []string{
		"", "Read", "Bash", "Edit", "agents", "subagent", "TaskCreate", "task_create",
		"run_subagent", "Agent ", " agent", "execution", "ToolSearch",
	} {
		if domain.IsSubagentLaunchTool(name) {
			t.Errorf("IsSubagentLaunchTool(%q) = true, want false", name)
		}
	}
}
