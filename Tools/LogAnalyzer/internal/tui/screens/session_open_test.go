package screens_test

// How the Session screen is opened and left: enter on a row of the Dispatches
// table, the By type drill-down and the Context table records a session request
// for the actor of the selected row, and the Session screen reports esc and the
// per-run view keys to its router.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sessionOpener is a per-run view that can open a Session screen.
type sessionOpener interface {
	screens.Screen
	SessionRequested() (domain.ActorRef, bool)
	ResetSessionRequest()
}

// sessionIDs are the actors that have a row in the Context table and Dispatches
// table of the standard run besides the orchestrator.
var sessionIDs = []string{"builder#1", "coder#1", "ghost#1", "reviewer#1", "tester#1"}

// selectedActorID names the actor whose row the cursor marks in the view.
func selectedActorID(t *testing.T, view string) string {
	t.Helper()
	line := selectedLine(view)
	if line == "" {
		t.Fatalf("no row is selected\n%s", stripStyling(view))
	}
	for _, id := range sessionIDs {
		if strings.Contains(line, id) {
			return id
		}
	}
	if strings.Contains(line, "orchestrator") {
		return "orchestrator"
	}
	t.Fatalf("the selected row names no actor: %q", line)
	return ""
}

// assertRequestsSelectedActor presses enter on the selected row of a view and
// requires the session request to name the actor of that row.
func assertRequestsSelectedActor(t *testing.T, s sessionOpener, width, height int) {
	t.Helper()
	want := selectedActorID(t, s.View(width, height))

	press(s, "enter")

	got, ok := s.SessionRequested()
	if !ok {
		t.Fatalf("enter on the row of %s requested no session", want)
	}
	if got != sessRef(want) {
		t.Errorf("enter on the row of %s requested %+v, want %+v", want, got, sessRef(want))
	}
}

func TestSessionOpen_DispatchesEnterRequestsTheSessionOfEveryRow(t *testing.T) {
	colourOff(t)
	for row := 0; row <= len(sessionIDs); row++ {
		s := screens.NewRunDetailScreen(sessionRun(), tableW, tallH, paletteStyles())
		press(s, repeat("down", row)...)

		assertRequestsSelectedActor(t, s, tableW, tallH)
	}
}

func TestSessionOpen_DispatchesFirstRowIsTheOrchestrator(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(sessionRun(), tableW, tallH, paletteStyles())

	press(s, "enter")

	if got, ok := s.SessionRequested(); !ok || got != domain.Orchestrator() {
		t.Errorf("SessionRequested() = (%+v, %v), want the orchestrator", got, ok)
	}
}

func TestSessionOpen_DispatchesEnterFollowsTheSortedRowNotTheListPosition(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(sessionRun(), tableW, tallH, paletteStyles())
	press(s, repeat(screens.KeySort, 4)...) // cost, tokens, duration, peak context, then name
	press(s, "down")

	assertRequestsSelectedActor(t, s, tableW, tallH)
}

func TestSessionOpen_ResetSessionRequestClearsTheRequest(t *testing.T) {
	colourOff(t)
	openers := map[string]sessionOpener{
		"dispatches": screens.NewRunDetailScreen(sessionRun(), tableW, tallH, paletteStyles()),
		"context":    sessContext(),
	}
	for name, s := range openers {
		t.Run(name, func(t *testing.T) {
			press(s, "down", "enter")
			if _, ok := s.SessionRequested(); !ok {
				t.Fatal("setup: enter requested no session")
			}

			s.ResetSessionRequest()

			if _, ok := s.SessionRequested(); ok {
				t.Error("ResetSessionRequest must clear the session request")
			}
		})
	}
}

// sessContext is the Context view of the session run.
func sessContext() *screens.ContextScreen {
	return screens.NewContextScreen(sessionRun(), tableW, tallH, paletteStyles())
}

func TestSessionOpen_NoRequestBeforeEnter(t *testing.T) {
	colourOff(t)
	openers := []sessionOpener{
		screens.NewRunDetailScreen(sessionRun(), tableW, tallH, paletteStyles()),
		screens.NewAgentTypesScreen(sessionRun(), tableW, tallH, paletteStyles()),
		sessContext(),
	}
	for _, s := range openers {
		press(s, "down", "up", screens.KeySort)

		if got, ok := s.SessionRequested(); ok {
			t.Errorf("%s requested the session of %+v without enter", s.ID(), got)
		}
	}
}

func TestSessionOpen_ContextEnterRequestsTheSessionOfTheSelectedRow(t *testing.T) {
	colourOff(t)
	for row := 1; row <= len(sessionIDs)+1; row++ {
		s := sessContext()
		press(s, repeat("down", row)...)

		assertRequestsSelectedActor(t, s, tableW, tallH)
	}
}

func TestSessionOpen_ContextOrchestratorRowRequestsTheOrchestrator(t *testing.T) {
	s := sessContext()

	press(s, "down", "enter")

	if got, ok := s.SessionRequested(); !ok || got != domain.Orchestrator() {
		t.Errorf("SessionRequested() = (%+v, %v), want the orchestrator", got, ok)
	}
}

func TestSessionOpen_ContextEnterWithoutASelectedRowRequestsNothing(t *testing.T) {
	s := sessContext()
	s.View(tableW, tallH)

	press(s, "enter")

	if got, ok := s.SessionRequested(); ok {
		t.Errorf("enter before any row is selected requested %+v", got)
	}
}

func TestSessionOpen_ContextEnterNoLongerShowsTheCallTableInPlace(t *testing.T) {
	for row := 1; row <= len(sessionIDs)+1; row++ {
		s := sessContext()
		press(s, repeat("down", row)...)
		id := selectedActorID(t, s.View(tableW, tallH))
		first, ok := sessFirstCall[id]
		if !ok {
			continue // an actor without calls has no call row to look for
		}

		press(s, "enter")
		view := stripStyling(s.View(tableW, tallH))

		assertNotContains(t, view, ctxCallLabels(first)[0])
		assertContains(t, view, "reviewer#1", "coder#1", "builder#1")
	}
}

func TestSessionOpen_ContextEscStillLeavesTheViewInOnePress(t *testing.T) {
	s := sessContext()
	press(s, "down", "enter")

	press(s, "esc")

	if !s.Back() {
		t.Error("esc on the Context table must leave the per-run views; there is no drill-down to close")
	}
}

func TestSessionOpen_AgentTypesEnterOnTheTypeTableStillDrillsDown(t *testing.T) {
	colourOff(t)
	s := screens.NewAgentTypesScreen(sessionRun(), tableW, tallH, paletteStyles())

	press(s, "enter")

	if _, ok := s.DrilledType(); !ok {
		t.Error("enter on the type table must show the dispatches of the type")
	}
	if got, ok := s.SessionRequested(); ok {
		t.Errorf("enter on the type table requested the session of %+v; only the drill-down opens one", got)
	}
}

func TestSessionOpen_AgentTypesDrillDownEnterRequestsTheSessionOfTheDispatch(t *testing.T) {
	colourOff(t)
	byType := map[string]string{"builder": "builder#1", "coder": "coder#1", "ghost": "ghost#1", "reviewer": "reviewer#1", "": "tester#1"}
	for typeRow := 0; typeRow < len(byType); typeRow++ {
		s := screens.NewAgentTypesScreen(sessionRun(), tableW, tallH, paletteStyles())
		press(s, repeat("down", typeRow)...)
		press(s, "enter")
		agentType, ok := s.DrilledType()
		if !ok {
			t.Fatalf("type row %d: enter did not drill down", typeRow)
		}

		press(s, "enter")

		got, requested := s.SessionRequested()
		if !requested {
			t.Fatalf("type %q: enter on its dispatch requested no session", agentType)
		}
		if want := sessRef(byType[agentType]); got != want {
			t.Errorf("type %q: requested %+v, want %+v", agentType, got, want)
		}
	}
}

func TestSessionOpen_AgentTypesDrillDownKeepsItsStateAfterTheRequestIsReset(t *testing.T) {
	colourOff(t)
	s := screens.NewAgentTypesScreen(sessionRun(), tableW, tallH, paletteStyles())
	press(s, "enter", "enter")
	before := s.View(tableW, tallH)

	s.ResetSessionRequest()

	if _, ok := s.DrilledType(); !ok {
		t.Error("the type drill-down must stay open when the session request is cleared")
	}
	if after := s.View(tableW, tallH); after != before {
		t.Errorf("clearing the request must not change the view\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestSessionScreen_IdentifiesItselfAndItsActor(t *testing.T) {
	s := sessOpen(t, "coder#1")

	if s.ID() != screens.ScreenSession {
		t.Errorf("ID() = %q, want %q", s.ID(), screens.ScreenSession)
	}
	if got := s.Actor(); got != sessRef("coder#1") {
		t.Errorf("Actor() = %+v, want %+v", got, sessRef("coder#1"))
	}
	if got := sessOpen(t, "orchestrator").Actor(); got != domain.Orchestrator() {
		t.Errorf("Actor() of the orchestrator session = %+v", got)
	}
}

func TestSessionScreen_EscAndQAskToGoBackAndResetClearsIt(t *testing.T) {
	for _, key := range []string{"esc", "q"} {
		s := sessOpen(t, "builder#1")
		if s.Back() {
			t.Fatal("a fresh Session screen must not ask to go back")
		}

		press(s, key)
		if !s.Back() {
			t.Errorf("%q must ask to go back", key)
		}
		s.Reset()

		if s.Back() {
			t.Errorf("Reset must clear the back request after %q", key)
		}
	}
}

func TestSessionScreen_EveryViewKeyIsRequestedIncludingTheOpenersOwn(t *testing.T) {
	keys := map[string]screens.ScreenID{
		screens.KeyViewDispatches: screens.ScreenRunDetail, screens.KeyViewOverview: screens.ScreenOverview,
		screens.KeyViewAgentTypes: screens.ScreenAgentTypes, screens.KeyViewStatus: screens.ScreenStatus,
		screens.KeyViewTools: screens.ScreenTools, screens.KeyViewTimeline: screens.ScreenTimeline,
		screens.KeyViewContext: screens.ScreenContext,
	}
	for _, opener := range []screens.ScreenID{screens.ScreenRunDetail, screens.ScreenAgentTypes, screens.ScreenContext} {
		for key, want := range keys {
			s := sessOpenFrom(t, sessionRun(), "builder#1", opener, tableW, tallH)

			press(s, key)

			if got, ok := s.Requested(); !ok || got != want {
				t.Errorf("opened from %s, key %q: Requested() = (%q, %v), want %q", opener, key, got, ok, want)
			}
			s.Reset()
			if _, ok := s.Requested(); ok {
				t.Errorf("Reset must clear the view request after %q", key)
			}
		}
	}
}

func TestSessionScreen_FrameHighlightsTheTabOfTheOpener(t *testing.T) {
	tabs := map[screens.ScreenID]string{
		screens.ScreenRunDetail: "1 Dispatches", screens.ScreenAgentTypes: "3 By type", screens.ScreenContext: "7 Context",
	}
	for opener, tab := range tabs {
		view := sessOpenFrom(t, sessionRun(), "builder#1", opener, tableW, tallH).View(tableW, tallH)

		lines := framedLines(t, view)

		assertContains(t, lines[1], "["+tab+"]")
		if n := strings.Count(lines[1], "["); n != 1 {
			t.Errorf("opened from %s: %d tabs are highlighted in %q, want 1", opener, n, lines[1])
		}
	}
}
