package session_test

// Tests for the E501 retry budget in the session: an agent's BLOCKED/E501
// reply, a harness error and a failed raw-text bypass are each one attempt of
// the same budget of three per row and stage; the count comes from the
// Execution Log, so it survives a restart. Other BLOCKED codes and statuses
// are never re-dispatched mechanically.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
)

func e501Entry() harness.ScriptedEntry {
	return statusEntry(domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "tool unavailable")
}

func failEntry(msg string) harness.ScriptedEntry {
	return harness.ScriptedEntry{Err: errors.New(msg)}
}

func rawTextEntry() harness.ScriptedEntry {
	return harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrMalformedJSON)}
}

func TestSession_E501_AgentReply_ThreeDispatchesThenOneConsultation(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("budget used up")
			ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
			queueRepeated(f, "agent-a", engine.E501AttemptLimit+3, e501Entry())

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := countInvocationsFor(f.Invocations(), "agent-a"); n != engine.E501AttemptLimit {
				t.Errorf("want %d agent-a dispatches, got %d", engine.E501AttemptLimit, n)
			}
			if consultant.CallCount != 1 {
				t.Fatalf("want exactly one consultation after the budget, got %d", consultant.CallCount)
			}
			requireBlockedE501Deviation(t, consultant.Requests[0])
			if n := countE501RowsFor(store, "agent-a"); n != engine.E501AttemptLimit {
				t.Errorf("want %d BLOCKED/E501 rows, got %d", engine.E501AttemptLimit, n)
			}
		})
	}
}

func TestSession_E501_SuccessAfterRetries_NeedsNoConsultation(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
			f.Queue("agent-a", e501Entry(), e501Entry(), statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))
			f.Queue("agent-b", statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if consultant.CallCount != 0 {
				t.Errorf("want no consultation, got %d", consultant.CallCount)
			}
		})
	}
}

func TestSession_HarnessError_ThreeDispatchesThenOneConsultation(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("budget used up")
			ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
			for i := 0; i < engine.HarnessErrorAttemptLimit+3; i++ {
				f.Queue("agent-a", failEntry("harness: subprocess timed out"))
			}

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := countInvocationsFor(f.Invocations(), "agent-a"); n != engine.HarnessErrorAttemptLimit {
				t.Errorf("want %d agent-a dispatches, got %d", engine.HarnessErrorAttemptLimit, n)
			}
			if consultant.CallCount != 1 {
				t.Fatalf("want exactly one consultation after the budget, got %d", consultant.CallCount)
			}
			requireHarnessErrorDeviation(t, consultant.Requests[0])
			requireErrorReasonCarries(t, consultant.Requests[0], "subprocess timed out")
			if n := countHarnessErrorRowsFor(store, "agent-a"); n != engine.HarnessErrorAttemptLimit {
				t.Errorf("want %d HARNESS_ERROR rows, got %d", engine.HarnessErrorAttemptLimit, n)
			}
			if n := countE501RowsFor(store, "agent-a"); n != 0 {
				t.Errorf("want no BLOCKED/E501 rows for harness failures, got %d", n)
			}
		})
	}
}

// A harness error plus a failed raw-text bypass are two HARNESS_ERROR rows and
// two of the three attempts: one more dispatch follows, then the consultation.
func TestSession_HarnessError_FailedBypass_IsLoggedAsOwnRowAndCountsAsAnAttempt(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("budget used up")
			ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
			f.Queue("agent-a", rawTextEntry(), failEntry("harness: bypass timed out"), failEntry("harness: third attempt failed"),
				statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "must stay unused"))

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
				t.Errorf("want 3 agent-a invocations (error, bypass, one more attempt), got %d", n)
			}
			if consultant.CallCount != 1 {
				t.Errorf("want exactly one consultation, got %d", consultant.CallCount)
			}
			rows := agentRows(store, "agent-a")
			if len(rows) != 3 {
				t.Fatalf("want 3 HARNESS_ERROR rows (error, failed bypass, third attempt), got %d: %+v", len(rows), rows)
			}
			for i, r := range rows {
				if r.Status != domain.StatusHARNESS_ERROR || r.ErrorCode != domain.ErrorNone {
					t.Errorf("row %d: want HARNESS_ERROR without error code, got %s/%s", i+1, r.Status, r.ErrorCode)
				}
				if r.WorkflowRow != rows[0].WorkflowRow || r.Phase != rows[0].Phase || r.Stage != rows[0].Stage {
					t.Errorf("row %d: want the same row, phase and stage as the first attempt", i+1)
				}
			}
			// The bypass row sits under the bypass attempt's own instance id.
			if want := fmt.Sprintf("agent-a#%d", rows[1].Seq); rows[1].AgentInstance != want {
				t.Errorf("want the bypass row recorded under %q, got %q", want, rows[1].AgentInstance)
			}
			requireInstanceIDsMatchRecordedSeq(t, store)
		})
	}
}

// The bypass is one attempt of the budget, so it is not allowed once the
// budget is used up: a raw-text error as the third attempt is not retried.
func TestSession_E501_RawTextErrorWithNoBudgetLeft_SkipsBypass(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
	f.Queue("agent-a", failEntry("harness: first"), failEntry("harness: second"), rawTextEntry(),
		statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "a bypass would consume this"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
		t.Errorf("want 3 agent-a invocations (no bypass after the third attempt), got %d", n)
	}
	if consultant.CallCount != 1 {
		t.Errorf("want exactly one consultation, got %d", consultant.CallCount)
	}
}

// Orchestrated mode: E501 from an agent or a harness error is a consultation,
// not a mechanical re-dispatch.
func TestSession_E501_OrchestratedMode_ConsultsInsteadOfRedispatching(t *testing.T) {
	entries := map[string]harness.ScriptedEntry{
		"agent reply":   e501Entry(),
		"harness error": failEntry("harness: subprocess timed out"),
	}
	for name, entry := range entries {
		t.Run(name, func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueDispatch("agent-a", "do the work", 0)
			consultant.queueStop("decided after the failure")
			ses, f, _, orchPath := newOrchestratedSession(t, consultant)
			queueRepeated(f, "agent-a", 3, entry)

			got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 1 {
				t.Errorf("want 1 agent-a dispatch, got %d", n)
			}
			if consultant.CallCount != 2 {
				t.Errorf("want 2 consultations (dispatch, then after the failure), got %d", consultant.CallCount)
			}
		})
	}
}

// ===== the count survives a restart (real artifact file store) =====

type restartCase struct {
	name         string
	prior        []seedRow
	wantAttempts int // dispatches made after the resume; each fails the way the test scripts
}

// runBudgetRestart seeds the run with tc.prior, resumes it with an agent that
// fails every dispatch with entry, and asserts the dispatches made before the
// single consultation.
func runBudgetRestart(t *testing.T, tc restartCase, entry harness.ScriptedEntry) {
	t.Helper()
	run := seedFileStoreRun(t, tc.prior)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	f := harness.NewMockAdapter()
	queueRepeated(f, "agent-a", 5, entry)
	ses := run.newSession(f, consultant)

	got, err := ses.Start(context.Background(), run.resumeConfig())

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != tc.wantAttempts {
		t.Errorf("want %d dispatches after the resume, got %d", tc.wantAttempts, n)
	}
	if consultant.CallCount != 1 {
		t.Errorf("want exactly one consultation once the budget is used up, got %d", consultant.CallCount)
	}
}

func TestSession_E501_BudgetSurvivesRestartThroughArtifactFile(t *testing.T) {
	e501 := seedRow{domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE}
	harnessErr := seedRow{domain.StatusHARNESS_ERROR, domain.ErrorNone}
	cases := []restartCase{
		{"one earlier attempt leaves two", []seedRow{e501}, 2},
		{"two earlier attempts leave one", []seedRow{e501, e501}, 1},
		{"all three used up", []seedRow{e501, e501, e501}, 0},
		{"another error code at the row does not use the budget",
			[]seedRow{{domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING}, e501, e501}, 1},
		{"a SUCCESS at the row resets the budget",
			[]seedRow{e501, e501, e501, {domain.StatusSUCCESS, domain.ErrorNone}, e501}, 2},
		{"harness errors at the row do not use the E501 budget",
			[]seedRow{harnessErr, harnessErr, harnessErr, e501}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runBudgetRestart(t, tc, e501Entry())
		})
	}
}

// A trailing HARNESS_ERROR row is an unaccepted attempt: the resume re-runs it
// once whatever the budget, and the budget read from the whole log (including
// the rows before the restart) bounds what follows.
func TestSession_HarnessError_BudgetSurvivesRestartThroughArtifactFile(t *testing.T) {
	e501 := seedRow{domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE}
	harnessErr := seedRow{domain.StatusHARNESS_ERROR, domain.ErrorNone}
	cases := []restartCase{
		{"one earlier attempt leaves two", []seedRow{harnessErr}, 2},
		{"two earlier attempts leave one", []seedRow{harnessErr, harnessErr}, 1},
		{"all three used up still re-runs the unaccepted attempt once", []seedRow{harnessErr, harnessErr, harnessErr}, 1},
		{"another error code at the row does not use the budget",
			[]seedRow{{domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING}, harnessErr, harnessErr}, 1},
		{"a SUCCESS at the row resets the budget",
			[]seedRow{harnessErr, harnessErr, harnessErr, {domain.StatusSUCCESS, domain.ErrorNone}, harnessErr}, 2},
		{"old synthetic E501 rows do not use the harness error budget",
			[]seedRow{e501, e501}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runBudgetRestart(t, tc, failEntry("harness: still down"))
		})
	}
}

// The rows a live run records, including the failed bypass, carry their status
// through the Execution Log of the real file store, without an error code.
func TestSession_HarnessError_RowsReadBackFromArtifactFileKeepTheirStatus(t *testing.T) {
	run := emptyFileStoreRun(t)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	f := harness.NewMockAdapter()
	f.Queue("agent-a", rawTextEntry(), failEntry("harness: bypass failed"), failEntry("harness: third"))
	ses := run.newSession(f, consultant)

	got, err := ses.Start(context.Background(), run.newRunConfig())

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	state, err := artifact.NewFileStore(run.artifactPath).Read(context.Background())
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if left := engine.HarnessErrorBudgetRemaining(state.ExecutionLog, domain.RoutingRow{Index: 0}, ""); left != 0 {
		t.Errorf("want the budget read back from the file as used up, got %d left", left)
	}
	n := 0
	for _, e := range state.ExecutionLog {
		if e.Status == domain.StatusHARNESS_ERROR && e.ErrorCode == domain.ErrorNone {
			n++
		}
	}
	if n != engine.HarnessErrorAttemptLimit {
		t.Errorf("want %d HARNESS_ERROR rows read back, got %d", engine.HarnessErrorAttemptLimit, n)
	}
	if left := engine.E501BudgetRemaining(state.ExecutionLog, domain.RoutingRow{Index: 0}, ""); left != engine.E501AttemptLimit {
		t.Errorf("want the E501 budget untouched by harness errors, got %d left", left)
	}
	if cs := state.CurrentState; cs.LastAgent != "" || cs.LastStatus != "" {
		t.Errorf("want current_state untouched by HARNESS_ERROR rows, got last_agent=%q last_status=%q", cs.LastAgent, cs.LastStatus)
	}
}

// ===== other codes and statuses consult and are never re-dispatched =====

func TestSession_OtherFailures_ConsultWithoutMechanicalRedispatch(t *testing.T) {
	blocked := func(code domain.ErrorCode) harness.ScriptedEntry {
		return statusEntry(domain.StatusBLOCKED, code, "cannot continue")
	}
	cases := []struct {
		name  string
		entry harness.ScriptedEntry
	}{
		{"BLOCKED E100", blocked(domain.ErrorINVALID_INVOCATION)},
		{"BLOCKED E101", blocked(domain.ErrorINPUT_NOT_FOUND)},
		{"BLOCKED E401", blocked(domain.ErrorDEPENDENCY_MISSING)},
		{"BLOCKED E502", blocked(domain.ErrorPERMISSION_DENIED)},
		{"BLOCKED E503", blocked(domain.ErrorUSER_CONTACT)},
		{"NEEDS_CLARIFICATION", statusEntry(domain.StatusNEEDS_CLARIFICATION, domain.ErrorNone, "which database?")},
		{"CAPABILITY_EXCEEDED", statusEntry(domain.StatusCAPABILITY_EXCEEDED, domain.ErrorNone, "too hard")},
	}
	for _, mode := range autoModes {
		for _, tc := range cases {
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				consultant := &scriptedRoutingConsultant{}
				consultant.queueStop("consulted")
				ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
				queueRepeated(f, "agent-a", 4, tc.entry)

				got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

				requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
				if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 1 {
					t.Errorf("want 1 agent-a dispatch (no mechanical re-dispatch), got %d", n)
				}
				if consultant.CallCount != 1 {
					t.Errorf("want exactly one consultation, got %d", consultant.CallCount)
				}
			})
		}
	}
}
