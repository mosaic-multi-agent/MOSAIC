package tui

// Fixtures for the anti-loop guard tests: a session double that reports the
// waiver as it is when the session starts, and a model fixture for a run that
// sits on its progress screen with an existing artifact.

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

const (
	guardToggleTrigger  = "key"
	guardResumeTrigger  = "resume-guard-off"
	guardStartTrigger   = "start"
	guardSwitchedEvent  = domain.EventTUIGuardSwitched
	guardIndicatorLine  = "LOOP GUARD OFF for this session - same-agent dispatches are not limited (g to turn it back on)"
	guardDoneHelpOption = "g  continue with loop guard off"
)

// guardProbeSession records the waiver at the moment Start runs.
type guardProbeSession struct {
	*capturingSession
	waiver        *session.GuardWaiver
	waivedAtStart chan bool
}

func newGuardProbeSession(waiver *session.GuardWaiver) *guardProbeSession {
	return &guardProbeSession{
		capturingSession: newCapturingSession(),
		waiver:           waiver,
		waivedAtStart:    make(chan bool, 8),
	}
}

func (s *guardProbeSession) Start(ctx context.Context, cfg domain.RunConfig) (domain.RunOutcome, error) {
	s.waivedAtStart <- s.waiver.Waived()
	return s.capturingSession.Start(ctx, cfg)
}

// takeWaivedAtStart returns the waiver value the session saw when it started.
func (s *guardProbeSession) takeWaivedAtStart(t *testing.T) bool {
	t.Helper()
	select {
	case v := <-s.waivedAtStart:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("the session was not started")
		return false
	}
}

// guardFixture is a model on its progress screen for a run whose artifact
// exists, wired with a shared waiver, a recording debug log and the probe.
type guardFixture struct {
	m      *rootModel
	waiver *session.GuardWaiver
	log    *tuiRecordingLogger
	sess   *guardProbeSession
}

func newGuardFixture(t *testing.T) *guardFixture {
	t.Helper()
	waiver := session.NewGuardWaiver()
	probe := newGuardProbeSession(waiver)
	rec := &tuiRecordingLogger{}
	store := storeWithLog(sampleLog()...)
	m := newRootModel(context.Background(), probe, Options{
		Theme:       tuicommon.DefaultTheme(),
		StopSignal:  session.NewStopSignal(),
		GuardWaiver: waiver,
		Debug:       rec,
		SessionFactory: func(string, bool, string, runconfig.ConfigSelection) session.Session {
			return probe
		},
		ArtifactStoreFactory: func(string) domain.ArtifactStore { return store },
	})
	m.selections.runID = restartTestRunID
	m.selections.runFolder = restartTestRunFolder
	m.selections.orchestratorFile = "/ws/orchestrator.md"
	m.selections.workflowID = "wf"
	m.selections.task = "build it"
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return &guardFixture{m: m, waiver: waiver, log: rec, sess: probe}
}

// pressRune sends one printable key to the model.
func pressRune(m *rootModel, key string) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return cmd
}

// switchGuardOffWithKey presses g on the progress screen and fails the test
// unless that waived the guard: later assertions are about restarts clearing a
// waiver that really was set.
func (fx *guardFixture) switchGuardOffWithKey(t *testing.T) {
	t.Helper()
	pressRune(fx.m, "g")
	if !fx.waiver.Waived() {
		t.Fatal("precondition: the g key on the progress screen did not switch the guard off")
	}
}

// switchEvents returns the recorded tui.guard.switched entries as
// (waived, trigger) pairs in log order.
func (fx *guardFixture) switchEvents() [][2]string {
	var out [][2]string
	for _, e := range fx.log.eventsNamed(guardSwitchedEvent) {
		w, _ := e.field("waived")
		tr, _ := e.field("trigger")
		out = append(out, [2]string{w, tr})
	}
	return out
}

// guardStopOutcome is the outcome of a run the anti-loop guard stopped.
func guardStopOutcome() domain.RunOutcome {
	return domain.RunOutcome{
		Status:       domain.RunStopped,
		Message:      "anti-loop guard: the consultant kept requesting a dispatch; run stopped, resume to continue",
		AntiLoopStop: true,
	}
}
