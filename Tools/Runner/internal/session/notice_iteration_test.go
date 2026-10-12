package session_test

// The progress notices the session emits for a dispatched step carry the
// iteration of the step under the key "iteration", in the 3.0 value form
// `{ID}[.{Group}]` (an empty value for a row outside every block); no notice
// uses the key "stage".

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

// stepNotices returns the progress notices (running and completion) of the
// dispatches of agent, in emission order.
func stepNotices(c *noticeCapturingInteraction, agent string) []interaction.Notice {
	var out []interaction.Notice
	for _, n := range c.allNotices() {
		if strings.HasPrefix(n.Title, agent+"#") && strings.HasPrefix(n.Message, "phase=") {
			out = append(out, n)
		}
	}
	return out
}

// requireNoticesCarry asserts that agent has at least one running and one
// completion notice, that every one carries wantIteration under the key
// "iteration", and that none uses the key "stage".
func requireNoticesCarry(t *testing.T, c *noticeCapturingInteraction, agent, wantIteration string) {
	t.Helper()
	notices := stepNotices(c, agent)
	var running, completed int
	for _, n := range notices {
		if strings.HasSuffix(n.Message, "status=running") {
			running++
		} else {
			completed++
		}
		if want := `iteration="` + wantIteration + `"`; !strings.Contains(n.Message, want) {
			t.Errorf("%s notice %q does not contain %s", agent, n.Message, want)
		}
		if strings.Contains(n.Message, "stage=") {
			t.Errorf("%s notice %q still uses the stage key", agent, n.Message)
		}
	}
	if running == 0 || completed == 0 {
		t.Errorf("want a running and a completion notice for %s, got %d running and %d completed: %+v",
			agent, running, completed, notices)
	}
}

// startWithNotices replaces the rig's session by one that records its notices.
func (r *blockRig) startWithNotices() *noticeCapturingInteraction {
	notices := &noticeCapturingInteraction{}
	r.ses = session.New(session.Deps{
		Harness:  r.adapter,
		Store:    r.store,
		Routing:  r.consult,
		Clock:    fixedClock{t: epoch},
		Interact: notices,
	})
	return notices
}

func TestSessionNotices_GroupedBlockRow_CarryIdAndGroupUnderIterationKey(t *testing.T) {
	rig := newBlockRig(t, "notice-iteration-grouped-orch.md", nil, "test-writer", "tests-review", "reporter")
	plan := "# Plan\n\n## Stages\n\n| Stage | Name | Goal | HITL | Approach |\n|---|---|---|:---:|---|\n" +
		"| 1 | One | Goal | FALSE | Tests-Only |\n"
	rig.writeRunFile(t, "Plan.md", plan)
	rig.queueOK("test-writer")
	rig.queueOK("tests-review")
	rig.queueOK("reporter")
	notices := rig.startWithNotices()

	got, err := rig.ses.Start(context.Background(), rig.config("notice-iteration-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireNoticesCarry(t, notices, "test-writer", "1.Test")
	requireNoticesCarry(t, notices, "tests-review", "1.Test")
	requireNoticesCarry(t, notices, "reporter", "")
}

func TestSessionNotices_UngroupedBlockRow_CarryTheIDUnderIterationKey(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"numeric ID", "1"},
		{"non-numeric ID", "A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newBlockRig(t, "seeded-plan-first-block-orch.md", nil, "implementer", "reviewer", "test-runner")
			rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids(tc.id)))
			rig.queueOK("implementer")
			rig.queueOK("reviewer")
			rig.queueOK("test-runner")
			notices := rig.startWithNotices()

			got, err := rig.ses.Start(context.Background(),
				rig.config("seeded-plan-first-block", domain.ExecutionModeAuto))

			requireRunStatus(t, got, err, domain.RunCompleted)
			requireNoticesCarry(t, notices, "implementer", tc.id)
			requireNoticesCarry(t, notices, "reviewer", tc.id)
			requireNoticesCarry(t, notices, "test-runner", "")
		})
	}
}

// startConsulted replaces the rig's session by one that records its notices.
func (r *rowStageRig) startConsulted(consultant domain.RoutingConsultant, approvals domain.ApprovalReader) *noticeCapturingInteraction {
	notices := &noticeCapturingInteraction{}
	r.ses = session.New(session.Deps{
		Harness:   r.f,
		Store:     r.store,
		Routing:   consultant,
		Approvals: approvals,
		Clock:     fixedClock{t: epoch},
		Interact:  notices,
	})
	return notices
}

func TestSessionNotices_ConsultedGroupedBlockRow_CarryIdAndGroupUnderIterationKey(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStagedDispatch("implementer", "implement stage 2", rsImplementer, "2")
	consultant.queueStop("done")
	rig := newRowStageRig(t, consultant, domain.ExecutionModeOrchestrated, rowStageSeed("test-writer", 3, "EXECUTION", "1.Test"))
	queueSuccess(rig.f, "implementer", "implementer#3")
	notices := rig.startConsulted(consultant, nil)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireNoticesCarry(t, notices, "implementer", "2.Implementation")
}

func TestSessionNotices_ConsultedRowOutsideEveryBlock_CarryAnEmptyIteration(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("plan-review", "re-review the plan", rsPlanReview)
	consultant.queueStop("done")
	rig := newRowStageRig(t, consultant, domain.ExecutionModeOrchestrated, rowStageSeed("impl-review", 6, "EXECUTION", "3.Implementation"))
	queueSuccess(rig.f, "plan-review", "plan-review#3")
	notices := rig.startConsulted(consultant, nil)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireNoticesCarry(t, notices, "plan-review", "")
}

func TestSessionNotices_ConsultedHITLGatedGroupedRow_CarryIdAndGroupUnderIterationKey(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStagedDispatch("implementer", "implement stage 2", rsImplementer, "2")
	consultant.queueStop("done")
	approvals := &switchingApprovalReader{firstApproval: domain.ApprovalFalse, restApproval: domain.ApprovalTrue}
	rig := newRowStageRigWith(t, consultant, domain.ExecutionModeOrchestrated,
		rowStageSeed("test-writer", 3, "EXECUTION", "1.Test"),
		rowStageOpts{fixture: "consult-row-stage-hitl-orch.md", approvals: approvals})
	queueSuccess(rig.f, "implementer", "implementer#3")
	queueSuccess(rig.f, "implementer", "implementer#4")
	notices := rig.startConsulted(consultant, approvals)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireNoticesCarry(t, notices, "implementer", "2.Implementation")
}
