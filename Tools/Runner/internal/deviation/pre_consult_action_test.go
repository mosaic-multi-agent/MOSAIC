package deviation_test

// Tests for the pre-consultation reply's optional "action" member: a stop with
// a reason ends the run, every other action value is malformed, and a reply
// without "action" stays plain advice.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// preConsultReply sends reply through PreConsult and returns the result.
func preConsultReply(t *testing.T, reply string) (domain.PreConsultationAdvice, error) {
	t.Helper()
	fake := &fakeRawInvoker{reply: []byte(reply)}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))
	req := domain.ConsultationRequest{
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextPreConsultation,
	}
	return c.PreConsult(context.Background(), req)
}

// requirePreConsultFailure asserts that reply fails with the given class and
// that the error text contains every fragment.
func requirePreConsultFailure(t *testing.T, reply string, want domain.ConsultationFailure, fragments ...string) {
	t.Helper()
	advice, err := preConsultReply(t, reply)
	ce := assertConsultationError(t, err, want)
	if ce == nil {
		return
	}
	if advice.Stop != nil {
		t.Errorf("want no Stop on a malformed reply, got %+v", advice.Stop)
	}
	for _, frag := range fragments {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("want error text to name %q, got %q", frag, err.Error())
		}
	}
}

// TestPreConsult_StopWithReason_ReturnsStop verifies that a stop action with a
// non-empty reason is a decision, not an error: the reason reaches the caller
// and no advice is carried.
func TestPreConsult_StopWithReason_ReturnsStop(t *testing.T) {
	advice, err := preConsultReply(t, `{"action":"stop","reason":"Branch mosaic/main has uncommitted work"}`)

	if err != nil {
		t.Fatalf("want no error for a pre-consultation stop, got %v", err)
	}
	if advice.Stop == nil {
		t.Fatal("want advice.Stop set for a stop reply, got nil")
	}
	if advice.Stop.Reason != "Branch mosaic/main has uncommitted work" {
		t.Errorf("want Stop.Reason preserved, got %q", advice.Stop.Reason)
	}
	if advice.TaskDescription != "" || advice.Constraints != "" {
		t.Errorf("want no advice alongside a stop, got TaskDescription=%q Constraints=%q",
			advice.TaskDescription, advice.Constraints)
	}
}

// TestPreConsult_StopReason_PreservedVerbatim verifies that the reason is not
// trimmed: it is what the orchestrator wrote.
func TestPreConsult_StopReason_PreservedVerbatim(t *testing.T) {
	advice, err := preConsultReply(t, `{"action":"stop","reason":"  needs review\n"}`)

	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if advice.Stop == nil || advice.Stop.Reason != "  needs review\n" {
		t.Errorf("want reason kept verbatim %q, got %+v", "  needs review\n", advice.Stop)
	}
}

// TestPreConsult_StopWithAdviceFields_StopWinsAndAdviceIgnored pins the chosen
// behaviour: advice members next to a stop are ignored, even when wrongly typed.
func TestPreConsult_StopWithAdviceFields_StopWinsAndAdviceIgnored(t *testing.T) {
	cases := map[string]string{
		"string advice": `{"action":"stop","reason":"X","task_description":"do more","constraints":"c"}`,
		"mistyped":      `{"action":"stop","reason":"X","task_description":5,"constraints":["c"]}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			advice, err := preConsultReply(t, reply)

			if err != nil {
				t.Fatalf("want no error (advice ignored next to a stop), got %v", err)
			}
			if advice.Stop == nil || advice.Stop.Reason != "X" {
				t.Fatalf("want Stop with reason X, got %+v", advice.Stop)
			}
			if advice.TaskDescription != "" || advice.Constraints != "" {
				t.Errorf("want advice fields empty next to a stop, got %q / %q",
					advice.TaskDescription, advice.Constraints)
			}
		})
	}
}

// TestPreConsult_ActionOtherString_IsUnknownActionNamingValue verifies that any
// string action other than "stop" is rejected and the value is named.
func TestPreConsult_ActionOtherString_IsUnknownActionNamingValue(t *testing.T) {
	cases := []struct{ name, action string }{
		{"dispatch", "dispatch"},
		{"unknown", "proceed"},
		{"capitalised stop", "STOP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reply := `{"action":"` + tc.action + `","reason":"X","agent":"agent-a"}`
			requirePreConsultFailure(t, reply, domain.ConsultFailUnknownAction, tc.action)
		})
	}
}

// TestPreConsult_ActionEmptyString_IsUnknownAction verifies that an empty
// action is present and is not "stop".
func TestPreConsult_ActionEmptyString_IsUnknownAction(t *testing.T) {
	requirePreConsultFailure(t, `{"action":"","reason":"X"}`, domain.ConsultFailUnknownAction, "action")
}

// TestPreConsult_ActionNotAString_IsMalformedNamingAction verifies that a
// present action of any non-string JSON type (null included) is malformed.
func TestPreConsult_ActionNotAString_IsMalformedNamingAction(t *testing.T) {
	cases := map[string]string{
		"null":    `{"action":null,"reason":"X"}`,
		"number":  `{"action":5,"reason":"X"}`,
		"boolean": `{"action":true,"reason":"X"}`,
		"array":   `{"action":["stop"],"reason":"X"}`,
		"object":  `{"action":{"v":"stop"},"reason":"X"}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			requirePreConsultFailure(t, reply, domain.ConsultFailMalformedJSON, "action")
		})
	}
}

// TestPreConsult_StopWithoutUsableReason_IsMissingReason verifies that a stop
// whose reason is absent, null, empty or whitespace-only fails naming "reason".
func TestPreConsult_StopWithoutUsableReason_IsMissingReason(t *testing.T) {
	cases := map[string]string{
		"absent":     `{"action":"stop"}`,
		"null":       `{"action":"stop","reason":null}`,
		"empty":      `{"action":"stop","reason":""}`,
		"whitespace": `{"action":"stop","reason":"  \n\t "}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			requirePreConsultFailure(t, reply, domain.ConsultFailMissingField, "reason")
		})
	}
}

// TestPreConsult_StopWithNonStringReason_IsMalformedNamingReason verifies that
// a reason of a JSON type other than string fails naming "reason".
func TestPreConsult_StopWithNonStringReason_IsMalformedNamingReason(t *testing.T) {
	cases := map[string]string{
		"number":  `{"action":"stop","reason":5}`,
		"boolean": `{"action":"stop","reason":true}`,
		"object":  `{"action":"stop","reason":{"text":"X"}}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			requirePreConsultFailure(t, reply, domain.ConsultFailMalformedJSON, "reason")
		})
	}
}

// TestPreConsult_ReplyWithoutAction_StaysAdvice is regression coverage: a reply
// with no action member is advice as before, and "reason" next to it is ignored.
func TestPreConsult_ReplyWithoutAction_StaysAdvice(t *testing.T) {
	cases := []struct {
		name, reply, wantTask, wantConstraints string
	}{
		{"advice with reason member", `{"reason":"just a note","task_description":"t","constraints":"c"}`, "t", "c"},
		{"reason only", `{"reason":"just a note"}`, "", ""},
		{"mistyped reason ignored", `{"reason":5,"task_description":"t"}`, "t", ""},
		{"unknown fields ignored", `{"extra":1,"constraints":"c"}`, "", "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			advice, err := preConsultReply(t, tc.reply)

			if err != nil {
				t.Fatalf("want no error for a reply without action, got %v", err)
			}
			if advice.Stop != nil {
				t.Errorf("want no Stop without an action member, got %+v", advice.Stop)
			}
			if advice.TaskDescription != tc.wantTask || advice.Constraints != tc.wantConstraints {
				t.Errorf("want advice %q / %q, got %q / %q",
					tc.wantTask, tc.wantConstraints, advice.TaskDescription, advice.Constraints)
			}
		})
	}
}

// TestPreConsult_StopWordedAsProse_StaysNoInstruction is regression coverage: a
// stop that is not a JSON object is not understood as a stop.
func TestPreConsult_StopWordedAsProse_StaysNoInstruction(t *testing.T) {
	_, err := preConsultReply(t, "Please stop the run: the branch is dirty.")

	assertConsultationError(t, err, domain.ConsultFailNoInstruction)
}
