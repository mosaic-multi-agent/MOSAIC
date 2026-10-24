package logread_test

// Tests for decoding notification events.
//
// Rules under test:
//   - notification is its own event type and carries exactly the Notification
//     block, with notification_type, requires_response, resolution and
//     notification_id decoded.
//   - requires_response absent or wrong-typed is BoolAbsent, never BoolFalse.
//   - The message is never decoded: a marker placed in it appears nowhere in
//     the resulting event.
//   - Unknown extra fields are ignored.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestReader_Notification_DecodesAllFields(t *testing.T) {
	ev, findings := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z",
		`"notification_type":"permission_prompt","requires_response":true,"resolution":"approved","notification_id":"n-1","message":"hello"`))

	if ev.Type != domain.EventNotification {
		t.Fatalf("Type = %q, want %q", ev.Type, domain.EventNotification)
	}
	if ev.Notification == nil {
		t.Fatal("Notification block is nil")
	}
	want := domain.NotificationFields{
		NotificationType: "permission_prompt",
		RequiresResponse: domain.BoolTrue,
		Resolution:       "approved",
		NotificationID:   "n-1",
	}
	if *ev.Notification != want {
		t.Errorf("Notification = %+v, want %+v", *ev.Notification, want)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_Notification_RequiresResponseFalse_IsPresentFalse(t *testing.T) {
	ev, _ := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z",
		`"notification_type":"idle","requires_response":false`))

	if ev.Notification == nil {
		t.Fatal("Notification block is nil")
	}
	if ev.Notification.RequiresResponse != domain.BoolFalse {
		t.Errorf("RequiresResponse = %v, want BoolFalse", ev.Notification.RequiresResponse)
	}
}

func TestReader_Notification_OptionalFieldsAbsent_BlockPresentAndBoolAbsent(t *testing.T) {
	ev, findings := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z", ""))

	if ev.Type != domain.EventNotification || ev.Notification == nil {
		t.Fatalf("Type = %q, Notification = %v; want notification with a block", ev.Type, ev.Notification)
	}
	if *ev.Notification != (domain.NotificationFields{}) {
		t.Errorf("Notification = %+v, want all-empty with BoolAbsent", *ev.Notification)
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestReader_Notification_WrongTypedFields_AbsentAndNotMalformed(t *testing.T) {
	ev, findings := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z",
		`"notification_type":3,"requires_response":"yes","resolution":["a"],"notification_id":false`))

	if ev.Notification == nil {
		t.Fatal("Notification block is nil")
	}
	if *ev.Notification != (domain.NotificationFields{}) {
		t.Errorf("Notification = %+v, want all fields absent", *ev.Notification)
	}
	if len(findings) != 0 {
		t.Errorf("wrong-typed fields must not raise findings, got %v", findings)
	}
}

func TestReader_Notification_MessageMarkerAppearsNowhereInEvent(t *testing.T) {
	const marker = "SECRET-MARKER-4f9a"
	ev, _ := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z",
		`"notification_type":"idle","message":"`+marker+`"`))

	if ev.Type != domain.EventNotification || ev.Notification == nil {
		t.Fatalf("Type = %q, Notification = %v; want notification with a block", ev.Type, ev.Notification)
	}
	want := domain.NotificationFields{NotificationType: "idle"}
	if *ev.Notification != want {
		t.Errorf("Notification = %+v, want %+v (message must not be decoded)", *ev.Notification, want)
	}
	assertNoMarkers(t, ev, marker)
}

func TestReader_Notification_CarriesExactlyOneBlockAndKeepsEnvelope(t *testing.T) {
	ev, _ := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z", `"notification_type":"idle"`))

	if n := blocksSet(ev); n != 1 || ev.Notification == nil {
		t.Errorf("blocks set = %d, Notification = %v; want only the Notification block", n, ev.Notification)
	}
	if ev.SessionID != "s1" || ev.RunID != rmRunID || ev.Harness != domain.Harness("claude-code") {
		t.Errorf("envelope = session %q run %q harness %q, want s1, %s, claude-code",
			ev.SessionID, ev.RunID, ev.Harness, rmRunID)
	}
	if ev.Timestamp.IsZero() {
		t.Error("Timestamp must be decoded")
	}
}

func TestReader_Notification_WrongTypedRequiresResponse_NullAndObjectAreAbsent(t *testing.T) {
	for _, bad := range []string{`null`, `{"a":1}`, `1`} {
		t.Run(bad, func(t *testing.T) {
			ev, findings := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z",
				`"notification_type":"idle","requires_response":`+bad))

			if ev.Notification == nil || ev.Notification.RequiresResponse != domain.BoolAbsent {
				t.Errorf("Notification = %+v, want RequiresResponse BoolAbsent", ev.Notification)
			}
			if ev.Notification != nil && ev.Notification.NotificationType != "idle" {
				t.Errorf("NotificationType = %q, want idle", ev.Notification.NotificationType)
			}
			if len(findings) != 0 {
				t.Errorf("unexpected findings: %v", findings)
			}
		})
	}
}

func TestReader_Notification_UnknownExtraFieldsIgnored(t *testing.T) {
	ev, findings := readOne(t, rmLine("notification", "2026-10-01T10:00:01.000Z",
		`"notification_type":"idle","notification_id":"n-2","future_field":{"x":[1]},"cwd":"/secret/path"`))

	want := domain.NotificationFields{NotificationType: "idle", NotificationID: "n-2"}
	if ev.Notification == nil || *ev.Notification != want {
		t.Errorf("Notification = %+v, want %+v", ev.Notification, want)
	}
	assertNoMarkers(t, ev, "/secret/path")
	if len(findings) != 0 {
		t.Errorf("unknown fields must not raise findings, got %v", findings)
	}
}

func TestReader_UnknownExtraFieldOnAnyEvent_IsIgnored(t *testing.T) {
	events, findings := readTierLines(t,
		rmLine("turn", "2026-10-01T10:00:01.000Z", `"role":"assistant","brand_new":{"a":1}`),
		rmLine("invocation_start", "2026-10-01T10:00:02.000Z", `"agent_instance_id":"a#1","brand_new":1`),
		rmLine("tool_call_end", "2026-10-01T10:00:03.000Z", `"call_id":"c1","brand_new":"x"`))

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if len(findings) != 0 {
		t.Errorf("unknown fields must not raise findings, got %v", findings)
	}
}
