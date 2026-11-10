package session_test

// The serialized form of the requests a session sends to workflow agents:
// whatever the dispatch kind, input_artifacts and output_artifacts reach the
// harness as JSON arrays, `[]` when empty, never null. Each kind below has its
// own test; the infrastructure kinds are in session_request_wire_infra_test.go.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// Engine-routed dispatches of rows with an empty Input cell, an empty Output
// cell, or both.
func TestSession_EngineRoutedDispatch_EmptyInputOrOutputCell_SerializesBothListsAsArrays(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			ses, f, orchPath := newWireSession(t, nil)
			queueWireSuccess(f, "agent-a", "agent-b", "agent-c")

			got, err := ses.Start(context.Background(), wireConfig(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			invs := f.Invocations()
			if len(invs) != 3 {
				t.Fatalf("want 3 dispatches, got %d", len(invs))
			}
			requireWireArraysOfAll(t, invs)
		})
	}
}

// Consultant-routed dispatches: the consultant names no inputs or outputs, so
// the request lists are built from empty row defaults and deduplicated paths.
func TestSession_ConsultantRoutedDispatch_SerializesBothListsAsArrays(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "do the first row", 0)
	consultant.queueDispatch("agent-b", "do the second row", 1)
	consultant.queueStop("done")
	ses, f, orchPath := newWireSession(t, consultant)
	queueWireSuccess(f, "agent-a", "agent-b")

	got, err := ses.Start(context.Background(), wireConfig(orchPath, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	invs := f.Invocations()
	if len(invs) != 2 {
		t.Fatalf("want 2 consultant-routed dispatches, got %d", len(invs))
	}
	requireWireArraysOfAll(t, invs)
}

// A consultant that names explicit empty lists gets arrays as well.
func TestSession_ConsultantRoutedDispatch_ExplicitEmptyLists_SerializeAsArrays(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	empty := []string{}
	consultant.queueDispatchWithInputs("agent-a", "do the first row", 0, &empty)
	consultant.queueDispatchWithOutputs("agent-b", "do the second row", 1, &empty)
	consultant.queueStop("done")
	ses, f, orchPath := newWireSession(t, consultant)
	queueWireSuccess(f, "agent-a", "agent-b")

	got, err := ses.Start(context.Background(), wireConfig(orchPath, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireWireArraysOfAll(t, f.Invocations())
}

// The mechanical re-dispatch after PARTIALLY_DONE, in both auto modes.
func TestSession_PartiallyDoneRedispatch_SerializesBothListsAsArrays(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			ses, f, orchPath := newWireSession(t, &scriptedRoutingConsultant{})
			f.Queue("agent-a",
				statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half done"),
				statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"),
			)
			queueWireSuccess(f, "agent-b", "agent-c")

			got, err := ses.Start(context.Background(), wireConfig(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if n := requireWireArraysOfAgent(t, f.Invocations(), "agent-a"); n != 2 {
				t.Errorf("want the original dispatch and one re-dispatch of agent-a, got %d", n)
			}
		})
	}
}

// The re-dispatch of the E501 raw-text bypass copies the original request.
func TestSession_RawTextBypassRedispatch_SerializesBothListsAsArrays(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			ses, f, orchPath := newWireSession(t, &scriptedRoutingConsultant{})
			f.Queue("agent-a",
				rawTextEntry(),
				statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done on bypass"),
			)
			queueWireSuccess(f, "agent-b", "agent-c")

			got, err := ses.Start(context.Background(), wireConfig(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if n := requireWireArraysOfAgent(t, f.Invocations(), "agent-a"); n != 2 {
				t.Errorf("want the original dispatch and the bypass dispatch of agent-a, got %d", n)
			}
		})
	}
}

// The re-dispatch of an E501 reply (a plain retry of the same row).
func TestSession_E501Redispatch_SerializesBothListsAsArrays(t *testing.T) {
	ses, f, orchPath := newWireSession(t, &scriptedRoutingConsultant{})
	f.Queue("agent-a",
		statusEntry(domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "tool unavailable"),
		statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"),
	)
	queueWireSuccess(f, "agent-b", "agent-c")

	got, err := ses.Start(context.Background(), wireConfig(orchPath, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := requireWireArraysOfAgent(t, f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the original dispatch and one retry of agent-a, got %d", n)
	}
}

// The re-dispatch of the producing row after it wrote a defective schedule.
func TestSession_ScheduleDefectRedispatch_SerializesBothListsAsArrays(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
			rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
			rig.queueDefectivePlanner(1)
			rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			rig.queueOKTimes("auditor", 2)
			rig.queueOK("merger")

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			planner := rig.invocationsFor("planner")
			if len(planner) != 2 {
				t.Fatalf("want the planner dispatched twice (original and defect re-dispatch), got %d", len(planner))
			}
			requireWireArtifactArrays(t, "planner defect re-dispatch", planner[1].Request)
			requireWireArraysOfAll(t, rig.adapter.Invocations())
		})
	}
}

// A restore-class agent has no automatic trigger path; a consultant reaches it
// through its routing row.
func TestSession_RestoreClassAgentDispatchedByConsultant_SerializesBothListsAsArrays(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "do the first row", 0)
	consultant.queueDispatch("checkpoint-restore-git", "restore the checkpoint", 1)
	consultant.queueStop("done")
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "request-wire-restore-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "checkpoint-restore-git")
	f := harness.NewMockAdapter()
	ses := newWireSessionOn(f, consultant)
	queueWireSuccess(f, "agent-a", "checkpoint-restore-git")
	cfg := wireConfig(orchPath, domain.ExecutionModeOrchestrated)
	cfg.WorkflowID = "request-wire-restore"

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireWireArraysOfAgent(t, f.Invocations(), "checkpoint-restore-git")
}
