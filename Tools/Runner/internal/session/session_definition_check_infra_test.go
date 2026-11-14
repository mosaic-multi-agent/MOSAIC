package session_test

// Run start checks every declared infrastructure agent that the infrastructure
// filter admits, whichever agent each class selects and whether or not commits
// are enabled: the selection and the commits setting are only settled after the
// resume damage repair, so the check cannot depend on them. An agent whose
// definition file does not exist keeps its dispatch-time handling.

import (
	"os"
	"testing"

	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
)

// TestSession_Start_CLIHarness_InfrastructureAgentWithoutModel_RefusesRun covers
// selected and unselected agents of both gated classes, commits and checkpoints off.
func TestSession_Start_CLIHarness_InfrastructureAgentWithoutModel_RefusesRun(t *testing.T) {
	for _, l := range cliLayouts {
		for _, target := range multiInfraAgents {
			t.Run(l.harnessID+"/"+target, func(t *testing.T) {
				rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
				rig.mutate(t, target, withoutModel)

				res := rig.start(t, rig.config())

				requireDefinitionRefusal(t, res, target, rig.path(target), agentresolve.ErrDefinitionModelMissing)
			})
		}
	}
}

// TestSession_Start_CLIHarness_CommitAgentWithoutModel_RefusesRunWithCommitsEnabled
// covers the commit setup agent of a commits-enabled run.
func TestSession_Start_CLIHarness_CommitAgentWithoutModel_RefusesRunWithCommitsEnabled(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
			rig.mutate(t, selectedCommit, withoutModel)
			cfg := rig.config()
			cfg.Commits = true
			cfg.CommitBranchVariant = domain.CommitBranchMOSAICOwned

			res := rig.start(t, cfg)

			requireDefinitionRefusal(t, res, selectedCommit, rig.path(selectedCommit), agentresolve.ErrDefinitionModelMissing)
		})
	}
}

// TestSession_Start_CLIHarness_SingleCommitAgentWithoutModel_RefusesRunWithCommitsOff
// covers the commit-class agent of a workflow that declares exactly one.
func TestSession_Start_CLIHarness_SingleCommitAgentWithoutModel_RefusesRunWithCommitsOff(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			rig := newDefRig(t, l, "commit-agent-orch.md", []string{selectedCommit})
			rig.mutate(t, selectedCommit, withEmptyModel)
			cfg := rig.config()
			cfg.InfraClassSelections = nil
			cfg.Commits = false

			res := rig.start(t, cfg)

			requireDefinitionRefusal(t, res, selectedCommit, rig.path(selectedCommit), agentresolve.ErrDefinitionModelMissing)
		})
	}
}

// TestSession_Start_CLIHarness_InfrastructureFilter_DecidesWhichAgentsAreChecked
// verifies that the filter-admitted unselected agent is checked and the one the
// filter excludes is not.
func TestSession_Start_CLIHarness_InfrastructureFilter_DecidesWhichAgentsAreChecked(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID+"/admitted unselected agent is checked", func(t *testing.T) {
			rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
			rig.mutate(t, unselectedCheckpoint, withoutModel)
			cfg := rig.config()
			cfg.InfrastructureFilter = multiInfraAgents

			res := rig.start(t, cfg)

			requireDefinitionRefusal(t, res, unselectedCheckpoint, rig.path(unselectedCheckpoint), agentresolve.ErrDefinitionModelMissing)
		})
		t.Run(l.harnessID+"/excluded agent is not checked", func(t *testing.T) {
			rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
			rig.mutate(t, unselectedCheckpoint, withoutModel)
			rig.mutate(t, unselectedCommit, withName("some-other-agent"), withoutModel)
			cfg := rig.config()
			cfg.InfrastructureFilter = []string{selectedCheckpoint, selectedCommit}

			requireStarted(t, rig.start(t, cfg))
		})
	}
}

// TestSession_Start_CLIHarness_InfrastructureAgentWithoutDefinitionFile_NotRefusedAtRunStart
// verifies that a declared agent with no definition file does not refuse the
// run at start; only a found file with a bad definition does.
func TestSession_Start_CLIHarness_InfrastructureAgentWithoutDefinitionFile_NotRefusedAtRunStart(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
			if err := os.Remove(rig.path(unselectedCommit)); err != nil {
				t.Fatalf("test setup: %v", err)
			}

			requireStarted(t, rig.start(t, rig.config()))
		})
	}
}
