package session_test

// Tests that a manual step chosen through the in-run request at an
// engine-routed decision (auto, auto-review) is recorded and dispatched exactly
// like the manual step of the stop screen.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// firstWorkflowStep returns the first recorded step that is not infrastructure.
func firstWorkflowStep(t *testing.T, store *memStore) domain.CompletedStep {
	t.Helper()
	for _, s := range store.Applied {
		if !s.IsInfrastructure {
			return s
		}
	}
	t.Fatalf("want a recorded workflow step, got %+v", store.Applied)
	return domain.CompletedStep{}
}

// newEngineParityRig is the resumed manual-routing run of the stop-screen
// parity case, in the given mode with the in-run request pending from the
// start, so the first engine dispatch decision is the manual one.
func newEngineParityRig(t *testing.T, mode domain.ExecutionMode) (session.Session, *harness.MockAdapter, *memStore, domain.RunConfig) {
	t.Helper()
	user := newManualUser(t).completeDialogue("3", "docs/extra.md")
	base := newManualRig(t, user, nil)
	flag := &requestFlag{}
	flag.request()
	f := harness.NewMockAdapter()
	state := base.store.state
	state.RunSettings.Mode = mode
	store := &memStore{state: state, exists: true}
	queueSuccess(f, "build-review", "build-review#3")
	ses := session.New(session.Deps{
		Harness: f, Store: store, Manual: &deviation.ManualResolver{Interact: user},
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
		TakeManualDispatchRequest: flag.take,
	})
	cfg := base.cfg
	cfg.ManualDispatch = false
	cfg.RunSettings.Mode = mode
	return ses, f, store, cfg
}

func TestSession_InRunManual_EngineRoutedChosenStep_IsRecordedLikeTheStopScreenManualDispatch(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			stopSes, stopF, stopStore, stopCfg := newParityRig(t, false)
			runSes, runF, runStore, runCfg := newEngineParityRig(t, mode)
			stopSes.Start(context.Background(), stopCfg) //nolint:errcheck
			runSes.Start(context.Background(), runCfg)   //nolint:errcheck

			stopStep := firstWorkflowStep(t, stopStore)
			runStep := firstWorkflowStep(t, runStore)

			if !reflect.DeepEqual(runStep, stopStep) {
				t.Errorf("want the same recorded step\n stop screen: %+v\n in run:      %+v", stopStep, runStep)
			}
			stopInvs, runInvs := stopF.Invocations(), runF.Invocations()
			if len(runInvs) == 0 || len(stopInvs) == 0 || !sameDispatch(runInvs[0], stopInvs[0]) {
				t.Errorf("want the same first dispatch\n stop screen: %+v\n in run:      %+v", stopInvs, runInvs)
			}
			if !reflect.DeepEqual(firstLogEntry(runStore, stopStep.Seq), firstLogEntry(stopStore, stopStep.Seq)) {
				t.Errorf("want the same Execution Log row for the chosen step")
			}
		})
	}
}

// firstLogEntry returns the Execution Log row recorded with sequence number seq.
func firstLogEntry(store *memStore, seq int) domain.ExecutionLogEntry {
	for _, e := range store.state.ExecutionLog {
		if e.Seq == seq {
			return e
		}
	}
	return domain.ExecutionLogEntry{}
}

// sameDispatch compares what was dispatched, ignoring the temp directory the
// agent definition was loaded from.
func sameDispatch(a, b harness.Invocation) bool {
	return a.Agent.Identifier == b.Agent.Identifier && reflect.DeepEqual(a.Request, b.Request)
}
