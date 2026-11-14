package app_test

// thirdformat_deny_tools_parity_test.go specifies that deny tools behave identically for the
// test-only agent format and for Markdown. A deny reaches the tools field of a twin agent when a
// permission-shaped source harness denies a tool the target does not list as its own: the name is
// written as a deny entry in the permission-shaped tools field, and it wins over an allow the
// existing destination file holds for the same name. The source is a copy of the OpenCode
// descriptor with its own agents folder and extension, so it never shares paths with a twin.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	twinDenySourceID  = "permsource"
	twinDenySourceExt = ".agent.md"
)

// twinDenySource returns the permission-shaped source harness: the built-in OpenCode descriptor
// under its own id, agents folder and agent extension.
func twinDenySource(t *testing.T) domain.HarnessModule {
	t.Helper()
	inner := rbModule(t, hOpen)
	desc := *inner.Descriptor()
	desc.ID = twinDenySourceID
	desc.DisplayName = "Permission Source"
	desc.Paths.Agents.Project = ".permsource/agents"
	desc.Extensions = map[domain.ArtifactKind]string{}
	for kind, ext := range inner.Descriptor().Extensions {
		desc.Extensions[kind] = ext
	}
	desc.Extensions[domain.ArtifactAgent] = twinDenySourceExt
	ref := inner.Ref()
	ref.ID, ref.DisplayName = desc.ID, desc.DisplayName
	return &twinModule{HarnessModule: inner, ref: ref, desc: &desc}
}

// twinDenyPair builds a twin pair that also registers the permission-shaped source harness.
func twinDenyPair(t *testing.T) twinPair {
	t.Helper()
	return newTestTwinPairWith(t, []domain.HarnessModule{twinDenySource(t)})
}

// twinDeploySourceRunnerStep runs Deploy agents for the one sample subagent on the source harness.
func twinDeploySourceRunnerStep(t *testing.T, side twinSide) (domain.RunSummary, error) {
	t.Helper()
	return side.Service.DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: twinDenySourceID, WorkspacePath: side.Workspace, Scope: domain.ScopeProject,
		SubagentIDs: []string{owRunner}, UtilityAgentIDs: []string{}, InfrastructureAgentIDs: []string{},
		StandaloneAgentIDs: []string{}, AgentModels: twinModels(), AutoConfirmPlan: true,
		ConflictDefault: domain.DecisionSkip})
}

// twinDenyTransformStep transforms the source harness into the twin.
func twinDenyTransformStep(overwrite bool) twinFlowStep {
	return func(t *testing.T, side twinSide) (twinFlowResult, error) {
		t.Helper()
		res, err := side.Service.TransformHarness(context.Background(), app.TransformHarnessRequest{
			SourceHarnessID: twinDenySourceID, TargetHarnessID: twinHarnessID, Workspace: side.Workspace,
			TargetModel: parityModel, AutoConfirmPlan: true, Overwrite: overwrite})
		if err != nil {
			return twinFlowResult{}, err
		}
		return twinFlowResult{Transform: &res}, nil
	}
}

// twinDenyInSource edits the deployed source runner of a side so that it denies the name, which
// the target harness does not list among its own tools.
func twinDenyInSource(t *testing.T, side twinSide, name string) {
	t.Helper()
	rel := twinCounterpartRel(t, side, owRunner)
	doc := gamRead(t, side.Workspace, rel)
	gamWrite(t, side.Workspace, rel, string(twinSetPermission(t, name, twinDeny)(doc)))
}

// twinDenyScenario deploys the source runner to both twins' workspaces and plants the deny in it.
// With existingAllow set, each twin also has a deployed runner that allows the same name.
func twinDenyScenario(t *testing.T, existingAllow bool) twinPair {
	t.Helper()
	p := twinDenyPair(t)
	runTwinStep(t, p, twinDeploySourceRunnerStep)
	if existingAllow {
		runTwinStep(t, p, twinDeployAgentsStep)
	}
	for _, side := range twinSides(p) {
		twinDenyInSource(t, side, twinDeniedSrv)
		if existingAllow {
			editTwinAgent(t, side, owRunner, twinSetPermission(t, twinDeniedSrv, twinAllow))
		}
	}
	return p
}

// twinRequireRunnerTransformed fails unless the transform wrote the runner.
func twinRequireRunnerTransformed(t *testing.T, label string, obs twinObservation) {
	t.Helper()
	if f := twinTransformFile(t, obs, owRunner); f.Status != app.StatusTransformed {
		t.Fatalf("%s: runner status %q (reason %q); want transformed", label, f.Status, f.Reason)
	}
}

func TestTwinParity_DenyTools_DenyOfAbsentTool_WrittenAsDenyEntry(t *testing.T) {
	// Arrange: the destination has no runner, so the denied name is not present there
	p := twinDenyScenario(t, false)

	// Act
	test, md := runTwinFlowStep(t, p, twinDenyTransformStep(false))

	// Assert
	assertTwinParity(t, "transform with a deny of an absent tool", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		twinRequireRunnerTransformed(t, label, obs)
		twinRequireToolsEntry(t, label, twinToolsOf(t, obs, owRunner), twinDeniedSrv, twinDeny)
	}
}

func TestTwinParity_DenyTools_DenyOverridesExistingAllow(t *testing.T) {
	// Arrange: the existing destination runner allows the name the source denies
	p := twinDenyScenario(t, true)

	// Act
	test, md := runTwinFlowStep(t, p, twinDenyTransformStep(true))

	// Assert
	assertTwinParity(t, "transform with a deny over an allow", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		twinRequireRunnerTransformed(t, label, obs)
		tools := twinToolsOf(t, obs, owRunner)
		twinRequireToolsEntry(t, label, tools, twinDeniedSrv, twinDeny)
		if strings.Contains(tools, twinDeniedSrv+":"+twinAllow) {
			t.Errorf("%s: tools field %q still allows %s", label, tools, twinDeniedSrv)
		}
	}
}

func TestTwinParity_DenyTools_DenyIsWrittenOnceAndLeavesBuiltInToolsAlone(t *testing.T) {
	// Arrange
	p := twinDenyScenario(t, true)

	// Act
	test, md := runTwinFlowStep(t, p, twinDenyTransformStep(true))

	// Assert
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		tools := twinToolsOf(t, obs, owRunner)
		if strings.Count(tools, twinDeniedSrv+":") != 1 {
			t.Errorf("%s: tools field %q does not hold %s exactly once", label, tools, twinDeniedSrv)
		}
		twinRequireToolsEntry(t, label, tools, "read", twinAllow)
		twinRequireToolsEntry(t, label, tools, "patch", twinDeny)
	}
}
