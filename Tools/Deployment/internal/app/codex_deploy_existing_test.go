package app_test

// codex_deploy_existing_test.go covers Deploy workspace and Deploy agents over an agent
// file that already exists in a Codex workspace.
//
// Both flows must succeed when the planned action is an update (the catalog source version
// was bumped) or a conflict resolved by overwrite, and the rewritten file must keep every
// TOML key the user added to the existing file (user-owned keys), while MOSAIC-owned
// content (version stamp, instructions) is refreshed from the catalog source.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

const (
	existingUserKey   = "user_key"
	existingUserValue = "keep-me"
)

// codexCapturingExecutor is a deploy.Executor that builds the content of every agent item
// that would be written (create, update, conflict resolved by overwrite) and records the
// bytes by target path. A Content error is returned as the Execute error.
type codexCapturingExecutor struct {
	written map[string][]byte
}

func (e *codexCapturingExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.written = map[string][]byte{}
	var actions []domain.ActionRecord
	for _, item := range req.Plan.Items {
		write := false
		switch item.Action {
		case domain.ActionCreate, domain.ActionUpdate:
			write = true
		case domain.ActionConflict:
			write = req.Conflicts[item.TargetPath] == domain.DecisionOverwrite
		}
		if write {
			content, err := req.Content(item)
			if err != nil {
				return deploy.ExecResult{}, fmt.Errorf("Content(%q): %w", item.Ref.Key, err)
			}
			e.written[item.TargetPath] = content
		}
		actions = append(actions, domain.ActionRecord{
			Ref:        item.Ref,
			TargetPath: filepath.Join(req.Plan.WorkspacePath, item.TargetPath),
			Taken:      domain.TakenCreated,
		})
	}
	return deploy.ExecResult{DeploymentRoot: req.Plan.WorkspacePath, Actions: actions}, nil
}

// seedExistingCodexAgent writes a deployed Codex agent file with an older version stamp and
// a user-owned key to the workspace, at the path the plan items point to.
func seedExistingCodexAgent(t *testing.T, workspace string) {
	t.Helper()
	dir := filepath.Join(workspace, ".codex", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "# mosaic_version: 0.9\n\n" +
		"sandbox_mode = \"read-only\"\n" +
		existingUserKey + " = \"" + existingUserValue + "\"\n" +
		"developer_instructions = \"You are the old Codex Op test agent.\\n\"\n"
	if err := os.WriteFile(filepath.Join(dir, codexOpAgent.Key+".toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("seed existing codex agent: %v", err)
	}
}

// assertRewrittenKeepsUserKey checks that the single written agent file is valid TOML,
// still carries the user-owned key with its value, and carries the refreshed instructions.
func assertRewrittenKeepsUserKey(t *testing.T, exec *codexCapturingExecutor) {
	t.Helper()
	path := ".codex/agents/" + codexOpAgent.Key + ".toml"
	out, ok := exec.written[path]
	if !ok {
		t.Fatalf("no content was built for %s; written paths = %v", path, len(exec.written))
	}
	var doc map[string]any
	if err := toml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("rewritten file is not valid TOML: %v\n%s", err, out)
	}
	if got := doc[existingUserKey]; got != existingUserValue {
		t.Errorf("user-owned key %q = %v, want %q preserved from the existing file\n%s",
			existingUserKey, got, existingUserValue, out)
	}
	instr, _ := doc["developer_instructions"].(string)
	if !strings.Contains(instr, "You are the Codex Op test agent.") || strings.Contains(instr, "old Codex") {
		t.Errorf("developer_instructions = %q, want the instructions refreshed from the catalog source", instr)
	}
}

// TestCodexDeployNew_ExistingFile_VersionBumpUpdate_PreservesUserKey verifies that Deploy
// workspace over an existing Codex agent file planned as an update succeeds and keeps the
// user-owned TOML key.
func TestCodexDeployNew_ExistingFile_VersionBumpUpdate_PreservesUserKey(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	seedExistingCodexAgent(t, workspace)
	exec := &codexCapturingExecutor{}
	deps.Planner = &stubPlanner{plan: newCodexOpAgentPlan(workspace, domain.ActionUpdate)}
	deps.Executor = exec

	// Act
	_, err := app.New(deps).DeployNew(context.Background(), codexOpDeployRequest(workspace))

	// Assert
	if err != nil {
		t.Fatalf("DeployNew over an existing Codex agent (update) failed: %v", err)
	}
	assertRewrittenKeepsUserKey(t, exec)
}

// TestCodexDeployNew_ExistingFile_ConflictOverwrite_PreservesUserKey verifies that Deploy
// workspace over an existing Codex agent file planned as a conflict, resolved by overwrite,
// succeeds and keeps the user-owned TOML key.
func TestCodexDeployNew_ExistingFile_ConflictOverwrite_PreservesUserKey(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	seedExistingCodexAgent(t, workspace)
	exec := &codexCapturingExecutor{}
	deps.Planner = &stubPlanner{plan: newCodexOpAgentPlan(workspace, domain.ActionConflict)}
	deps.Executor = exec
	req := codexOpDeployRequest(workspace)
	req.ConflictDefault = domain.DecisionOverwrite

	// Act
	_, err := app.New(deps).DeployNew(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("DeployNew over an existing Codex agent (conflict overwrite) failed: %v", err)
	}
	assertRewrittenKeepsUserKey(t, exec)
}

// TestCodexDeployAgents_ExistingFile_VersionBumpUpdate_PreservesUserKey verifies that
// Deploy agents over an existing Codex agent file planned as an update succeeds and keeps
// the user-owned TOML key.
func TestCodexDeployAgents_ExistingFile_VersionBumpUpdate_PreservesUserKey(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	seedExistingCodexAgent(t, workspace)
	exec := &codexCapturingExecutor{}
	deps.Planner = &stubPlanner{plan: newCodexOpAgentDeployAgentsPlan(workspace, domain.ActionUpdate)}
	deps.Executor = exec

	// Act
	_, err := app.New(deps).DeployAgents(context.Background(), codexOpDeployAgentsRequest(workspace))

	// Assert
	if err != nil {
		t.Fatalf("DeployAgents over an existing Codex agent (update) failed: %v", err)
	}
	assertRewrittenKeepsUserKey(t, exec)
}

// TestCodexDeployAgents_ExistingFile_ConflictOverwrite_PreservesUserKey verifies that
// Deploy agents over an existing Codex agent file planned as a conflict, resolved by
// overwrite, succeeds and keeps the user-owned TOML key.
func TestCodexDeployAgents_ExistingFile_ConflictOverwrite_PreservesUserKey(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	seedExistingCodexAgent(t, workspace)
	exec := &codexCapturingExecutor{}
	deps.Planner = &stubPlanner{plan: newCodexOpAgentDeployAgentsPlan(workspace, domain.ActionConflict)}
	deps.Executor = exec
	req := codexOpDeployAgentsRequest(workspace)
	req.ConflictDefault = domain.DecisionOverwrite

	// Act
	_, err := app.New(deps).DeployAgents(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("DeployAgents over an existing Codex agent (conflict overwrite) failed: %v", err)
	}
	assertRewrittenKeepsUserKey(t, exec)
}
