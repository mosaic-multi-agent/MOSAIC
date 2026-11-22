package analysis_test

// Tests for the context at first write computed once per session.
//
// Rules under test:
//   - A write-class tool call belongs to the last session whose first timed call
//     starts at or before the write; a write before every session's first timed
//     call belongs to the first session.
//   - Per session, the figure is the context of the last main call of that
//     session at or before the session's earliest write. A session without a
//     write, or without such a call, has no figure.
//   - FirstWrites lists the figures in session order; the calls they come from
//     are marked. FirstWrite is the first entry's figure, FirstWriteAt stays the
//     actor-wide earliest write.
//   - A call without a session id joins the nearest preceding session, else the
//     nearest following one; an actor without ids is a single session.
//   - The orchestrator has no first-write figure.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// fwTwoSessionAgent has session "a1" with calls of context 100 (10s) and 200
// (20s) and session "b2" with calls of context 300 (40s) and 400 (50s).
func fwTwoSessionAgent(tools ...domain.ToolCall) ctxAgent {
	return ctxAgent{id: ctxAgentID, tools: tools, usage: sessJoin(
		sessAll("a1", ctxCall("r1", 10, 100, 0, 0, 1), ctxCall("r2", 20, 150, 50, 0, 1)),
		sessAll("b2", ctxCall("r3", 40, 200, 100, 0, 1), ctxCall("r4", 50, 250, 100, 50, 1)),
	)}
}

func fwContext(t *testing.T, agent ctxAgent) domain.ActorContext {
	t.Helper()
	return ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)
}

// fwAssertEntry checks one per-session entry: ordinal, id, write start, source
// call index and figure size.
func fwAssertEntry(t *testing.T, got domain.SessionFirstWrite, session int, id string, atSec, call int, tokens int64) {
	t.Helper()
	if got.Session != session || got.SessionID != id || got.Call != call {
		t.Errorf("entry = session %d id %q call %d, want session %d id %q call %d",
			got.Session, got.SessionID, got.Call, session, id, call)
	}
	if !got.At.Equal(ctxAt(atSec)) {
		t.Errorf("entry session %d at = %v, want %v", session, got.At, ctxAt(atSec))
	}
	ctxAssertTokens(t, "entry figure", got.Figure.Tokens, tokens)
}

func fwRequireEntries(t *testing.T, a domain.ActorContext, n int) {
	t.Helper()
	if len(a.FirstWrites) != n {
		t.Fatalf("first-write entries = %d (%+v), want %d", len(a.FirstWrites), a.FirstWrites, n)
	}
}

func TestFirstWriteSessions_EachSessionThatWritesGetsItsOwnFigure(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(
		ctxTool("w2", "Write", 45, 46),
		ctxTool("w1", "Write", 25, 26),
		ctxTool("late", "Edit", 55, 56),
	))

	fwRequireEntries(t, got, 2)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 25, 1, 200)
	fwAssertEntry(t, got.FirstWrites[1], 2, "b2", 45, 2, 300)
}

func TestFirstWriteSessions_MarksTheSourceCallOfEverySession(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("w1", "Write", 25, 26), ctxTool("w2", "Write", 45, 46)))

	marked := map[string]bool{}
	for _, c := range got.Calls {
		marked[c.RecordID] = c.FirstWrite
	}
	if marked["r1"] || !marked["r2"] || !marked["r3"] || marked["r4"] {
		t.Errorf("first-write marks = %v, want r2 and r3 only", marked)
	}
}

func TestFirstWriteSessions_FirstWriteKeepsTheFirstSessionsFigure(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("w1", "Write", 25, 26), ctxTool("w2", "Write", 45, 46)))

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
	if !got.FirstWriteAt.Equal(ctxAt(25)) {
		t.Errorf("first write at = %v, want %v", got.FirstWriteAt, ctxAt(25))
	}
}

func TestFirstWriteSessions_SessionWithoutAWriteHasNoFigure(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("w2", "Write", 45, 46), ctxTool("r", "Read", 15, 16)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 2, "b2", 45, 2, 300)
	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 300)
	for _, c := range got.Calls[:2] {
		if c.FirstWrite {
			t.Errorf("call %s of the session without a write is marked first write", c.RecordID)
		}
	}
}

func TestFirstWriteSessions_NoWriteInAnySession_HasNoEntries(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("r", "Read", 25, 26)))

	fwRequireEntries(t, got, 0)
	ctxAssertAbsent(t, "first write", got.FirstWrite.Tokens)
}

func TestFirstWriteSessions_WriteJustAfterASessionSwitchBelongsToTheNewSession(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("w", "Write", 41, 42)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 2, "b2", 41, 2, 300)
}

func TestFirstWriteSessions_WriteStartingAtTheFirstCallOfASession_BelongsToThatSession(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("w", "Write", 40, 41)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 2, "b2", 40, 2, 300)
}

func TestFirstWriteSessions_WriteBeforeEveryCallBelongsToTheFirstSession(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("early", "Write", 5, 6), ctxTool("w2", "Write", 45, 46)))

	// The first session's write precedes its calls, so it has no figure.
	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 2, "b2", 45, 2, 300)
	if !got.FirstWriteAt.Equal(ctxAt(5)) {
		t.Errorf("first write at = %v, want the actor-wide earliest write %v", got.FirstWriteAt, ctxAt(5))
	}
	// FirstWrite is the first entry's figure, not the figure of the earliest write.
	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 300)
}

func TestFirstWriteSessions_WriteBetweenSessionsBelongsToThePrecedingSession(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("gap", "Write", 30, 31)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 30, 1, 200)
}

func TestFirstWriteSessions_FigureComesFromTheLastCallWithContextOfTheSession(t *testing.T) {
	outputOnly := domain.InvocationUsage{
		Model: ctxModel, Harness: domain.HarnessClaudeCode, RecordID: "r-out", At: ctxAt(44),
		Usage: domain.TokenUsage{Output: domain.Tokens(5)},
	}
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w2", "Write", 45, 46)}, usage: sessJoin(
		sessAll("a1", ctxCall("r1", 10, 100, 0, 0, 1), ctxCall("r2", 20, 150, 50, 0, 1)),
		sessAll("b2", ctxCall("r3", 40, 200, 100, 0, 1), outputOnly),
	)}

	got := fwContext(t, agent)

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 2, "b2", 45, 2, 300)
}

func TestFirstWriteSessions_WriteWithoutStartTimeDefinesNoSessionsFirstWrite(t *testing.T) {
	untimed := domain.ToolCall{CallID: "untimed", ToolName: "Write", Pairing: domain.ToolCallPaired}

	got := fwContext(t, fwTwoSessionAgent(untimed, ctxTool("w1", "Write", 25, 26)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 25, 1, 200)
}

func TestFirstWriteSessions_SessionsWithoutAFigureKeepTheirOrdinals(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w1", "Write", 15, 16), ctxTool("w3", "Write", 35, 36)}, usage: sessJoin(
		sessAll("a1", ctxCall("r1", 10, 100, 0, 0, 1)),
		sessAll("b2", ctxCall("r2", 20, 150, 50, 0, 1)),
		sessAll("c3", ctxCall("r3", 30, 200, 100, 0, 1)),
	)}

	got := fwContext(t, agent)

	fwRequireEntries(t, got, 2)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 15, 0, 100)
	fwAssertEntry(t, got.FirstWrites[1], 3, "c3", 35, 2, 300)
}

func TestFirstWriteSessions_PartialMarkIsCarriedPerSessionFigure(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w1", "Write", 25, 26), ctxTool("w2", "Write", 45, 46)}, usage: sessJoin(
		sessAll("a1", ctxCall("r1", 10, 100, 0, 0, 1), ctxInputOnlyCall("r2", 20, 200, 1)),
		sessAll("b2", ctxCall("r3", 40, 200, 100, 0, 1), ctxCall("r4", 50, 250, 100, 50, 1)),
	)}

	got := fwContext(t, agent)

	fwRequireEntries(t, got, 2)
	if !got.FirstWrites[0].Figure.Partial || got.FirstWrites[1].Figure.Partial {
		t.Errorf("partial marks = %v, %v, want true, false",
			got.FirstWrites[0].Figure.Partial, got.FirstWrites[1].Figure.Partial)
	}
}

func TestFirstWriteSessions_SingleSessionWithoutIds_IsOneEntryEqualToTheFigure(t *testing.T) {
	got := fwContext(t, ctxThreeCallAgent(ctxTool("t2", "Write", 25, 26)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "", 25, 1, 200)
	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
}

func TestFirstWriteSessions_SingleNamedSession_IsOneEntryEqualToTheFigure(t *testing.T) {
	agent := ctxThreeCallAgent(ctxTool("t2", "Write", 25, 26), ctxTool("t3", "Edit", 35, 36))
	agent.usage = sessAll("only", agent.usage...)

	got := fwContext(t, agent)

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "only", 25, 1, 200)
	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 200)
}

func TestFirstWriteSessions_CallsWithoutIdJoinTheFollowingSessionWhenNoneComesBefore(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "Write", 15, 16)}, usage: sessJoin(
		[]domain.InvocationUsage{ctxCall("r1", 10, 100, 0, 0, 1)},
		sessAll("a1", ctxCall("r2", 20, 150, 50, 0, 1)),
		sessAll("b2", ctxCall("r3", 40, 200, 100, 0, 1)),
	)}

	got := fwContext(t, agent)

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 15, 0, 100)
}

func TestFirstWriteSessions_CallWithoutIdJoinsThePrecedingSession(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "Write", 22, 23)}, usage: sessJoin(
		sessAll("a1", ctxCall("r1", 10, 100, 0, 0, 1)),
		[]domain.InvocationUsage{ctxCall("r2", 20, 150, 50, 0, 1)},
		sessAll("b2", ctxCall("r3", 40, 200, 100, 0, 1)),
	)}

	got := fwContext(t, agent)

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 22, 1, 200)
}

func TestFirstWriteSessions_RecurringIdsKeepTheWriteInTheEarlierNamedSession(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("w", "Write", 35, 36)}, usage: sessJoin(
		sessAll("a1", ctxCall("r1", 10, 100, 0, 0, 1)),
		sessAll("b2", ctxCall("r2", 20, 150, 50, 0, 1)),
		sessAll("a1", ctxCall("r3", 30, 200, 100, 0, 1), ctxCall("r4", 40, 250, 100, 50, 1)),
	)}

	got := fwContext(t, agent)

	// a1 and b2 are two sessions; the write among the later a1 calls is a1's.
	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 1, "a1", 35, 2, 300)
}

func TestFirstWriteSessions_WriteAfterTheLastCallBelongsToTheLastSession(t *testing.T) {
	got := fwContext(t, fwTwoSessionAgent(ctxTool("w", "Write", 60, 61)))

	fwRequireEntries(t, got, 1)
	fwAssertEntry(t, got.FirstWrites[0], 2, "b2", 60, 3, 400)
}

func TestFirstWriteSessions_OrchestratorHasNoFigureEvenWithSessionsAndWrites(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage: sessJoin(
			sessAll("a1", ctxCall("o1", 10, 100, 0, 0, 1)),
			sessAll("b2", ctxCall("o2", 40, 200, 100, 0, 1)),
		),
		orchTools: []domain.ToolCall{ctxTool("w1", "Write", 25, 26), ctxTool("w2", "Write", 45, 46)},
		agents:    []ctxAgent{ctxThreeCallAgent()},
	}

	orch := ctxPriceDefault(t, spec).Context.Orchestrator

	if len(orch.FirstWrites) != 0 {
		t.Errorf("orchestrator first-write entries = %+v, want none", orch.FirstWrites)
	}
	ctxAssertAbsent(t, "orchestrator first write", orch.FirstWrite.Tokens)
	for _, c := range orch.Calls {
		if c.FirstWrite {
			t.Errorf("orchestrator call %s is marked first write", c.RecordID)
		}
	}
}
