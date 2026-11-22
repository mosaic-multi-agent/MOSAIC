package opencodev2_test

// Tests that the permissions the OpenCode V2 module renders let a deployed workspace run
// natively without hand-fixing: a subagent is granted exactly its declared tools, and the
// orchestrator's resolved rules allow dispatching every deployed agent, asking the user, and
// writing the files a native orchestration run needs, while everything else stays denied.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

const orchestratorPlaceholder = "{tool-permissions}"

func TestTools_SubagentIsGrantedExactlyItsDeclaredTools(t *testing.T) {
	mod := newModule(t)

	rules := rulesFor(t, mod, domain.ToolRequest{AgentKey: "worker", Generic: []string{"file_read", "file_search", "content_search"}})

	for action, want := range map[string]string{
		"read": "allow", "glob": "allow", "grep": "allow",
		"edit": "deny", "shell": "deny", "subagent": "deny", "question": "deny", "skill": "deny", "webfetch": "deny",
		"some_future_action": "deny",
	} {
		if got := effectFor(rules, action, "src/file.go"); got != want {
			t.Errorf("subagent rule for %q = %q, want %q", action, got, want)
		}
	}
}

func TestTools_OrchestratorMayDispatchEveryDeployedAgent(t *testing.T) {
	mod := newModule(t)
	rules := rulesFor(t, mod, domain.ToolRequest{AgentKey: "orchestrator", Placeholder: orchestratorPlaceholder})

	for _, agentID := range []string{"test-runner", "planner-tdd-soft", "any-future-agent", "a1b2"} {
		if got := effectFor(rules, "subagent", agentID); got != "allow" {
			t.Errorf("dispatching %q resolves to %q, want allow", agentID, got)
		}
	}
}

func TestTools_OrchestratorMayAskTheUserAndWriteTheFilesARunNeeds(t *testing.T) {
	mod := newModule(t)
	rules := rulesFor(t, mod, domain.ToolRequest{AgentKey: "orchestrator", Placeholder: orchestratorPlaceholder})

	for _, action := range []string{"question", "read", "edit", "glob", "grep", "shell"} {
		for _, resource := range []string{"Orchestration-run/Progress.md", "src/main.go"} {
			if got := effectFor(rules, action, resource); got != "allow" {
				t.Errorf("%s on %q resolves to %q, want allow", action, resource, got)
			}
		}
	}
}

func TestTools_OrchestratorStillDeniesWhatTheNativeRunDoesNotNeed(t *testing.T) {
	mod := newModule(t)
	rules := rulesFor(t, mod, domain.ToolRequest{AgentKey: "orchestrator", Placeholder: orchestratorPlaceholder})

	for _, action := range []string{"execute", "websearch", "webfetch", "skill", "some_future_action"} {
		if got := effectFor(rules, action, "x"); got != "deny" {
			t.Errorf("%s resolves to %q for the orchestrator, want deny", action, got)
		}
	}
	if got := effectFor(rules, "external_directory", "/elsewhere"); got != "ask" {
		t.Errorf("access outside the workspace resolves to %q, want ask", got)
	}
}

func TestTools_NoRuleCarriesAnAbsolutePath(t *testing.T) {
	mod := newModule(t)

	for _, req := range []domain.ToolRequest{
		{AgentKey: "orchestrator", Placeholder: orchestratorPlaceholder},
		{AgentKey: "worker", Generic: []string{"file_read", "file_write", "terminal"}},
	} {
		for _, r := range rulesFor(t, mod, req) {
			if len(r.resource) > 0 && (r.resource[0] == '/' || (len(r.resource) > 1 && r.resource[1] == ':')) {
				t.Errorf("rule %+v carries an absolute path resource", r)
			}
		}
	}
}
