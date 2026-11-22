package harness

// Rendering of loaded Codex settings as `-c key=value` override values. The
// value side is TOML; every rendering stays on one line (strings are escaped),
// so argv never carries a newline.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// codexOverrides flattens one setting into `path=value` overrides: tables become
// dotted leaves, everything else (including arrays and arrays of tables) one
// inline value. Paths in skip are not emitted.
func codexOverrides(path string, value any, skip map[string]bool) []string {
	if table, ok := value.(map[string]any); ok && len(table) > 0 {
		var out []string
		for _, key := range sortedKeys(table) {
			out = append(out, codexOverrides(path+"."+codexKey(key), table[key], skip)...)
		}
		return out
	}
	if skip[path] {
		return nil
	}
	return []string{path + "=" + codexValue(value)}
}

func sortedKeys(table map[string]any) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// codexKey renders one key segment, quoting it unless it is a bare TOML key.
func codexKey(key string) string {
	if key != "" && strings.IndexFunc(key, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) < 0 {
		return key
	}
	return codexString(key)
}

// codexValue renders a decoded setting value as a single-line TOML value.
func codexValue(value any) string {
	switch v := value.(type) {
	case string:
		return codexString(v)
	case bool:
		return strconv.FormatBool(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case float64:
		return codexFloat(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = codexValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		parts := make([]string, 0, len(v))
		for _, key := range sortedKeys(v) {
			parts = append(parts, codexKey(key)+" = "+codexValue(v[key]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case fmt.Stringer:
		return v.String()
	default:
		return codexString(fmt.Sprint(v))
	}
}

// codexString renders a TOML basic string on one line.
func codexString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func codexFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	switch s {
	case "+Inf":
		return "inf"
	case "-Inf":
		return "-inf"
	case "NaN":
		return "nan"
	}
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}
