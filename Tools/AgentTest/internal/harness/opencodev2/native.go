package opencodev2

// Wire types between the generated interceptor plugin and the translation:
// the interceptor payloads the plugin writes to the interceptor's stdin and
// the reply it reads from stdout. The shape is ours, so hook fields keep V2's
// names. These are the only types in the package that speak this wire; no
// package outside internal/harness/ may name them.

import (
	"encoding/json"
	"fmt"
)

// Hook event names, the phase discriminators of the payloads.
const (
	hookEventBefore    = "execute.before"
	hookEventAfter     = "execute.after"
	hookEventCompleted = "subagent.completed"
)

// nativeDispatchTool is V2's dispatch tool name.
const nativeDispatchTool = "subagent"

// defaultDenyReason is the rejection message when none is given.
const defaultDenyReason = "This operation cannot proceed."

// wireObject is a decoded payload object whose fields are read one at a time,
// so that the order problems are reported in is under the translation's
// control rather than the decoder's.
type wireObject map[string]json.RawMessage

func decodeWireObject(raw []byte) (wireObject, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, false
	}
	return wireObject(obj), true
}

// str reads a string field. present is false for an absent or null field;
// ok is false when the field is present but not a string.
func (o wireObject) str(key string) (value string, present, ok bool) {
	raw, found := o[key]
	if !found || string(raw) == "null" {
		return "", false, true
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", true, false
	}
	return value, true, true
}

// object reads an object field. present is false for an absent or null field;
// ok is false when the field is present but not an object.
func (o wireObject) object(key string) (value wireObject, present, ok bool) {
	raw, found := o[key]
	if !found || string(raw) == "null" {
		return nil, false, true
	}
	value, ok = decodeWireObject(raw)
	return value, true, ok
}

// hookReply is the reply the interceptor writes to stdout and the plugin
// reads. An empty Decision means no decision: the call proceeds unchanged.
type hookReply struct {
	Decision     string         `json:"decision,omitempty"`
	UpdatedInput map[string]any `json:"updatedInput,omitempty"`
	Reason       string         `json:"reason,omitempty"`
}

func marshalReply(r hookReply) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("opencode-v2: marshaling reply: %w", err)
	}
	return b, nil
}

// neutralReply is the reply that changes nothing.
func neutralReply() []byte { return []byte(`{}`) }
