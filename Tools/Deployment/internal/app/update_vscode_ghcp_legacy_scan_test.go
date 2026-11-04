package app

// update_vscode_ghcp_legacy_scan_test.go verifies that the deployed-agent scan Update and
// workflow Update rely on, run over the vscode-ghcp agents directory, still finds agents
// deployed as legacy "x.md" as well as "x.agent.md".

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

func vscodeGhcpAgentsDir(t *testing.T) string {
	t.Helper()
	d, err := descriptor.Load(filepath.Join("..", "harness", "builtin", "vscodeghcp", "vscode-ghcp.yaml"))
	if err != nil {
		t.Fatalf("load vscode-ghcp descriptor: %v", err)
	}
	if d.Paths.Agents.Project == "" {
		t.Fatal("vscode-ghcp declares no project agents directory")
	}
	return filepath.FromSlash(d.Paths.Agents.Project)
}

func TestScanWorkspaceAgents_VSCodeGhcpDir_FindsLegacyMdAndAgentMdAgents(t *testing.T) {
	workspace := t.TempDir()
	agentsDir := vscodeGhcpAgentsDir(t)
	dir := filepath.Join(workspace, agentsDir)
	writeScanFileNoID(t, dir, "legacy-agent.md")
	writeScanFileNoID(t, dir, "new-agent.agent.md")

	cat := newScanCatalog()
	cat.byKey["legacy-agent"] = domain.Agent{Key: "legacy-agent", Role: domain.RoleSubagent}
	cat.byKey["new-agent"] = domain.Agent{Key: "new-agent", Role: domain.RoleSubagent}

	scan := scanWorkspaceAgents(workspace, agentsDir, cat)

	keys := scan.MatchedKeys()
	if len(keys) != 2 || keys[0] != "legacy-agent" || keys[1] != "new-agent" {
		t.Fatalf("matched keys: want [legacy-agent new-agent], got %v", keys)
	}
	byName := map[string]string{}
	for _, m := range scan.Matched {
		byName[m.FileName] = m.TargetPath
	}
	for _, name := range []string{"legacy-agent.md", "new-agent.agent.md"} {
		if want := filepath.Join(agentsDir, name); byName[name] != want {
			t.Errorf("target path for %s: want %q, got %q", name, want, byName[name])
		}
	}
}
