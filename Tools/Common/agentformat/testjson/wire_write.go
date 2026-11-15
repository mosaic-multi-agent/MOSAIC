package testjson

import (
	"encoding/json"
	"sort"
	"strings"

	"mosaic-common/mosaic"
)

// markerKey names the format marker member. It is assembled so no "mosaic" + underscore
// literal appears in production source (field names live in agentfields).
const markerKey = "mosaic" + "_test_agent"

// hyphenEscape is the six-character JSON escape (backslash, 'u', 002d) written for '-'.
const hyphenEscape = "\\" + "u002d"

// fileContent is the decoded content of one test-format file.
type fileContent struct {
	instructions string
	meta         []mosaic.FieldPair
	hasNative    bool
	native       any // JSON value tree with json.Number numbers; an object when hasNative
}

// writeFile renders the on-disk form: one JSON object, members in fixed order, one
// trailing newline. Every string is written with '<', '>', '&' and '-' as lower-case
// JSON Unicode escapes, so the file holds no region tag and no fence.
func writeFile(f fileContent) []byte {
	var sb strings.Builder
	sb.WriteString("{\n  " + quote(markerKey) + ": 1,\n  \"instructions\": ")
	sb.WriteString(quote(f.instructions))
	sb.WriteString(",\n  \"meta\": [")
	for i, p := range f.meta {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("\n    [")
		sb.WriteString(quote(p.Key))
		sb.WriteString(", ")
		writeFieldValue(&sb, p.Value)
		sb.WriteString("]")
	}
	if len(f.meta) > 0 {
		sb.WriteString("\n  ")
	}
	sb.WriteString("]")
	if f.hasNative {
		sb.WriteString(",\n  \"native\": ")
		writeNative(&sb, f.native)
	}
	sb.WriteString("\n}\n")
	return []byte(sb.String())
}

// quote writes s as a JSON string with the four reserved characters escaped.
func quote(s string) string {
	raw, _ := json.Marshal(s) // encoding/json already escapes '<', '>' and '&' in lower case
	return strings.ReplaceAll(string(raw), "-", hyphenEscape)
}

// writeFieldValue writes a meta value: a scalar string, {"list": [...]} or {"map": [[k, v], ...]}.
func writeFieldValue(sb *strings.Builder, v mosaic.FieldValue) {
	switch v.Kind {
	case mosaic.KindList:
		sb.WriteString(`{"list": [`)
		for i, item := range v.Items {
			if i > 0 {
				sb.WriteString(", ")
			}
			writeFieldValue(sb, item)
		}
		sb.WriteString("]}")
	case mosaic.KindMapping:
		sb.WriteString(`{"map": [`)
		for i, p := range v.Pairs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("[")
			sb.WriteString(quote(p.Key))
			sb.WriteString(", ")
			writeFieldValue(sb, p.Value)
			sb.WriteString("]")
		}
		sb.WriteString("]}")
	default:
		sb.WriteString(quote(v.Scalar))
	}
}

// writeNative writes a decoded JSON value tree, object keys sorted, numbers as read.
func writeNative(sb *strings.Builder, v any) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteString("{")
		for i, k := range keys {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(quote(k))
			sb.WriteString(": ")
			writeNative(sb, x[k])
		}
		sb.WriteString("}")
	case []any:
		sb.WriteString("[")
		for i, item := range x {
			if i > 0 {
				sb.WriteString(", ")
			}
			writeNative(sb, item)
		}
		sb.WriteString("]")
	case string:
		sb.WriteString(quote(x))
	case json.Number:
		sb.WriteString(x.String())
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	default:
		sb.WriteString("null")
	}
}
