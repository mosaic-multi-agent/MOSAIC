package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type openCodeMessage struct {
	Role  string
	Texts []string
}

// readOpenCodeFirstPrompt walks the session-messages document with a token
// decoder, decoding one message at a time, and stops after the first user
// message. A malformed document yields no prompt.
func readOpenCodeFirstPrompt(r io.Reader) (string, bool) {
	dec := json.NewDecoder(bufio.NewReaderSize(r, 1<<20))
	if !expectDelim(dec, '{') {
		return "", false
	}
	for dec.More() {
		key, ok := nextKey(dec)
		if !ok {
			return "", false
		}
		if key != "data" {
			if skipValue(dec) != nil {
				return "", false
			}
			continue
		}
		return firstUserInData(dec)
	}
	return "", false
}

func firstUserInData(dec *json.Decoder) (string, bool) {
	if !expectDelim(dec, '[') {
		return "", false
	}
	for dec.More() {
		msg, ok := decodeMessage(dec)
		if !ok {
			return "", false
		}
		if msg.Role == "user" {
			if len(msg.Texts) == 0 {
				return "", false
			}
			return strings.Join(msg.Texts, "\n"), true
		}
	}
	return "", false
}

// decodeMessage decodes one {info, parts} message; keys may come in any order.
func decodeMessage(dec *json.Decoder) (openCodeMessage, bool) {
	var msg openCodeMessage
	if !expectDelim(dec, '{') {
		return msg, false
	}
	for dec.More() {
		key, ok := nextKey(dec)
		if !ok {
			return msg, false
		}
		switch key {
		case "info":
			var info struct {
				Role string `json:"role"`
			}
			if dec.Decode(&info) != nil {
				return msg, false
			}
			msg.Role = info.Role
		case "parts":
			var parts []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if dec.Decode(&parts) != nil {
				return msg, false
			}
			for _, p := range parts {
				if p.Type == "text" {
					msg.Texts = append(msg.Texts, p.Text)
				}
			}
		default:
			if skipValue(dec) != nil {
				return msg, false
			}
		}
	}
	_, err := dec.Token() // closing brace
	return msg, err == nil
}

func expectDelim(dec *json.Decoder, want json.Delim) bool {
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	d, ok := tok.(json.Delim)
	return ok && d == want
}

func nextKey(dec *json.Decoder) (string, bool) {
	tok, err := dec.Token()
	if err != nil {
		return "", false
	}
	s, ok := tok.(string)
	return s, ok
}

// skipValue discards the next value without retaining it.
func skipValue(dec *json.Decoder) error {
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else {
				depth--
			}
		}
		if depth == 0 {
			return nil
		}
	}
}
