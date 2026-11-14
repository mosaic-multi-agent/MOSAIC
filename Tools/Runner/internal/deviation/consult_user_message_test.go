package deviation_test

// Tests for the user_message member of the consultation wire request: always
// present, the user's text verbatim for a routing consultation, JSON null
// otherwise (no message, pre-consultation), identical across the retries of
// one ConsultRouting call.

import (
	"context"
	"encoding/json"
	"testing"

	"mosaic-run/internal/domain"
)

func routingRequestWithMessage(msg *string) domain.ConsultationRequest {
	return domain.ConsultationRequest{
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextRouting,
		UserMessage:           msg,
	}
}

func TestOrchestratorConsultant_RoutingRequest_CarriesUserMessageVerbatim(t *testing.T) {
	messages := map[string]string{
		"single line": "Go to implementation.",
		"multi-line":  "Stop routing back to planning.\nGo to implementation.\n\n  \"quoted\" \\ and ä ",
	}
	for name, text := range messages {
		t.Run(name, func(t *testing.T) {
			fake := &fakeRawInvoker{reply: routingStopReply("done")}
			c := newTestOrchestratorConsultant(fake, mustParseTable(t))

			c.ConsultRouting(context.Background(), routingRequestWithMessage(&text)) //nolint:errcheck

			var w struct {
				UserMessage *string `json:"user_message"`
			}
			mustUnmarshal(t, fake.sent, &w)
			if w.UserMessage == nil || *w.UserMessage != text {
				t.Errorf("want user_message %q verbatim, got %v", text, w.UserMessage)
			}
		})
	}
}

func TestOrchestratorConsultant_RoutingRequest_SendsNullUserMessageWhenNil(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.ConsultRouting(context.Background(), routingRequestWithMessage(nil)) //nolint:errcheck

	raw := wireKeys(t, fake.sent)
	got, present := raw["user_message"]
	if !present {
		t.Fatal("want user_message present (as null), got absent")
	}
	if string(got) != "null" {
		t.Errorf("want user_message=null, got %s", got)
	}
}

func TestOrchestratorConsultant_RoutingRequest_AllSevenMembersPresentWithAMessage(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))
	msg := "hello"

	c.ConsultRouting(context.Background(), routingRequestWithMessage(&msg)) //nolint:errcheck

	raw := wireKeys(t, fake.sent)
	assertMemberSet(t, raw,
		"orchestration_artifact", "context", "last_status", "last_status_message",
		"last_error_reason", "user_message", "artifact_damage")
	if string(raw["user_message"]) != `"hello"` {
		t.Errorf("want user_message=\"hello\", got %s", raw["user_message"])
	}
}

func TestOrchestratorConsultant_PreConsultSendsNullUserMessageEvenWhenRequestCarriesOne(t *testing.T) {
	fake := &fakeRawInvoker{reply: []byte(`{}`)}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))
	msg := "must not travel"

	c.PreConsult(context.Background(), domain.ConsultationRequest{ //nolint:errcheck
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextPreConsultation,
		UserMessage:           &msg,
	})

	raw := wireKeys(t, fake.sent)
	got, present := raw["user_message"]
	if !present {
		t.Fatal("want user_message present (as null) in pre-consultation request, got absent")
	}
	if string(got) != "null" {
		t.Errorf("want user_message=null for pre-consultation, got %s", got)
	}
}

func TestOrchestratorConsultant_RetriesResendTheSameUserMessage(t *testing.T) {
	invoker := &sequencedRawInvoker{replies: []sequencedReply{
		malformedSchemaReply(),
		malformedSchemaReply(),
		{reply: routingStopReply("done")},
	}}
	c := newTestOrchestratorConsultant(invoker, mustParseTable(t))
	msg := "line one\nline two"

	c.ConsultRouting(context.Background(), routingRequestWithMessage(&msg)) //nolint:errcheck

	if len(invoker.sent) < 2 {
		t.Fatalf("want the invalid reply retried, got %d call(s)", len(invoker.sent))
	}
	for i, payload := range invoker.sent {
		var w struct {
			UserMessage *string `json:"user_message"`
		}
		mustUnmarshal(t, payload, &w)
		if w.UserMessage == nil || *w.UserMessage != msg {
			t.Errorf("attempt %d: want user_message %q, got %v", i+1, msg, w.UserMessage)
		}
	}
}

func mustUnmarshal(t *testing.T, payload []byte, into any) {
	t.Helper()
	if err := json.Unmarshal(payload, into); err != nil {
		t.Fatalf("want valid JSON wire payload, got %v", err)
	}
}

func TestOrchestratorConsultant_DispatchLogRecordsTheRequestAsSentWithTheUserMessage(t *testing.T) {
	invoker, logger := newOrderingPair(routingStopReply("done"), nil)
	c := newTestConsultantWithLogger(invoker, mustParseTable(t), logger)
	msg := "first line\nsecond line"

	c.ConsultRouting(context.Background(), routingRequestWithMessage(&msg)) //nolint:errcheck

	if len(logger.requests) != 1 {
		t.Fatalf("want one logged request, got %d", len(logger.requests))
	}
	var w struct {
		UserMessage *string `json:"user_message"`
	}
	mustUnmarshal(t, []byte(logger.requests[0].TaskDescription), &w)
	if w.UserMessage == nil || *w.UserMessage != msg {
		t.Errorf("want the logged request to carry user_message %q, got %v", msg, w.UserMessage)
	}
}
