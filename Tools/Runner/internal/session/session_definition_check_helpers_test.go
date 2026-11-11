package session_test

// Rig and assertions for the run-start check of agent definition frontmatter on
// the CLI harnesses: every agent a run can dispatch must declare a model, and on
// Claude Code a name equal to its identifier.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// cliLayout is a CLI harness and the directory it deploys agents to.
type cliLayout struct {
	harnessID string
	agentsRel string
}

var cliLayouts = []cliLayout{
	{"claude-code", ".claude/agents"},
	{"opencode", ".opencode/agents"},
	{"ghcp-cli", ".github/agents"},
}

const (
	// multiInfraFixture declares two checkpoint-class and two commit-class
	// infrastructure agents next to the linear workflow.
	multiInfraFixture = "multi-two-gated-classes-orch.md"

	selectedCheckpoint   = "checkpoint-manager-git"
	unselectedCheckpoint = "checkpoint-manager-alt"
	selectedCommit       = "commit-manager-git"
	unselectedCommit     = "commit-manager-alt"
)

var multiInfraAgents = []string{selectedCheckpoint, unselectedCheckpoint, selectedCommit, unselectedCommit}

// defRig is a deployed CLI-harness agents directory whose every definition is valid.
type defRig struct {
	harnessID string
	workDir   string
	agentsDir string
	orchPath  string
	fixture   string
	infra     []string
}

// newDefRig deploys the orchestrator fixture, the two workflow agents and the
// named infrastructure agents under the layout's agents directory.
func newDefRig(t *testing.T, l cliLayout, fixture string, infra []string) *defRig {
	t.Helper()
	workDir := t.TempDir()
	agentsDir := filepath.Join(workDir, filepath.FromSlash(l.agentsRel))
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("newDefRig: create agents dir: %v", err)
	}
	r := &defRig{harnessID: l.harnessID, workDir: workDir, agentsDir: agentsDir, fixture: fixture, infra: infra}
	r.orchPath = writeOrchestratorDefinition(t, agentsDir, l.harnessID, fixture)
	for _, id := range append([]string{"agent-a", "agent-b"}, infra...) {
		writeDefinitionFile(t, agentsDir, l.harnessID, id)
	}
	return r
}

// path is the deployed definition file of agent id.
func (r *defRig) path(id string) string {
	return filepath.Join(r.agentsDir, id+".md")
}

// mutate rewrites the definition of agent id (or of the orchestrator) with the
// given changes applied to an otherwise valid frontmatter.
func (r *defRig) mutate(t *testing.T, id string, muts ...defMut) {
	t.Helper()
	if id == "orchestrator" {
		writeOrchestratorDefinition(t, r.agentsDir, r.harnessID, r.fixture, muts...)
		return
	}
	writeDefinitionFile(t, r.agentsDir, r.harnessID, id, muts...)
}

// config is the request for a new auto-mode run of the linear workflow.
func (r *defRig) config() domain.RunConfig {
	cfg := baseCLIHarnessConfig(r.orchPath, "def-check-run")
	cfg.HarnessID = r.harnessID
	if len(r.infra) > 0 {
		cfg.InfraClassSelections = map[string]string{"checkpoint": selectedCheckpoint, "commit": selectedCommit}
	}
	return cfg
}

// startResult is what a run start left behind.
type startResult struct {
	out   domain.RunOutcome
	err   error
	calls *harness.MockAdapter
	store *memStore
}

// start runs a new run with cfg on a recording adapter and an in-memory store.
func (r *defRig) start(t *testing.T, cfg domain.RunConfig) startResult {
	t.Helper()
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	store := &memStore{}
	ses := session.New(session.Deps{Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})
	out, err := ses.Start(context.Background(), cfg)
	return startResult{out: out, err: err, calls: f, store: store}
}

// requireDefinitionRefusal asserts a new run was refused for the agent's
// definition: the refusal names the agent and its file, carries the sentinel,
// no artifact was created and no agent was invoked.
func requireDefinitionRefusal(t *testing.T, res startResult, id, path string, sentinel error) {
	t.Helper()
	requireRefusedBy(t, res.out, res.err, id, path, sentinel)
	if len(res.calls.Invocations()) != 0 {
		t.Errorf("want no agent invoked after a refusal, got %d invocation(s)", len(res.calls.Invocations()))
	}
	if res.store.exists {
		t.Error("want no artifact created for a refused new run")
	}
}

// requireRefusedBy asserts the outcome is a refusal naming the agent and its
// definition file whose cause is the sentinel.
func requireRefusedBy(t *testing.T, out domain.RunOutcome, err error, id, path string, sentinel error) {
	t.Helper()
	msg := requireRefused(t, out, err)
	if !strings.Contains(msg, id) {
		t.Errorf("refusal %q does not name the agent %q", msg, id)
	}
	if !strings.Contains(msg, path) {
		t.Errorf("refusal %q does not name the definition file %q", msg, path)
	}
	var refErr *domain.RefusalError
	if !errors.As(out.Cause, &refErr) {
		t.Errorf("want the refusal's cause to be a *domain.RefusalError, got %T: %v", out.Cause, out.Cause)
	}
	if !errors.Is(out.Cause, sentinel) {
		t.Errorf("want the refusal's cause to match %v, got %v", sentinel, out.Cause)
	}
}

// requireStarted asserts the run started and completed.
func requireStarted(t *testing.T, res startResult) {
	t.Helper()
	requireRunStatus(t, res.out, res.err, domain.RunCompleted)
	if len(res.calls.Invocations()) == 0 {
		t.Error("want agents invoked by a started run, got none")
	}
}
