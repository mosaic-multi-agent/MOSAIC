package app_test

// thirdformat_deploy_parity_test.go specifies that Deploy workspace and Deploy agents behave
// identically for the test-only agent format and for Markdown: the same plan actions and
// reasons, the same executed actions, the same decoded agents (user content included) and the
// same manifest records, for a create, an unchanged re-run and an update after a version bump.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// twinDeployFlow is one deploy flow.
type twinDeployFlow struct {
	name string
	step twinStep
}

func twinDeployFlows() []twinDeployFlow {
	return []twinDeployFlow{
		{"DeployNew", twinDeployNewStep},
		{"DeployAgents", twinDeployAgentsStep},
	}
}

func TestTwinParity_Deploy_CreateInFreshWorkspace(t *testing.T) {
	for _, flow := range twinDeployFlows() {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			p := newTestTwinPair(t)

			// Act
			test, md := runTwinStep(t, p, flow.step)

			// Assert
			assertTwinParity(t, "create", test, md)
			if got := twinItem(t, test, owRunner).Action; got != domain.ActionCreate {
				t.Errorf("plan action of %s = %s; want %s", owRunner, got, domain.ActionCreate)
			}
			if _, ok := test.Agents[owRunner]; !ok {
				t.Errorf("%s was not written as a decodable agent on the test format twin", owRunner)
			}
		})
	}
}

func TestTwinParity_Deploy_UnchangedRerun(t *testing.T) {
	for _, flow := range twinDeployFlows() {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			p := newTestTwinPair(t)
			runTwinStep(t, p, flow.step)

			// Act
			test, md := runTwinStep(t, p, flow.step)

			// Assert
			assertTwinParity(t, "unchanged re-run", test, md)
			it := twinItem(t, test, owRunner)
			if it.Action != domain.ActionUnchanged {
				t.Errorf("plan action of %s = %s on a re-run over an untouched deployment; want %s",
					owRunner, it.Action, domain.ActionUnchanged)
			}
			if it.BackupBeforeWrite {
				t.Errorf("BackupBeforeWrite set for an untouched agent")
			}
		})
	}
}

func TestTwinParity_Deploy_VersionBumpUpdate_KeepsUserContent(t *testing.T) {
	for _, flow := range twinDeployFlows() {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange: a deployment whose runner is stale and carries user content
			p := newTestTwinPair(t)
			runTwinStep(t, p, flow.step)
			for _, side := range twinSides(p) {
				editTwinAgent(t, side, owRunner, twinChain(twinPlantUserContent(t), twinAgeVersion()))
			}

			// Act
			test, md := runTwinStep(t, p, flow.step)

			// Assert
			assertTwinParity(t, "version-bump update", test, md)
			it := twinItem(t, test, owRunner)
			if it.Action != domain.ActionUpdate {
				t.Errorf("plan action of %s = %s after a version bump; want %s", owRunner, it.Action, domain.ActionUpdate)
			}
			twinRequireUserContent(t, "test format", test, owRunner)
			twinRequireUserContent(t, "markdown", md, owRunner)
		})
	}
}
