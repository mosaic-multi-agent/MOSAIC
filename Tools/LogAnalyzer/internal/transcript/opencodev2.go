package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// subagentPromptPrefix is what OpenCode V2 puts before the dispatched prompt in
// a child session's first user message.
const subagentPromptPrefix = "You are a subagent spawned by another session.\n"

// readOpenCodeV2FirstPrompt walks a V2 session export ({info, messages}) with
// a token decoder and stops after the first message of type "user". The prompt
// is that message's string text without one leading subagent prefix. A
// malformed document, or a first user message without a string text, yields
// no prompt.
func readOpenCodeV2FirstPrompt(r io.Reader) (string, bool) {
	dec := json.NewDecoder(bufio.NewReaderSize(r, 1<<20))
	if !expectDelim(dec, '{') {
		return "", false
	}
	for dec.More() {
		key, ok := nextKey(dec)
		if !ok {
			return "", false
		}
		if key != "messages" {
			if skipValue(dec) != nil {
				return "", false
			}
			continue
		}
		return firstV2UserText(dec)
	}
	return "", false
}

func firstV2UserText(dec *json.Decoder) (string, bool) {
	if !expectDelim(dec, '[') {
		return "", false
	}
	for dec.More() {
		isUser, text, ok := decodeV2Message(dec)
		if !ok {
			return "", false
		}
		if !isUser {
			continue
		}
		if text == nil {
			return "", false
		}
		return strings.TrimPrefix(*text, subagentPromptPrefix), true
	}
	return "", false
}

// decodeV2Message decodes one message; keys may come in any order. text is nil
// unless the message carries a string text.
func decodeV2Message(dec *json.Decoder) (isUser bool, text *string, ok bool) {
	if !expectDelim(dec, '{') {
		return false, nil, false
	}
	for dec.More() {
		key, keyOK := nextKey(dec)
		if !keyOK {
			return false, nil, false
		}
		switch key {
		case "type":
			var kind any
			if dec.Decode(&kind) != nil {
				return false, nil, false
			}
			isUser = kind == "user"
		case "text":
			var raw any
			if dec.Decode(&raw) != nil {
				return false, nil, false
			}
			if s, isString := raw.(string); isString {
				text = &s
			}
		default:
			if skipValue(dec) != nil {
				return false, nil, false
			}
		}
	}
	_, err := dec.Token() // closing brace
	return isUser, text, err == nil
}
