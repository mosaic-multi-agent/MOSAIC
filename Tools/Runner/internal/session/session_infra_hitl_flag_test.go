package session_test

// Infrastructure-agent dispatches (including commit setup) are sent with
// human_in_the_loop false, so adapters bound them by the non-HITL timeout.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_Start_InfrastructureAndCommitSetupDispatches_AreSentWithHITLFalse(t *testing.T) {
	rig := newCommitGateRig(t, commitGateFixture)
	rig.queueCommitSetup()
	rig.queueOKTimes(infraCommitAgent, 2)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	checked := 0
	for _, inv := range rig.adapter.Invocations() {
		if inv.Agent.Identifier != infraCommitAgent {
			continue
		}
		checked++
		if inv.Request.HumanInTheLoop {
			t.Errorf("dispatch %s was sent with human_in_the_loop true, want false", inv.Request.AgentInstanceID)
		}
	}
	if checked != 3 {
		t.Errorf("want 3 commit-agent dispatches (setup plus two iteration ends), got %d", checked)
	}
}
