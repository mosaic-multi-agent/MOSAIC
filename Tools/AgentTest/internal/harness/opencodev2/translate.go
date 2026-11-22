package opencodev2

// Call and outcome translation between the interceptor wire payloads and the
// normalized interception model.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mosaic-agent-test/internal/domain"
)

var (
	errPayloadUnrecognised  = errors.New("opencode-v2: unrecognised interception payload")
	errPayloadMalformed     = errors.New("opencode-v2: malformed interception payload")
	errIdentityUndetermined = errors.New("opencode-v2: collaborator identity not determinable from payload")
	errUnrecognisedTool     = errors.New("opencode-v2: unrecognised native dispatch-tool name")
	errContinuationCall     = errors.New("opencode-v2: continuation call is not a dispatch")

	// errSubstitutionUnsupported: the interception point can rewrite a call's
	// input or reject the call, but nothing hands the parent a fabricated
	// child reply.
	errSubstitutionUnsupported = errors.New("opencode-v2: this harness cannot substitute a result directly")
)

// TranslateCall implements domain.HarnessAdapter. Never panics.
func (a *Adapter) TranslateCall(phase domain.InterceptionPhase, native []byte) (domain.InterceptedCall, error) {
	var wantEvent string
	switch phase {
	case domain.PhasePre:
		wantEvent = hookEventBefore
	case domain.PhasePost:
		wantEvent = hookEventAfter
	case domain.PhaseCompletion:
		wantEvent = hookEventCompleted
	default:
		return domain.InterceptedCall{}, fmt.Errorf("%w: interception phase %q", errPayloadUnrecognised, phase)
	}

	obj, ok := decodeWireObject(native)
	if !ok {
		return domain.InterceptedCall{}, fmt.Errorf("%w: not a JSON object", errPayloadMalformed)
	}
	if name, _, ok := obj.str("hook_event_name"); !ok || name != wantEvent {
		return domain.InterceptedCall{}, fmt.Errorf("%w: hook_event_name does not match phase %q", errPayloadUnrecognised, phase)
	}

	if phase == domain.PhaseCompletion {
		return a.translateCompletion(obj, native)
	}
	return a.translateDispatch(phase, obj, native)
}

// dispatchInput is the subagent call's input, with absent values empty.
type dispatchInput struct {
	agent       string
	description string
	prompt      string
	hasInput    bool
}

// readDispatchInput reads input.{agent,description,prompt}; present values of
// the wrong type are malformed, absent ones are empty.
func readDispatchInput(obj wireObject) (dispatchInput, wireObject, error) {
	in, present, ok := obj.object("input")
	if !ok {
		return dispatchInput{}, nil, fmt.Errorf("%w: input is not an object", errPayloadMalformed)
	}
	out := dispatchInput{hasInput: present}
	for _, f := range []struct {
		key string
		dst *string
	}{{"agent", &out.agent}, {"description", &out.description}, {"prompt", &out.prompt}} {
		v, _, ok := in.str(f.key)
		if !ok {
			return dispatchInput{}, nil, fmt.Errorf("%w: input.%s is not a string", errPayloadMalformed, f.key)
		}
		*f.dst = v
	}
	return out, in, nil
}

// translateDispatch handles the pre and post payloads, which describe one
// subagent call.
func (a *Adapter) translateDispatch(phase domain.InterceptionPhase, obj wireObject, native []byte) (domain.InterceptedCall, error) {
	tool, _, ok := obj.str("tool")
	if !ok {
		return domain.InterceptedCall{}, fmt.Errorf("%w: tool is not a string", errPayloadMalformed)
	}
	if tool == "" {
		return domain.InterceptedCall{}, fmt.Errorf("%w: tool is empty", errIdentityUndetermined)
	}
	if tool != nativeDispatchTool {
		return domain.InterceptedCall{}, fmt.Errorf("%w: %q", errUnrecognisedTool, tool)
	}
	id, _, ok := obj.str("id")
	if !ok || id == "" {
		return domain.InterceptedCall{}, fmt.Errorf("%w: id is empty or not a string", errPayloadMalformed)
	}
	in, rawInput, err := readDispatchInput(obj)
	if err != nil {
		return domain.InterceptedCall{}, err
	}
	if phase == domain.PhasePre {
		if sid, _, _ := rawInput.str("sessionID"); sid != "" {
			return domain.InterceptedCall{}, fmt.Errorf("%w: input.sessionID %q continues an existing child", errContinuationCall, sid)
		}
	}

	call := domain.InterceptedCall{
		Phase:            phase,
		Identity:         domain.CollaboratorIdentity{ToolName: domain.DispatchToolName, AgentIdentity: in.agent},
		Message:          parseTaskMessage(in.prompt),
		CorrelationToken: id,
		RawPayload:       json.RawMessage(native),
		Capabilities:     a.Capabilities(),
	}
	if phase == domain.PhasePost {
		call.ObservedResponse = launchAcknowledgement(obj)
	}
	return call, nil
}

// launchAcknowledgement is the text of an execute.after payload: what the
// subagent call returned at launch, or its error message.
func launchAcknowledgement(obj wireObject) string {
	status, _, _ := obj.str("status")
	switch status {
	case "completed":
		result, _, _ := obj.object("result")
		if output, _, _ := result.object("output"); output != nil {
			if s, present, ok := output.str("output"); present && ok {
				return s
			}
		}
		return contentText(result)
	case "error":
		errObj, _, _ := obj.object("error")
		msg, _, _ := errObj.str("message")
		return msg
	}
	return ""
}

// contentText concatenates the text of the result's text content items.
func contentText(result wireObject) string {
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if raw, ok := result["content"]; !ok || json.Unmarshal(raw, &items) != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		if it.Type == "text" {
			b.WriteString(it.Text)
		}
	}
	return b.String()
}

// translateCompletion handles the completion payload: the child's first
// finished wake-up, forwarded with its dispatch's call id and agent.
func (a *Adapter) translateCompletion(obj wireObject, native []byte) (domain.InterceptedCall, error) {
	agent, _, ok := obj.str("agent")
	if !ok {
		return domain.InterceptedCall{}, fmt.Errorf("%w: agent is not a string", errPayloadMalformed)
	}
	text, _, ok := obj.str("text")
	if !ok {
		return domain.InterceptedCall{}, fmt.Errorf("%w: text is not a string", errPayloadMalformed)
	}
	id, _, ok := obj.str("id")
	if !ok {
		return domain.InterceptedCall{}, fmt.Errorf("%w: id is not a string", errPayloadMalformed)
	}
	return domain.InterceptedCall{
		Phase:            domain.PhaseCompletion,
		Identity:         domain.CollaboratorIdentity{ToolName: domain.DispatchToolName, AgentIdentity: agent},
		CorrelationToken: id,
		RawPayload:       json.RawMessage(native),
		Capabilities:     a.Capabilities(),
		ObservedResponse: unwrapSubagentText(text),
	}, nil
}

// unwrapSubagentText returns the child's reply from a wake-up text of the form
// "<subagent ...>\n<reply>\n</subagent>", or the trimmed text itself when it
// has another form.
func unwrapSubagentText(text string) string {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "<subagent") && strings.HasSuffix(t, "</subagent>") && strings.Contains(t, "\n") {
		inner := t[strings.Index(t, "\n")+1 : strings.LastIndex(t, "</subagent>")]
		return strings.TrimSpace(inner)
	}
	return t
}

// TranslateOutcome implements domain.HarnessAdapter.
func (a *Adapter) TranslateOutcome(outcome domain.InterceptionOutcome, call domain.InterceptedCall) ([]byte, error) {
	if call.Phase == domain.PhasePost || call.Phase == domain.PhaseCompletion {
		return neutralReply(), nil
	}
	switch outcome.Kind {
	case domain.OutcomeRewritePrompt:
		return translateRewritePrompt(outcome, call)
	case domain.OutcomeHalt:
		reason := outcome.Message
		if reason == "" {
			reason = defaultDenyReason
		}
		return marshalReply(hookReply{Decision: "deny", Reason: reason})
	case domain.OutcomePassthrough:
		return neutralReply(), nil
	case domain.OutcomeSubstitute:
		return nil, errSubstitutionUnsupported
	default:
		return nil, fmt.Errorf("%w: outcome kind %q", errPayloadUnrecognised, outcome.Kind)
	}
}

// translateRewritePrompt builds the allow-with-updated-input reply. The
// original agent and description come from the call's own raw payload, so the
// call keeps its identity across the rewrite.
func translateRewritePrompt(outcome domain.InterceptionOutcome, call domain.InterceptedCall) ([]byte, error) {
	obj, ok := decodeWireObject(call.RawPayload)
	if !ok {
		return nil, fmt.Errorf("%w: recovering the original input", errPayloadMalformed)
	}
	in, _, ok := obj.object("input")
	if !ok {
		return nil, fmt.Errorf("%w: recovering the original input", errPayloadMalformed)
	}
	updated := map[string]any{}
	for _, key := range []string{"agent", "description"} {
		if v, present, ok := in.str(key); ok && present {
			updated[key] = v
		}
	}
	updated["prompt"] = outcome.RewrittenPrompt
	return marshalReply(hookReply{Decision: "allow", UpdatedInput: updated})
}
