package session_test

// Tests of the live standing-guidance holder: guidance is visible while a
// session runs, an edit saved during the run applies from the next dispatch
// (a dispatch already running is unaffected), and an edit is refused when no
// session holds guidance or the file cannot be saved.

import (
	"context"
	"errors"
	"testing"

	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

var editedInRun = domain.Guidance{TaskDescription: "edited-in-run-task-z5p", Constraints: "edited-in-run-constraints-z5p"}

// updateFrom runs live.Update from another goroutine, as a frontend would, and
// waits for it.
func updateFrom(live *session.StandingGuidance, g domain.Guidance) error {
	done := make(chan error, 1)
	go func() { done <- live.Update(context.Background(), g) }()
	return <-done
}

// onFirstDispatch runs fn once, just before the first dispatch reaches the harness.
func (g *guidanceRig) onFirstDispatch(fn func()) {
	fired := false
	g.onInvoke = func(string) {
		if !fired {
			fired = true
			fn()
		}
	}
	g.rebuild()
}

func TestSession_GuidanceLive_EditDuringRun_AppliesFromNextDispatchOnly(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	var updateErr error
	g.onFirstDispatch(func() { updateErr = updateFrom(g.live, editedInRun) })
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if updateErr != nil {
		t.Fatalf("want the update during the run to succeed, got %v", updateErr)
	}
	running := g.firstRequestOf(t, "agent-a")
	if !carriesGuidance(running, savedGuidanceValue) || !carriesNoGuidance(running, editedInRun) {
		t.Errorf("want the dispatch already running unaffected, got task=%q constraints=%q", running.TaskDescription, running.Constraints)
	}
	next := g.firstRequestOf(t, "agent-b")
	if !carriesGuidance(next, editedInRun) || !carriesNoGuidance(next, savedGuidanceValue) {
		t.Errorf("want the next dispatch to use the edited guidance, got task=%q constraints=%q", next.TaskDescription, next.Constraints)
	}
}

func TestSession_GuidanceLive_EditDuringRun_IsSavedToTheRunFolder(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.onFirstDispatch(func() { _ = updateFrom(g.live, editedInRun) })
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if file, _ := g.files.fileOf(g.runFolder); file != editedInRun {
		t.Errorf("want the edit saved in the run folder, got %+v", file)
	}
}

func TestSession_GuidanceLive_CurrentDuringRun_ReportsReviewedGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	var cur domain.Guidance
	var ok bool
	g.onFirstDispatch(func() { cur, ok = g.live.Current() })
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if !ok || cur != adviceValue {
		t.Errorf("want the accepted guidance current during the run, got %+v ok=%v", cur, ok)
	}
}

func TestSession_GuidanceLive_CurrentAfterReviewEdit_ReportsEditedGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.ask.review = []interaction.ChoiceAnswer{answerReview(domain.GuidanceOptionEdit)}
	g.ask.queueText(domain.QuestionGuidanceTask, answerText("reviewed-task"))
	var cur domain.Guidance
	g.onFirstDispatch(func() { cur, _ = g.live.Current() })
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	if want := (domain.Guidance{TaskDescription: "reviewed-task", Constraints: adviceConstraint}); cur != want {
		t.Errorf("want %+v current, got %+v", want, cur)
	}
}

func TestSession_GuidanceLive_NoActiveSession_CurrentFalseAndUpdateRefused(t *testing.T) {
	live := session.NewStandingGuidance()
	files := &memGuidanceStore{}

	if _, ok := live.Current(); ok {
		t.Error("want no current guidance before any session")
	}
	err := live.Update(context.Background(), editedInRun)

	if !errors.Is(err, session.ErrNoStandingGuidance) {
		t.Errorf("want ErrNoStandingGuidance, got %v", err)
	}
	if len(files.saves) != 0 {
		t.Errorf("want nothing saved, got %d saves", len(files.saves))
	}
}

func TestSession_GuidanceLive_AfterStartReturns_CurrentFalseAndUpdateRefused(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()
	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))
	requireRunStatus(t, got, err, domain.RunCompleted)
	savesBefore := len(g.files.saves)

	_, ok := g.live.Current()
	updateErr := g.live.Update(context.Background(), editedInRun)

	if ok {
		t.Error("want no current guidance once Start has returned")
	}
	if !errors.Is(updateErr, session.ErrNoStandingGuidance) {
		t.Errorf("want ErrNoStandingGuidance after Start returned, got %v", updateErr)
	}
	if len(g.files.saves) != savesBefore {
		t.Error("want nothing saved by the refused update")
	}
}

func TestSession_GuidanceLive_Update_StoresTheNormalizedGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	var cur domain.Guidance
	g.onFirstDispatch(func() {
		_ = updateFrom(g.live, domain.Guidance{TaskDescription: "x\r\ny\n\n", Constraints: "z\n"})
		cur, _ = g.live.Current()
	})
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	want := domain.Guidance{TaskDescription: "x\ny", Constraints: "z"}
	if cur != want {
		t.Errorf("want the normalized guidance current, got %q", cur)
	}
	if file, _ := g.files.fileOf(g.runFolder); file != want {
		t.Errorf("want the normalized guidance saved, got %q", file)
	}
}

func TestSession_GuidanceLive_UpdateSaveError_KeepsGuidanceInEffect(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	saveErr := errors.New("disk full")
	var updateErr error
	var cur domain.Guidance
	g.onFirstDispatch(func() {
		g.files.saveErr = saveErr
		updateErr = updateFrom(g.live, editedInRun)
		g.files.saveErr = nil
		cur, _ = g.live.Current()
	})
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !errors.Is(updateErr, saveErr) {
		t.Errorf("want the store's error returned, got %v", updateErr)
	}
	if cur != savedGuidanceValue {
		t.Errorf("want the guidance in effect unchanged, got %+v", cur)
	}
	if file, _ := g.files.fileOf(g.runFolder); file != savedGuidanceValue {
		t.Errorf("want the file unchanged, got %+v", file)
	}
	if next := g.firstRequestOf(t, "agent-b"); !carriesGuidance(next, savedGuidanceValue) {
		t.Errorf("want the next dispatch to keep the old guidance, got %q", next.TaskDescription)
	}
}
