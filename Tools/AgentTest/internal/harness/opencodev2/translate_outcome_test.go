package opencodev2_test

// Tests for TranslateOutcome: the reply the interceptor writes back for the
// plugin to apply. Post and completion phases always answer {}; a pre-phase
// rewrite replaces only the prompt and keeps the call's agent and description;
// a halt becomes a deny; a substitution is refused because V2 has no way to
// hand the parent a fabricated child reply.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func preCall(t *testing.T, input map[string]any) domain.InterceptedCall {
	t.Helper()
	native := marshalPayload(t, prePayload(input))
	return domain.InterceptedCall{
		Phase:            domain.PhasePre,
		Identity:         domain.CollaboratorIdentity{ToolName: domain.DispatchToolName, AgentIdentity: "worker"},
		CorrelationToken: "call_1",
		RawPayload:       native,
	}
}

func decodeReply(t *testing.T, native []byte) map[string]any {
	t.Helper()
	var reply map[string]any
	if err := json.Unmarshal(native, &reply); err != nil {
		t.Fatalf("reply %q is not a JSON object: %v", native, err)
	}
	return reply
}

func outcomeReply(t *testing.T, outcome domain.InterceptionOutcome, call domain.InterceptedCall) map[string]any {
	t.Helper()
	native, err := opencodev2.New(opencodev2.Options{}).TranslateOutcome(outcome, call)
	if err != nil {
		t.Fatalf("TranslateOutcome(%s): %v", outcome.Kind, err)
	}
	return decodeReply(t, native)
}

func TestTranslateOutcome_RewritePromptKeepsAgentAndDescriptionAndReplacesPrompt(t *testing.T) {
	call := preCall(t, map[string]any{"agent": "worker", "description": "do the work", "prompt": "original"})

	reply := outcomeReply(t, domain.InterceptionOutcome{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: "rewritten", CorrelationToken: "call_1"}, call)

	want := map[string]any{
		"decision":     "allow",
		"updatedInput": map[string]any{"agent": "worker", "description": "do the work", "prompt": "rewritten"},
	}
	if !reflect.DeepEqual(reply, want) {
		t.Errorf("reply = %v, want %v", reply, want)
	}
}

func TestTranslateOutcome_RewritePromptOmitsAbsentAgentAndDescription(t *testing.T) {
	call := preCall(t, map[string]any{"prompt": "original"})

	reply := outcomeReply(t, domain.InterceptionOutcome{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: "rewritten"}, call)

	updated, _ := reply["updatedInput"].(map[string]any)
	if !reflect.DeepEqual(updated, map[string]any{"prompt": "rewritten"}) {
		t.Errorf("updatedInput = %v, want only the rewritten prompt", updated)
	}
}

func TestTranslateOutcome_RewritePromptKeepsOnlyTheOriginalFieldsThatArePresent(t *testing.T) {
	cases := map[string]struct {
		input map[string]any
		want  map[string]any
	}{
		"agent only":       {map[string]any{"agent": "worker", "prompt": "o"}, map[string]any{"agent": "worker", "prompt": "new"}},
		"description only": {map[string]any{"description": "d", "prompt": "o"}, map[string]any{"description": "d", "prompt": "new"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			reply := outcomeReply(t, domain.InterceptionOutcome{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: "new"}, preCall(t, c.input))

			if updated, _ := reply["updatedInput"].(map[string]any); !reflect.DeepEqual(updated, c.want) {
				t.Errorf("updatedInput = %v, want %v", updated, c.want)
			}
		})
	}
}

func TestTranslateOutcome_RewritePromptWritesNoCorrelationToken(t *testing.T) {
	call := preCall(t, map[string]any{"agent": "worker", "prompt": "original"})

	native, err := opencodev2.New(opencodev2.Options{}).TranslateOutcome(
		domain.InterceptionOutcome{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: "rewritten", CorrelationToken: "tok-123"}, call)
	if err != nil {
		t.Fatalf("TranslateOutcome: %v", err)
	}

	if strings.Contains(string(native), "tok-123") {
		t.Errorf("reply %s carries the correlation token; the prompt must not change to carry one", native)
	}
}

func TestTranslateOutcome_RewritePromptWithUnreadableRawPayloadIsAnError(t *testing.T) {
	call := preCall(t, map[string]any{"agent": "worker"})
	call.RawPayload = []byte("not json {{{")

	_, err := opencodev2.New(opencodev2.Options{}).TranslateOutcome(
		domain.InterceptionOutcome{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: "x"}, call)

	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Errorf("TranslateOutcome error = %v, want the malformed-payload error", err)
	}
}

func TestTranslateOutcome_HaltDeniesWithTheMessageAsReason(t *testing.T) {
	reply := outcomeReply(t, domain.InterceptionOutcome{Kind: domain.OutcomeHalt, Message: "stop here"}, preCall(t, map[string]any{"agent": "worker"}))

	want := map[string]any{"decision": "deny", "reason": "stop here"}
	if !reflect.DeepEqual(reply, want) {
		t.Errorf("reply = %v, want %v", reply, want)
	}
}

func TestTranslateOutcome_HaltWithoutMessageUsesTheDefaultReason(t *testing.T) {
	reply := outcomeReply(t, domain.InterceptionOutcome{Kind: domain.OutcomeHalt}, preCall(t, map[string]any{"agent": "worker"}))

	if reply["decision"] != "deny" || reply["reason"] != "This operation cannot proceed." {
		t.Errorf("reply = %v, want a deny with the default reason", reply)
	}
}

func TestTranslateOutcome_PassthroughIsAnEmptyObject(t *testing.T) {
	reply := outcomeReply(t, domain.InterceptionOutcome{Kind: domain.OutcomePassthrough}, preCall(t, map[string]any{"agent": "worker"}))

	if len(reply) != 0 {
		t.Errorf("reply = %v, want {}", reply)
	}
}

func TestTranslateOutcome_SubstitutionIsRefused(t *testing.T) {
	native, err := opencodev2.New(opencodev2.Options{}).TranslateOutcome(
		domain.InterceptionOutcome{Kind: domain.OutcomeSubstitute, StubResponse: json.RawMessage(`{"a":1}`)}, preCall(t, map[string]any{"agent": "worker"}))

	if err == nil || !strings.Contains(err.Error(), "substitute") {
		t.Fatalf("TranslateOutcome(substitute) error = %v (reply %s), want the cannot-substitute error", err, native)
	}
	if native != nil {
		t.Errorf("reply = %s, want none alongside the error", native)
	}
}

func TestTranslateOutcome_UnknownKindIsAnError(t *testing.T) {
	_, err := opencodev2.New(opencodev2.Options{}).TranslateOutcome(
		domain.InterceptionOutcome{Kind: domain.OutcomeKind("bogus")}, preCall(t, map[string]any{"agent": "worker"}))

	if err == nil || !strings.Contains(err.Error(), "unrecognised") {
		t.Errorf("TranslateOutcome(unknown kind) error = %v, want the unrecognised error", err)
	}
}

func TestTranslateOutcome_PostAndCompletionAlwaysAnswerEmptyObjectForEveryOutcome(t *testing.T) {
	outcomes := []domain.InterceptionOutcome{
		{Kind: domain.OutcomePassthrough},
		{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: "x"},
		{Kind: domain.OutcomeHalt, Message: "m"},
		{Kind: domain.OutcomeSubstitute, StubResponse: json.RawMessage(`{}`)},
	}
	for _, phase := range []domain.InterceptionPhase{domain.PhasePost, domain.PhaseCompletion} {
		call := domain.InterceptedCall{Phase: phase, RawPayload: []byte("not even json")}
		for _, o := range outcomes {
			native, err := opencodev2.New(opencodev2.Options{}).TranslateOutcome(o, call)
			if err != nil {
				t.Errorf("TranslateOutcome(%s, %s): %v, want no error", phase, o.Kind, err)
				continue
			}
			if reply := decodeReply(t, native); len(reply) != 0 {
				t.Errorf("TranslateOutcome(%s, %s) = %s, want {}", phase, o.Kind, native)
			}
		}
	}
}
