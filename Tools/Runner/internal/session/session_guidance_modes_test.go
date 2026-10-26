package session_test

// Tests of when standing guidance applies: only auto and auto-review runs with
// pre-consultation on (judged from the effective settings after adoption),
// never across runs, and a frontend that cannot ask proceeds without review.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// requireNoGuidanceActivity asserts the rig read, wrote and asked nothing about
// guidance.
func requireNoGuidanceActivity(t *testing.T, g *guidanceRig) {
	t.Helper()
	if len(g.files.loads) != 0 || len(g.files.saves) != 0 {
		t.Errorf("want no guidance file read or written, got %d loads and %d saves", len(g.files.loads), len(g.files.saves))
	}
	if len(g.ask.reviews) != 0 || len(g.ask.asks) != 0 {
		t.Errorf("want no guidance question, got %d reviews and %d text questions", len(g.ask.reviews), len(g.ask.asks))
	}
	if len(g.guidanceEvents()) != 0 {
		t.Errorf("want no guidance change logged, got %d", len(g.guidanceEvents()))
	}
}

func TestSession_Guidance_PreConsultationOff_NoFileNoReviewNothingAppended(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueLinear()
	cfg := g.config("linear", domain.ExecutionModeAuto)
	cfg.PreConsultation = false

	got, err := g.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireNoGuidanceActivity(t, g)
	for _, agent := range []string{"agent-a", "agent-b"} {
		if !carriesNoGuidance(g.requestsFor(agent)[0], savedGuidanceValue) {
			t.Errorf("want no guidance appended to %s when pre-consultation is off", agent)
		}
	}
}

func TestSession_Guidance_OrchestratedMode_NoFileNoReview(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.consult.queueDispatch("agent-a", "consultant task", 0)
	g.consult.queueStop("done")
	g.queueOK("agent-a")
	cfg := g.config("linear", domain.ExecutionModeOrchestrated)
	cfg.PreConsultation = true

	_, _ = g.ses.Start(context.Background(), cfg)

	requireNoGuidanceActivity(t, g)
}

func TestSession_Guidance_NotAutoRun_HolderReportsNoActiveGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	var during, duringOK bool
	g.onInvoke = func(string) {
		_, duringOK = g.live.Current()
		during = true
	}
	g.rebuild()
	g.queueLinear()
	cfg := g.config("linear", domain.ExecutionModeAuto)
	cfg.PreConsultation = false

	_, _ = g.ses.Start(context.Background(), cfg)

	if !during {
		t.Fatal("want a dispatch to have run")
	}
	if duringOK {
		t.Error("want no standing guidance held while pre-consultation is off")
	}
}

func TestSession_Guidance_AutoReviewMode_SavesAndReviews(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()
	cfg := g.config("linear", domain.ExecutionModeAutoReview)
	cfg.PreConsultation = true

	got, err := g.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if saves := g.files.savedGuidance(); len(saves) != 1 || saves[0] != adviceValue {
		t.Errorf("want the result saved in auto-review mode, got %+v", saves)
	}
	if len(g.ask.reviews) != 1 {
		t.Errorf("want one review in auto-review mode, got %d", len(g.ask.reviews))
	}
}

func TestSession_Guidance_NotSharedAcrossRuns_OtherRunsFileIsNotUsed(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload("some/other/run-folder", savedGuidanceValue)
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !g.pc.Called {
		t.Error("want pre-consultation run because this run has no saved guidance")
	}
	if !carriesNoGuidance(g.firstRequestOf(t, "agent-a"), savedGuidanceValue) {
		t.Error("want another run's guidance not appended")
	}
}

// ===== A frontend that cannot ask =====

func TestSession_Guidance_CannotAsk_FirstRun_RunsPreConsultationAndProceeds(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.ask.unsupported = true
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if saves := g.files.savedGuidance(); len(saves) != 1 || saves[0] != adviceValue {
		t.Errorf("want the fresh result saved, got %+v", saves)
	}
	if !carriesGuidance(g.firstRequestOf(t, "agent-a"), adviceValue) {
		t.Error("want the fresh guidance appended without a review answer")
	}
	if len(g.ask.asks) != 0 {
		t.Errorf("want no text question after an unsupported review, got %d", len(g.ask.asks))
	}
}

func TestSession_Guidance_CannotAsk_Resume_UsesSavedGuidanceWithoutPreConsultation(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.ask.unsupported = true
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.pc.advice = adviceOf(adviceValue)
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), g.linearResume())

	requireRunStatus(t, got, err, domain.RunCompleted)
	if g.pc.Called {
		t.Error("want pre-consultation not run when saved guidance exists")
	}
	if !carriesGuidance(g.firstRequestOf(t, "agent-b"), savedGuidanceValue) {
		t.Error("want the saved guidance appended")
	}
	if file, _ := g.files.fileOf(g.runFolder); file != savedGuidanceValue {
		t.Errorf("want the saved file unchanged, got %+v", file)
	}
}

func TestSession_Guidance_CannotAsk_NewRunWithExistingFile_UsesFile(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.ask.unsupported = true
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if g.pc.Called {
		t.Error("want pre-consultation not run when the folder already holds guidance")
	}
	if !carriesGuidance(g.firstRequestOf(t, "agent-a"), savedGuidanceValue) {
		t.Error("want the file's guidance appended")
	}
}

// ===== Settings adoption =====

func nativeAdoptionResume(g *guidanceRig, mode domain.ExecutionMode, preConsultation bool) domain.RunConfig {
	g.blockRig.store.exists = true
	g.blockRig.store.state = nativeCreatedState()
	cfg := g.resumeConfig("linear", mode)
	cfg.PreConsultation = preConsultation
	cfg.Supplied = domain.SuppliedSettings{Mode: true, PreConsultation: true, ManualResolution: true}
	return cfg
}

func TestSession_Guidance_AdoptedAutoWithPreConsultation_UsesSavedGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), nativeAdoptionResume(g, domain.ExecutionModeAuto, true))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if g.pc.Called {
		t.Error("want pre-consultation not run when saved guidance exists")
	}
	if !carriesGuidance(g.firstRequestOf(t, "agent-b"), savedGuidanceValue) {
		t.Error("want the saved guidance appended under the adopted settings")
	}
}

func TestSession_Guidance_AdoptedPreConsultationOff_IgnoresSavedGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueOK("agent-b")

	got, err := g.ses.Start(context.Background(), nativeAdoptionResume(g, domain.ExecutionModeAuto, false))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireNoGuidanceActivity(t, g)
	if !carriesNoGuidance(g.firstRequestOf(t, "agent-b"), savedGuidanceValue) {
		t.Error("want no guidance appended when the adopted setting is off")
	}
}
