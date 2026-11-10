package session_test

// Tests of the schedule re-read for rows that need human approval and for rows
// a routing consultant selected: the block runs the schedule the producing row
// wrote. The re-read before the approval of the producing row is resolved is
// observable through the iterations the block then runs. Without an output
// write detector, a wildcard output is reported as the existing files that
// match it, whatever the schedule holds.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"mosaic-run/internal/domain"
)

// blockApprovalRecorder answers every path with approval and remembers the
// paths it was asked about.
type blockApprovalRecorder struct {
	mu       sync.Mutex
	approval domain.HumanApproval
	paths    []string
}

func (r *blockApprovalRecorder) ReadApproval(_ context.Context, path string) domain.HumanApproval {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, path)
	return r.approval
}

// asked reports whether a path with the given suffix was asked about.
func (r *blockApprovalRecorder) asked(suffix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.paths {
		if strings.HasSuffix(strings.ReplaceAll(p, `\`, "/"), suffix) {
			return true
		}
	}
	return false
}

// TestSession_Start_ProducerNeedingApproval_BlockRunsTheScheduleItWrote
// verifies that a planner row that needs approval and outputs the declared
// Source hands the block the schedule it wrote during the dispatch, not the
// one on disk at start.
func TestSession_Start_ProducerNeedingApproval_BlockRunsTheScheduleItWrote(t *testing.T) {
	approved := &fixedApprovalReader{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "audit-plan-block-hitl-orch.md", approved, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("old")))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B", "C"))))
	rig.queueOKTimes("auditor", 3)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block-hitl", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B", "C"})
}

// TestSession_Start_ConsultedRoute_ReadsScheduleWrittenByPreviousRow verifies
// that, in orchestrated mode, an iteration the consultant selects after the
// producing row is validated against the schedule that row wrote: the on-disk
// schedule at start holds only "old", so iteration B is selectable only if the
// schedule was read again after the planner.
func TestSession_Start_ConsultedRoute_ReadsScheduleWrittenByPreviousRow(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("old")))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item B", 1, "B")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"B"})
}

// TestSession_Start_ConsultedRouteNeedingApproval_ReadsScheduleWrittenByPreviousRow
// is the same check when the consulted producer row needs approval.
func TestSession_Start_ConsultedRouteNeedingApproval_ReadsScheduleWrittenByPreviousRow(t *testing.T) {
	approved := &fixedApprovalReader{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "audit-plan-block-hitl-orch.md", approved, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("old")))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item B", 1, "B")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block-hitl", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"B"})
}

// requireAsked asserts that the recorder was asked about every given path.
func requireAsked(t *testing.T, approvals *blockApprovalRecorder, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !approvals.asked(want) {
			t.Errorf("want the approval of %s asked for, got paths %q", want, approvals.paths)
		}
	}
}

// TestSession_Start_ProducerNeedingApproval_ApprovalAskedForTheNotesItWrote
// verifies that the approval of a producing row is asked for the files its
// Stage-*/notes.md output matches. No detector is wired, so the written
// outputs are the existing files that match the declared wildcard.
func TestSession_Start_ProducerNeedingApproval_ApprovalAskedForTheNotesItWrote(t *testing.T) {
	approvals := &blockApprovalRecorder{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "audit-plan-block-stagestar-hitl-orch.md", approvals, "planner", "auditor")
	rig.queueOK("planner",
		runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))),
		runFileWrite("Stage-A/notes.md", "# notes A"),
		runFileWrite("Stage-B/notes.md", "# notes B"))
	rig.queueOKTimes("auditor", 2)

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block-stagestar-hitl", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAsked(t, approvals, "Stage-A/notes.md", "Stage-B/notes.md")
}

// TestSession_Start_ConsultedProducerNeedingApproval_ApprovalAskedForTheNotesItWrote
// is the same check for a producing row the routing consultant selected.
func TestSession_Start_ConsultedProducerNeedingApproval_ApprovalAskedForTheNotesItWrote(t *testing.T) {
	approvals := &blockApprovalRecorder{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "audit-plan-block-stagestar-hitl-orch.md", approvals, "planner", "auditor")
	rig.queueOK("planner",
		runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))),
		runFileWrite("Stage-A/notes.md", "# notes A"),
		runFileWrite("Stage-B/notes.md", "# notes B"))
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block-stagestar-hitl", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireAsked(t, approvals, "Stage-A/notes.md", "Stage-B/notes.md")
}
