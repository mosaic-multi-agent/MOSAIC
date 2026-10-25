package analysis_test

// Sessions: one entry per distinct session id, in feed order of the first
// event naming it, with start/end timestamps, resumed, reason, model and
// provider as logged.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func TestRunSessions_SingleSession_KeepsEveryLoggedFact(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfSessionStart("s1", rmAt(10), domain.SessionStartFields{Resumed: domain.BoolTrue, Model: "sm", Provider: "sp"}),
		rfSessionEnd("s1", rmAt(90), "user_exit"),
	))

	if len(run.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(run.Sessions))
	}
	s := run.Sessions[0]
	if s.ID != "s1" || s.Model != "sm" || s.Provider != "sp" || s.Reason != "user_exit" {
		t.Errorf("session = %+v", s)
	}
	if !s.StartedAt.Equal(rmAt(10)) || !s.EndedAt.Equal(rmAt(90)) {
		t.Errorf("StartedAt/EndedAt = %v/%v, want %v/%v", s.StartedAt, s.EndedAt, rmAt(10), rmAt(90))
	}
	if v, present := s.Resumed.Value(); !present || !v {
		t.Errorf("Resumed = %v, want present true", s.Resumed)
	}
}

func TestRunSessions_ResumedFalseIsDistinctFromAbsent(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfSessionStart("explicit", rmAt(1), domain.SessionStartFields{Resumed: domain.BoolFalse}),
		rfSessionStart("silent", rmAt(2), domain.SessionStartFields{}),
	))

	byID := map[string]domain.RunSession{}
	for _, s := range run.Sessions {
		byID[s.ID] = s
	}
	if byID["explicit"].Resumed != domain.BoolFalse {
		t.Errorf("explicit resumed=false became %v", byID["explicit"].Resumed)
	}
	if byID["silent"].Resumed != domain.BoolAbsent {
		t.Errorf("unlogged resumed became %v, want absent", byID["silent"].Resumed)
	}
}

func TestRunSessions_AbsentOptionalFields_StayAbsent(t *testing.T) {
	run := rfRun(t, rfOrch(rfSessionStart("s1", time.Time{}, domain.SessionStartFields{})))

	if len(run.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(run.Sessions))
	}
	s := run.Sessions[0]
	if s.ID != "s1" {
		t.Errorf("ID = %q, want s1", s.ID)
	}
	if !s.StartedAt.IsZero() || !s.EndedAt.IsZero() || s.Model != "" || s.Provider != "" || s.Reason != "" {
		t.Errorf("absent fields were invented: %+v", s)
	}
}

func TestRunSessions_SessionEndWithoutStart_KeepsEndFacts(t *testing.T) {
	run := rfRun(t, rfOrch(rfSessionEnd("s9", rmAt(70), "crash")))

	if len(run.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(run.Sessions))
	}
	s := run.Sessions[0]
	if s.ID != "s9" || s.Reason != "crash" || !s.EndedAt.Equal(rmAt(70)) || !s.StartedAt.IsZero() {
		t.Errorf("session = %+v", s)
	}
}

func TestRunSessions_RepeatedSessionId_IsOneEntryWithFirstStartAndLastLoggedValues(t *testing.T) {
	run := rfRun(t, rfOrch(
		rfSessionStart("s1", rmAt(10), domain.SessionStartFields{Model: "m1", Provider: "p1"}),
		rfSessionStart("s1", rmAt(20), domain.SessionStartFields{Resumed: domain.BoolTrue}),
		rfSessionEnd("s1", rmAt(50), "paused"),
		rfSessionEnd("s1", rmAt(80), ""),
	))

	if len(run.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1 per distinct id", len(run.Sessions))
	}
	s := run.Sessions[0]
	if !s.StartedAt.Equal(rmAt(10)) {
		t.Errorf("StartedAt = %v, want the first session_start %v", s.StartedAt, rmAt(10))
	}
	if !s.EndedAt.Equal(rmAt(80)) {
		t.Errorf("EndedAt = %v, want the last session_end %v", s.EndedAt, rmAt(80))
	}
	if v, present := s.Resumed.Value(); !present || !v {
		t.Errorf("Resumed = %v, want true from the later session_start", s.Resumed)
	}
	if s.Model != "m1" || s.Provider != "p1" {
		t.Errorf("Model/Provider = %q/%q, want earlier values kept when the later start carried none", s.Model, s.Provider)
	}
	if s.Reason != "paused" {
		t.Errorf("Reason = %q, want the last one that carried a reason", s.Reason)
	}
}

func TestRunSessions_DistinctIds_AreInFeedOrderOfFirstEventNamingEach(t *testing.T) {
	run := rfRun(t,
		rfOrch(
			rfSessionStart("s2", rmAt(30), domain.SessionStartFields{}),
			rfSessionStart("s1", rmAt(10), domain.SessionStartFields{}),
		),
		rfAgent("w#1", rfSessionStart("s3", rmAt(5), domain.SessionStartFields{}), rfSessionEnd("s2", rmAt(60), "done")),
	)

	got := make([]string, len(run.Sessions))
	for i, s := range run.Sessions {
		got[i] = s.ID
	}
	if !rmEqualStrings(got, []string{"s2", "s1", "s3"}) {
		t.Fatalf("session order = %v, want [s2 s1 s3] (feed order, not time order)", got)
	}
	if len(run.Sessions) == 0 || !run.Sessions[0].EndedAt.Equal(rmAt(60)) {
		t.Errorf("session_end from another stream not merged into s2: %+v", run.Sessions[0])
	}
}

func TestRunSessions_AnyEventNamingASession_FixesItsPosition(t *testing.T) {
	turn := rfTurn(domain.TurnAssistant, rmAt(1), domain.TokenUsage{Input: domain.Tokens(1)})
	turn.SessionID = "s5"
	run := rfRun(t, rfOrch(
		turn,
		rfSessionStart("s1", rmAt(2), domain.SessionStartFields{}),
		rfSessionStart("s5", rmAt(3), domain.SessionStartFields{Model: "m"}),
	))

	got := make([]string, len(run.Sessions))
	for i, s := range run.Sessions {
		got[i] = s.ID
	}
	if !rmEqualStrings(got, []string{"s5", "s1"}) {
		t.Fatalf("session order = %v, want [s5 s1]", got)
	}
	if !run.Sessions[0].StartedAt.Equal(rmAt(3)) || run.Sessions[0].Model != "m" {
		t.Errorf("s5 lost its session_start facts: %+v", run.Sessions[0])
	}
}

func TestRunSessions_NoSessionEvents_ListIsEmpty(t *testing.T) {
	run := rfRun(t, rfOrch(usageRecordEvent("", "rec-1", "m", domain.TokenUsage{Input: domain.Tokens(1)})))

	if len(run.Sessions) != 0 {
		t.Errorf("sessions invented: %+v", run.Sessions)
	}
}

func TestRunSessions_AreKeptPerRunAndInUnattributableBucket(t *testing.T) {
	agg, _ := rfAggregate(
		rfOrch(rfSessionStart("named-s", rmAt(1), domain.SessionStartFields{})),
		rfUnattributable(rfSessionStart("stray-s", rmAt(2), domain.SessionStartFields{Provider: "sp"})),
	)

	if len(agg.Runs[0].Sessions) != 1 || agg.Runs[0].Sessions[0].ID != "named-s" {
		t.Errorf("named run sessions = %+v", agg.Runs[0].Sessions)
	}
	if agg.Unattributable == nil || len(agg.Unattributable.Sessions) != 1 {
		t.Fatalf("bucket sessions missing: %+v", agg.Unattributable)
	}
	if s := agg.Unattributable.Sessions[0]; s.ID != "stray-s" || s.Provider != "sp" {
		t.Errorf("bucket session = %+v", s)
	}
}
