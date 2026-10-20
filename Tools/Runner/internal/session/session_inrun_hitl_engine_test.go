package session_test

// Tests of the in-run HITL request at engine-routed dispatches (auto and
// auto-review): a request pending when agent A's outcome is accepted raises the
// next new workflow step to HITL (verified by the HITL gate), applies to that
// one step only, and is never used up by infrastructure dispatches or by
// mechanical re-dispatches.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// threeStepRig is a rig over the three-step workflow whose steps all start with
// HITL off.
func threeStepRig(t *testing.T, approvals *approvalLog, consultant *scriptedRoutingConsultant) *inRunRig {
	t.Helper()
	rig := newHITLRig(t, "linear3-orch.md", threeAgents, approvals, consultant, nil)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.queueOK("agent-c", 1)
	return rig
}

func TestSession_InRunHITL_RequestSetWhileAgentRuns_NextDispatchRunsWithHITLOnly(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			approvals := &approvalLog{}
			rig := threeStepRig(t, approvals, nil)
			rig.requestAfter("agent-a", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			want := []string{"agent-a=false", "agent-b=true", "agent-c=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Fatalf("want HITL raised for the next step only: %v, got %v", want, seq)
			}
			if rig.hitl.isPending() {
				t.Error("want the request consumed")
			}
		})
	}
}

func TestSession_InRunHITL_RaisedStep_IsVerifiedByTheGateAndNoOtherStepIs(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			approvals := &approvalLog{}
			rig := threeStepRig(t, approvals, nil)
			rig.requestAfter("agent-a", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			if approvals.readCount("b-out.md") == 0 {
				t.Error("want the raised step's output artifact checked for approval")
			}
			for _, file := range []string{"plan.md", "c-out.md"} {
				if n := approvals.readCount(file); n != 0 {
					t.Errorf("want no approval check of %s, got %d", file, n)
				}
			}
		})
	}
}

func TestSession_InRunHITL_RaisedStepNeverApproved_IsRedispatchedWithHITLThenEscalated(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			approvals := &approvalLog{answer: refuseFile("b-out.md")}
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("escalated")
			rig := threeStepRig(t, approvals, consultant)
			rig.queueOK("agent-b", 1)
			rig.requestAfter("agent-a", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			want := []string{"agent-a=false", "agent-b=true", "agent-b=true"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want one HITL re-dispatch of the raised step, then the escalation: %v, got %v", want, seq)
			}
			if consultant.CallCount != 1 {
				t.Errorf("want the escalation consulted once, got %d", consultant.CallCount)
			}
		})
	}
}

func TestSession_InRunHITL_RaisedStepRepairedOnRedispatch_RedispatchKeepsHITLAndNextStepIsNormal(t *testing.T) {
	approvals := &approvalLog{answer: func(file string, nth int) domain.HumanApproval {
		if file == "b-out.md" && nth == 1 {
			return domain.ApprovalFalse
		}
		return domain.ApprovalTrue
	}}
	rig := threeStepRig(t, approvals, nil)
	rig.queueOK("agent-b", 1)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := rig.startAuto(domain.ExecutionModeAuto)

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := []string{"agent-a=false", "agent-b=true", "agent-b=true", "agent-c=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want %v, got %v", want, seq)
	}
}

func TestSession_InRunHITL_StepThatAlreadyHasHITL_ConsumesTheRequestAndLeavesTheStepUnchanged(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			approvals := &approvalLog{}
			rig := newHITLRig(t, "linear3-b-hitl-orch.md", threeAgents, approvals, nil, nil)
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.queueOK("agent-c", 1)
			rig.requestAfter("agent-a", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			want := []string{"agent-a=false", "agent-b=true", "agent-c=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want %v, got %v", want, seq)
			}
			if rig.hitl.isPending() {
				t.Error("want the request consumed by a step that already has HITL")
			}
			if f := rig.appliedFor(domain.InRunRequestHITLNext); f == nil || f["prior_hitl"] != "true" {
				t.Errorf("want the application logged with prior_hitl=true, got %v", f)
			}
		})
	}
}

func TestSession_InRunHITL_E501AndPartiallyDoneRedispatchesOfCurrentStep_DoNotConsumeTheRequest(t *testing.T) {
	for _, name := range []string{"E501", "PARTIALLY_DONE"} {
		for _, mode := range autoModes {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				rig := newHITLRig(t, "linear3-orch.md", threeAgents, &approvalLog{}, nil, nil)
				if name == "E501" {
					rig.f.Queue("agent-a", e501Entry(), okEntry())
				} else {
					rig.f.Queue("agent-a", statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half"), okEntry())
				}
				rig.queueOK("agent-b", 1)
				rig.queueOK("agent-c", 1)
				rig.requestAfter("agent-a", rig.hitl)

				got, err := rig.startAuto(mode)

				requireRunStatus(t, got, err, domain.RunCompleted)
				want := []string{"agent-a=false", "agent-a=false", "agent-b=true", "agent-c=false"}
				if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
					t.Errorf("want the retry of agent-a unchanged and agent-b raised: %v, got %v", want, seq)
				}
			})
		}
	}
}

func TestSession_InRunHITL_RedispatchOfRaisedStep_KeepsHITL(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newHITLRig(t, "linear3-orch.md", threeAgents, &approvalLog{}, nil, nil)
			rig.queueOK("agent-a", 1)
			rig.f.Queue("agent-b", e501Entry(), okEntry())
			rig.queueOK("agent-c", 1)
			rig.requestAfter("agent-a", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			want := []string{"agent-a=false", "agent-b=true", "agent-b=true", "agent-c=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want the re-dispatch of the raised step to keep HITL: %v, got %v", want, seq)
			}
		})
	}
}

func TestSession_InRunHITL_InfrastructureTriggerAfterAgent_NeitherReceivesNorConsumesTheRequest(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newInRunRig(t, newManualUser(t), inRunOptions{
				fixture: "interval-agent-orch.md", agents: append([]string{"checkpoint-manager-git"}, linearAgents...),
				approvals: &approvalLog{},
			})
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.queueOK("checkpoint-manager-git", 4)
			rig.requestAfter("agent-a", rig.hitl)
			cfg := configForMode(rig.orchPath, mode)
			cfg.Checkpoints = true

			got, err := rig.ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunCompleted)
			seq := hitlSequence(rig.f)
			if len(seq) < 3 || !slices.Equal(seq[:3], []string{"agent-a=false", "checkpoint-manager-git=false", "agent-b=true"}) {
				t.Errorf("want the checkpoint unraised and the next workflow step raised, got %v", seq)
			}
			if rig.hitl.isPending() {
				t.Error("want the request consumed by the workflow step after the checkpoint")
			}
			if n := rig.appliedCount(); n != 1 {
				t.Errorf("want exactly one application logged, got %d", n)
			}
		})
	}
}

func TestSession_InRunHITL_RequestSetWhileLastAgentRuns_StaysPendingAtCompletion(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := threeStepRig(t, &approvalLog{}, nil)
			rig.requestAfter("agent-c", rig.hitl)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			if !rig.hitl.isPending() {
				t.Error("want the request left pending for the caller to discard")
			}
			if n := rig.appliedCount(); n != 0 {
				t.Errorf("want no application logged, got %d", n)
			}
		})
	}
}

func TestSession_InRunHITL_RequestCancelledBeforeNextDispatch_HasNoEffect(t *testing.T) {
	rig := threeStepRig(t, &approvalLog{}, nil)
	rig.afterInvoke = func(id string) {
		if id == "agent-a" {
			rig.hitl.request()
			rig.hitl.cancel()
		}
	}

	got, err := rig.startAuto(domain.ExecutionModeAuto)

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := []string{"agent-a=false", "agent-b=false", "agent-c=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want every step unraised: %v, got %v", want, seq)
	}
}

func TestSession_InRunHITL_Application_IsLoggedWithTheStepItWasAppliedTo(t *testing.T) {
	rig := threeStepRig(t, &approvalLog{}, nil)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := rig.startAuto(domain.ExecutionModeAuto)

	requireRunStatus(t, got, err, domain.RunCompleted)
	f := rig.appliedFor(domain.InRunRequestHITLNext)
	if f == nil {
		t.Fatalf("want a %q debug entry for the HITL request, got events %v",
			domain.EventSessionRequestApplied, rig.logger.allEvents())
	}
	if !strings.Contains(f["agent"], "agent-b") {
		t.Errorf("want the dispatched agent agent-b named, got %q", f["agent"])
	}
	if _, ok := f["stage"]; !ok {
		t.Error("want the stage field present in the application entry")
	}
	if f["row"] != "2" {
		t.Errorf("want the 1-based row 2, got %q", f["row"])
	}
	if f["prior_hitl"] != "false" {
		t.Errorf("want prior_hitl=false, got %q", f["prior_hitl"])
	}
	if n := rig.appliedCount(); n != 1 {
		t.Errorf("want one application logged, got %d", n)
	}
}

func TestSession_InRunHITL_NoRequestPending_EveryStepKeepsItsTableHITL(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			approvals := &approvalLog{}
			rig := newHITLRig(t, "linear3-b-hitl-orch.md", threeAgents, approvals, nil, nil)
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.queueOK("agent-c", 1)

			got, err := rig.startAuto(mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			want := []string{"agent-a=false", "agent-b=true", "agent-c=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want %v, got %v", want, seq)
			}
			if n := rig.appliedCount(); n != 0 {
				t.Errorf("want no application logged, got %d", n)
			}
		})
	}
}
