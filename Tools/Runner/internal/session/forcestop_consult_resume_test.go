package session_test

// Tests for resume after a force-stopped routing consultation, on a run
// recorded through the real artifact file store: the consultation left no
// trace, so resume routes normally from the last accepted state.

import (
	"context"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// consultResumeRun is an orchestrated run on a file store whose first session
// force-stops its nth routing consultation and stops the run.
type consultResumeRun struct {
	run        seededRun
	win        *forceWindow
	user       *choiceUser
	consultant *forceBlockingConsultant
	inner      *scriptedRoutingConsultant
	f          *harness.MockAdapter
	flag       *requestFlag
}

func newConsultResumeRun(t *testing.T, nth int) *consultResumeRun {
	t.Helper()
	r := &consultResumeRun{
		run: emptyFileStoreRun(t), win: &forceWindow{}, user: &choiceUser{option: domain.ForceStopNextStop},
		inner: &scriptedRoutingConsultant{}, f: harness.NewMockAdapter(), flag: &requestFlag{},
	}
	r.consultant = &forceBlockingConsultant{inner: r.inner, win: r.win, nth: nth}
	return r
}

// firstSession builds the session of the first, force-stopped run part.
func (r *consultResumeRun) firstSession() session.Session {
	return session.New(session.Deps{
		Harness: r.f, Store: artifact.NewFileStore(r.run.artifactPath), Routing: r.consultant,
		Manual: &scriptedRoutingConsultant{}, Clock: fixedClock{t: epoch}, Interact: r.user, ForceStop: r.win,
		TakeManualDispatchRequest: r.flag.take,
	})
}

func (r *consultResumeRun) startConfig() domain.RunConfig {
	cfg := r.run.newRunConfig()
	cfg.RunSettings.Mode = domain.ExecutionModeOrchestrated
	return cfg
}

func (r *consultResumeRun) resumeConfig() domain.RunConfig {
	cfg := r.run.resumeConfig()
	cfg.RunSettings.Mode = domain.ExecutionModeOrchestrated
	return cfg
}

// resumeSession builds the session of the resumed run, with a fresh consultant.
func (r *consultResumeRun) resumeSession(f *harness.MockAdapter, consultant domain.RoutingConsultant) session.Session {
	return session.New(session.Deps{
		Harness: f, Store: artifact.NewFileStore(r.run.artifactPath), Routing: consultant,
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
	})
}

func readLog(t *testing.T, path string) domain.ArtifactState {
	t.Helper()
	state, err := artifact.NewFileStore(path).Read(context.Background())
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	return state
}

func TestSession_ForceStopConsultationResume_FirstDecision_RoutesNormallyAndCompletes(t *testing.T) {
	r := newConsultResumeRun(t, 1)
	first, err := r.firstSession().Start(context.Background(), r.startConfig())
	requireRunStatus(t, first, err, domain.RunStopped)
	if state := readLog(t, r.run.artifactPath); len(state.ExecutionLog) != 0 || state.GlobalSequence != 0 {
		t.Fatalf("want an artifact without rows after the force stop, got %d rows, sequence %d", len(state.ExecutionLog), state.GlobalSequence)
	}
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueDispatch("agent-b", "finish", 1)
	consultant.queueStop("workflow done")
	f := harness.NewMockAdapter()
	f.Queue("agent-a", successEntry("agent-a finished"))
	f.Queue("agent-b", successEntry("agent-b finished"))

	got, err := r.resumeSession(f, consultant).Start(context.Background(), r.resumeConfig())

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if consultant.CallCount != 3 {
		t.Errorf("want the orchestrator consulted again from the first decision, got %d consultations", consultant.CallCount)
	}
	if want := []string{"agent-a", "agent-b"}; len(invokedAgents(f)) != 2 || invokedAgents(f)[0] != want[0] || invokedAgents(f)[1] != want[1] {
		t.Errorf("want agent-a then agent-b dispatched, got %v", invokedAgents(f))
	}
	if state := readLog(t, r.run.artifactPath); len(state.ExecutionLog) != 2 || state.ExecutionLog[0].Seq != 1 {
		t.Errorf("want two rows numbered from 1 (the consultation consumed no sequence), got %+v", state.ExecutionLog)
	}
}

func TestSession_ForceStopConsultationResume_MidRun_ContinuesFromTheLastAcceptedState(t *testing.T) {
	r := newConsultResumeRun(t, 2)
	r.inner.queueDispatch("agent-a", "start", 0)
	r.f.Queue("agent-a", successEntry("agent-a finished"))
	first, err := r.firstSession().Start(context.Background(), r.startConfig())
	requireRunStatus(t, first, err, domain.RunStopped)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-b", "finish", 1)
	consultant.queueStop("workflow done")
	f := harness.NewMockAdapter()
	f.Queue("agent-b", successEntry("agent-b finished"))

	got, err := r.resumeSession(f, consultant).Start(context.Background(), r.resumeConfig())

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 0 {
		t.Errorf("want the accepted agent-a not dispatched again, got %d", n)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want agent-b dispatched once from the consultation, got %d", n)
	}
	state := readLog(t, r.run.artifactPath)
	if len(state.ExecutionLog) != 2 || state.ExecutionLog[1].Seq != 2 {
		t.Errorf("want agent-b recorded as the second row, got %+v", state.ExecutionLog)
	}
}

// A manual-dispatch request raised while the consultation ran is not part of
// the artifact: the resumed run starts with no request pending and its
// consultation goes to the orchestrator again.
func TestSession_ForceStopConsultationResume_AfterInRunManualRequest_RoutesNormally(t *testing.T) {
	r := newConsultResumeRun(t, 1)
	r.consultant.onBlock = r.flag.request
	first, err := r.firstSession().Start(context.Background(), r.startConfig())
	requireRunStatus(t, first, err, domain.RunStopped)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("asked again")

	got, err := r.resumeSession(harness.NewMockAdapter(), consultant).Start(context.Background(), r.resumeConfig())

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if consultant.CallCount != 1 {
		t.Errorf("want the orchestrator asked once at the first decision, got %d", consultant.CallCount)
	}
	if got.StopReason != "asked again" {
		t.Errorf("want the stop of the orchestrator's answer, got %q", got.StopReason)
	}
}

// In auto mode a consultation exists only for a deviation; the deviation's row
// is accepted state, so a resume reaches the same deviation and consults again
// without failing.
func TestSession_ForceStopConsultationResume_AutoModeDeviation_ConsultsAgainWithoutFailing(t *testing.T) {
	r := newConsultResumeRun(t, 1)
	r.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
	startCfg := r.run.newRunConfig()
	first, err := r.firstSession().Start(context.Background(), startCfg)
	requireRunStatus(t, first, err, domain.RunStopped)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("asked again")

	got, err := r.resumeSession(harness.NewMockAdapter(), consultant).Start(context.Background(), r.run.resumeConfig())

	if got.Status == domain.RunFailed {
		t.Fatalf("want a resume that does not fail, got %q (%s)", got.Status, got.Message)
	}
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if consultant.CallCount != 1 {
		t.Errorf("want the deviation put to the orchestrator once, got %d", consultant.CallCount)
	}
}
