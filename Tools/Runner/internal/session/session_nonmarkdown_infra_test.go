package session_test

// Infrastructure agents declared by a non-Markdown orchestrator script are read
// through the shared loader at run start: the definition check covers the
// definition files of the declared agents, so a declared agent whose file
// cannot be decoded refuses the run, and one with a valid file lets it start.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentload"
)

const declaredInfraAgent = "checkpoint-manager-git"

func TestSession_Start_NonMarkdownWorkspace_DeclaredInfraAgentWithDefinition_Starts(t *testing.T) {
	ws := newJSONWorkspace(t, "json-infra-ok", "checkpoint-agent-orch.md", "agent-a", "agent-b", declaredInfraAgent)

	res := ws.start(t)

	requireStarted(t, res)
}

func TestSession_Start_NonMarkdownWorkspace_DeclaredInfraAgentUndecodable_RefusedNamingTheAgentFile(t *testing.T) {
	ws := newJSONWorkspace(t, "json-infra-bad", "checkpoint-agent-orch.md", "agent-a", "agent-b", declaredInfraAgent)
	infraPath := filepath.Join(ws.agentsDir, declaredInfraAgent+jsonWorkspaceExtension)
	ws.write(t, infraPath, []byte(`{"not":"an agent file"}`))

	res := ws.start(t)

	msg := requireRefused(t, res.out, res.err)
	if !strings.Contains(msg, declaredInfraAgent) {
		t.Errorf("refusal %q does not name the infrastructure agent %q", msg, declaredInfraAgent)
	}
	if !strings.Contains(msg, agentload.ErrUndecodable.Error()) {
		t.Errorf("refusal %q does not name the load failure %q", msg, agentload.ErrUndecodable.Error())
	}
	if len(res.calls.Invocations()) != 0 {
		t.Errorf("want no agent invoked after a refusal, got %d invocation(s)", len(res.calls.Invocations()))
	}
}
