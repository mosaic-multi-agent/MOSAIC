package app_test

// thirdformat_update_parity_test.go specifies that Update and Update workflows behave
// identically for the test-only agent format and for Markdown, including the orchestrator parse
// check: a valid deployed orchestrator yields no conflict, a corrupt one yields a conflict that
// the conflict decision resolves, and a custom workflow deployed into the orchestrator is kept.

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestTwinParity_Update_UnchangedWorkspace(t *testing.T) {
	// Arrange
	p := twinDeployed(t)

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update of an unchanged workspace", test, md)
	for _, it := range test.Items {
		if it.Ref.Kind == domain.ArtifactAgent && it.Action != domain.ActionUnchanged {
			t.Errorf("%s: plan action = %s; want %s", it.Ref.Key, it.Action, domain.ActionUnchanged)
		}
	}
}

func TestTwinParity_Update_StaleAgentWithUserContent_IsUpdatedAndUserContentKept(t *testing.T) {
	// Arrange
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinChain(twinPlantUserContent(t), twinAgeVersion()))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update of a stale agent", test, md)
	if got := twinItem(t, test, owRunner).Action; got != domain.ActionUpdate {
		t.Errorf("plan action of %s = %s; want %s", owRunner, got, domain.ActionUpdate)
	}
	twinRequireUserContent(t, "test format", test, owRunner)
	twinRequireUserContent(t, "markdown", md, owRunner)
}

func TestTwinParity_UpdateWorkflows_UnchangedWorkspace(t *testing.T) {
	// Arrange
	p := twinDeployed(t)

	// Act
	test, md := runTwinStep(t, p, twinUpdateWorkflowsStep(domain.DecisionSkip))

	// Assert
	assertTwinParity(t, "update workflows of an unchanged workspace", test, md)
}

func TestTwinParity_UpdateWorkflows_ValidOrchestrator_YieldsNoConflict(t *testing.T) {
	// Arrange
	p := twinDeployed(t)

	// Act
	test, md := runTwinStep(t, p, twinUpdateWorkflowsStep(domain.DecisionSkip))

	// Assert
	assertTwinParity(t, "parse check of a valid orchestrator", test, md)
	for _, obs := range []twinObservation{test, md} {
		if got := twinItem(t, obs, wpsOrch).Action; got == domain.ActionConflict {
			t.Errorf("a valid deployed orchestrator was planned as %s", got)
		}
	}
}

func TestTwinParity_UpdateWorkflows_CorruptOrchestrator_YieldsConflict(t *testing.T) {
	// Arrange
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		corruptTwinAgent(t, side, wpsOrch)
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateWorkflowsStep(domain.DecisionSkip))

	// Assert
	assertTwinParity(t, "parse check of a corrupt orchestrator", test, md, twinIgnoreProblemDetail())
	for _, obs := range []twinObservation{test, md} {
		if got := twinItem(t, obs, wpsOrch).Action; got != domain.ActionConflict {
			t.Errorf("a corrupt deployed orchestrator was planned as %s; want %s", got, domain.ActionConflict)
		}
	}
}

func TestTwinParity_UpdateWorkflows_CorruptOrchestrator_SkipLeavesFileUntouched(t *testing.T) {
	// Arrange
	p := twinDeployed(t)
	before := map[twinFormat][]byte{}
	for _, side := range twinSides(p) {
		corruptTwinAgent(t, side, wpsOrch)
		before[side.Format] = gamRead(t, side.Workspace, twinAgentRel(t, side, wpsOrch))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateWorkflowsStep(domain.DecisionSkip))

	// Assert
	for _, side := range twinSides(p) {
		after := gamRead(t, side.Workspace, twinAgentRel(t, side, wpsOrch))
		if !bytes.Equal(before[side.Format], after) {
			t.Errorf("%s twin: the skipped corrupt orchestrator was modified", side.name())
		}
	}
	for _, obs := range []twinObservation{test, md} {
		if got := twinTaken(t, obs, wpsOrch); got != domain.TakenSkipped {
			t.Errorf("action taken for the corrupt orchestrator = %s; want %s", got, domain.TakenSkipped)
		}
	}
}

func TestTwinParity_UpdateWorkflows_CorruptOrchestrator_OverwriteRestoresAnAgent(t *testing.T) {
	// Arrange
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		corruptTwinAgent(t, side, wpsOrch)
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateWorkflowsStep(domain.DecisionOverwrite))

	// Assert: the owned-key differences of a file that fails to parse are a per-format detail
	// (Markdown still reads a file without frontmatter), so only the outcome is compared.
	if twinTaken(t, test, wpsOrch) != twinTaken(t, md, wpsOrch) {
		t.Errorf("action taken for the overwritten orchestrator differs: test format %s, markdown %s",
			twinTaken(t, test, wpsOrch), twinTaken(t, md, wpsOrch))
	}
	if !twinSameAgent(test.Agents[wpsOrch], md.Agents[wpsOrch]) {
		t.Errorf("the rewritten orchestrator differs between the formats:\n%+v\n%+v", test.Agents[wpsOrch], md.Agents[wpsOrch])
	}
	if _, ok := test.Agents[wpsOrch]; !ok {
		t.Errorf("the overwritten orchestrator is not a decodable agent on the test format twin")
	}
}

func twinSameAgent(a, b twinAgent) bool { return reflect.DeepEqual(a, b) }

func TestTwinParity_UpdateWorkflows_CustomWorkflowInOrchestrator_OfferedAndKept(t *testing.T) {
	// Arrange
	checklists := map[twinFormat]*wukChecklist{}
	p := newTestTwinPairWith(t, nil, func(f twinFormat, _ string) func(*app.Deps) {
		c := newWukChecklist(wpsFlowParity, wukIDX)
		checklists[f] = c
		return func(d *app.Deps) { d.Interaction = c }
	})
	runTwinStep(t, p, twinDeployNewStep)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, wpsOrch, twinPlantWorkflow(t, wukBlockX))
		twinReRecord(t, side, wpsOrch)
	}
	ask := func(t *testing.T, side twinSide) (domain.RunSummary, error) {
		return side.Service.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: twinHarnessID, WorkspacePath: side.Workspace, AutoConfirmPlan: true,
			ConflictDefault: domain.DecisionSkip})
	}

	// Act
	test, md := runTwinStep(t, p, ask)

	// Assert
	assertTwinParity(t, "update workflows over a custom workflow", test, md)
	for f, c := range checklists {
		if len(c.asked) != 1 {
			t.Fatalf("format %d: the workflow checklist was asked %d times; want once", f, len(c.asked))
		}
		if opt, ok := wukOption(c.asked[0], wukIDX); !ok || opt.Hint != wukCustomHint {
			t.Errorf("format %d: checklist option for %s = %+v (found %v); want a deployed-only option", f, wukIDX, opt, ok)
		}
	}
	for _, side := range twinSides(p) {
		if doc := string(readTwinAgent(t, side, wpsOrch)); !strings.Contains(doc, wukBlockX) {
			t.Errorf("%s twin: the custom workflow block was not kept byte for byte in the orchestrator", side.name())
		}
	}
}

// twinPlantWorkflow returns the edit that inserts a workflow block at the end of the workflow
// region of the canonical orchestrator.
func twinPlantWorkflow(t *testing.T, block string) func([]byte) []byte {
	return twinBodyEdit(func(doc string) string {
		at := strings.Index(doc, wpsRegionClose)
		if at < 1 {
			t.Fatalf("the deployed orchestrator has no workflow region close tag")
		}
		return doc[:at] + block + doc[at:]
	})
}

// twinTaken returns the action taken for the agent key in an observation.
func twinTaken(t *testing.T, obs twinObservation, key string) domain.ActionTaken {
	t.Helper()
	for _, a := range obs.Actions {
		if a.Ref.Kind == domain.ArtifactAgent && a.Ref.Key == key {
			return a.Taken
		}
	}
	t.Fatalf("no executed action for agent %s", key)
	return ""
}

// twinIDKey is the deployed id stamp key of the canonical frontmatter.
const twinIDKey = "mosaic_id"

func TestTwinParity_Update_AgentAtPlannedPathWithoutIDStamp_IsAConflict(t *testing.T) {
	// Arrange: the deployed runner lost its id stamp, so it cannot be vouched for as the runner
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinRemoveField(twinIDKey))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update over an agent without an id stamp", test, md, twinIgnoreProblemDetail())
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		if got := twinItem(t, obs, owRunner).Action; got != domain.ActionConflict {
			t.Errorf("%s: plan action = %s for an agent without an id stamp at its planned path; want %s", label, got, domain.ActionConflict)
		}
	}
}

func TestTwinParity_Update_AgentAtPlannedPathWithAnotherID_BehavesAsMarkdown(t *testing.T) {
	// Arrange: the deployed runner carries the id of a different agent
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinSetField(twinIDKey, "9999"))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update over an agent with another agent's id", test, md, twinIgnoreProblemDetail())
	twinRequireSameOutcome(t, test, md, owRunner)
}
