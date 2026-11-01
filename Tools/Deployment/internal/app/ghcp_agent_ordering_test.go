package app_test

// ghcp_agent_ordering_test.go covers two guards around the legacy X.md migration: the
// cross-target refusal still runs before any migration on a legacy workspace, and the other
// harnesses keep writing plain X.md agent files through every agent-writing flow.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

// TestGhcpDeployFlows_OtherTargetRequestedOnLegacyWorkspace_RefusedBeforeMigration verifies a
// workspace holding recorded X.md agents for one GHCP target is refused when the other target
// is requested, and that nothing is migrated, backed up or rewritten.
func TestGhcpDeployFlows_OtherTargetRequestedOnLegacyWorkspace_RefusedBeforeMigration(t *testing.T) {
	directions := []struct{ present, requested string }{
		{hCLI, hVSCode},
		{hVSCode, hCLI},
	}
	for _, d := range directions {
		for _, flow := range gamFlows() {
			t.Run(d.present+" workspace, "+d.requested+" requested/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, d.present)
				before := captureFS(t, ws)

				// Act
				err := flow.run(t, ws, d.requested, gamOpts{})

				// Assert
				if !errors.Is(err, app.ErrGhcpOtherTarget) {
					t.Fatalf("error %v does not match ErrGhcpOtherTarget", err)
				}
				if changed, desc := fsChanged(before, captureFS(t, ws)); changed {
					t.Errorf("workspace changed after refusal (%s)", desc)
				}
				for _, a := range parityAgents {
					gamAssertKept(t, ws, a.key, legacy[a.key])
				}
			})
		}
	}
}

// TestNonGhcpDeploys_AllAgentFlows_KeepWritingPlainMdAgentFiles verifies full deploy, deploy
// agents only, Update and workflow update write X.md agent files for the other harnesses.
// The workflow update flow deploys the orchestrator only; the script orchestrator is covered
// by full deploy and Update.
func TestNonGhcpDeploys_AllAgentFlows_KeepWritingPlainMdAgentFiles(t *testing.T) {
	for _, h := range []string{hClaude, hOpen} {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws := parityDeploy(t, h)
				dir := rbModule(t, h).Descriptor().Paths.Agents.Project

				// Act
				err := flow.run(t, ws, h, gamOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				files, rerr := os.ReadDir(gamAbs(ws, dir))
				if rerr != nil {
					t.Fatalf("read %s: %v", dir, rerr)
				}
				for _, f := range files {
					if strings.HasSuffix(f.Name(), ".agent.md") {
						t.Errorf("%s: agent file %s was named X.agent.md", h, f.Name())
					}
				}
				for _, key := range flow.migrates {
					if !gamExists(ws, dir+"/"+key+".md") {
						t.Errorf("%s: %s.md is missing", h, key)
					}
				}
			})
		}
	}
}
