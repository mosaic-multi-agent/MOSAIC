package session_test

// A complete workspace whose agent files are in a format the Runner has no code
// for (the test-only JSON format, given to a harness without a catalog entry
// through the test-only format seam) gets through run start: agent resolution,
// the definition check and the reads of the orchestrator script's workflow
// regions all go through the shared loader, and the run dispatches its first
// invocation to the mock adapter.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentformat/testjson"
	"mosaic-common/agentload"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const (
	jsonWorkspaceAgentsRel = ".jsonfmt/agents"
	jsonWorkspaceExtension = ".json"
)

// jsonWorkspace is a deployed agents directory whose files are all test-only
// JSON agent files.
type jsonWorkspace struct {
	harnessID string
	agentsDir string
	orchPath  string
}

// jsonAgentText renders a test-only JSON agent file declaring name and model
// with the given instructions.
func jsonAgentText(t *testing.T, name, instructions string) []byte {
	t.Helper()
	body, err := json.Marshal(instructions)
	if err != nil {
		t.Fatalf("marshal instructions: %v", err)
	}
	return []byte(`{"mosaic_test_agent":1,"instructions":` + string(body) +
		`,"meta":[["name","` + name + `"],["description","test definition"],["model","` + testDefinitionModel + `"]]}`)
}

// newJSONWorkspace deploys the orchestrator fixture and the named agents under
// a harness that has no catalog entry, all as test-only JSON files.
func newJSONWorkspace(t *testing.T, harnessID, fixture string, agents ...string) *jsonWorkspace {
	t.Helper()
	testjson.Register()
	unregister := agentfile.RegisterTestOnly(harnessID, agentfile.Format{
		AgentsDir: jsonWorkspaceAgentsRel,
		FormatID:  string(testjson.ID),
		Extension: jsonWorkspaceExtension,
	})
	t.Cleanup(unregister)

	agentsDir := filepath.Join(t.TempDir(), filepath.FromSlash(jsonWorkspaceAgentsRel))
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("newJSONWorkspace: create agents dir: %v", err)
	}
	script, err := os.ReadFile(orchFilePath(fixture))
	if err != nil {
		t.Fatalf("newJSONWorkspace: read %s: %v", fixture, err)
	}
	ws := &jsonWorkspace{
		harnessID: harnessID,
		agentsDir: agentsDir,
		orchPath:  filepath.Join(agentsDir, "orchestrator-script"+jsonWorkspaceExtension),
	}
	// The deployed script body is carried without the fixture's frontmatter,
	// as a deployer would write it.
	ws.write(t, ws.orchPath, jsonAgentText(t, "orchestrator-script", scriptBody(string(script))))
	for _, id := range agents {
		ws.write(t, filepath.Join(agentsDir, id+jsonWorkspaceExtension), jsonAgentText(t, id, "# Agent: "+id+"\n"))
	}
	return ws
}

// scriptBody returns the text of a script fixture after its frontmatter block
// (the whole text when it has none).
func scriptBody(text string) string {
	const fence = "---\n"
	if !strings.HasPrefix(text, fence) {
		return text
	}
	rest := text[len(fence):]
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		return text
	}
	return rest[end+len("\n"+fence):]
}

func (ws *jsonWorkspace) write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// start runs a new auto-mode run of the linear workflow on a recording adapter.
func (ws *jsonWorkspace) start(t *testing.T) startResult {
	t.Helper()
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	store := &memStore{}
	ses := session.New(session.Deps{Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})
	cfg := baseCLIHarnessConfig(ws.orchPath, "json-workspace-run")
	cfg.HarnessID = ws.harnessID
	out, err := ses.Start(context.Background(), cfg)
	return startResult{out: out, err: err, calls: f, store: store}
}

func TestSession_Start_NonMarkdownWorkspace_ReachesFirstDispatchOnMockAdapter(t *testing.T) {
	ws := newJSONWorkspace(t, "json-run-start", "linear-orch.md", "agent-a", "agent-b")

	res := ws.start(t)

	requireStarted(t, res)
	if len(res.calls.Invocations()) == 0 {
		t.FailNow()
	}
	first := res.calls.Invocations()[0]
	if first.Agent.Identifier != "agent-a" {
		t.Errorf("first dispatched agent: want agent-a, got %q", first.Agent.Identifier)
	}
	if want := filepath.Join(ws.agentsDir, "agent-a"+jsonWorkspaceExtension); first.Agent.DefinitionPath != want {
		t.Errorf("first dispatched agent definition: want %q, got %q", want, first.Agent.DefinitionPath)
	}
}

func TestSession_Start_NonMarkdownWorkspace_DispatchesEveryWorkflowAgent(t *testing.T) {
	ws := newJSONWorkspace(t, "json-run-all", "linear-orch.md", "agent-a", "agent-b")

	res := ws.start(t)

	requireRunStatus(t, res.out, res.err, domain.RunCompleted)
	var got []string
	for _, inv := range res.calls.Invocations() {
		got = append(got, inv.Agent.Identifier)
	}
	if len(got) != 2 || got[0] != "agent-a" || got[1] != "agent-b" {
		t.Errorf("dispatched agents: want [agent-a agent-b], got %v", got)
	}
}

func TestSession_Start_NonMarkdownWorkspace_UndecodableOrchestratorScript_RefusesBeforeAnyInvocation(t *testing.T) {
	ws := newJSONWorkspace(t, "json-run-badorch", "linear-orch.md", "agent-a", "agent-b")
	ws.write(t, ws.orchPath, []byte(`{"not":"an agent file"}`))

	res := ws.start(t)

	msg := requireRefused(t, res.out, res.err)
	if !strings.Contains(msg, agentload.ErrUndecodable.Error()) {
		t.Errorf("refusal %q does not name the load failure %q", msg, agentload.ErrUndecodable.Error())
	}
	if len(res.calls.Invocations()) != 0 {
		t.Errorf("want no agent invoked after a refusal, got %d invocation(s)", len(res.calls.Invocations()))
	}
	if res.store.exists {
		t.Error("want no artifact created for a refused new run")
	}
}
