package tui

// In-run guidance editing: a progress-screen key opens the standing guidance,
// a saved edit goes through the session's StandingGuidance (file save and log),
// cancelling changes nothing, and the progress screen comes back with its rows.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runflow"
)

const (
	guidanceTaskLabel        = "Appended to task_description"
	guidanceConstraintsLabel = "Appended to constraints"
	guidanceSavedStatus      = "Guidance saved - applies from the next dispatch"
	guidanceHelpBar          = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  e guidance  a artifact-view  ctrl+c force-quit"
	guidanceHelpBarWithout   = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  a artifact-view  ctrl+c force-quit"
)

// guidanceStoreDouble is a domain.GuidanceStore that records saves.
type guidanceStoreDouble struct {
	mu    sync.Mutex
	saved []domain.Guidance
	err   error
}

func (s *guidanceStoreDouble) Load(string) (domain.Guidance, bool, error) {
	return domain.Guidance{}, false, nil
}

func (s *guidanceStoreDouble) Save(_ string, g domain.Guidance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, g)
	return nil
}

func (s *guidanceStoreDouble) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *guidanceStoreDouble) saves() []domain.Guidance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Guidance(nil), s.saved...)
}

type guidanceFixture struct {
	m      *rootModel
	holder *session.StandingGuidance
	store  *guidanceStoreDouble
	log    *tuiRecordingLogger
}

// newGuidanceFixture is a model on its progress screen, with one running row,
// whose session holds the given guidance. active false models a run without
// pre-consultation guidance.
func newGuidanceFixture(t *testing.T, initial domain.Guidance, active bool) *guidanceFixture {
	t.Helper()
	store := &guidanceStoreDouble{}
	rec := &tuiRecordingLogger{}
	holder := session.NewStandingGuidance()
	if active {
		holder = session.NewActiveStandingGuidance(initial, store, "/run/folder", rec)
	}
	m := newRootModel(context.Background(), newCapturingSession(), Options{
		Theme:      tuicommon.DefaultTheme(),
		StopSignal: session.NewStopSignal(),
		Guidance:   holder,
		Debug:      rec,
	})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m.progressScreen.AppendRow(runflow.ProgressRow{AgentInstance: "planner#1", Phase: "PLANNING", Status: "SUCCESS"})
	return &guidanceFixture{m: m, holder: holder, store: store, log: rec}
}

func (fx *guidanceFixture) openEditor(t *testing.T) {
	t.Helper()
	pressRune(fx.m, "e")
	if view := fx.m.View(); !strings.Contains(view, guidanceTaskLabel) {
		t.Fatalf("precondition: the e key did not open the guidance editor:\n%s", view)
	}
}

func (fx *guidanceFixture) changeEvents() []tuiLogEntry {
	return fx.log.eventsNamed(domain.EventSessionGuidanceChanged)
}

func TestGuidanceKey_OpensCurrentGuidanceInBothFields(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "do it well\nand twice", Constraints: "no shortcuts"}, true)

	pressRune(fx.m, "e")

	view := fx.m.View()
	for _, want := range []string{guidanceTaskLabel, guidanceConstraintsLabel, "do it well", "and twice", "no shortcuts"} {
		if !strings.Contains(view, want) {
			t.Errorf("guidance view lacks %q:\n%s", want, view)
		}
	}
}

func TestGuidanceKey_SavedEditReachesTheSessionHolderFileAndLog(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "base", Constraints: "c1"}, true)
	fx.openEditor(t)

	typeLines(fx.m, "", "more")
	sendKey(fx.m, tea.KeyCtrlS)

	want := domain.Guidance{TaskDescription: "base\nmore", Constraints: "c1"}
	if got, _ := fx.holder.Current(); got != want {
		t.Errorf("guidance in effect = %+v, want %+v", got, want)
	}
	if saves := fx.store.saves(); len(saves) != 1 || saves[0] != want {
		t.Errorf("saved = %+v, want exactly one save of %+v", saves, want)
	}
	events := fx.changeEvents()
	if len(events) != 1 {
		t.Fatalf("session.guidance.changed logged %d times, want 1", len(events))
	}
	if change, _ := events[0].field("change"); change != string(domain.GuidanceEditedInRun) {
		t.Errorf("change = %q, want %q", change, domain.GuidanceEditedInRun)
	}
	if !strings.Contains(events[0].message, "more") {
		t.Errorf("the log entry does not carry the new text: %q", events[0].message)
	}
}

func TestGuidanceKey_MultiParagraphTextInBothFieldsIsSavedIntact(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{}, true)
	fx.openEditor(t)

	typeLines(fx.m, "para one", "", "para two")
	sendKey(fx.m, tea.KeyTab)
	typeLines(fx.m, "rule a", "rule b", "", "rule c")
	sendKey(fx.m, tea.KeyCtrlS)

	want := domain.Guidance{TaskDescription: "para one\n\npara two", Constraints: "rule a\nrule b\n\nrule c"}
	if saves := fx.store.saves(); len(saves) != 1 || saves[0] != want {
		t.Errorf("saved = %+v, want exactly one save of %+v", saves, want)
	}
}

func TestGuidanceKey_ReturnsToProgressWithRowsAndSavedStatus(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "base"}, true)
	fx.openEditor(t)

	typeLines(fx.m, "", "x")
	sendKey(fx.m, tea.KeyCtrlS)

	if fx.m.screen != screenProgress {
		t.Fatalf("screen = %v after saving, want the progress screen", fx.m.screen)
	}
	view := fx.m.View()
	for _, want := range []string{"planner#1", guidanceSavedStatus} {
		if !strings.Contains(view, want) {
			t.Errorf("progress view lacks %q after saving:\n%s", want, view)
		}
	}
	if strings.Contains(view, guidanceTaskLabel) {
		t.Error("the guidance editor is still shown after saving")
	}
}

func TestGuidanceKey_EscLeavesGuidanceUnchangedAndRestoresProgress(t *testing.T) {
	initial := domain.Guidance{TaskDescription: "base", Constraints: "c1"}
	fx := newGuidanceFixture(t, initial, true)
	fx.openEditor(t)

	typeLines(fx.m, "", "discarded")
	sendKey(fx.m, tea.KeyEsc)

	if got, _ := fx.holder.Current(); got != initial {
		t.Errorf("guidance in effect = %+v after esc, want unchanged %+v", got, initial)
	}
	if n := len(fx.store.saves()) + len(fx.changeEvents()); n != 0 {
		t.Errorf("esc caused %d saves or log entries, want 0", n)
	}
	view := fx.m.View()
	if !strings.Contains(view, "planner#1") || strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("progress screen with its rows not restored after esc:\n%s", view)
	}
	if strings.Contains(view, guidanceSavedStatus) {
		t.Error("the saved status is shown for a cancelled edit")
	}
}

func TestGuidanceKey_SaveErrorKeepsEditorOpenAndLaterSaveSucceeds(t *testing.T) {
	initial := domain.Guidance{TaskDescription: "base"}
	fx := newGuidanceFixture(t, initial, true)
	fx.store.setErr(errors.New("disk is read-only"))
	fx.openEditor(t)

	typeLines(fx.m, "", "edit")
	sendKey(fx.m, tea.KeyCtrlS)

	view := fx.m.View()
	if !strings.Contains(view, guidanceTaskLabel) || !strings.Contains(view, "disk is read-only") {
		t.Errorf("editor closed or error not shown after a failed save:\n%s", view)
	}
	if got, _ := fx.holder.Current(); got != initial {
		t.Errorf("guidance in effect = %+v after a failed save, want unchanged", got)
	}

	fx.store.setErr(nil)
	sendKey(fx.m, tea.KeyCtrlS)

	want := domain.Guidance{TaskDescription: "base\nedit"}
	if saves := fx.store.saves(); len(saves) != 1 || saves[0] != want {
		t.Errorf("saved = %+v, want one save of the kept text %+v", saves, want)
	}
	if fx.m.screen != screenProgress {
		t.Errorf("screen = %v after the successful retry, want the progress screen", fx.m.screen)
	}
}

func TestGuidanceKey_ListedInHelpBarOnlyWhileGuidanceIsInEffect(t *testing.T) {
	with := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	without := newGuidanceFixture(t, domain.Guidance{}, false)

	if view := with.m.View(); !strings.Contains(view, guidanceHelpBar) {
		t.Errorf("help bar lacks the guidance key with guidance in effect:\n%s", view)
	}
	if view := without.m.View(); !strings.Contains(view, guidanceHelpBarWithout) || strings.Contains(view, "e guidance") {
		t.Errorf("help bar offers the guidance key without guidance in effect:\n%s", view)
	}
}

func TestGuidanceKey_NotOfferedWithoutGuidanceInEffect(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{}, false)

	pressRune(fx.m, "e")

	if fx.m.screen != screenProgress {
		t.Errorf("screen = %v, want the progress screen: the key is not offered without guidance", fx.m.screen)
	}
	if view := fx.m.View(); strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("the guidance editor opened for a run without guidance:\n%s", view)
	}
}

func TestGuidanceKey_IgnoredWhileTheStopGateIsPending(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	pressRune(fx.m, "s")
	if !fx.m.progressScreen.ConfirmPending() {
		t.Fatal("precondition: the stop gate is not pending after s")
	}

	pressRune(fx.m, "e")

	if view := fx.m.View(); strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("the guidance editor opened while the stop gate was pending:\n%s", view)
	}
	if fx.m.screen != screenProgress || !fx.m.progressScreen.ConfirmPending() {
		t.Errorf("screen = %v, confirm pending = %v; want the progress screen with the stop gate still pending",
			fx.m.screen, fx.m.progressScreen.ConfirmPending())
	}
}

func TestGuidanceKey_NotTakenFromAPendingSessionQuestion(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	reply := make(chan answerMsg, 1)
	fx.m.Update(questionMsg{kind: questionSelectOne, choiceQ: guidanceReviewQuestion("d", false), reply: reply})

	pressRune(fx.m, "e")

	if fx.m.screen != screenQuestion {
		t.Errorf("screen = %v, want the pending question to stay in front", fx.m.screen)
	}
	if view := fx.m.View(); strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("the guidance editor opened over a pending question:\n%s", view)
	}
}
