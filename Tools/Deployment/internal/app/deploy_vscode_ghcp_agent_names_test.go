package app_test

// deploy_vscode_ghcp_agent_names_test.go pins what the deploy flows (Deploy, Update and
// workflow Update all enumerate target paths through plan.EnumerateTargetPaths) write for
// GHCP agents: the harness's agent naming rule applies, so a catalog source "x.md" is
// deployed as "x.agent.md" on both GHCP targets, while other harnesses keep the plain name.

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

func plannedAgentPath(t *testing.T, harnessID, sourcePath string) string {
	t.Helper()
	agent := domain.Agent{Key: "code-reviewer", SourcePath: sourcePath}
	paths, err := plan.EnumerateTargetPaths(
		plan.ArtifactSet{Agents: []domain.Agent{agent}},
		rbModule(t, harnessID), domain.ScopeProject, "linux")
	if err != nil {
		t.Fatalf("EnumerateTargetPaths(%s): %v", harnessID, err)
	}
	got, ok := paths.Path(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "code-reviewer"})
	if !ok {
		t.Fatalf("no planned path for the agent on %s", harnessID)
	}
	return got
}

func TestDeployPlanning_GhcpAgentsUseTheAgentDotMdName(t *testing.T) {
	src := filepath.Join("catalog", "agents", "code-reviewer.md")
	for _, id := range []string{hVSCode, hCLI} {
		if got, want := plannedAgentPath(t, id, src), ".github/agents/code-reviewer.agent.md"; got != want {
			t.Errorf("deploy target for %s agent: want %q, got %q", id, want, got)
		}
	}
}

func TestDeployPlanning_VSCodeGhcpAgentPathMatchesGhcpCli(t *testing.T) {
	src := filepath.Join("catalog", "agents", "code-reviewer.md")

	vscode := plannedAgentPath(t, hVSCode, src)
	cli := plannedAgentPath(t, hCLI, src)

	if vscode != cli {
		t.Errorf("deploy flows must name the same file on both GHCP targets: vscode-ghcp %q, ghcp-cli %q", vscode, cli)
	}
}

func TestDeployPlanning_NonGhcpAgentsKeepThePlainName(t *testing.T) {
	src := filepath.Join("catalog", "agents", "code-reviewer.md")
	for _, id := range []string{hClaude, hOpen} {
		got := plannedAgentPath(t, id, src)
		if filepath.Base(got) != "code-reviewer.md" {
			t.Errorf("deploy target for %s agent: want file code-reviewer.md, got %q", id, got)
		}
	}
}
