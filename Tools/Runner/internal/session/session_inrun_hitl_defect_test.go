package session_test

// Tests that a schedule-defect re-dispatch of the step currently running
// neither receives nor consumes the in-run HITL request: the re-dispatch keeps
// the step's own HITL value and the next new workflow step is raised.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// newHITLRequestBlockRig is a block rig over fixture with the in-run HITL
// request wired in and a hook that runs after every successful harness call.
func newHITLRequestBlockRig(t *testing.T, fixture string, hitl *requestFlag, afterInvoke func(string),
	agents ...string) *blockRig {
	t.Helper()
	parent := t.TempDir()
	orchPath := copyOrchestratorFile(t, parent, fixture)
	for _, a := range agents {
		writeAgentFile(t, parent, a)
	}
	runFolder := filepath.Join(parent, domain.RunScopedFolder(testRunID))
	if err := os.MkdirAll(runFolder, 0o700); err != nil {
		t.Fatalf("create run folder: %v", err)
	}
	f := harness.NewMockAdapter()
	f.SetWriteRoot(parent)
	store := &memStore{}
	consult := &scriptedRoutingConsultant{}
	ses := session.New(session.Deps{
		Harness:         &callbackHarness{delegate: f, onInvoke: afterInvoke},
		Store:           store,
		Routing:         consult,
		Approvals:       &approvalLog{},
		Clock:           fixedClock{t: epoch},
		Interact:        &noopInteraction{},
		TakeHITLRequest: hitl.take,
	})
	return &blockRig{ses: ses, adapter: f, store: store, consult: consult, orchPath: orchPath, runFolder: runFolder}
}

func TestSession_InRunHITL_ScheduleDefectRedispatchOfRaisedStep_KeepsHITL(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			hitl := &requestFlag{}
			rig := newHITLRequestBlockRig(t, "audit-plan-block-orch.md", hitl, func(string) {},
				"planner", "auditor", "merger")
			hitl.request()
			rig.queueDefectivePlanner(1)
			rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			rig.queueOKTimes("auditor", 2)
			rig.queueOK("merger")

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if h := rig.hitlOf("planner"); !slices.Equal(h, []bool{true, true}) {
				t.Errorf("want the defective attempt and its re-dispatch both raised, got %v", h)
			}
			if h := rig.hitlOf("auditor"); !slices.Equal(h, []bool{false, false}) {
				t.Errorf("want the request used up before the auditor, got %v", h)
			}
		})
	}
}

func TestSession_InRunHITL_ScheduleDefectRedispatch_NeitherReceivesNorConsumesTheRequest(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			hitl := &requestFlag{}
			done := false
			rig := newHITLRequestBlockRig(t, "audit-plan-block-orch.md", hitl, func(id string) {
				if id == "planner" && !done {
					done = true
					hitl.request()
				}
			}, "planner", "auditor", "merger")
			rig.queueDefectivePlanner(1)
			rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			rig.queueOKTimes("auditor", 2)
			rig.queueOK("merger")

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if h := rig.hitlOf("planner"); !slices.Equal(h, []bool{false, false}) {
				t.Errorf("want the defective attempt and its re-dispatch both unraised, got %v", h)
			}
			if h := rig.hitlOf("auditor"); !slices.Equal(h, []bool{true, false}) {
				t.Errorf("want only the first new step after the re-dispatch raised, got %v", h)
			}
			if h := rig.hitlOf("merger"); !slices.Equal(h, []bool{false}) {
				t.Errorf("want the request used up before the merger, got %v", h)
			}
			if hitl.isPending() {
				t.Error("want the request consumed by the next new step")
			}
		})
	}
}
