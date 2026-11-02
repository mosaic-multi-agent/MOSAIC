package tui

// progress_liveness_test.go verifies that the progress screen stays live while
// and after a question overlay, the artifact view or the guidance editor is
// shown: its one-second tick chain never ends, only one chain runs, and its
// pending-request banner follows the shared request state without a key press.

import (
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runflow"
)

var (
	tickTypeOnce sync.Once
	tickMsgType  reflect.Type
)

// progressTickType returns the dynamic type of the message the progress
// screen's tick command produces, so the tests need no access to the type name.
func progressTickType(t *testing.T) reflect.Type {
	t.Helper()
	tickTypeOnce.Do(func() {
		m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
		tickMsgType = reflect.TypeOf(m.progressScreen.Init()())
	})
	return tickMsgType
}

// newTickMsg builds one progress tick message for the current time.
func newTickMsg(t *testing.T) tea.Msg {
	t.Helper()
	return reflect.ValueOf(time.Now()).Convert(progressTickType(t)).Interface()
}

// countTicks runs cmd (batches expanded) and returns how many progress tick
// messages the commands produced. A nil command produces none.
func countTicks(t *testing.T, cmd tea.Cmd) int {
	t.Helper()
	return len(collectTicks(t, cmd))
}

// collectTicks runs cmd (batches expanded) and returns the progress tick
// messages the commands produced, so a test can feed them back to the model
// the way the Bubble Tea runtime would.
func collectTicks(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a command returned by the model did not produce a message within 5s")
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectTicks(t, c)...)
		}
		return out
	}
	if reflect.TypeOf(msg) == progressTickType(t) {
		return []tea.Msg{msg}
	}
	return nil
}

var elapsedPattern = regexp.MustCompile(`\(([0-9hms.]+) elapsed\)`)

// shownElapsed returns the elapsed time the progress screen's title shows.
func shownElapsed(t *testing.T, m *rootModel) time.Duration {
	t.Helper()
	match := elapsedPattern.FindStringSubmatch(m.progressScreen.View())
	if match == nil {
		t.Fatalf("progress view shows no elapsed time:\n%s", m.progressScreen.View())
	}
	d, err := time.ParseDuration(match[1])
	if err != nil {
		t.Fatalf("elapsed text %q is not a duration: %v", match[1], err)
	}
	return d
}

// liveOverlay opens one non-progress state over the progress screen and knows
// how to close it again.
type liveOverlay struct {
	name  string
	open  func(t *testing.T) *rootModel
	close func(m *rootModel)
}

func liveOverlays() []liveOverlay {
	return []liveOverlay{
		{
			name: "question overlay",
			open: func(t *testing.T) *rootModel {
				m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
				m.Update(questionMsg{
					kind:    questionSelectOne,
					choiceQ: newTestChoiceQuestion("Pick one", []string{"a", "b"}),
					reply:   make(chan answerMsg, 1),
				})
				if m.screen != screenQuestion {
					t.Fatalf("precondition: screen = %v, want screenQuestion", m.screen)
				}
				return m
			},
			close: func(m *rootModel) { sendKey(m, tea.KeyEnter) },
		},
		{
			name: "artifact view",
			open: func(t *testing.T) *rootModel {
				m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
				m.selections.orchestratorFile = "/nonexistent/Orchestrator.md"
				pressProgressRune(m, "a")
				if m.screen != screenArtifact {
					t.Fatalf("precondition: screen = %v, want screenArtifact", m.screen)
				}
				return m
			},
			close: func(m *rootModel) { sendKey(m, tea.KeyEsc) },
		},
		{
			name: "guidance editor",
			open: func(t *testing.T) *rootModel {
				fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "base"}, true)
				fx.openEditor(t)
				if fx.m.screen != screenGuidance {
					t.Fatalf("precondition: screen = %v, want screenGuidance", fx.m.screen)
				}
				return fx.m
			},
			close: func(m *rootModel) { sendKey(m, tea.KeyEsc) },
		},
	}
}

func TestProgressTick_OnProgressScreen_ReArmsExactlyOneTick(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())

	_, cmd := m.Update(newTickMsg(t))

	if got := countTicks(t, cmd); got != 1 {
		t.Errorf("a tick on the progress screen returned %d follow-up ticks, want 1", got)
	}
}

func TestProgressTick_WhileAnOverlayIsOpen_KeepsTheChainAlive(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			m := ov.open(t)

			_, cmd := m.Update(newTickMsg(t))

			if got := countTicks(t, cmd); got != 1 {
				t.Errorf("a tick while the %s is open returned %d follow-up ticks, want 1 (the chain must not end)", ov.name, got)
			}
		})
	}
}

// The chain is driven the way the runtime drives it: the command returned for a
// tick that arrived while the overlay was open is run, and the tick it yields
// is delivered after the overlay closed. A chain that died while the overlay
// was open yields no tick, so nothing arrives after the close, the elapsed
// display never refreshes and the test fails.
func TestProgressTick_AfterAnOverlayCloses_ChainStillDeliversAndElapsedAdvances(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			m := ov.open(t)
			_, cmd := m.Update(newTickMsg(t)) // a tick arrives while the overlay is open
			ticks := collectTicks(t, cmd)     // the next tick, one second later
			if len(ticks) != 1 {
				t.Fatalf("the tick while the %s was open scheduled %d ticks, want 1 (the chain ended)", ov.name, len(ticks))
			}
			ov.close(m)
			if m.screen != screenProgress {
				t.Fatalf("precondition: screen = %v after closing the %s, want screenProgress", m.screen, ov.name)
			}

			_, cmd = m.Update(ticks[0])

			if got := countTicks(t, cmd); got != 1 {
				t.Errorf("the tick delivered after the %s closed returned %d follow-up ticks, want 1", ov.name, got)
			}
			if got := shownElapsed(t, m); got < time.Second {
				t.Errorf("elapsed shown = %v after a full tick interval across the %s, want at least 1s", got, ov.name)
			}
		})
	}
}

func TestProgressTick_ReachesOnlyTheProgressScreen_AndLeavesTheOverlayOpen(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			m := ov.open(t)
			before := m.screen

			m.Update(newTickMsg(t))

			if m.screen != before {
				t.Errorf("screen = %v after a tick, want %v (a tick must not close the %s)", m.screen, before, ov.name)
			}
		})
	}
}

func TestProgressTick_RestartAfterTheChainIsRunning_StartsNoSecondChain(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
	m.Update(newTickMsg(t)) // the chain is running

	cmd := m.restartAsResume(false)

	if got := countTicks(t, cmd); got != 0 {
		t.Errorf("restart returned %d ticks while a chain was already running, want 0 (no double ticking)", got)
	}
}

func TestProgressTick_RestartBeforeAnyTick_StartsExactlyOneChain(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())

	cmd := m.restartAsResume(false)

	if got := countTicks(t, cmd); got != 1 {
		t.Errorf("restart returned %d ticks before any chain started, want 1", got)
	}
}

func TestProgressTick_SetupStart_BeforeAnyTick_StartsExactlyOneChain(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())

	cmd := m.launchSession()

	if got := countTicks(t, cmd); got != 1 {
		t.Errorf("starting a run from setup returned %d ticks before any chain started, want 1", got)
	}
}

func TestProgressTick_SetupStart_AfterTheChainIsRunning_StartsNoSecondChain(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
	m.Update(newTickMsg(t)) // the chain is running

	cmd := m.launchSession()

	if got := countTicks(t, cmd); got != 0 {
		t.Errorf("starting a run from setup returned %d ticks while a chain was already running, want 0 (no double ticking)", got)
	}
}

func TestProgressTick_RestartAfterAnOverlayTick_StartsNoSecondChain(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			m := ov.open(t)
			m.Update(newTickMsg(t)) // the chain is running while the overlay is open
			ov.close(m)
			m.Update(newTickMsg(t)) // and keeps running after control returns

			cmd := m.restartAsResume(false)

			if got := countTicks(t, cmd); got != 0 {
				t.Errorf("restart returned %d ticks after the %s and further ticks, want 0 (no double ticking)", got, ov.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pending-request banner
// ---------------------------------------------------------------------------

func TestPendingBanner_ShowsOnTickWithoutAKeyPress_WhileAnOverlayIsOpen(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			m := ov.open(t)
			m.inRunRequests = reqs
			reqs.Request(domain.InRunRequestManualDispatch)

			m.Update(newTickMsg(t))

			if !m.progressScreen.PendingRequests().ManualDispatch {
				t.Errorf("a tick while the %s was open did not show the pending request", ov.name)
			}
		})
	}
}

func TestPendingBanner_ClearsOnTickWithoutAKeyPress_WhileAnOverlayIsOpen(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			reqs := session.NewInRunRequests() // the session already took the request
			m := ov.open(t)
			m.inRunRequests = reqs
			// The banner was showing the request when the overlay opened.
			m.progressScreen.SetPendingRequests(runflow.PendingRequests{ManualDispatch: true})

			m.Update(newTickMsg(t))

			if got := m.progressScreen.PendingRequests(); got.ManualDispatch {
				t.Errorf("PendingRequests() = %+v after a tick while the %s was open and the request was taken, want it cleared", got, ov.name)
			}
		})
	}
}

// Run with -race: the session goroutine mutates the shared requests while the
// UI goroutine reads them on every tick.
func TestPendingBanner_SessionTakesAndRequestsConcurrentlyWithTicks_EndsInSync(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			reqs.Request(domain.InRunRequestManualDispatch)
			reqs.Take(domain.InRunRequestManualDispatch)
		}
	}()

	for i := 0; i < 500; i++ {
		m.Update(newTickMsg(t))
	}
	<-done
	m.Update(newTickMsg(t))

	if got := m.progressScreen.PendingRequests(); got.ManualDispatch {
		t.Errorf("PendingRequests() = %+v after the session took every request and a tick arrived, want it cleared", got)
	}
}

func TestPendingBanner_ClearsWhenControlReturnsToProgress_WithoutAKeyOrTick(t *testing.T) {
	for _, ov := range liveOverlays() {
		t.Run(ov.name, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			m := ov.open(t)
			m.inRunRequests = reqs
			reqs.Request(domain.InRunRequestManualDispatch)
			// The banner was showing the request when the overlay opened.
			m.progressScreen.SetPendingRequests(runflow.PendingRequests{ManualDispatch: true})
			reqs.Take(domain.InRunRequestManualDispatch) // taken while the overlay is open

			ov.close(m)

			if m.screen != screenProgress {
				t.Fatalf("precondition: screen = %v after closing the %s, want screenProgress", m.screen, ov.name)
			}
			if got := m.progressScreen.PendingRequests(); got.ManualDispatch {
				t.Errorf("PendingRequests() = %+v right after the %s closed, want the taken request cleared", got, ov.name)
			}
		})
	}
}

func TestPendingBanner_ShowsARequestMadeBySomeoneElse_OnTheNextTick(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)

	reqs.Request(domain.InRunRequestHITLNext)
	m.Update(newTickMsg(t))

	if !m.progressScreen.PendingRequests().HITLNext {
		t.Error("the screen does not show a request that the shared state holds after a tick")
	}
}
