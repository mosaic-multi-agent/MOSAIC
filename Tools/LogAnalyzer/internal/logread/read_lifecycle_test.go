package logread_test

// Tests for decoding the run and session lifecycle facts.
//
// Rules under test:
//   - run_start decodes adapter_version, model and provider; run_end decodes
//     outcome; session_start decodes resumed, model and provider; session_end
//     decodes reason.
//   - The block of a lifecycle event is non-nil even when every optional field
//     is absent, and absent strings decode as "".
//   - An absent boolean (resumed) is BoolAbsent, never BoolFalse; a wrong-typed
//     value is absent and does not make the line malformed.
//   - Unknown extra fields are ignored; usage_record identity and timestamp
//     keep reaching the decoded event.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func TestReader_RunStart_DecodesAdapterVersionModelAndProvider(t *testing.T) {
	ev, findings := readOne(t, rmLine("run_start", "2026-10-01T10:00:00.000Z",
		`"adapter_version":"0.9.1","model":"claude-opus","provider":"anthropic"`))

	if ev.Type != domain.EventRunStart || ev.RunStart == nil {
		t.Fatalf("Type = %q, RunStart = %v; want run_start with a block", ev.Type, ev.RunStart)
	}
	want := domain.RunStartFields{AdapterVersion: "0.9.1", Model: "claude-opus", Provider: "anthropic"}
	if *ev.RunStart != want {
		t.Errorf("RunStart = %+v, want %+v", *ev.RunStart, want)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_RunStart_AllOptionalFieldsAbsent_BlockPresentWithEmptyValues(t *testing.T) {
	ev, _ := readOne(t, rmLine("run_start", "2026-10-01T10:00:00.000Z", ""))

	if ev.RunStart == nil {
		t.Fatal("RunStart block must be non-nil even with no optional fields")
	}
	if *ev.RunStart != (domain.RunStartFields{}) {
		t.Errorf("RunStart = %+v, want all-empty", *ev.RunStart)
	}
}

func TestReader_RunStart_EachFieldDecodesIndependently(t *testing.T) {
	ev, _ := readOne(t, rmLine("run_start", "2026-10-01T10:00:00.000Z", `"provider":"openai"`))

	if ev.RunStart == nil {
		t.Fatal("RunStart block is nil")
	}
	want := domain.RunStartFields{Provider: "openai"}
	if *ev.RunStart != want {
		t.Errorf("RunStart = %+v, want %+v", *ev.RunStart, want)
	}
}

func TestReader_RunEnd_DecodesOutcome(t *testing.T) {
	ev, _ := readOne(t, rmLine("run_end", "2026-10-01T11:00:00.000Z", `"outcome":"completed"`))

	if ev.Type != domain.EventRunEnd || ev.RunEnd == nil {
		t.Fatalf("Type = %q, RunEnd = %v; want run_end with a block", ev.Type, ev.RunEnd)
	}
	if ev.RunEnd.Outcome != "completed" {
		t.Errorf("Outcome = %q, want completed", ev.RunEnd.Outcome)
	}
}

func TestReader_RunEnd_OutcomeAbsent_BlockPresentEmpty(t *testing.T) {
	ev, _ := readOne(t, rmLine("run_end", "2026-10-01T11:00:00.000Z", ""))

	if ev.RunEnd == nil {
		t.Fatal("RunEnd block must be non-nil even with no outcome")
	}
	if ev.RunEnd.Outcome != "" {
		t.Errorf("Outcome = %q, want empty", ev.RunEnd.Outcome)
	}
}

func TestReader_SessionStart_DecodesResumedModelAndProvider(t *testing.T) {
	cases := []struct {
		name   string
		extra  string
		resume domain.OptionalBool
	}{
		{"resumed true", `"resumed":true,"model":"m1","provider":"p1"`, domain.BoolTrue},
		{"resumed false", `"resumed":false,"model":"m1","provider":"p1"`, domain.BoolFalse},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, findings := readOne(t, rmLine("session_start", "2026-10-01T10:00:00.500Z", c.extra))

			if ev.Type != domain.EventSessionStart || ev.SessionStart == nil {
				t.Fatalf("Type = %q, SessionStart = %v; want session_start with a block", ev.Type, ev.SessionStart)
			}
			want := domain.SessionStartFields{Resumed: c.resume, Model: "m1", Provider: "p1"}
			if *ev.SessionStart != want {
				t.Errorf("SessionStart = %+v, want %+v", *ev.SessionStart, want)
			}
			if ev.SessionID != "s1" {
				t.Errorf("SessionID = %q, want s1 (stays in the envelope)", ev.SessionID)
			}
			if len(findings) != 0 {
				t.Errorf("unexpected findings: %v", findings)
			}
		})
	}
}

func TestReader_SessionStart_ResumedAbsent_IsAbsentNotFalse(t *testing.T) {
	ev, _ := readOne(t, rmLine("session_start", "2026-10-01T10:00:00.500Z", `"model":"m1"`))

	if ev.SessionStart == nil {
		t.Fatal("SessionStart block is nil")
	}
	if ev.SessionStart.Resumed != domain.BoolAbsent {
		t.Errorf("Resumed = %v, want BoolAbsent", ev.SessionStart.Resumed)
	}
	if ev.SessionStart.Model != "m1" || ev.SessionStart.Provider != "" {
		t.Errorf("SessionStart = %+v, want Model m1 and empty Provider", *ev.SessionStart)
	}
}

func TestReader_SessionStart_AllOptionalFieldsAbsent_BlockPresent(t *testing.T) {
	ev, _ := readOne(t, rmLine("session_start", "2026-10-01T10:00:00.500Z", ""))

	if ev.SessionStart == nil {
		t.Fatal("SessionStart block must be non-nil even with no optional fields")
	}
	if *ev.SessionStart != (domain.SessionStartFields{}) {
		t.Errorf("SessionStart = %+v, want all-empty with BoolAbsent", *ev.SessionStart)
	}
}

func TestReader_SessionStart_WrongTypedResumed_AbsentAndNotMalformed(t *testing.T) {
	for _, bad := range []string{`"yes"`, `1`, `null`, `{"a":1}`} {
		t.Run(bad, func(t *testing.T) {
			ev, findings := readOne(t, rmLine("session_start", "2026-10-01T10:00:00.500Z",
				`"resumed":`+bad+`,"model":"m1"`))

			if ev.SessionStart == nil {
				t.Fatal("SessionStart block is nil")
			}
			if ev.SessionStart.Resumed != domain.BoolAbsent {
				t.Errorf("Resumed = %v, want BoolAbsent", ev.SessionStart.Resumed)
			}
			if ev.SessionStart.Model != "m1" {
				t.Errorf("Model = %q, want m1 (other fields still decode)", ev.SessionStart.Model)
			}
			if len(findings) != 0 {
				t.Errorf("wrong-typed field must not raise findings, got %v", findings)
			}
		})
	}
}

func TestReader_SessionEnd_DecodesReason(t *testing.T) {
	ev, _ := readOne(t, rmLine("session_end", "2026-10-01T10:30:00.000Z", `"reason":"user_exit"`))

	if ev.Type != domain.EventSessionEnd || ev.SessionEnd == nil {
		t.Fatalf("Type = %q, SessionEnd = %v; want session_end with a block", ev.Type, ev.SessionEnd)
	}
	if ev.SessionEnd.Reason != "user_exit" {
		t.Errorf("Reason = %q, want user_exit", ev.SessionEnd.Reason)
	}
}

func TestReader_SessionEnd_ReasonAbsent_BlockPresentEmpty(t *testing.T) {
	ev, _ := readOne(t, rmLine("session_end", "2026-10-01T10:30:00.000Z", ""))

	if ev.SessionEnd == nil {
		t.Fatal("SessionEnd block must be non-nil even with no reason")
	}
	if ev.SessionEnd.Reason != "" {
		t.Errorf("Reason = %q, want empty", ev.SessionEnd.Reason)
	}
}

func TestReader_LifecycleEvents_WrongTypedStrings_DecodeAsAbsent(t *testing.T) {
	events, findings := readTierLines(t,
		rmLine("run_start", "2026-10-01T10:00:00.000Z", `"adapter_version":5,"model":["x"],"provider":true`),
		rmLine("run_end", "2026-10-01T11:00:00.000Z", `"outcome":7`),
		rmLine("session_end", "2026-10-01T10:30:00.000Z", `"reason":{"a":1}`))

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if events[0].RunStart == nil || *events[0].RunStart != (domain.RunStartFields{}) {
		t.Errorf("RunStart = %+v, want all-empty", events[0].RunStart)
	}
	if events[1].RunEnd == nil || events[1].RunEnd.Outcome != "" {
		t.Errorf("RunEnd = %+v, want empty outcome", events[1].RunEnd)
	}
	if events[2].SessionEnd == nil || events[2].SessionEnd.Reason != "" {
		t.Errorf("SessionEnd = %+v, want empty reason", events[2].SessionEnd)
	}
	if len(findings) != 0 {
		t.Errorf("wrong-typed fields must not raise findings, got %v", findings)
	}
}

func TestReader_LifecycleEvents_UnknownExtraFieldsIgnored(t *testing.T) {
	events, findings := readTierLines(t,
		rmLine("run_start", "2026-10-01T10:00:00.000Z", `"model":"m","future_field":{"a":1},"cwd":"/secret/path-MK1","prompt":"PROMPT-MK2"`),
		rmLine("run_end", "2026-10-01T11:00:00.000Z", `"outcome":"ok","future_field":[1,2],"response":"RESPONSE-MK3"`),
		rmLine("session_start", "2026-10-01T10:00:00.500Z", `"resumed":true,"future_field":"x","cwd":"/secret/path-MK1"`),
		rmLine("session_end", "2026-10-01T10:30:00.000Z", `"reason":"r","future_field":null,"error":"ERROR-MK4"`))

	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	if events[0].RunStart == nil || *events[0].RunStart != (domain.RunStartFields{Model: "m"}) {
		t.Errorf("RunStart = %+v, want only Model m", events[0].RunStart)
	}
	if events[1].RunEnd == nil || events[1].RunEnd.Outcome != "ok" {
		t.Errorf("RunEnd = %+v, want outcome ok", events[1].RunEnd)
	}
	if events[2].SessionStart == nil || *events[2].SessionStart != (domain.SessionStartFields{Resumed: domain.BoolTrue}) {
		t.Errorf("SessionStart = %+v, want only Resumed true", events[2].SessionStart)
	}
	if events[3].SessionEnd == nil || events[3].SessionEnd.Reason != "r" {
		t.Errorf("SessionEnd = %+v, want reason r", events[3].SessionEnd)
	}
	if len(findings) != 0 {
		t.Errorf("unknown fields must not raise findings, got %v", findings)
	}
	for _, ev := range events {
		assertNoMarkers(t, ev, "/secret/path-MK1", "PROMPT-MK2", "RESPONSE-MK3", "ERROR-MK4")
	}
}

func TestReader_ContentFieldsOnOtherEvents_NeverReachEvent(t *testing.T) {
	events, _ := readTierLines(t,
		rmLine("turn", "2026-10-01T10:00:01.000Z", `"role":"assistant","content":"CONTENT-MK5"`),
		rmLine("tool_call_start", "2026-10-01T10:00:02.000Z", `"call_id":"c1","tool_name":"Read","tool_input":"INPUT-MK6"`),
		rmLine("tool_call_end", "2026-10-01T10:00:03.000Z", `"call_id":"c1","status":"error","tool_output":"OUTPUT-MK7","error":"ERROR-MK8"`))

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	for _, ev := range events {
		assertNoMarkers(t, ev, "CONTENT-MK5", "INPUT-MK6", "OUTPUT-MK7", "ERROR-MK8")
	}
}

func TestReader_NotificationAndLifecycle_OnPriorSchemaVersion_Recognised(t *testing.T) {
	line := `{"schema_version":"1.0.0","event":"notification","timestamp":"2026-10-01T10:00:01.000Z","harness":"claude-code","session_id":"s1","run_id":"` + rmRunID + `","notification_type":"idle"}`
	events, _ := readTierLines(t, line)

	if len(events) != 1 || events[0].Type != domain.EventNotification || events[0].Notification == nil {
		t.Fatalf("events = %+v; want one notification with a block", events)
	}
}

func TestReader_LifecycleEvents_CarryExactlyOneBlock(t *testing.T) {
	events, _ := readTierLines(t,
		rmLine("run_start", "2026-10-01T10:00:00.000Z", `"model":"m"`),
		rmLine("run_end", "2026-10-01T11:00:00.000Z", `"outcome":"ok"`),
		rmLine("session_start", "2026-10-01T10:00:00.500Z", `"resumed":true`),
		rmLine("session_end", "2026-10-01T10:30:00.000Z", `"reason":"r"`))

	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	for i, ev := range events {
		if n := blocksSet(ev); n != 1 {
			t.Errorf("event %d (%s) has %d blocks set, want exactly 1", i, ev.Type, n)
		}
	}
}

func TestReader_LifecycleEvents_KeepTimestamps(t *testing.T) {
	ev, _ := readOne(t, rmLine("run_end", "2026-10-01T11:00:00.250Z", `"outcome":"ok"`))

	want := time.Date(2026, 10, 1, 11, 0, 0, 250_000_000, time.UTC)
	if !ev.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", ev.Timestamp, want)
	}
}

func TestReader_UsageRecord_StillExposesIdentityTierAndTimestamp(t *testing.T) {
	ev, _ := readOne(t, rmLine("usage_record", "2026-10-01T10:00:03.000Z",
		`"agent_instance_id":"a#1","record_id":"rec-9","service_tier":"priority","model":"m"`))

	if ev.UsageRecord == nil {
		t.Fatal("UsageRecord block is nil")
	}
	if ev.UsageRecord.RecordID != "rec-9" || ev.UsageRecord.ServiceTier != "priority" {
		t.Errorf("UsageRecord = %+v, want RecordID rec-9 and ServiceTier priority", *ev.UsageRecord)
	}
	want := time.Date(2026, 10, 1, 10, 0, 3, 0, time.UTC)
	if !ev.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", ev.Timestamp, want)
	}
}
