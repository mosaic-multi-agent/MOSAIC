package tui

// The guard-stop ending screen offers a resume with the anti-loop guard off,
// and that is the only start that carries the waiver: a plain Continue, a
// retry, a manual-dispatch restart and a new start all begin with the guard on.

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// startGuardResume sends the guard-stop outcome, presses key on the done
// screen and returns the command the restart produced.
func startGuardResume(t *testing.T, fx *guardFixture, outcome domain.RunOutcome, key string) tea.Cmd {
	t.Helper()
	fx.m.Update(runDoneMsg{outcome: outcome})
	if fx.m.screen != screenDone {
		t.Fatalf("precondition: screen = %v after the outcome, want screenDone", fx.m.screen)
	}
	return pressRune(fx.m, key)
}

func TestGuardStopDone_OffersResumeWithTheGuardOff(t *testing.T) {
	fx := newGuardFixture(t)

	fx.m.Update(runDoneMsg{outcome: guardStopOutcome()})

	if view := fx.m.View(); !containsStr(view, guardDoneHelpOption) {
		t.Errorf("the ending screen of a guard stop lacks %q:\n%s", guardDoneHelpOption, view)
	}
}

func TestGuardStopDone_ResumeWithGuardOff_StartsTheResumedSessionWithTheGuardOff(t *testing.T) {
	outcomes := map[string]domain.RunOutcome{
		"stopped by the escalation limit": guardStopOutcome(),
		"deviation unresolved, no routing consultant": {
			Status: domain.RunDeviationUnresolved, Message: "anti-loop guard: no routing consultant configured", AntiLoopStop: true,
		},
	}
	for name, outcome := range outcomes {
		t.Run(name, func(t *testing.T) {
			fx := newGuardFixture(t)

			cmd := startGuardResume(t, fx, outcome, "g")
			requireOnProgress(t, fx.m)
			cfg := startedConfig(t, cmd, fx.sess.capturingSession)

			if !fx.sess.takeWaivedAtStart(t) {
				t.Error("the resumed session started with the guard on, want off")
			}
			if cfg.IsNewRun {
				t.Error("RunConfig.IsNewRun = true: the run must be resumed, not started again")
			}
			if cfg.RunFolder != restartTestRunFolder {
				t.Errorf("RunConfig.RunFolder = %q, want the same run %q", cfg.RunFolder, restartTestRunFolder)
			}
			if view := fx.m.View(); !containsStr(view, guardIndicatorLine) {
				t.Errorf("the progress screen of the resumed run lacks the guard-off indicator:\n%s", view)
			}
			want := [][2]string{{"true", guardResumeTrigger}}
			if got := fx.switchEvents(); !reflect.DeepEqual(got, want) {
				t.Errorf("tui.guard.switched entries (waived, trigger) = %v, want %v", got, want)
			}
		})
	}
}

// A stop that the guard did not cause offers no such choice, and the key does
// nothing.
func TestGuardStopDone_NotAGuardStop_OffersNoResumeWithTheGuardOff(t *testing.T) {
	fx := newGuardFixture(t)
	plain := domain.RunOutcome{Status: domain.RunStopped, Message: "run stopped: graceful stop confirmed"}

	fx.m.Update(runDoneMsg{outcome: plain})
	pressRune(fx.m, "g")

	if fx.m.screen != screenDone {
		t.Errorf("screen = %v after g on an ordinary stop, want to stay on the ending screen", fx.m.screen)
	}
	if view := fx.m.View(); containsStr(view, guardDoneHelpOption) {
		t.Errorf("an ordinary stop offers a resume with the guard off:\n%s", view)
	}
	if fx.waiver.Waived() {
		t.Error("the guard was switched off by g on an ordinary stop")
	}
}

// A plain Continue after a guard stop starts with the guard on.
func TestGuardStopDone_PlainContinue_StartsWithTheGuardOn(t *testing.T) {
	fx := newGuardFixture(t)
	fx.switchGuardOffWithKey(t)

	cmd := startGuardResume(t, fx, guardStopOutcome(), "c")
	requireOnProgress(t, fx.m)
	startedConfig(t, cmd, fx.sess.capturingSession)

	if fx.sess.takeWaivedAtStart(t) {
		t.Error("a plain Continue started the session with the guard off, want on")
	}
	if fx.waiver.Waived() {
		t.Error("the waiver is still set after a plain Continue")
	}
	if view := fx.m.View(); containsStr(view, guardIndicatorLine) {
		t.Error("the progress screen shows the guard-off indicator after a plain Continue")
	}
}

// Every restart path begins with the guard on, also after the guard was
// switched off during the previous session.
func TestGuardRestart_EveryOtherRestartPath_StartsWithTheGuardOn(t *testing.T) {
	for _, path := range restartDrivers {
		t.Run(path.name, func(t *testing.T) {
			fx := newGuardFixture(t)
			fx.switchGuardOffWithKey(t)

			cmd := path.drive(t, fx.m)
			startedConfig(t, cmd, fx.sess.capturingSession)

			if fx.sess.takeWaivedAtStart(t) {
				t.Error("the restarted session started with the guard off, want on")
			}
			got := fx.switchEvents()
			want := [][2]string{{"true", guardToggleTrigger}, {"false", guardStartTrigger}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("tui.guard.switched entries (waived, trigger) = %v, want %v", got, want)
			}
		})
	}
}

// A new start from the setup flow also begins with the guard on.
func TestGuardRestart_NewStartFromSetup_StartsWithTheGuardOn(t *testing.T) {
	fx := newGuardFixture(t)
	fx.switchGuardOffWithKey(t)
	fx.m.selections.isNewRun = true

	cmd := fx.m.launchSession()
	startedConfig(t, cmd, fx.sess.capturingSession)

	if fx.sess.takeWaivedAtStart(t) {
		t.Error("a new start began with the guard off, want on")
	}
}

// The waiver carried by the guard-stop resume does not outlive that session: a
// later Continue after an ordinary stop starts with the guard on again.
func TestGuardRestart_ContinueAfterAGuardOffResume_StartsWithTheGuardOn(t *testing.T) {
	fx := newGuardFixture(t)
	startedConfig(t, startGuardResume(t, fx, guardStopOutcome(), "g"), fx.sess.capturingSession)
	if !fx.sess.takeWaivedAtStart(t) {
		t.Fatal("precondition: the guard-off resume did not start with the guard off")
	}

	startedConfig(t, driveDoneContinue(t, fx.m), fx.sess.capturingSession)

	if fx.sess.takeWaivedAtStart(t) {
		t.Error("the Continue after a guard-off resume started with the guard off, want on")
	}
}
