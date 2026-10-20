package session_test

// Tests that the in-run manual-dispatch request is not used up by the
// mechanical follow-ups of the running step (HITL repair, raw-text bypass,
// schedule-defect re-dispatch, E501 retries) and that the consultations
// entered after those paths run out are replaced by the dialogue.

import (
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const inRunHITLFixture = "hitl-linear-orch.md"

func inRunHITLRig(t *testing.T, user *manualUser, approvals domain.ApprovalReader, consultant *scriptedRoutingConsultant) *inRunRig {
	t.Helper()
	return newInRunRig(t, user, inRunOptions{
		fixture: inRunHITLFixture, agents: linearAgents, approvals: approvals, consultant: consultant,
	})
}

// fileNameApprovalReader keys approvals by the artifact's file name, because
// the session reads run-scoped paths (Orchestration-<run>/result.md).
type fileNameApprovalReader struct {
	byName   map[string]domain.HumanApproval
	fallback domain.HumanApproval
}

func (r *fileNameApprovalReader) ReadApproval(_ context.Context, artifact string) domain.HumanApproval {
	if v, ok := r.byName[path.Base(strings.ReplaceAll(artifact, "\\", "/"))]; ok {
		return v
	}
	return r.fallback
}

func TestSession_InRunManual_HITLRepairRedispatch_RunsBeforeTheDialogueOpens(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			approvals := &switchingApprovalReader{firstApproval: domain.ApprovalFalse, restApproval: domain.ApprovalTrue}
			rig := inRunHITLRig(t, linearDialogue(t, "1", "redo"), approvals, nil)
			rig.queueOK("agent-a", 3)
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if want := []string{"agent-a", "agent-a", "agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Fatalf("want the repair, then the dialogue's agent-a, then agent-b: %v, got %v", want, invokedAgents(rig.f))
			}
			if want := []bool{false, false, true, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
				t.Errorf("want the dialogue to open only after the accepted repair: %v, got %v", want, rig.dialogueOpenAtInvoke)
			}
		})
	}
}

func TestSession_InRunManual_HITLEscalationConsultation_IsReplacedByTheDialogueAppliedOnce(t *testing.T) {
	approvals := &fileNameApprovalReader{
		byName:   map[string]domain.HumanApproval{"plan.md": domain.ApprovalFalse, "result.md": domain.ApprovalTrue},
		fallback: domain.ApprovalFalse,
	}
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "draft the plan", 0)
	consultant.queueStop("after the manual step")
	rig := inRunHITLRig(t, linearDialogue(t, "2", "do b now"), approvals, consultant)
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []string{"agent-a", "agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Fatalf("want the repair, then the dialogue's agent-b in place of the escalation consultation: %v, got %v",
			want, invokedAgents(rig.f))
	}
	if want := []bool{false, false, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
		t.Errorf("want the dialogue open only for agent-b: %v, got %v", want, rig.dialogueOpenAtInvoke)
	}
	if consultant.CallCount != 2 {
		t.Errorf("want the consultant asked at the first and the final decision only, got %d", consultant.CallCount)
	}
	if n := rig.user.rowQuestions(); n != 1 {
		t.Errorf("want one dialogue for one request, got %d", n)
	}
	if n := rig.appliedCount(); n != 1 {
		t.Errorf("want the request applied once, got %d", n)
	}
}

func TestSession_InRunManual_RawTextBypassAttempt_DoesNotConsumeTheRequest(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newInRunRig(t, linearDialogue(t, "1", "redo"), inRunOptions{
				fixture: "linear-orch.md", agents: linearAgents, consultant: &scriptedRoutingConsultant{},
			})
			bypassed := harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrProtocolNotExtractable)}
			rig.f.Queue("agent-a", bypassed, okEntry(), okEntry())
			rig.queueOK("agent-b", 1)
			rig.requestOnceBefore("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if want := []string{"agent-a", "agent-a", "agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Fatalf("want the bypass, then the dialogue's agent-a, then agent-b: %v, got %v", want, invokedAgents(rig.f))
			}
			if want := []bool{false, false, true, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
				t.Errorf("want the dialogue to open only after the bypass attempt: %v, got %v", want, rig.dialogueOpenAtInvoke)
			}
		})
	}
}

func TestSession_InRunManual_E501BudgetExhausted_DialogueReplacesTheConsultation(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			rig := newInRunRig(t, linearDialogue(t, "1", "redo"), inRunOptions{
				fixture: "linear-orch.md", agents: linearAgents, consultant: consultant,
			})
			queueRepeated(rig.f, "agent-a", engine.E501AttemptLimit, e501Entry())
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.requestOnceBefore("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if n := countInvocationsFor(rig.f.Invocations(), "agent-a"); n != engine.E501AttemptLimit+1 {
				t.Fatalf("want the budget of attempts plus the dialogue's agent-a, got %d", n)
			}
			if consultant.CallCount != 0 {
				t.Errorf("want the dialogue instead of the budget-exhaustion consultation, got %d consultations", consultant.CallCount)
			}
			if n := rig.user.rowQuestions(); n != 1 {
				t.Errorf("want one dialogue, got %d", n)
			}
		})
	}
}

// guardRequestingConsultant answers every consultation with a dispatch of
// agent-a at row 0 and makes the in-run request while it answers the call that
// the anti-loop guard blocks, so the request is pending for the escalation.
type guardRequestingConsultant struct {
	flag      *requestFlag
	blockedAt int
	calls     int
}

func (c *guardRequestingConsultant) ConsultRouting(_ context.Context, _ domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.calls++
	if c.calls == c.blockedAt {
		c.flag.request()
	}
	if c.calls > c.blockedAt {
		return domain.RoutingInstruction{Stop: &domain.StopInstruction{Reason: "after the manual step"}}, nil
	}
	return domain.RoutingInstruction{Dispatch: &domain.DispatchInstruction{
		Agent: "agent-a", RowIndex: 0, TaskDescription: "again",
	}}, nil
}

func TestSession_InRunManual_AntiLoopEscalationConsultation_IsReplacedByTheDialogue(t *testing.T) {
	consultant := &guardRequestingConsultant{blockedAt: guardBound + 1}
	rig := newInRunRig(t, linearDialogue(t, "2", "do b now"), inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents, routing: consultant,
	})
	consultant.flag = rig.flag
	rig.queueOK("agent-a", guardBound)
	rig.queueOK("agent-b", 1)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := append(slices.Repeat([]string{"agent-a"}, guardBound), "agent-b")
	if !slices.Equal(invokedAgents(rig.f), want) {
		t.Fatalf("want the dialogue's agent-b once the guard blocks the repeated dispatch: %v, got %v", want, invokedAgents(rig.f))
	}
	if n := rig.user.rowQuestions(); n != 1 {
		t.Errorf("want one dialogue, got %d", n)
	}
}

// scheduleDefectRig is a block-workflow session whose planner writes a
// defective schedule, with the in-run request wired in and made while the
// planner first runs.
func scheduleDefectRig(t *testing.T, defective int) (*blockRig, *inRunRig) {
	t.Helper()
	return scheduleDefectRigWith(t, defective, true)
}

// scheduleDefectRigWith is scheduleDefectRig with the routing consultant
// optionally left unset.
func scheduleDefectRigWith(t *testing.T, defective int, withRouting bool) (*blockRig, *inRunRig) {
	t.Helper()
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
	rig.queueDefectivePlanner(defective)
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueProducer("planner", "again", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	flag := &requestFlag{}
	probe := &inRunRig{flag: flag, user: linearDialogue(t, "1", "redo the plan")}
	opened := false
	before := &beforeInvokeHarness{delegate: rig.adapter, before: func(id string) {
		probe.dialogueOpenAtInvoke = append(probe.dialogueOpenAtInvoke, probe.user.rowQuestions() > 0)
		if id == "planner" && !opened {
			opened = true
			flag.request()
		}
	}}
	deps := session.Deps{
		Harness: before, Store: rig.store, Clock: fixedClock{t: epoch},
		Interact: &noopInteraction{}, Manual: &deviation.ManualResolver{Interact: probe.user},
		TakeManualDispatchRequest: flag.take,
	}
	if withRouting {
		deps.Routing = rig.consult
	}
	rig.ses = session.New(deps)
	return rig, probe
}

func TestSession_InRunManual_ScheduleDefectRedispatch_RunsBeforeTheDialogueOpens(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, probe := scheduleDefectRig(t, 1)

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			order := rig.dispatchOrder()
			if len(order) < 3 || !slices.Equal(order[:3], []string{"planner", "planner", "planner"}) {
				t.Fatalf("want the defect re-dispatch, then the dialogue's planner: %v", order)
			}
			if want := []bool{false, false, true}; !slices.Equal(probe.dialogueOpenAtInvoke[:3], want) {
				t.Errorf("want the dialogue to open only after the corrected schedule: %v, got %v",
					want, probe.dialogueOpenAtInvoke[:3])
			}
		})
	}
}

func TestSession_InRunManual_ScheduleDefectConsultation_IsReplacedByTheDialogue(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, probe := scheduleDefectRig(t, 3)

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if rig.consult.CallCount != 0 {
				t.Errorf("want the dialogue instead of the schedule-defect consultation, got %d consultations", rig.consult.CallCount)
			}
			if n := probe.user.rowQuestions(); n != 1 {
				t.Errorf("want one dialogue, got %d", n)
			}
			if want := []string{"planner", "planner", "planner", "planner"}; !slices.Equal(rig.dispatchOrder()[:4], want) {
				t.Errorf("want three producer attempts, then the dialogue's planner: %v", rig.dispatchOrder())
			}
		})
	}
}

func TestSession_InRunManual_ScheduleDefectConsultation_OpensTheDialogueWithoutARoutingConsultant(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, probe := scheduleDefectRigWith(t, 3, false)

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if n := probe.user.rowQuestions(); n != 1 {
				t.Errorf("want the dialogue opened once although no routing consultant is set, got %d", n)
			}
			want := []string{"planner", "planner", "planner", "planner"}
			if order := rig.dispatchOrder(); len(order) < 4 || !slices.Equal(order[:4], want) {
				t.Errorf("want three producer attempts, then the dialogue's planner: %v", order)
			}
		})
	}
}
