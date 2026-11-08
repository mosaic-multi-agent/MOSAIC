package session_test

// Tests for how a HARNESS_ERROR row is routed on the engine-routed and the
// consultant-routed dispatch paths: raw-text bypass first, mechanical
// re-dispatch of the same row while the budget remains, deviation once it is
// spent, immediate consultation in orchestrated mode, an unresolved deviation
// without a routing consultant, and current_state untouched throughout.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// The re-dispatch after a harness error is the same row, with the same task and
// the same inputs and outputs, under a fresh instance; the next row waits.
func TestSession_HarnessError_AutoModes_RedispatchesTheSameRowMechanically(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
			f.Queue("agent-a", failEntry("harness: subprocess timed out"), successEntry("done on re-dispatch"))
			f.Queue("agent-b", successEntry("done"))

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			invs := f.Invocations()
			if len(invs) != 3 || invs[0].Agent.Identifier != "agent-a" || invs[1].Agent.Identifier != "agent-a" ||
				invs[2].Agent.Identifier != "agent-b" {
				t.Fatalf("want agent-a, agent-a again, then agent-b, got %d invocations: %+v", len(invs), invs)
			}
			first, second := invs[0].Request, invs[1].Request
			if first.AgentInstanceID == second.AgentInstanceID {
				t.Errorf("want a fresh instance for the re-dispatch, both were %q", first.AgentInstanceID)
			}
			if first.TaskDescription != second.TaskDescription ||
				!reflect.DeepEqual(first.InputArtifacts, second.InputArtifacts) ||
				!reflect.DeepEqual(first.OutputArtifacts, second.OutputArtifacts) {
				t.Errorf("want the re-dispatch to repeat the failed request, got\n%+v\nafter\n%+v", second, first)
			}
			if consultant.CallCount != 0 {
				t.Errorf("want no consultation while the budget remains, got %d", consultant.CallCount)
			}
			rows := agentRows(store, "agent-a")
			if len(rows) != 2 || rows[0].Status != domain.StatusHARNESS_ERROR || rows[1].Status != domain.StatusSUCCESS {
				t.Fatalf("want the HARNESS_ERROR row then the SUCCESS row for agent-a, got %+v", rows)
			}
			if rows[0].WorkflowRow != rows[1].WorkflowRow || rows[0].Phase != rows[1].Phase || rows[0].Stage != rows[1].Stage {
				t.Errorf("want both rows at the same row, phase and stage, got %+v", rows)
			}
		})
	}
}

// A HARNESS_ERROR row never moves current_state: a failure after an accepted
// step leaves the accepted step as the workflow position, for every attempt.
func TestSession_HarnessError_CurrentStateStaysAtTheLastAcceptedStep(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("budget used up")
			dir := t.TempDir()
			orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
			writeAgentFile(t, dir, "agent-a")
			writeAgentFile(t, dir, "agent-b")
			f, store := harness.NewMockAdapter(), &memStore{}
			var seen []domain.CurrentState
			h := &beforeInvokeHarness{delegate: f, before: func(agentID string) {
				if agentID == "agent-b" {
					seen = append(seen, store.state.CurrentState)
				}
			}}
			ses := session.New(session.Deps{
				Harness: h, Store: store, Routing: consultant, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
			})
			f.Queue("agent-a", successEntry("planned"))
			queueRepeated(f, "agent-b", engine.HarnessErrorAttemptLimit+2, failEntry("harness: subprocess timed out"))

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if len(seen) != engine.HarnessErrorAttemptLimit {
				t.Fatalf("want %d agent-b dispatches, got %d", engine.HarnessErrorAttemptLimit, len(seen))
			}
			for i, cs := range seen {
				if cs.LastAgent != "agent-a#1" || cs.LastStatus != domain.StatusSUCCESS {
					t.Errorf("before agent-b dispatch %d: want current_state at agent-a#1/SUCCESS, got %+v", i+1, cs)
				}
			}
			if cs := store.state.CurrentState; cs.LastAgent != "agent-a#1" || cs.LastStatus != domain.StatusSUCCESS {
				t.Errorf("after the budget is used up: want current_state still at agent-a#1/SUCCESS, got %+v", cs)
			}
			if n := countHarnessErrorRowsFor(store, "agent-b"); n != engine.HarnessErrorAttemptLimit {
				t.Errorf("want %d HARNESS_ERROR rows for agent-b, got %d", engine.HarnessErrorAttemptLimit, n)
			}
		})
	}
}

// The budget belongs to a row: failures of an earlier row that ended in SUCCESS
// do not shorten the budget of the next row.
func TestSession_HarnessError_BudgetIsPerRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
	f.Queue("agent-a", failEntry("harness: first"), failEntry("harness: second"), successEntry("planned"))
	queueRepeated(f, "agent-b", engine.HarnessErrorAttemptLimit+2, failEntry("harness: still down"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(f.Invocations(), "agent-b"); n != engine.HarnessErrorAttemptLimit {
		t.Errorf("want the full budget of %d agent-b dispatches, got %d", engine.HarnessErrorAttemptLimit, n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 2 {
		t.Errorf("want 2 HARNESS_ERROR rows for agent-a, got %d", n)
	}
	if consultant.CallCount != 1 {
		t.Errorf("want one consultation, after agent-b's budget, got %d", consultant.CallCount)
	}
}

// A raw-text-eligible error is retried by the bypass before the engine's
// budgeted re-dispatch, and a non-eligible error is not.
func TestSession_HarnessError_BypassComesBeforeTheBudgetedRedispatch(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
	f.Queue("agent-a", rawTextEntry(), failEntry("harness: bypass timed out"), successEntry("done on engine re-dispatch"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	rows := agentRows(store, "agent-a")
	if len(rows) != 3 {
		t.Fatalf("want 3 agent-a rows (error, failed bypass, re-dispatch), got %d: %+v", len(rows), rows)
	}
	if rows[0].Status != domain.StatusHARNESS_ERROR || rows[1].Status != domain.StatusHARNESS_ERROR ||
		rows[2].Status != domain.StatusSUCCESS {
		t.Errorf("want HARNESS_ERROR, HARNESS_ERROR, SUCCESS, got %s, %s, %s", rows[0].Status, rows[1].Status, rows[2].Status)
	}
	if consultant.CallCount != 0 {
		t.Errorf("want no consultation, got %d", consultant.CallCount)
	}
}

// Without a routing consultant the run ends as an unresolved deviation right
// after the row: no bypass, no re-dispatch.
func TestSession_HarnessError_NoConsultant_EndsUnresolvedWithoutBypassOrRedispatch(t *testing.T) {
	for name, entry := range map[string]harness.ScriptedEntry{
		"plain failure":    failEntry("harness: subprocess timed out"),
		"raw-text failure": rawTextEntry(),
	} {
		t.Run(name, func(t *testing.T) {
			ses, f, store, orchPath := newNoConsultantAutoSession(t)
			queueRepeated(f, "agent-a", 4, entry)

			got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

			requireRunStatus(t, got, err, domain.RunDeviationUnresolved)
			if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 1 {
				t.Errorf("want a single agent-a invocation, got %d", n)
			}
			if n := countHarnessErrorRowsFor(store, "agent-a"); n != 1 {
				t.Errorf("want one HARNESS_ERROR row, got %d", n)
			}
			requireCurrentStateUntouchedByFailure(t, store)
		})
	}
}

// Orchestrated mode: a harness error is an immediate consultation, after the
// raw-text bypass when the error is eligible, never a mechanical re-dispatch.
func TestSession_HarnessError_OrchestratedMode_BypassThenConsultsAtOnce(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "do the work", 0)
	consultant.queueStop("decided after the failure")
	ses, f, store, orchPath := newOrchestratedSession(t, consultant)
	queueRepeated(f, "agent-a", 4, rawTextEntry())

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the original dispatch and the bypass only, got %d invocations", n)
	}
	if consultant.CallCount != 2 {
		t.Fatalf("want 2 consultations (dispatch, then after the failure), got %d", consultant.CallCount)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 2 {
		t.Errorf("want 2 HARNESS_ERROR rows (error and failed bypass), got %d", n)
	}
	requireCurrentStateUntouchedByFailure(t, store)
}

// Consultant-routed dispatch: the consultant's re-dispatch after the harness
// error runs, and the run goes on; the failure left current_state alone.
func TestSession_HarnessError_ConsultPath_ConsultantCanRedispatchAfterTheFailure(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "first try", 0)
	consultant.queueDispatch("agent-a", "second try", 0)
	consultant.queueStop("done")
	ses, f, store, orchPath := newOrchestratedSession(t, consultant)
	f.Queue("agent-a", failEntry("harness: subprocess timed out"), successEntry("done on retry"))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	rows := agentRows(store, "agent-a")
	if len(rows) != 2 || rows[0].Status != domain.StatusHARNESS_ERROR || rows[1].Status != domain.StatusSUCCESS {
		t.Fatalf("want the HARNESS_ERROR row then the SUCCESS row, got %+v", rows)
	}
	if cs := store.state.CurrentState; cs.LastAgent != rows[1].AgentInstance || cs.LastStatus != domain.StatusSUCCESS {
		t.Errorf("want current_state at the successful retry %q, got %+v", rows[1].AgentInstance, cs)
	}
}

// The failed attempt's row is recorded even for an error that wraps one of the
// raw-text sentinels but is classified by an earlier kind: it stays eligible
// for the bypass, exactly as before.
func TestSession_HarnessError_ClassifiedAsTimeoutButWrappingMalformedJSON_StillGetsTheBypass(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
	both := errors.Join(commonharness.ErrTimeout, commonharness.ErrMalformedJSON)
	f.Queue("agent-a", harness.ScriptedEntry{Err: both}, successEntry("done on bypass"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the original dispatch and the bypass, got %d", n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 1 {
		t.Errorf("want one HARNESS_ERROR row, got %d", n)
	}
}
