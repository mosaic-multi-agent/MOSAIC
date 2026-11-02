package app_test

// ghcp_agent_naming_test.go verifies the file name the agent-writing deploy flows give agents:
// for both GHCP targets every agent file is written and recorded as X.agent.md, the
// orchestrator and the script orchestrator included, and the file keeps the content of its
// target. Deploys for other harnesses keep writing X.md.

import (
	"context"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

var gamAllAgentKeys = []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"}

// TestGhcpDeployNew_FreshWorkspace_WritesOnlyAgentDotMdFiles covers a full deploy.
func TestGhcpDeployNew_FreshWorkspace_WritesOnlyAgentDotMdFiles(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			ws := t.TempDir()

			if err := gamFlows()[0].run(t, ws, h, gamOpts{}); err != nil {
				t.Fatalf("DeployNew for %s: %v", h, err)
			}

			gamAssertOnlyAgentExt(t, ws)
			gamAssertAgentsRecorded(t, ws, gamAllAgentKeys)
		})
	}
}

// TestGhcpDeployAgents_FreshWorkspace_WritesOnlyAgentDotMdFiles covers deploy agents only.
func TestGhcpDeployAgents_FreshWorkspace_WritesOnlyAgentDotMdFiles(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			ws := t.TempDir()

			if err := gamFlows()[1].run(t, ws, h, gamOpts{}); err != nil {
				t.Fatalf("DeployAgents for %s: %v", h, err)
			}

			gamAssertOnlyAgentExt(t, ws)
			gamAssertAgentsRecorded(t, ws, []string{"parity-runner"})
		})
	}
}

// TestGhcpUpdate_AfterFreshDeploy_LeavesOnlyAgentDotMdFiles covers Update.
func TestGhcpUpdate_AfterFreshDeploy_LeavesOnlyAgentDotMdFiles(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			ws := parityDeploy(t, h)

			if err := gamFlows()[2].run(t, ws, h, gamOpts{}); err != nil {
				t.Fatalf("Update for %s: %v", h, err)
			}

			gamAssertOnlyAgentExt(t, ws)
			gamAssertAgentsRecorded(t, ws, gamAllAgentKeys)
		})
	}
}

// TestGhcpUpdateWorkflows_AfterFreshDeploy_LeavesOnlyAgentDotMdFiles covers workflow update,
// whose orchestrator path it computes on its own.
func TestGhcpUpdateWorkflows_AfterFreshDeploy_LeavesOnlyAgentDotMdFiles(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			ws := parityDeploy(t, h)

			if err := gamFlows()[3].run(t, ws, h, gamOpts{}); err != nil {
				t.Fatalf("UpdateWorkflows for %s: %v", h, err)
			}

			gamAssertOnlyAgentExt(t, ws)
			gamAssertAgentsRecorded(t, ws, []string{"orchestrator"})
		})
	}
}

// TestGhcpDeployNew_AgentContentStaysSpecificToTheTarget verifies the agent files of the two
// targets are not interchangeable: renaming does not make one target write the other's content.
func TestGhcpDeployNew_AgentContentStaysSpecificToTheTarget(t *testing.T) {
	cli, vscode := t.TempDir(), t.TempDir()
	if err := gamFlows()[0].run(t, cli, hCLI, gamOpts{}); err != nil {
		t.Fatalf("DeployNew for %s: %v", hCLI, err)
	}
	if err := gamFlows()[0].run(t, vscode, hVSCode, gamOpts{}); err != nil {
		t.Fatalf("DeployNew for %s: %v", hVSCode, err)
	}

	for _, key := range gamAllAgentKeys {
		rel := gamRel(key, ".agent.md")
		if !gamExists(cli, rel) || !gamExists(vscode, rel) {
			t.Fatalf("%s: %s missing for a target", key, rel)
		}
		if got, want := string(gamRead(t, cli, rel)), parityAgentFile(t, cli, hCLI, key); got != want {
			t.Errorf("%s: ghcp-cli file differs from the target's own deploy output", key)
		}
		if got, want := string(gamRead(t, vscode, rel)), parityAgentFile(t, vscode, hVSCode, key); got != want {
			t.Errorf("%s: vscode-ghcp file differs from the target's own deploy output", key)
		}
	}
}

// TestNonGhcpDeploys_KeepWritingPlainMdAgentFiles verifies the other harnesses are unchanged
// by a full deploy followed by an Update.
func TestNonGhcpDeploys_KeepWritingPlainMdAgentFiles(t *testing.T) {
	for _, h := range []string{hClaude, hOpen} {
		t.Run(h, func(t *testing.T) {
			ws := parityDeploy(t, h)
			_, err := parityService(t).Update(context.Background(), app.UpdateRequest{
				HarnessID: h, WorkspacePath: ws, AgentModels: gamModels(), AutoConfirmPlan: true,
				ConflictDefault: domain.DecisionSkip})
			if err != nil {
				t.Fatalf("Update for %s: %v", h, err)
			}

			dir := rbModule(t, h).Descriptor().Paths.Agents.Project
			files, rerr := os.ReadDir(gamAbs(ws, dir))
			if rerr != nil {
				t.Fatalf("read %s: %v", dir, rerr)
			}
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".agent.md") {
					t.Errorf("%s: agent file %s was named X.agent.md", h, f.Name())
				}
			}
			for _, key := range gamAllAgentKeys {
				if !gamExists(ws, dir+"/"+key+".md") {
					t.Errorf("%s: %s.md was not written", h, key)
				}
			}
		})
	}
}
