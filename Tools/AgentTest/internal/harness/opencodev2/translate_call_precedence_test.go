package opencodev2_test

// Tests for the order in which TranslateCall reports problems (the first
// matching rule wins) and for the post observation precedence rules.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func TestTranslateCall_FirstMatchingRuleWins(t *testing.T) {
	reCallBadPrompt := prePayload(map[string]any{"agent": "w", "prompt": 3, "sessionID": "ses_c"})
	wrongDiscriminatorOtherTool := prePayload(map[string]any{"agent": "w"})
	wrongDiscriminatorOtherTool["hook_event_name"] = "execute.after"
	wrongDiscriminatorOtherTool["tool"] = "bash"
	postNoTool := postPayload(map[string]any{"agent": "w"}, "completed", nil)
	delete(postNoTool, "tool")
	postNoID := postPayload(map[string]any{"agent": "w"}, "completed", nil)
	delete(postNoID, "id")
	completionWrongDiscriminatorBadAgent := completionPayload(map[string]any{"hook_event_name": "execute.after", "agent": 5})

	cases := []struct {
		name    string
		phase   domain.InterceptionPhase
		payload map[string]any
		wantMsg string
	}{
		{"malformed prompt beats the re-call", domain.PhasePre, reCallBadPrompt, "malformed"},
		{"wrong discriminator beats the tool name", domain.PhasePre, wrongDiscriminatorOtherTool, "unrecognised"},
		{"post without a tool", domain.PhasePost, postNoTool, "identity"},
		{"post without an id", domain.PhasePost, postNoID, "malformed"},
		{"post with an input that is not an object", domain.PhasePost, postPayload("text", "completed", nil), "malformed"},
		{"post with an agent that is not a string", domain.PhasePost, postPayload(map[string]any{"agent": 7}, "completed", nil), "malformed"},
		{"completion discriminator beats field types", domain.PhaseCompletion, completionWrongDiscriminatorBadAgent, "unrecognised"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call, err := translate(t, c.phase, c.payload)
			if err == nil || !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("TranslateCall(%s) = %+v, %v, want the %s error", c.phase, call, err, c.wantMsg)
			}
		})
	}
}

func TestTranslateCall_UnsupportedPhaseBeatsANonObjectPayload(t *testing.T) {
	a := opencodev2.New(opencodev2.Options{})

	_, err := a.TranslateCall(domain.InterceptionPhase("bogus"), []byte("not json"))

	if err == nil || !strings.Contains(err.Error(), "unrecognised") {
		t.Errorf("TranslateCall(unknown phase, not JSON) error = %v, want the unrecognised error", err)
	}
}

func TestTranslateCall_Pre_OnlyANonEmptyStringSessionIDMakesAReCall(t *testing.T) {
	for name, sessionID := range map[string]any{"number": 5, "null": nil, "boolean": true} {
		t.Run(name, func(t *testing.T) {
			call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": "p", "sessionID": sessionID}))

			if call.Identity.AgentIdentity != "worker" {
				t.Errorf("Identity = %+v, want a worker dispatch", call.Identity)
			}
		})
	}
}

func TestTranslateCall_Post_OutputStringWinsOverContentText(t *testing.T) {
	extra := map[string]any{"result": map[string]any{
		"output":  map[string]any{"output": "from output"},
		"content": []map[string]any{{"type": "text", "text": "from content"}},
	}}

	call := mustTranslate(t, domain.PhasePost, postPayload(map[string]any{"agent": "worker"}, "completed", extra))

	if call.ObservedResponse != "from output" {
		t.Errorf("ObservedResponse = %q, want result.output.output when it is a string", call.ObservedResponse)
	}
}

func TestTranslateCall_Post_NonStringOutputFallsBackToContentText(t *testing.T) {
	extra := map[string]any{"result": map[string]any{
		"output":  map[string]any{"output": 42},
		"content": []map[string]any{{"type": "text", "text": "from content"}},
	}}

	call := mustTranslate(t, domain.PhasePost, postPayload(map[string]any{"agent": "worker"}, "completed", extra))

	if call.ObservedResponse != "from content" {
		t.Errorf("ObservedResponse = %q, want the content text when output.output is not a string", call.ObservedResponse)
	}
}
