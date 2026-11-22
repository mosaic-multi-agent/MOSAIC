package opencodev2_test

// Tests for TranslateCall. A dispatch is one forwarded subagent call; its
// pre, post and completion payloads all carry the V2 tool call id as the
// correlation token. The post payload's result is only the launch
// acknowledgement; the real reply is the child's first completion wake-up.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

const taskMessageJSON = `{"agent_instance_id":"Worker#1","task_description":"do the work","input_artifacts":["a.md"],"output_artifacts":["b.md"]}`

func marshalPayload(t *testing.T, v map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func prePayload(input any) map[string]any {
	p := map[string]any{
		"hook_event_name": "execute.before", "tool": "subagent", "sessionID": "ses_p",
		"agent": "orchestrator", "messageID": "msg_1", "id": "call_1",
	}
	if input != nil {
		p["input"] = input
	}
	return p
}

func postPayload(input any, status string, extra map[string]any) map[string]any {
	p := prePayload(input)
	p["hook_event_name"] = "execute.after"
	p["status"] = status
	for k, v := range extra {
		p[k] = v
	}
	return p
}

func completionPayload(extra map[string]any) map[string]any {
	p := map[string]any{
		"hook_event_name": "subagent.completed", "sessionID": "ses_p", "id": "call_1",
		"agent": "worker", "childSessionID": "ses_c", "state": "completed",
		"text": "<subagent sessionID=\"ses_c\" state=\"completed\" description=\"do the work\">\nthe child's final text\n</subagent>",
	}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

func translate(t *testing.T, phase domain.InterceptionPhase, p map[string]any) (domain.InterceptedCall, error) {
	t.Helper()
	return opencodev2.New(opencodev2.Options{}).TranslateCall(phase, marshalPayload(t, p))
}

func mustTranslate(t *testing.T, phase domain.InterceptionPhase, p map[string]any) domain.InterceptedCall {
	t.Helper()
	call, err := translate(t, phase, p)
	if err != nil {
		t.Fatalf("TranslateCall(%s): %v", phase, err)
	}
	return call
}

// --- pre ---

func TestTranslateCall_Pre_IdentityComesFromInputAgentNotTheCallingAgent(t *testing.T) {
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "description": "d", "prompt": taskMessageJSON}))

	want := domain.CollaboratorIdentity{ToolName: domain.DispatchToolName, AgentIdentity: "worker"}
	if call.Identity != want {
		t.Errorf("Identity = %+v, want %+v (the top-level agent is the caller and must not be read)", call.Identity, want)
	}
	if call.Phase != domain.PhasePre {
		t.Errorf("Phase = %q, want %q", call.Phase, domain.PhasePre)
	}
}

func TestTranslateCall_Pre_CorrelationTokenIsTheToolCallID(t *testing.T) {
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": taskMessageJSON}))

	if call.CorrelationToken != "call_1" {
		t.Errorf("CorrelationToken = %q, want the tool call id %q", call.CorrelationToken, "call_1")
	}
	if call.AgentID != "" {
		t.Errorf("AgentID = %q, want empty in every phase", call.AgentID)
	}
}

func TestTranslateCall_Pre_WholeJSONPromptIsParsedAndKeptVerbatim(t *testing.T) {
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": taskMessageJSON}))

	if call.Message.Extraction != domain.ExtractionParsed {
		t.Errorf("Extraction = %q, want %q", call.Message.Extraction, domain.ExtractionParsed)
	}
	if call.Message.AgentInstanceID != "Worker#1" || call.Message.TaskDescription != "do the work" {
		t.Errorf("Message = %+v, want the parsed task message", call.Message)
	}
	if call.Message.Raw != taskMessageJSON {
		t.Errorf("Message.Raw = %q, want the prompt verbatim", call.Message.Raw)
	}
}

func TestTranslateCall_Pre_EmbeddedObjectIsRecovered(t *testing.T) {
	prompt := "Please do this: " + taskMessageJSON + " thanks"
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": prompt}))

	if call.Message.Extraction != domain.ExtractionRecovered {
		t.Errorf("Extraction = %q, want %q", call.Message.Extraction, domain.ExtractionRecovered)
	}
	if call.Message.AgentInstanceID != "Worker#1" {
		t.Errorf("AgentInstanceID = %q, want the embedded object's", call.Message.AgentInstanceID)
	}
	if call.Message.Raw != prompt {
		t.Errorf("Message.Raw = %q, want the prompt verbatim", call.Message.Raw)
	}
}

func TestTranslateCall_Pre_PlainPromptIsDegradedButKeptVerbatim(t *testing.T) {
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": "just do it"}))

	if call.Message.Extraction != domain.ExtractionDegraded {
		t.Errorf("Extraction = %q, want %q", call.Message.Extraction, domain.ExtractionDegraded)
	}
	if call.Message.Raw != "just do it" {
		t.Errorf("Message.Raw = %q, want the prompt verbatim", call.Message.Raw)
	}
}

func TestTranslateCall_Pre_KeepsRawPayloadAndCapabilities(t *testing.T) {
	a := opencodev2.New(opencodev2.Options{})
	native := marshalPayload(t, prePayload(map[string]any{"agent": "worker", "prompt": taskMessageJSON}))

	call, err := a.TranslateCall(domain.PhasePre, native)
	if err != nil {
		t.Fatalf("TranslateCall: %v", err)
	}

	if string(call.RawPayload) != string(native) {
		t.Errorf("RawPayload = %s, want the native payload", call.RawPayload)
	}
	if !reflect.DeepEqual(call.Capabilities, a.Capabilities()) {
		t.Errorf("Capabilities = %+v, want the adapter's %+v", call.Capabilities, a.Capabilities())
	}
}

func TestTranslateCall_Pre_AbsentValuesAreNotErrors(t *testing.T) {
	cases := map[string]map[string]any{
		"no input":            prePayload(nil),
		"input without agent": prePayload(map[string]any{"prompt": "p"}),
		"empty agent":         prePayload(map[string]any{"agent": "", "prompt": "p"}),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			call := mustTranslate(t, domain.PhasePre, p)
			want := domain.CollaboratorIdentity{ToolName: domain.DispatchToolName}
			if call.Identity != want {
				t.Errorf("Identity = %+v, want %+v", call.Identity, want)
			}
		})
	}
}

func TestTranslateCall_Pre_AbsentPromptIsDegradedWithEmptyRaw(t *testing.T) {
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker"}))

	if call.Message.Raw != "" || call.Message.Extraction != domain.ExtractionDegraded {
		t.Errorf("Message = %+v, want empty Raw and %q", call.Message, domain.ExtractionDegraded)
	}
}

func TestTranslateCall_Pre_ReCallWithSessionIDIsNotADispatch(t *testing.T) {
	_, err := translate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": "more", "sessionID": "ses_c"}))

	if err == nil {
		t.Fatalf("TranslateCall: want an error for a re-call carrying a sessionID")
	}
	if !strings.Contains(err.Error(), "continuation") {
		t.Errorf("error = %q, want it to be the continuation-call error", err.Error())
	}
}

func TestTranslateCall_Pre_EmptySessionIDIsStillADispatch(t *testing.T) {
	call := mustTranslate(t, domain.PhasePre, prePayload(map[string]any{"agent": "worker", "prompt": "p", "sessionID": ""}))

	if call.Identity.AgentIdentity != "worker" {
		t.Errorf("Identity = %+v, want the worker dispatch", call.Identity)
	}
}

func TestTranslateCall_Pre_RejectsPayloadsItCannotTrust(t *testing.T) {
	notSubagent := prePayload(map[string]any{"agent": "worker"})
	notSubagent["tool"] = "bash"
	noTool := prePayload(map[string]any{"agent": "worker"})
	delete(noTool, "tool")
	noID := prePayload(map[string]any{"agent": "worker"})
	delete(noID, "id")
	cases := []struct {
		name    string
		payload map[string]any
		wantMsg string
	}{
		{"other tool", notSubagent, "tool name"},
		{"missing tool", noTool, "identity"},
		{"missing id", noID, "malformed"},
		{"input not an object", prePayload("text"), "malformed"},
		{"agent not a string", prePayload(map[string]any{"agent": 7}), "malformed"},
		{"description not a string", prePayload(map[string]any{"agent": "w", "description": []string{"x"}}), "malformed"},
		{"prompt not a string", prePayload(map[string]any{"agent": "w", "prompt": 3}), "malformed"},
		{"wrong discriminator", func() map[string]any { p := prePayload(nil); p["hook_event_name"] = "execute.after"; return p }(), "unrecognised"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call, err := translate(t, domain.PhasePre, c.payload)
			if err == nil {
				t.Fatalf("TranslateCall: want an error, got call %+v", call)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error = %q, want it to say %q", err.Error(), c.wantMsg)
			}
		})
	}
}

// --- post ---

func TestTranslateCall_Post_ObservedResponseIsTheLaunchAcknowledgement(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{"output string", map[string]any{"result": map[string]any{"output": map[string]any{"output": "child started"}}}, "child started"},
		{"content text items", map[string]any{"result": map[string]any{"content": []map[string]any{
			{"type": "text", "text": "one "}, {"type": "image", "text": "skipped"}, {"type": "text", "text": "two"},
		}}}, "one two"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call := mustTranslate(t, domain.PhasePost, postPayload(map[string]any{"agent": "worker", "prompt": "p"}, "completed", c.extra))
			if call.ObservedResponse != c.want {
				t.Errorf("ObservedResponse = %q, want %q", call.ObservedResponse, c.want)
			}
		})
	}
}

func TestTranslateCall_Post_ErrorStatusObservesTheErrorMessage(t *testing.T) {
	extra := map[string]any{"error": map[string]any{"_tag": "Tool.Error", "message": "refused"}}
	call := mustTranslate(t, domain.PhasePost, postPayload(map[string]any{"agent": "worker"}, "error", extra))

	if call.ObservedResponse != "refused" {
		t.Errorf("ObservedResponse = %q, want the error message", call.ObservedResponse)
	}
}

func TestTranslateCall_Post_OtherStatusObservesNothing(t *testing.T) {
	call := mustTranslate(t, domain.PhasePost, postPayload(map[string]any{"agent": "worker"}, "pending", nil))

	if call.ObservedResponse != "" {
		t.Errorf("ObservedResponse = %q, want empty", call.ObservedResponse)
	}
}

func TestTranslateCall_Post_SharesIdentityMessageAndTokenWithPre(t *testing.T) {
	input := map[string]any{"agent": "worker", "description": "d", "prompt": taskMessageJSON}
	pre := mustTranslate(t, domain.PhasePre, prePayload(input))
	post := mustTranslate(t, domain.PhasePost, postPayload(input, "completed", nil))

	if pre.Identity != post.Identity || pre.CorrelationToken != post.CorrelationToken || pre.Message.AgentInstanceID != post.Message.AgentInstanceID {
		t.Errorf("pre %+v and post %+v do not describe the same dispatch", pre, post)
	}
	if post.Phase != domain.PhasePost {
		t.Errorf("Phase = %q, want %q", post.Phase, domain.PhasePost)
	}
}

func TestTranslateCall_Post_InputSessionIDDoesNotMakeItAReCallError(t *testing.T) {
	// The continuation check applies to the pre-invocation payload only.
	_, err := translate(t, domain.PhasePost, postPayload(map[string]any{"agent": "worker", "sessionID": "ses_c"}, "completed", nil))

	if err != nil {
		t.Errorf("TranslateCall(post): %v, want success", err)
	}
}

func TestTranslateCall_Post_RejectsWrongToolAndWrongDiscriminator(t *testing.T) {
	other := postPayload(map[string]any{"agent": "worker"}, "completed", nil)
	other["tool"] = "bash"
	if _, err := translate(t, domain.PhasePost, other); err == nil || !strings.Contains(err.Error(), "tool name") {
		t.Errorf("TranslateCall(post, tool bash) error = %v, want the tool-name error", err)
	}
	if _, err := translate(t, domain.PhasePost, prePayload(map[string]any{"agent": "worker"})); err == nil {
		t.Errorf("TranslateCall(post, execute.before payload): want an error")
	}
}

// --- completion ---

func TestTranslateCall_Completion_IdentityIsTheDispatchedAgentWithNoMessage(t *testing.T) {
	call := mustTranslate(t, domain.PhaseCompletion, completionPayload(nil))

	want := domain.CollaboratorIdentity{ToolName: domain.DispatchToolName, AgentIdentity: "worker"}
	if call.Identity != want {
		t.Errorf("Identity = %+v, want %+v", call.Identity, want)
	}
	if !reflect.DeepEqual(call.Message, domain.TaskMessage{}) {
		t.Errorf("Message = %+v, want the zero message", call.Message)
	}
	if call.Phase != domain.PhaseCompletion {
		t.Errorf("Phase = %q, want %q", call.Phase, domain.PhaseCompletion)
	}
}

func TestTranslateCall_Completion_CarriesTheDispatchCallIDAsTheToken(t *testing.T) {
	call := mustTranslate(t, domain.PhaseCompletion, completionPayload(nil))

	if call.CorrelationToken != "call_1" {
		t.Errorf("CorrelationToken = %q, want %q", call.CorrelationToken, "call_1")
	}
	if call.AgentID != "" {
		t.Errorf("AgentID = %q, want empty: no agent-start association is used", call.AgentID)
	}
}

func TestTranslateCall_Completion_ObservedResponseUnwrapsTheSubagentEnvelope(t *testing.T) {
	call := mustTranslate(t, domain.PhaseCompletion, completionPayload(nil))

	if call.ObservedResponse != "the child's final text" {
		t.Errorf("ObservedResponse = %q, want the child's text without the envelope", call.ObservedResponse)
	}
}

func TestTranslateCall_Completion_ObservedResponseVariants(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"multi-line body", "<subagent state=\"completed\">\nline one\nline two\n</subagent>", "line one\nline two"},
		{"surrounding whitespace", "  \n<subagent state=\"completed\">\n  padded  \n</subagent>\n ", "padded"},
		{"no envelope", "  plain reply  ", "plain reply"},
		{"envelope without newline", "<subagent>x</subagent>", "<subagent>x</subagent>"},
		{"not closed", "<subagent state=\"completed\">\nopen", "<subagent state=\"completed\">\nopen"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call := mustTranslate(t, domain.PhaseCompletion, completionPayload(map[string]any{"text": c.text}))
			if call.ObservedResponse != c.want {
				t.Errorf("ObservedResponse = %q, want %q", call.ObservedResponse, c.want)
			}
		})
	}
}

func TestTranslateCall_Completion_AbsentValuesAreNotErrors(t *testing.T) {
	p := completionPayload(nil)
	delete(p, "agent")
	delete(p, "text")
	delete(p, "id")

	call := mustTranslate(t, domain.PhaseCompletion, p)

	if call.Identity.AgentIdentity != "" || call.ObservedResponse != "" || call.CorrelationToken != "" {
		t.Errorf("call = %+v, want empty agent, response and token", call)
	}
}

func TestTranslateCall_Completion_RejectsWrongTypesAndDiscriminator(t *testing.T) {
	cases := []struct {
		name    string
		extra   map[string]any
		wantMsg string
	}{
		{"agent not a string", map[string]any{"agent": 5}, "malformed"},
		{"text not a string", map[string]any{"text": []string{"x"}}, "malformed"},
		{"wrong discriminator", map[string]any{"hook_event_name": "execute.after"}, "unrecognised"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := translate(t, domain.PhaseCompletion, completionPayload(c.extra))
			if err == nil || !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error = %v, want the %s error", err, c.wantMsg)
			}
		})
	}
}

// --- every phase ---

func TestTranslateCall_RejectsNonObjectsAndUnsupportedPhases(t *testing.T) {
	a := opencodev2.New(opencodev2.Options{})
	for _, phase := range []domain.InterceptionPhase{domain.PhasePre, domain.PhasePost, domain.PhaseCompletion} {
		for _, native := range []string{"", "not json {{{", "[1,2]", `"text"`, "null"} {
			if call, err := a.TranslateCall(phase, []byte(native)); err == nil || !strings.Contains(err.Error(), "malformed") {
				t.Errorf("TranslateCall(%s, %q) = %+v, %v, want the malformed-payload error", phase, native, call, err)
			}
		}
	}
	if _, err := a.TranslateCall(domain.PhaseAgentStart, marshalPayload(t, completionPayload(nil))); err == nil || !strings.Contains(err.Error(), "unrecognised") {
		t.Errorf("TranslateCall(agent_start): want an error, this harness has no agent-start phase")
	}
	if _, err := a.TranslateCall(domain.InterceptionPhase("bogus"), []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "unrecognised") {
		t.Errorf("TranslateCall(unknown phase): want an error")
	}
}

func TestTranslateCall_IgnoresUnknownPayloadFields(t *testing.T) {
	p := prePayload(map[string]any{"agent": "worker", "prompt": taskMessageJSON, "extra_input": 1})
	p["something_new"] = map[string]any{"x": 1}

	call := mustTranslate(t, domain.PhasePre, p)

	if call.Identity.AgentIdentity != "worker" {
		t.Errorf("Identity = %+v, want worker", call.Identity)
	}
}
