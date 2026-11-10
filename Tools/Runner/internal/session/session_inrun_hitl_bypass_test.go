package session_test

// Tests that a raw-text bypass attempt of a step raised by the in-run HITL
// request keeps the step's HITL value, and that the request is not used up by
// the bypass.

import (
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"

	commonharness "mosaic-common/harness"
)

func TestSession_InRunHITL_RawTextBypassOfRaisedStep_KeepsHITL(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newHITLRig(t, "linear3-orch.md", threeAgents, &approvalLog{}, &scriptedRoutingConsultant{}, nil)
			rig.queueOK("agent-a", 1)
			bypassed := harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrProtocolNotExtractable)}
			rig.f.Queue("agent-b", bypassed, okEntry())
			rig.queueOK("agent-c", 1)
			rig.requestAfter("agent-a", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			want := []string{"agent-a=false", "agent-b=true", "agent-b=true", "agent-c=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want the bypass attempt of the raised step to keep HITL: %v, got %v", want, seq)
			}
		})
	}
}
