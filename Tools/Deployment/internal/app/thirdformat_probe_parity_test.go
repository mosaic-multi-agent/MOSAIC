package app_test

// thirdformat_probe_parity_test.go specifies that the planned-path probe treats the test-only
// agent format exactly like Markdown when the file at an agent's planned path is not that agent's
// own: it is claimed by another deployed agent (indexed under another id), or it has no id stamp,
// and that the outcome is the same for Deploy agents as for Update.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// twinOtherID is the id stamp of an agent that is not in the catalog.
const twinOtherID = "9998"

// twinRequireSameOutcome fails unless the test format plan item of the agent has the action and
// reasons the Markdown plan item has: the outcome of the scenario is the Markdown outcome.
func twinRequireSameOutcome(t *testing.T, test, md twinObservation, key string) {
	t.Helper()
	ti, mi := twinItem(t, test, key), twinItem(t, md, key)
	if ti.Action != mi.Action {
		t.Errorf("plan action of %s: test format %s, markdown %s; want the markdown outcome", key, ti.Action, mi.Action)
	}
	if len(ti.Reasons) != len(mi.Reasons) {
		t.Errorf("plan reasons of %s: test format %q, markdown %q; want the markdown outcome", key, ti.Reasons, mi.Reasons)
	}
}

func TestTwinParity_Update_AgentAtPlannedPathClaimedByAnotherAgent_BehavesAsMarkdown(t *testing.T) {
	// Arrange: another agent, deployed elsewhere and indexed under its id stamp, shares that id
	// with the file at the runner's planned path, so the file there is claimed by the other agent
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		other := twinSetField(twinIDKey, twinOtherID)(twinHarnessOnlyCanonical())
		writeTwinAgent(t, side, twinHarnessOnlyKey, other)
		editTwinAgent(t, side, owRunner, twinSetField(twinIDKey, twinOtherID))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update over an agent claimed by another agent", test, md, twinIgnoreProblemDetail())
	twinRequireSameOutcome(t, test, md, owRunner)
}

func TestTwinParity_DeployAgents_AgentAtPlannedPathWithoutIDStamp_BehavesAsMarkdown(t *testing.T) {
	// Arrange
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinRemoveField(twinIDKey))
	}

	// Act
	test, md := runTwinStep(t, p, twinDeployAgentsStep)

	// Assert
	assertTwinParity(t, "deploy agents over an agent without an id stamp", test, md, twinIgnoreProblemDetail())
	twinRequireSameOutcome(t, test, md, owRunner)
	if got := twinItem(t, test, owRunner).Action; got != domain.ActionConflict {
		t.Errorf("plan action of %s = %s for an agent without an id stamp at its planned path; want %s", owRunner, got, domain.ActionConflict)
	}
}

func TestTwinParity_DeployAgents_AgentAtPlannedPathWithAnotherID_BehavesAsMarkdown(t *testing.T) {
	// Arrange
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinSetField(twinIDKey, "9999"))
	}

	// Act
	test, md := runTwinStep(t, p, twinDeployAgentsStep)

	// Assert
	assertTwinParity(t, "deploy agents over an agent with another agent's id", test, md, twinIgnoreProblemDetail())
	twinRequireSameOutcome(t, test, md, owRunner)
}

func TestTwinParity_Update_AgentAtPlannedPathFlaggedByTheScan_BehavesAsMarkdown(t *testing.T) {
	// Arrange: the agent's file cannot be read as an agent document, so the workspace scan flags
	// its path as parse-failed
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		corruptTwinAgent(t, side, owRunner)
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update over an agent flagged by the scan", test, md, twinIgnoreProblemDetail())
	twinRequireSameOutcome(t, test, md, owRunner)
	if got := twinItem(t, test, owRunner).Action; got != domain.ActionConflict {
		t.Errorf("plan action of %s = %s for a parse-failed file at its planned path; want %s", owRunner, got, domain.ActionConflict)
	}
}
