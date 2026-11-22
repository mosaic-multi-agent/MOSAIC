package opencodev2

// Reading V2 configuration documents: JSON with comments and trailing commas,
// judged only for a non-empty plugin registration key.

import (
	"encoding/json"
	"errors"
	"fmt"
)

// pluginKeys are the top-level config keys that register plugins.
var pluginKeys = []string{"plugin", "plugins"}

// stripJSONC removes line and block comments and trailing commas outside
// string literals, leaving plain JSON.
func stripJSONC(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inString:
			out = append(out, c)
			if c == '\\' && i+1 < len(src) {
				i++
				out = append(out, src[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return removeTrailingCommas(out)
}

// removeTrailingCommas drops a comma that is followed only by whitespace and a
// closing bracket, outside string literals.
func removeTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(src) {
				i++
				out = append(out, src[i])
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
		}
		if c == ',' {
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// registeredPluginKeys returns the plugin-registration keys that hold a
// non-empty value in the config document doc. An error means the document is
// not a JSON(C) object.
func registeredPluginKeys(doc []byte) ([]string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(stripJSONC(doc), &top); err != nil {
		return nil, err
	}
	if top == nil {
		return nil, errors.New("document is not an object")
	}
	var found []string
	for _, key := range pluginKeys {
		if hasValue(top[key]) {
			found = append(found, key)
		}
	}
	return found, nil
}

// hasValue reports whether a raw JSON value is anything but absent, null, or
// an empty array, object or string.
func hasValue(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return true
	}
	switch t := v.(type) {
	case nil:
		return false
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	case string:
		return t != ""
	}
	return true
}

// quotedKeys renders keys as a comma-separated list of quoted names.
func quotedKeys(keys []string) string {
	s := ""
	for i, k := range keys {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%q", k)
	}
	return s
}
