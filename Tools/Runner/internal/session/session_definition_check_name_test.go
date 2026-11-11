package session_test

// Claude Code resolves `--agent <identifier>` by the frontmatter name of the
// definition, not by its file name. Run start on Claude Code therefore refuses a
// run when an agent it can dispatch declares a name other than its identifier
// (or none). OpenCode and GHCP CLI do not apply the check.

import (
	"context"
	"testing"

	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

var misnamed = []struct {
	name string
	mut  defMut
}{
	{"different name", withName("some-other-agent")},
	{"name absent", withoutName},
}

// TestSession_Start_ClaudeCode_NameDiffersFromIdentifier_RefusesRun covers the
// routing-table agents, the orchestrator and every declared infrastructure agent.
func TestSession_Start_ClaudeCode_NameDiffersFromIdentifier_RefusesRun(t *testing.T) {
	claude := cliLayouts[0]
	targets := append([]string{"agent-a", "agent-b", "orchestrator"}, multiInfraAgents...)
	for _, target := range targets {
		for _, m := range misnamed {
			t.Run(target+"/"+m.name, func(t *testing.T) {
				rig := newDefRig(t, claude, multiInfraFixture, multiInfraAgents)
				rig.mutate(t, target, m.mut)

				res := rig.start(t, rig.config())

				requireDefinitionRefusal(t, res, target, rig.path(target), agentresolve.ErrDefinitionNameMismatch)
			})
		}
	}
}

// TestSession_Start_OpenCodeAndGHCPCLI_NameDiffersFromIdentifier_StartsNormally
// verifies that the name check is specific to Claude Code.
func TestSession_Start_OpenCodeAndGHCPCLI_NameDiffersFromIdentifier_StartsNormally(t *testing.T) {
	for _, l := range cliLayouts[1:] {
		for _, target := range []string{"agent-a", "orchestrator", unselectedCheckpoint} {
			for _, m := range misnamed {
				t.Run(l.harnessID+"/"+target+"/"+m.name, func(t *testing.T) {
					rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)
					rig.mutate(t, target, m.mut)

					requireStarted(t, rig.start(t, rig.config()))
				})
			}
		}
	}
}

// TestSession_Start_ClaudeCode_ModelFailureReportedBeforeNameFailure verifies
// that an agent with both faults is refused for its missing model.
func TestSession_Start_ClaudeCode_ModelFailureReportedBeforeNameFailure(t *testing.T) {
	rig := newDefRig(t, cliLayouts[0], "linear-orch.md", nil)
	rig.mutate(t, "agent-a", withoutModel, withName("some-other-agent"))

	res := rig.start(t, rig.config())

	requireDefinitionRefusal(t, res, "agent-a", rig.path("agent-a"), agentresolve.ErrDefinitionModelMissing)
}

// TestSession_Start_FakeHarness_MisnamedDefinitions_StartNormally verifies that
// the fake harness is exempt from the name check even when its agent files
// carry a frontmatter.
func TestSession_Start_FakeHarness_MisnamedDefinitions_StartNormally(t *testing.T) {
	rig := newDefRig(t, cliLayouts[0], "linear-orch.md", nil)
	rig.mutate(t, "agent-a", withName("some-other-agent"), withoutModel)
	cfg := rig.config()
	cfg.HarnessID = "fake"
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	ses := session.New(session.Deps{Harness: f, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
}
