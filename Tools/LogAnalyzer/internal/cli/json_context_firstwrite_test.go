package cli_test

// Tests for the per-session first-write figures in the report JSON.
//
// Rules under test:
//   - context.first_write keeps encoding ActorContext.FirstWrite and its shape.
//   - context.first_write_sessions is always an array: one object per
//     per-session figure, in session order, each with the figure fields of
//     first_write plus session, session_id (null when none) and at.
//   - Single-session invocations keep first_write unchanged.
//   - The orchestrator's list is empty.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// fwjMultiContext is an invocation with two sessions that each wrote.
func fwjMultiContext() domain.ActorContext {
	a := jcAgentContext()
	a.FirstWrite = jcFig(41_200, false)
	a.FirstWriteAt = jcStart.Add(5 * time.Second)
	a.FirstWrites = []domain.SessionFirstWrite{
		{Session: 1, SessionID: "a1", At: jcStart.Add(5 * time.Second), Call: 0, Figure: jcFig(41_200, false)},
		{Session: 2, SessionID: "b2", At: jcStart.Add(90 * time.Minute), Call: 2, Figure: jcFig(358_800, true)},
	}
	return a
}

func fwjSessions(t *testing.T, ctx map[string]any) []any {
	t.Helper()
	return vwList(t, ctx, "first_write_sessions")
}

func fwjAssertEntry(t *testing.T, v any, session float64, id any, tokens float64, band string, partial bool, at time.Time) {
	t.Helper()
	m := vwObj(t, v)
	if m["tokens"] != tokens || m["band"] != band || m["partial"] != partial || m["heuristic"] != true {
		t.Errorf("entry = %v, want tokens %v band %q partial %v heuristic true", m, tokens, band, partial)
	}
	if m["session"] != session || m["session_id"] != id {
		t.Errorf("entry = %v, want session %v and session_id %v", m, session, id)
	}
	if m["at"] != at.Format(time.RFC3339Nano) {
		t.Errorf("entry at = %v, want %s", m["at"], at.Format(time.RFC3339Nano))
	}
}

func TestEncodeReport_Context_FirstWriteSessionsListsOneObjectPerSessionInOrder(t *testing.T) {
	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), fwjMultiContext()))

	list := fwjSessions(t, ctx)

	if len(list) != 2 {
		t.Fatalf("first_write_sessions has %d entries, want 2: %v", len(list), list)
	}
	fwjAssertEntry(t, list[0], 1, "a1", 41_200, "optimal", false, jcStart.Add(5*time.Second))
	fwjAssertEntry(t, list[1], 2, "b2", 358_800, "degraded", true, jcStart.Add(90*time.Minute))
}

func TestEncodeReport_Context_FirstWriteKeepsItsShapeWhenSeveralSessionsWrote(t *testing.T) {
	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), fwjMultiContext()))

	fw := objectAt(t, ctx, "first_write")

	if fw["tokens"] != float64(41_200) || fw["partial"] != false || fw["heuristic"] != true {
		t.Errorf("first_write = %v, want the first session's figure with heuristic true", fw)
	}
	for _, key := range []string{"session", "session_id", "at"} {
		if _, has := fw[key]; has {
			t.Errorf("first_write carries %q; the key belongs to first_write_sessions only", key)
		}
	}
}

func TestEncodeReport_Context_SingleSessionFirstWrite_IsUnchangedAndListedOnce(t *testing.T) {
	a := jcAgentContext()
	a.FirstWrites = []domain.SessionFirstWrite{
		{Session: 1, SessionID: "", At: jcStart.Add(30 * time.Second), Call: 0, Figure: jcFig(10_000, false)},
	}

	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), a))

	jcAssertFig(t, "context", ctx, "first_write", 10_000, "optimal", false)
	if objectAt(t, ctx, "first_write")["heuristic"] != true {
		t.Errorf("first_write.heuristic = %v, want true", objectAt(t, ctx, "first_write")["heuristic"])
	}
	list := fwjSessions(t, ctx)
	if len(list) != 1 {
		t.Fatalf("first_write_sessions has %d entries, want 1", len(list))
	}
	fwjAssertEntry(t, list[0], 1, nil, 10_000, "optimal", false, jcStart.Add(30*time.Second))
}

func TestEncodeReport_Context_LoneLaterSessionFigure_IsFirstWriteAndTheOnlyListedEntry(t *testing.T) {
	a := fwjMultiContext()
	a.FirstWrite = a.FirstWrites[1].Figure
	a.FirstWriteAt = a.FirstWrites[1].At
	a.FirstWrites = a.FirstWrites[1:]

	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), a))

	jcAssertFig(t, "context", ctx, "first_write", 358_800, "degraded", true)
	list := fwjSessions(t, ctx)
	if len(list) != 1 {
		t.Fatalf("first_write_sessions has %d entries, want 1: %v", len(list), list)
	}
	fwjAssertEntry(t, list[0], 2, "b2", 358_800, "degraded", true, jcStart.Add(90*time.Minute))
}

func TestEncodeReport_Context_SessionWithoutIdEncodesNullSessionId(t *testing.T) {
	a := fwjMultiContext()
	a.FirstWrites = a.FirstWrites[:1]
	a.FirstWrites[0].SessionID = ""

	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), a))

	entry := vwObj(t, fwjSessions(t, ctx)[0])
	if v, present := entry["session_id"]; !present || v != nil {
		t.Errorf("session_id = %v (present %v), want null", v, present)
	}
}

func TestEncodeReport_Context_FirstWriteSessionsIsAnEmptyArrayWithoutFigures(t *testing.T) {
	a := jcAgentContext()
	a.FirstWrite = domain.ContextFigure{}
	a.FirstWriteAt = time.Time{}

	run := jcRun(jcOrchestratorContext(), a)

	for name, ctx := range map[string]map[string]any{
		"agent":        jcAgentObj(t, run),
		"orchestrator": jcOrchestratorObj(t, run),
	} {
		if list := fwjSessions(t, ctx); len(list) != 0 {
			t.Errorf("%s first_write_sessions = %v, want an empty array", name, list)
		}
	}
}
