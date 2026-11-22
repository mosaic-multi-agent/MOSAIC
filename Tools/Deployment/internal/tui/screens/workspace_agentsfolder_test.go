package screens_test

// workspace_agentsfolder_test.go verifies the agents-folder kind of the path screen used by
// batch promote: an existing, readable directory is accepted (write permission is not needed
// because source files are never written), a file or a missing path is rejected, and the
// wording names an agents folder rather than a workspace.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mosaic-deploy/internal/tui/screens"
)

func newAgentsFolderScreen(path string) *screens.WorkspaceScreen {
	s := screens.NewWorkspaceScreen(80, 24, plainStyles())
	s.SetPathKind(screens.PathKindAgentsFolder)
	s.SetPrefilledPath(path)
	return s
}

func TestWorkspaceScreen_AgentsFolder_AcceptsReadableDirectory(t *testing.T) {
	dir := t.TempDir()
	s := newAgentsFolderScreen(dir)

	s.Update(wsEnterKey())

	if !s.Done() {
		t.Errorf("Done() = false for the existing directory %q; want true", dir)
	}
	if got := s.WorkspacePath(); !filepath.IsAbs(got) {
		t.Errorf("WorkspacePath() = %q; want an absolute path", got)
	}
}

func TestWorkspaceScreen_AgentsFolder_RejectsFileWithMessage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(file, []byte("# agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newAgentsFolderScreen(file)
	before := s.View()

	s.Update(wsEnterKey())

	if s.Done() {
		t.Errorf("Done() = true for the file %q; an agents folder must be a directory", file)
	}
	if s.View() == before {
		t.Errorf("no message shown after a file was entered:\n%s", s.View())
	}
}

func TestWorkspaceScreen_AgentsFolder_RejectsMissingAndEmptyPath(t *testing.T) {
	for name, path := range map[string]string{
		"missing": filepath.Join(t.TempDir(), "no-such-folder"),
		"empty":   "",
	} {
		s := newAgentsFolderScreen(path)

		s.Update(wsEnterKey())

		if s.Done() {
			t.Errorf("%s path: Done() = true; want the path rejected", name)
		}
	}
}

// TestWorkspaceScreen_AgentsFolder_ReadOnlyDirectoryIsAccepted: the promote never writes into
// the agents folder, so a directory without write permission is a valid agents folder.
func TestWorkspaceScreen_AgentsFolder_ReadOnlyDirectoryIsAccepted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	s := newAgentsFolderScreen(dir)

	s.Update(wsEnterKey())

	if !s.Done() {
		t.Errorf("Done() = false for the read-only directory %q; write permission must not be required", dir)
	}
}

// TestWorkspaceScreen_AgentsFolder_AcceptsADirectoryTheWriteProbeCannotUse is the
// platform-independent check that no write probe runs for an agents folder. A directory
// named like the workspace write probe occupies the probe's name, so the exclusive create of
// that probe fails on every OS; a validation that probes for write access rejects the
// folder, one that only needs a readable directory accepts it.
func TestWorkspaceScreen_AgentsFolder_AcceptsADirectoryTheWriteProbeCannotUse(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".mosaic-write-probe"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newAgentsFolderScreen(dir)

	s.Update(wsEnterKey())

	if !s.Done() {
		t.Errorf("Done() = false for %q; the agents folder must not be probed for write access", dir)
	}
}

func TestWorkspaceScreen_AgentsFolder_WordingNamesAnAgentsFolderNotAWorkspace(t *testing.T) {
	s := newAgentsFolderScreen("")

	lower := strings.ToLower(collapseWhitespace(s.View()))

	if !strings.Contains(lower, "agents folder") {
		t.Errorf("view does not name an agents folder:\n%s", s.View())
	}
	if strings.Contains(lower, "workspace") {
		t.Errorf("view uses workspace wording for the agents folder:\n%s", s.View())
	}
	if !strings.Contains(lower, "not changed") && !strings.Contains(lower, "never changed") &&
		!strings.Contains(lower, "not modified") && !strings.Contains(lower, "never modified") &&
		!strings.Contains(lower, "never written") && !strings.Contains(lower, "not written") {
		t.Errorf("view does not state that the files of the folder are not changed:\n%s", s.View())
	}
}
