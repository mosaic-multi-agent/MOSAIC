package deviation_test

// Tests for routing replies whose "action" member is absent: reported as a
// missing required field, without retry. A present but empty or unknown action
// keeps its unknown-action classification.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// consultRoutingSequence runs ConsultRouting against an invoker that answers
// with replies in order, and returns the invocation count and the error.
func consultRoutingSequence(t *testing.T, replies ...string) (calls int, err error) {
	t.Helper()
	seq := make([]sequencedReply, len(replies))
	for i, r := range replies {
		seq[i] = sequencedReply{reply: []byte(r)}
	}
	invoker := &sequencedRawInvoker{replies: seq}
	c := newTestOrchestratorConsultant(invoker, mustParseTable(t))
	_, err = c.ConsultRouting(context.Background(), validRoutingRequest("Orchestration-abc/Orchestration.md"))
	return invoker.pos, err
}

// TestConsultRouting_AbsentAction_IsMissingFieldWithoutRetry verifies that a
// routing object with no action member fails as a missing field naming
// "action", after exactly one invocation.
func TestConsultRouting_AbsentAction_IsMissingFieldWithoutRetry(t *testing.T) {
	cases := map[string]string{
		"agent only":       `{"agent":"agent-a"}`,
		"dispatch fields":  `{"agent":"agent-a","task_description":"do the thing"}`,
		"reason only":      `{"reason":"done"}`,
		"empty object":     `{}`,
		"unrelated member": `{"note":"hello"}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			calls, err := consultRoutingSequence(t, reply, reply, reply)

			ce := assertConsultationError(t, err, domain.ConsultFailMissingField)
			if ce != nil && !strings.Contains(err.Error(), "action") {
				t.Errorf("want error text to name the missing field \"action\", got %q", err.Error())
			}
			if calls != 1 {
				t.Errorf("want exactly 1 invocation (a missing field is not retried), got %d", calls)
			}
		})
	}
}

// TestConsultRouting_PresentButInvalidAction_StaysUnknownAction is regression
// coverage: only an absent member is reclassified. Empty, null and unknown
// values keep failing as unknown-action, without retry.
func TestConsultRouting_PresentButInvalidAction_StaysUnknownAction(t *testing.T) {
	cases := map[string]string{
		"empty string": `{"action":""}`,
		"unknown":      `{"action":"proceed"}`,
		"null":         `{"action":null}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			calls, err := consultRoutingSequence(t, reply, reply, reply)

			assertConsultationError(t, err, domain.ConsultFailUnknownAction)
			if calls != 1 {
				t.Errorf("want exactly 1 invocation, got %d", calls)
			}
		})
	}
}

// TestConsultRouting_AbsentActionWithUndecodableField_StaysMalformedAndRetried
// is regression coverage for precedence: a reply that fails decoding is
// malformed-json and retried, whether or not action is also absent.
func TestConsultRouting_AbsentActionWithUndecodableField_StaysMalformedAndRetried(t *testing.T) {
	reply := `{"agent":"agent-a","hitl_override":5}`

	calls, err := consultRoutingSequence(t, reply, reply, reply)

	assertConsultationError(t, err, domain.ConsultFailMalformedJSON)
	if calls != 3 {
		t.Errorf("want 3 invocations for a retried malformed reply, got %d", calls)
	}
}
