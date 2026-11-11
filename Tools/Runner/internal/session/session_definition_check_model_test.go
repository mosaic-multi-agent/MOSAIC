package session_test

// Run start on Claude Code, OpenCode and GHCP CLI refuses a run when an agent it
// can dispatch (a routing-table agent or the orchestrator) has no model in its
// definition: there is never a fallback to the harness default model. The
// refusal names the agent and its file, and leaves no artifact and no
// invocation behind.

import (
	"context"
	"testing"

	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
)

// TestSession_Start_CLIHarness_DefinitionWithoutModel_RefusesRun covers every
// way a definition can lack a model, for the routing-table agents and the
// orchestrator, on every CLI harness.
func TestSession_Start_CLIHarness_DefinitionWithoutModel_RefusesRun(t *testing.T) {
	variants := []struct {
		name string
		mut  defMut
	}{
		{"model key absent", withoutModel},
		{"model empty", withEmptyModel},
		{"model blank", withBlankModel},
	}
	for _, l := range cliLayouts {
		for _, target := range []string{"agent-a", "agent-b", "orchestrator"} {
			for _, v := range variants {
				t.Run(l.harnessID+"/"+target+"/"+v.name, func(t *testing.T) {
					rig := newDefRig(t, l, "linear-orch.md", nil)
					rig.mutate(t, target, v.mut)

					res := rig.start(t, rig.config())

					requireDefinitionRefusal(t, res, target, rig.path(target), agentresolve.ErrDefinitionModelMissing)
				})
			}
		}
	}
}

// TestSession_Start_CLIHarness_DefinitionWithoutFrontmatter_RefusesRun verifies
// that a definition with no frontmatter at all counts as having no model.
func TestSession_Start_CLIHarness_DefinitionWithoutFrontmatter_RefusesRun(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID, func(t *testing.T) {
			rig := newDefRig(t, l, "linear-orch.md", nil)
			writeAgentFile(t, rig.agentsDir, "agent-b")

			res := rig.start(t, rig.config())

			requireDefinitionRefusal(t, res, "agent-b", rig.path("agent-b"), agentresolve.ErrDefinitionModelMissing)
		})
	}
}

// TestSession_Start_CLIHarness_OrchestratorCheckedBeforeRoutingAgents verifies
// the check order: when the orchestrator and a routing-table agent both lack a
// model, the refusal is the orchestrator's.
func TestSession_Start_CLIHarness_OrchestratorCheckedBeforeRoutingAgents(t *testing.T) {
	rig := newDefRig(t, cliLayouts[0], "linear-orch.md", nil)
	rig.mutate(t, "agent-a", withoutModel)
	rig.mutate(t, "orchestrator", withoutModel)

	res := rig.start(t, rig.config())

	requireDefinitionRefusal(t, res, "orchestrator", rig.path("orchestrator"), agentresolve.ErrDefinitionModelMissing)
}

// TestSession_Start_CLIHarness_ValidDefinitions_StartNormally verifies that a
// run whose definitions all declare a model (and, on Claude Code, a matching
// name) starts and completes.
func TestSession_Start_CLIHarness_ValidDefinitions_StartNormally(t *testing.T) {
	for _, l := range cliLayouts {
		t.Run(l.harnessID+"/linear", func(t *testing.T) {
			rig := newDefRig(t, l, "linear-orch.md", nil)

			requireStarted(t, rig.start(t, rig.config()))
		})
		t.Run(l.harnessID+"/with infrastructure agents", func(t *testing.T) {
			rig := newDefRig(t, l, multiInfraFixture, multiInfraAgents)

			requireStarted(t, rig.start(t, rig.config()))
		})
	}
}

// TestSession_Start_FakeHarness_FrontmatterlessDefinitions_StartNormally
// verifies that runs on the fake harness are not subject to the check: agent
// files without any frontmatter keep working.
func TestSession_Start_FakeHarness_FrontmatterlessDefinitions_StartNormally(t *testing.T) {
	ses, f, _, orchPath := newLinearSession(t)
	queueLinearAgentsDone(f)
	cfg := baseLinearConfig(orchPath)
	cfg.HarnessID = "fake"

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(f.Invocations()) == 0 {
		t.Error("want agents invoked, got none")
	}
}
