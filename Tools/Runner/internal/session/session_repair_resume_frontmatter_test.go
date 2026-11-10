package session_test

// A resume whose Orchestration.md has a damaged frontmatter. The frontends
// give the run configuration the workflow the last-good copy records, so the
// repair at run start is made as for any other damage; without a workflow in
// the configuration the run is refused (see the refusal test in
// session_repair_resume_paths_test.go).

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// frontmatterDamagedRun resumes the incident run with the frontmatter's type
// line removed, so the frontmatter itself is damaged; the workflow is in the
// run configuration.
func frontmatterDamagedRun(t *testing.T) func(*repairRig) {
	return func(r *repairRig) {
		withDamagedRun(t, startOpts{edit: func(doc string) string {
			return replaceOnce(t, doc, "type: orchestration-artifact\n", "")
		}})(r)
		r.cfg.WorkflowID = "linear"
	}
}

// restoredIncidentWith is the repair of the damaged incident (frontmatter
// included) followed by one more change.
func restoredIncidentWith(old, repl string) docEdit {
	return func(t *testing.T, _ string) string {
		t.Helper()
		return replaceOnce(t, restoreIncident(t, ""), old, repl)
	}
}

func TestSession_ResumeWithDamagedFrontmatterAndAWorkflow_RepairsAndCarriesOutTheAcceptedDispatch(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: restoreIncident, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("done")},
	}, frontmatterDamagedRun(t))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	req := rig.cons.request(0)
	report := requireDamageReport(t, req)
	requireStartTriggersNull(t, req)
	if len(report.Problems) == 0 || strings.TrimSpace(report.LastGoodCopy) == "" {
		t.Errorf("want the problems and the last-good copy in the report, got %+v", report)
	}
	if names := rig.cons.damagedCopyNames(); len(names) == 0 {
		t.Error("want the damaged copy saved before the repair consultation")
	}
	for i := 1; i < rig.cons.calls(); i++ {
		requireNoDamageReport(t, rig.cons.request(i), "consultation after the repair")
	}
	if want := []string{"agent-a#84"}; !slices.Equal(invokedInstances(rig.f), want) {
		t.Errorf("want the accepted repair's dispatch carried out, got %v", invokedInstances(rig.f))
	}
}

func TestSession_ResumeWithDamagedFrontmatterRepairedWithoutTheFrontmatter_IsNotAccepted(t *testing.T) {
	// The reply names a dispatch, but the artifact still lacks its type line.
	rig := newRepairRig(t, []repairStep{
		{edit: func(t *testing.T, _ string) string {
			return replaceOnce(t, restoreIncident(t, ""), "type: orchestration-artifact\n", "")
		}, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("must not be reached")},
	}, frontmatterDamagedRun(t))

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch from a repair that left the frontmatter damaged, got %v", invokedInstances(rig.f))
	}
	if rig.cons.calls() != 2 {
		t.Errorf("want a second repair consultation after the rejected one, got %d", rig.cons.calls())
	}
}

func TestSession_StartRepairOfDamagedFrontmatterRejectedTwice_StopsWithTheReportAndPaths(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{reply: dispatchReply("agent-a", 0)},
		{reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("must not be reached")},
	}, frontmatterDamagedRun(t))

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedRejected || unrepaired.Consultations != 2 {
		t.Errorf("want reason rejected after 2 consultations, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if len(unrepaired.Problems) == 0 {
		t.Error("want the problems in the error")
	}
	requireEvidenceKept(t, unrepaired)
	for _, path := range []string{unrepaired.DamagedCopy, unrepaired.LastGoodCopy, rig.run.artifactPath} {
		if !strings.Contains(got.Message, path) {
			t.Errorf("want the outcome message to name %q, got %q", path, got.Message)
		}
	}
	if rig.cons.calls() != 2 || len(rig.f.Invocations()) != 0 {
		t.Errorf("want two consultations and no dispatch, got %d and %v", rig.cons.calls(), invokedInstances(rig.f))
	}
}

func TestSession_AcceptedRepairOfDamagedFrontmatterThatChangesWorkflowOrMode_IsRefusedNamingBothValues(t *testing.T) {
	tests := []struct {
		name      string
		old, repl string
		wantBoth  [2]string
	}{
		{"workflow", "workflow: linear\n", "workflow: other-flow\n", [2]string{"linear", "other-flow"}},
		{"mode", "runner_mode: orchestrated", "runner_mode: auto", [2]string{"orchestrated", "auto"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rig := newRepairRig(t, []repairStep{
				{edit: restoredIncidentWith(tc.old, tc.repl), reply: dispatchReply("agent-a", 0)},
				{reply: stopReply("must not be reached")},
			}, frontmatterDamagedRun(t))

			got := rig.start(t)

			requireRefused(t, got, nil)
			for _, v := range tc.wantBoth {
				if !strings.Contains(got.Message, v) {
					t.Errorf("want the refusal to name %q, got %q", v, got.Message)
				}
			}
			if rig.cons.calls() != 1 || len(rig.f.Invocations()) != 0 {
				t.Errorf("want one consultation and no dispatch, got %d and %v", rig.cons.calls(), invokedInstances(rig.f))
			}
		})
	}
}

func TestSession_AcceptedRepairOfDamagedFrontmatterWithAnotherRunIdentity_IsRefusedAsAnyResumeIs(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: restoredIncidentWith("run_id: "+testRunID, "run_id: 20260101T000000Z-abcd"), reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("must not be reached")},
	}, frontmatterDamagedRun(t))

	got := rig.start(t)

	requireRefused(t, got, nil)
	var idErr *domain.RunIdentityError
	if !errors.As(got.Cause, &idErr) {
		t.Errorf("want the run identity refusal as the cause, got %T (%v)", got.Cause, got.Cause)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch on a refused run, got %v", invokedInstances(rig.f))
	}
}
