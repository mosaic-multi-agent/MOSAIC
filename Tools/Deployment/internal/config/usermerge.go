package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// mergeUserConfigInPlace edits the four modeled keys in raw into the existing document text
// and returns the resulting bytes. It does not write anything to disk. A parse failure on
// existing is returned as an error and existing is never touched by the caller in that case.
//
// The existing file is parsed to decide, per key, whether its value changed; the edit itself
// is applied to the text, one top-level key block at a time. A key block is the key line plus
// the indented lines that belong to its value. Everything outside the block of a rewritten or
// removed key (comments included) is carried over byte for byte, and a rewritten value is
// always emitted with two spaces of indentation per level.
func mergeUserConfigInPlace(existing []byte, raw userConfigYAML) ([]byte, error) {
	file, err := parser.ParseBytes(existing, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse existing user config: %w", err)
	}

	var root *ast.MappingNode
	if len(file.Docs) > 0 && file.Docs[0].Body != nil {
		m, ok := file.Docs[0].Body.(*ast.MappingNode)
		if !ok {
			return nil, fmt.Errorf("root of existing user config is not a mapping")
		}
		root = m
	}

	// CRLF documents are edited as LF text and converted back, so every line keeps its ending.
	crlf := strings.Contains(string(existing), "\r\n")
	text := string(existing)
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	lines := splitConfigLines(text)
	fields := []struct {
		key   string
		value interface{}
	}{
		{"schema_version", raw.SchemaVersion},
		{"tier_models", raw.TierModels},
		{"custom_model_ids", raw.CustomModelIDs},
		{"tool_destinations", raw.ToolDestinations},
	}
	for _, f := range fields {
		lines, err = applyUserConfigField(lines, root, f.key, f.value)
		if err != nil {
			return nil, fmt.Errorf("update %s: %w", f.key, err)
		}
	}
	out := strings.Join(lines, "\n")
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return []byte(out), nil
}

// splitConfigLines splits text into lines such that joining them with "\n" restores text.
func splitConfigLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// applyUserConfigField updates key in lines to reflect value: an empty value removes the key
// block when present, a changed non-empty value replaces the block, an unchanged value is
// left untouched, and an absent key with a non-empty value is appended.
func applyUserConfigField(lines []string, root *ast.MappingNode, key string, value interface{}) ([]string, error) {
	start, end := findKeyBlock(lines, key)

	if isEmptyUserConfigValue(value) {
		if start < 0 {
			return lines, nil
		}
		return spliceLines(lines, start, end, nil), nil
	}

	if start >= 0 && root != nil {
		if idx := findUserConfigKey(root, key); idx >= 0 {
			if unchanged, err := userConfigValueUnchanged(root.Values[idx].Value, value); err == nil && unchanged {
				return lines, nil
			}
		}
	}

	block, err := marshalKeyBlock(key, value)
	if err != nil {
		return nil, err
	}
	if start < 0 {
		return appendKeyBlock(lines, block), nil
	}
	if comment := keyLineComment(lines[start], key); comment != "" {
		block[0] += " " + comment
	}
	return spliceLines(lines, start, end, block), nil
}

// keyLinePattern matches a column-0 mapping key line; group 1 is the key.
var keyLinePattern = regexp.MustCompile(`^([A-Za-z0-9_.-]+)[ \t]*:(?:[ \t]|$)`)

// keyCommentPattern captures the comment of a key line whose value is empty or an empty
// flow collection, the only shapes where a trailing comment can be told apart safely.
func keyCommentPattern(key string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `[ \t]*:[ \t]*(?:\{\}|\[\])?[ \t]*(#.*?)[ \t\r]*$`)
}

// keyLineComment returns the trailing comment of a key line with a trivial value, or "".
func keyLineComment(line, key string) string {
	m := keyCommentPattern(key).FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}

// findKeyBlock returns the half-open line range [start, end) of key's block, or (-1, -1).
// The block holds the key line and the indented (or column-0 sequence) lines of its value;
// trailing blank and comment lines are not part of it.
func findKeyBlock(lines []string, key string) (int, int) {
	start := -1
	for i, l := range lines {
		if m := keyLinePattern.FindStringSubmatch(l); m != nil && m[1] == key {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	end := start + 1
	for i := start + 1; i < len(lines); i++ {
		if isBlockContentLine(lines[i]) {
			end = i + 1
			continue
		}
		if !isBlankOrComment(lines[i]) {
			break
		}
	}
	return start, end
}

// isBlockContentLine reports whether l is a value line (not blank, not a comment) that
// belongs to the preceding key: indented, or a column-0 sequence entry.
func isBlockContentLine(l string) bool {
	if isBlankOrComment(l) {
		return false
	}
	return l[0] == ' ' || l[0] == '\t' || l[0] == '-'
}

// isBlankOrComment reports whether l is empty, whitespace, or a comment line.
func isBlankOrComment(l string) bool {
	t := strings.TrimSpace(l)
	return t == "" || strings.HasPrefix(t, "#")
}

// spliceLines replaces lines[start:end] with repl and returns the new slice.
func spliceLines(lines []string, start, end int, repl []string) []string {
	out := make([]string, 0, len(lines)-(end-start)+len(repl))
	out = append(out, lines[:start]...)
	out = append(out, repl...)
	out = append(out, lines[end:]...)
	return out
}

// appendKeyBlock adds block at the end of the document, keeping the final newline.
func appendKeyBlock(lines, block []string) []string {
	if len(lines) == 0 {
		return append(block, "")
	}
	if lines[len(lines)-1] == "" {
		out := append([]string{}, lines[:len(lines)-1]...)
		out = append(out, block...)
		return append(out, "")
	}
	out := append([]string{}, lines...)
	return append(out, block...)
}

// marshalKeyBlock renders key: value as YAML lines with the marshaler's two-space indent.
func marshalKeyBlock(key string, value interface{}) ([]string, error) {
	data, err := yaml.MarshalWithOptions(map[string]interface{}{key: value}, yaml.IndentSequence(true))
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", key, err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n"), nil
}

// findUserConfigKey returns the index of key within root.Values, or -1 if absent.
func findUserConfigKey(root *ast.MappingNode, key string) int {
	for i, v := range root.Values {
		if v.Key.GetToken().Value == key {
			return i
		}
	}
	return -1
}

// isEmptyUserConfigValue reports whether value is the zero-ish value that means "this key
// should not appear in the document" for a modeled user-config field.
func isEmptyUserConfigValue(value interface{}) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	default:
		return false
	}
}

// userConfigValueUnchanged reports whether existing, decoded as the same type as value,
// is deep-equal to value.
func userConfigValueUnchanged(existing ast.Node, value interface{}) (bool, error) {
	target := reflect.New(reflect.TypeOf(value))
	if err := yaml.NodeToValue(existing, target.Interface()); err != nil {
		return false, err
	}
	return reflect.DeepEqual(target.Elem().Interface(), value), nil
}
