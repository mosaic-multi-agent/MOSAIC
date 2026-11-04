package app_test

// update_missing_workspace_test.go verifies the workspace-path check of the update and
// workflows flows: both document the path as "the existing workspace", so a path that does not
// exist (or is not a directory) is refused before anything is written, while deploy keeps
// creating a new folder and an existing folder behaves as before.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
)

// missingWorkspaceDeps returns deps whose executor records whether it ran, plus a workspace
// path inside a temp directory that does not exist.
func missingWorkspaceDeps(t *testing.T) (app.Deps, *crSpyExecutor, string) {
	t.Helper()
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	exec := &crSpyExecutor{}
	deps.Executor = exec
	return deps, exec, filepath.Join(t.TempDir(), "no-such-workspace")
}

func requireNothingCreated(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("path %q must not exist after the refused run (stat err = %v)", path, err)
	}
}

func TestUpdate_MissingWorkspace_FailsWithWorkspaceNotFoundAndWritesNothing(t *testing.T) {
	deps, exec, missing := missingWorkspaceDeps(t)

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: missing, AutoConfirmPlan: true})

	if !errors.Is(err, app.ErrWorkspaceNotFound) {
		t.Fatalf("error = %v, want errors.Is(err, app.ErrWorkspaceNotFound)", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q must name the missing path %q", err.Error(), missing)
	}
	if exec.called {
		t.Error("the executor ran; a refused run must not reach it")
	}
	requireNothingCreated(t, missing)
}

func TestUpdateWorkflows_MissingWorkspace_FailsWithWorkspaceNotFoundAndWritesNothing(t *testing.T) {
	deps, exec, missing := missingWorkspaceDeps(t)

	_, err := app.New(deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: missing, WorkflowIDs: []string{}, AutoConfirmPlan: true})

	if !errors.Is(err, app.ErrWorkspaceNotFound) {
		t.Fatalf("error = %v, want errors.Is(err, app.ErrWorkspaceNotFound)", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q must name the missing path %q", err.Error(), missing)
	}
	if exec.called {
		t.Error("the executor ran; a refused run must not reach it")
	}
	requireNothingCreated(t, missing)
}

func TestUpdate_WorkspacePathIsAFile_FailsWithWorkspaceNotFoundNotADirectory(t *testing.T) {
	deps, exec, _ := missingWorkspaceDeps(t)
	file := writeTempFile(t, t.TempDir(), "plain-file", []byte("x"))

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: file, AutoConfirmPlan: true})

	if !errors.Is(err, app.ErrWorkspaceNotFound) {
		t.Fatalf("error = %v, want errors.Is(err, app.ErrWorkspaceNotFound)", err)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error %q must say the path is not a directory", err.Error())
	}
	if exec.called {
		t.Error("the executor ran; a refused run must not reach it")
	}
}

func TestUpdateWorkflows_WorkspacePathIsAFile_FailsWithWorkspaceNotFoundNotADirectory(t *testing.T) {
	deps, _, _ := missingWorkspaceDeps(t)
	file := writeTempFile(t, t.TempDir(), "plain-file", []byte("x"))

	_, err := app.New(deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: file, WorkflowIDs: []string{}, AutoConfirmPlan: true})

	if !errors.Is(err, app.ErrWorkspaceNotFound) {
		t.Fatalf("error = %v, want errors.Is(err, app.ErrWorkspaceNotFound)", err)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error %q must say the path is not a directory", err.Error())
	}
}

func TestUpdate_ExistingEmptyWorkspace_IsNotRefused(t *testing.T) {
	deps, exec, _ := missingWorkspaceDeps(t)
	empty := t.TempDir()

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: empty, AutoConfirmPlan: true})

	if errors.Is(err, app.ErrWorkspaceNotFound) {
		t.Fatalf("an existing empty folder was refused: %v", err)
	}
	if !exec.called {
		t.Errorf("the executor did not run for an existing empty workspace (err = %v)", err)
	}
}

func TestDeployNew_MissingWorkspace_IsNotRefused(t *testing.T) {
	deps, exec, missing := missingWorkspaceDeps(t)

	_, err := app.New(deps).DeployNew(context.Background(), newDeployRequest(missing, "stub-harness", nil))

	if errors.Is(err, app.ErrWorkspaceNotFound) {
		t.Fatalf("deploy must keep creating a new folder, got: %v", err)
	}
	if !exec.called {
		t.Errorf("the executor did not run for a deploy to a new path (err = %v)", err)
	}
}
