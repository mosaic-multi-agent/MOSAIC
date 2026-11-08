package session_test

// Tests for the resume of an unaccepted attempt, by the way the attempt was
// reached, plus what the resume keeps and what keeps counting afterwards.
//
// Coverage:
//
//   - Every origin that leaves an unaccepted workflow row in the log (route-back
//     target, consultant-routed target, manual dispatch, HITL-rejected attempt
//     after a consultation, rejected retry, trailing consultation entry)
//     re-dispatches that row with its recorded Inputs, one table case per origin.
//   - The accepted current_state is handed on unchanged to the first recorded step.
//   - A trailing agent unknown to the workflow ends the run without a dispatch.
//   - The PARTIALLY_DONE bound, E501 budget and review-loop limit stop a resumed
//     run exactly where they stop a live run.

import (
	"context"
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// snapshotStore records the current_state the session hands to Apply.
type snapshotStore struct {
	*memStore
	StatesAtApply []domain.CurrentState
}

func (s *snapshotStore) Apply(ctx context.Context, state domain.ArtifactState, step domain.CompletedStep) (domain.ArtifactState, error) {
	s.StatesAtApply = append(s.StatesAtApply, state.CurrentState)
	return s.memStore.Apply(ctx, state, step)
}

func countInvocations(f *harness.MockAdapter, agent string) int {
	n := 0
	for _, inv := range f.Invocations() {
		if inv.Agent.Identifier == agent {
			n++
		}
	}
	return n
}

func queueForAll(f *harness.MockAdapter) {
	for _, a := range []string{"agent-a", "agent-b", "agent-c"} {
		queueSuccesses(f, a, 4)
	}
}

func current(agent string, status domain.StatusCode) domain.CurrentState {
	return domain.CurrentState{Phase: "PLANNING", LastStatus: status, LastAgent: agent}
}

func consultEntry(seq int) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{Seq: seq, Agent: "orchestrator#" + strconv.Itoa(seq), Status: domain.StatusSUCCESS}
}


type originCase struct {
	name      string
	current   domain.CurrentState
	log       []domain.ExecutionLogEntry
	wantAgent string
	wantID    string
	wantRow   int // zero-based row index
	wantIn    []string
}

func originCases() []originCase {
	return []originCase{
		{
			name:    "route-back target",
			current: current("agent-b#2", domain.StatusCOMPLETED_NEEDS_ACTION),
			log: []domain.ExecutionLogEntry{
				logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
				logEntry(2, "agent-b#2", 2, domain.StatusCOMPLETED_NEEDS_ACTION, "design.md"),
				logEntry(3, "agent-a#3", 1, domain.StatusSUCCESS, "design.md, review.md"),
			},
			wantAgent: "agent-a", wantID: "agent-a#4", wantRow: 0, wantIn: []string{"design.md", "review.md"},
		},
		{
			name:    "consultant-routed target",
			current: current("agent-a#1", domain.StatusSUCCESS),
			log: []domain.ExecutionLogEntry{
				logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
				consultEntry(2),
				logEntry(3, "agent-c#3", 3, domain.StatusSUCCESS, "review.md"),
			},
			wantAgent: "agent-c", wantID: "agent-c#4", wantRow: 2, wantIn: []string{"review.md"},
		},
		{
			name:    "manual dispatch back to an earlier row",
			current: current("agent-b#2", domain.StatusSUCCESS),
			log: []domain.ExecutionLogEntry{
				logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
				logEntry(2, "agent-b#2", 2, domain.StatusSUCCESS, "design.md"),
				logEntry(3, "agent-a#3", 1, domain.StatusSUCCESS, "notes.md"),
			},
			wantAgent: "agent-a", wantID: "agent-a#4", wantRow: 0, wantIn: []string{"notes.md"},
		},
		{
			name:    "HITL-rejected attempt after a consultation",
			current: current("agent-a#1", domain.StatusSUCCESS),
			log: []domain.ExecutionLogEntry{
				logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
				consultEntry(2),
				logEntry(3, "agent-b#3", 2, domain.StatusPARTIALLY_DONE, "design.md, extra.md"),
			},
			wantAgent: "agent-b", wantID: "agent-b#4", wantRow: 1, wantIn: []string{"design.md", "extra.md"},
		},
		{
			name:    "rejected attempt that was itself a retry",
			current: current("agent-a#1", domain.StatusSUCCESS),
			log: []domain.ExecutionLogEntry{
				logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
				logEntry(2, "agent-b#2", 2, domain.StatusPARTIALLY_DONE, "design.md"),
				logEntry(3, "agent-b#3", 2, domain.StatusPARTIALLY_DONE, "design.md, retry.md"),
			},
			wantAgent: "agent-b", wantID: "agent-b#4", wantRow: 1, wantIn: []string{"design.md", "retry.md"},
		},
		{
			name:    "trailing consultation entry after the attempt",
			current: current("agent-a#1", domain.StatusSUCCESS),
			log: []domain.ExecutionLogEntry{
				logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
				logEntry(2, "agent-c#2", 3, domain.StatusSUCCESS, "review.md"),
				consultEntry(3),
			},
			wantAgent: "agent-c", wantID: "agent-c#4", wantRow: 2, wantIn: []string{"review.md"},
		},
	}
}

func TestSession_Resume_UnacceptedAttempt_RedispatchedByOrigin(t *testing.T) {
	for _, tc := range originCases() {
		t.Run(tc.name, func(t *testing.T) {
			ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
			cfg := hitlReviewResume(store, orchPath, tc.current, tc.log...)
			queueForAll(f)

			if _, err := ses.Start(context.Background(), cfg); err != nil {
				t.Fatalf("Start: %v", err)
			}

			invs := f.Invocations()
			if len(invs) == 0 || invs[0].Agent.Identifier != tc.wantAgent {
				t.Fatalf("want %s dispatched first, got %+v", tc.wantAgent, invs)
			}
			if id := invs[0].Request.AgentInstanceID; id != tc.wantID {
				t.Errorf("want instance %s, got %s", tc.wantID, id)
			}
			in := bareInputs(invs[0].Request.InputArtifacts)
			if len(in) != len(tc.wantIn) {
				t.Fatalf("want recorded Inputs %v, got %v", tc.wantIn, in)
			}
			for i := range in {
				if in[i] != tc.wantIn[i] {
					t.Errorf("want recorded Inputs %v, got %v", tc.wantIn, in)
				}
			}
			if len(store.Applied) == 0 || store.Applied[0].WorkflowRow != domain.WorkflowRowFromIndex(tc.wantRow) {
				t.Errorf("want the re-dispatch recorded against row index %d, got %+v", tc.wantRow, store.Applied)
			}
		})
	}
}

func TestSession_Resume_UnacceptedAttempt_KeepsAcceptedCurrentStateForTheFirstRecordedStep(t *testing.T) {
	_, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	snap := &snapshotStore{memStore: store}
	ses := session.New(session.Deps{
		Harness: f, Store: snap, Approvals: &fixedApprovalReader{domain.ApprovalTrue},
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
	})
	recorded := current("agent-b#2", domain.StatusCOMPLETED_NEEDS_ACTION)
	cfg := hitlReviewResume(store, orchPath, recorded,
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "agent-b#2", 2, domain.StatusCOMPLETED_NEEDS_ACTION, "design.md"),
		logEntry(3, "agent-a#3", 1, domain.StatusSUCCESS, "design.md, review.md"),
	)
	queueForAll(f)

	if _, err := ses.Start(context.Background(), cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(snap.StatesAtApply) == 0 {
		t.Fatal("want at least one recorded step")
	}
	if got := snap.StatesAtApply[0]; got != recorded {
		t.Errorf("want the accepted current_state %+v before the re-dispatch is applied, got %+v", recorded, got)
	}
}

func TestSession_Resume_UnacceptedAttemptOfUnknownAgent_EndsWithoutDispatch(t *testing.T) {
	ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	cfg := hitlReviewResume(store, orchPath, current("agent-a#1", domain.StatusSUCCESS),
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "mystery-agent#2", 2, domain.StatusSUCCESS, ""),
	)
	queueForAll(f)

	got, _ := ses.Start(context.Background(), cfg)

	if got.Status == domain.RunCompleted {
		t.Errorf("want the run not completed when the recorded attempt cannot be resolved, got %q", got.Status)
	}
	if n := len(f.Invocations()); n != 0 {
		t.Errorf("want no dispatch for an unresolvable attempt, got %d", n)
	}
}

// ---- counters keep counting after the re-dispatch ----

func liveConfig(orchPath string, limit int) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           "hitl-review",
		Task:                 "test task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAutoReview, ReviewLoopLimit: limit},
	}
}

// resumedRouteBackConfig resumes a run whose route-back to agent-a was recorded
// but not accepted.
func resumedRouteBackConfig(store *memStore, orchPath string, limit int) domain.RunConfig {
	cfg := hitlReviewResume(store, orchPath, current("agent-b#2", domain.StatusCOMPLETED_NEEDS_ACTION),
		logEntry(1, "agent-a#1", 1, domain.StatusSUCCESS, ""),
		logEntry(2, "agent-b#2", 2, domain.StatusCOMPLETED_NEEDS_ACTION, "design.md"),
		logEntry(3, "agent-a#3", 1, domain.StatusSUCCESS, "design.md, review.md"),
	)
	cfg.RunSettings.ReviewLoopLimit = limit
	store.state.RunSettings.ReviewLoopLimit = limit
	return cfg
}

// TestSession_Resume_RedispatchedAgentRepeatingARetryableStatus_StopsWhereALiveRunStops
// compares the retry counts of a run resumed at an unaccepted agent-a attempt
// with those of a live run whose agent-a reports the same status every time.
func TestSession_Resume_RedispatchedAgentRepeatingARetryableStatus_StopsWhereALiveRunStops(t *testing.T) {
	cases := map[string]harness.ScriptedEntry{
		"PARTIALLY_DONE bound": statusEntry(domain.StatusPARTIALLY_DONE, "", "partial"),
		"E501 budget":          statusEntry(domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "tool down"),
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			const queued = 12
			liveSes, liveF, _, liveOrch := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
			queueRepeated(liveF, "agent-a", queued, entry)
			_, _ = liveSes.Start(context.Background(), liveConfig(liveOrch, 0))
			liveCount := countInvocations(liveF, "agent-a")

			ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
			cfg := resumedRouteBackConfig(store, orchPath, 0)
			queueRepeated(f, "agent-a", queued, entry)
			_, _ = ses.Start(context.Background(), cfg)

			got := countInvocations(f, "agent-a")
			if liveCount < 2 || liveCount >= queued {
				t.Fatalf("live run must retry and then stop before the script runs out, got %d dispatches", liveCount)
			}
			if got != liveCount {
				t.Errorf("want the resumed run to stop after %d agent-a dispatches like a live run, got %d", liveCount, got)
			}
		})
	}
}

// TestSession_Resume_RedispatchedRouteBack_CountsAgainstTheReviewLoopLimit
// compares a resumed run with a live run whose reviewer always reports findings:
// the findings recorded before the stop count toward the limit.
func TestSession_Resume_RedispatchedRouteBack_CountsAgainstTheReviewLoopLimit(t *testing.T) {
	const queued = 12
	findings := statusEntry(domain.StatusCOMPLETED_NEEDS_ACTION, "", "findings")

	liveSes, liveF, _, liveOrch := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	queueSuccesses(liveF, "agent-a", queued)
	queueRepeated(liveF, "agent-b", queued, findings)
	_, _ = liveSes.Start(context.Background(), liveConfig(liveOrch, 2))
	liveReviews := countInvocations(liveF, "agent-b")

	ses, f, store, orchPath := newHITLReviewSession(t, &fixedApprovalReader{domain.ApprovalTrue})
	cfg := resumedRouteBackConfig(store, orchPath, 2)
	queueSuccesses(f, "agent-a", queued)
	queueRepeated(f, "agent-b", queued, findings)
	_, _ = ses.Start(context.Background(), cfg)

	// One review (agent-b#2) is already in the log of the resumed run.
	resumedReviews := countInvocations(f, "agent-b") + 1
	if liveReviews < 2 || liveReviews >= queued {
		t.Fatalf("live run must loop and then stop at the limit, got %d reviews", liveReviews)
	}
	if resumedReviews != liveReviews {
		t.Errorf("want %d reviews in total like a live run (limit counts the recorded findings), got %d", liveReviews, resumedReviews)
	}
}
