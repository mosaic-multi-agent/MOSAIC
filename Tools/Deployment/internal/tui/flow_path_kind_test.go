package tui

// flow_path_kind_test.go verifies that the modes which take a workspace directory (deploy,
// update, change workflows) keep directory-kind validation and wording on the path screen.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
)

// TestDeployMode_PathScreen_AcceptsWorkspaceDirectory verifies that entering an existing
// writable workspace directory in deploy-new mode advances to screenRunning. This is a
// regression test confirming that adding file-kind validation to promote mode does not
// accidentally change the path kind for deploy-new.
func TestDeployMode_PathScreen_AcceptsWorkspaceDirectory(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)
	driveToPath(t, m, domain.ModeDeployWorkspace, "")

	// Act: Enter on the workspace directory.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Assert: directory accepted.
	if m.screen != screenRunning {
		t.Errorf("screen = %v after Enter on directory in deploy-new mode; want screenRunning; "+
			"deploy-new must keep directory-kind validation after the promote path-kind fix", m.screen)
	}
}

// TestUpdateMode_PathScreen_AcceptsWorkspaceDirectory verifies that entering an existing
// writable workspace directory in update mode advances to screenRunning. This is a regression
// test ensuring the path-kind fix for promote mode does not affect update mode.
func TestUpdateMode_PathScreen_AcceptsWorkspaceDirectory(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)
	driveToPath(t, m, domain.ModeUpdateWorkspace, "")

	// Act: Enter on the workspace directory, then on the harness screen that follows it.
	confirmPath(t, m, "")

	// Assert: directory accepted.
	if m.screen != screenRunning {
		t.Errorf("screen = %v after Enter on directory in update mode; want screenRunning; "+
			"update must keep directory-kind validation after the promote path-kind fix", m.screen)
	}
}

// TestWorkflowsOnlyMode_PathScreen_AcceptsWorkspaceDirectory verifies that entering an
// existing writable workspace directory in workflows-only mode advances to screenRunning. This
// is a regression test ensuring the path-kind fix for promote mode does not affect
// workflows-only mode.
func TestWorkflowsOnlyMode_PathScreen_AcceptsWorkspaceDirectory(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	svc := newFlowSvc(workspace)
	svc.workflows = domain.RunSummary{
		Mode:           domain.ModeUpdateWorkflows,
		WorkspacePath:  workspace,
		DeploymentRoot: workspace + "/.ai",
		Outcome:        domain.OutcomeSuccess,
	}
	m := newFlowModel(svc, workspace)
	driveToPath(t, m, domain.ModeUpdateWorkflows, "")

	// Act: Enter on the workspace directory, then on the harness screen that follows it.
	confirmPath(t, m, "")

	// Assert: directory accepted.
	if m.screen != screenRunning {
		t.Errorf("screen = %v after Enter on directory in workflows-only mode; want screenRunning; "+
			"workflows-only must keep directory-kind validation after the promote path-kind fix", m.screen)
	}
}

// TestDeployMode_PathScreen_View_ContainsDirectoryWording verifies that the path screen in
// deploy-new mode shows directory-oriented wording. This is a regression test guarding against
// the path kind for deploy-new being accidentally changed to file kind, which would change the
// wording even if the validator were somehow still accepting directories.
func TestDeployMode_PathScreen_View_ContainsDirectoryWording(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)
	driveToPath(t, m, domain.ModeDeployWorkspace, "")

	// Act
	lower := strings.ToLower(m.View())

	// Assert: directory-oriented wording must be present; file-only wording must be absent.
	if !strings.Contains(lower, "directory") && !strings.Contains(lower, "workspace") {
		t.Errorf("path screen view in deploy-new mode contains neither 'directory' nor 'workspace' wording; "+
			"want directory-oriented wording:\n%s", m.View())
	}
	if strings.Contains(lower, "agent file") {
		t.Errorf("path screen view in deploy-new mode contains 'agent file' wording; "+
			"want directory-oriented wording only:\n%s", m.View())
	}
}
