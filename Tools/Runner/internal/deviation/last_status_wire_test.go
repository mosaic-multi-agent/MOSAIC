package deviation_test

// Tests for the last_status member of the consultation wire request: always
// present, JSON null when the request carries none, the status string when it
// does, and null for a pre-consultation. The existing members keep their names
// and null/presence rules.

import (
	"context"
	"encoding/json"
	"testing"

	"mosaic-run/internal/domain"
)

func statusptr(s domain.StatusCode) *domain.StatusCode { return &s }

func TestOrchestratorConsultant_RoutingRequestAfterHarnessError_CarriesStatusAndDescription(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))
	description := "unparseable-response: response received but not valid protocol JSON"

	c.ConsultRouting(context.Background(), domain.ConsultationRequest{ //nolint:errcheck
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextRouting,
		LastStatus:            statusptr(domain.StatusHARNESS_ERROR),
		LastStatusMessage:     &description,
		LastErrorReason:       &description,
	})

	raw := wireKeys(t, fake.sent)
	want := map[string]string{
		"last_status":         "HARNESS_ERROR",
		"last_status_message": description,
		"last_error_reason":   description,
	}
	for key, value := range want {
		var got *string
		if err := json.Unmarshal(raw[key], &got); err != nil || got == nil {
			t.Errorf("want %s to be a string, got %s (%v)", key, raw[key], err)
			continue
		}
		if *got != value {
			t.Errorf("want %s %q, got %q", key, value, *got)
		}
	}
}

func TestOrchestratorConsultant_RoutingRequest_LastStatusIsNullWhenNil(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.ConsultRouting(context.Background(), domain.ConsultationRequest{ //nolint:errcheck
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextRouting,
	})

	raw := wireKeys(t, fake.sent)
	got, present := raw["last_status"]
	if !present {
		t.Fatal("want last_status present (as null), got absent")
	}
	if string(got) != "null" {
		t.Errorf("want last_status=null, got %s", got)
	}
}

func TestOrchestratorConsultant_RoutingRequestAfterBlocked_KeepsExistingMembersAndAddsStatus(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))
	reason := "cannot read input"

	c.ConsultRouting(context.Background(), domain.ConsultationRequest{ //nolint:errcheck
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextRouting,
		LastStatus:            statusptr(domain.StatusBLOCKED),
		LastStatusMessage:     strptr("input missing"),
		LastErrorReason:       &reason,
	})

	raw := wireKeys(t, fake.sent)
	for _, key := range []string{"orchestration_artifact", "context", "last_status", "last_status_message", "last_error_reason"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("want key %q present, got keys %v", key, raw)
		}
	}
	if string(raw["last_status"]) != `"BLOCKED"` {
		t.Errorf("want last_status=\"BLOCKED\", got %s", raw["last_status"])
	}
	if string(raw["last_status_message"]) != `"input missing"` {
		t.Errorf("want last_status_message unchanged, got %s", raw["last_status_message"])
	}
	if string(raw["last_error_reason"]) != `"cannot read input"` {
		t.Errorf("want last_error_reason unchanged, got %s", raw["last_error_reason"])
	}
}

func TestOrchestratorConsultant_PreConsultSendsNullLastStatusEvenWhenRequestCarriesOne(t *testing.T) {
	fake := &fakeRawInvoker{reply: []byte(`{}`)}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.PreConsult(context.Background(), domain.ConsultationRequest{ //nolint:errcheck
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextPreConsultation,
		LastStatus:            statusptr(domain.StatusHARNESS_ERROR),
	})

	raw := wireKeys(t, fake.sent)
	got, present := raw["last_status"]
	if !present {
		t.Fatal("want last_status present (as null) in pre-consultation request, got absent")
	}
	if string(got) != "null" {
		t.Errorf("want last_status=null for pre-consultation, got %s", got)
	}
}
