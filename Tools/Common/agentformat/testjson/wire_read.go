package testjson

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"mosaic-common/docformat"
	"mosaic-common/mosaic"
)

// decodeFile parses the on-disk form strictly. Any deviation from the format is a
// malformed-deployed error.
func decodeFile(data []byte) (fileContent, error) {
	var f fileContent
	dec := json.NewDecoder(bytes.NewReader(data))
	var top map[string]json.RawMessage
	if err := dec.Decode(&top); err != nil {
		return f, malformed("", "not a JSON object: "+err.Error())
	}
	if top == nil {
		return f, malformed("", "top level is not a JSON object")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return f, malformed("", "unexpected content after the JSON object")
	}
	for k := range top {
		switch k {
		case markerKey, "instructions", "meta", "native":
		default:
			return f, malformed(k, "unknown top-level member")
		}
	}

	if marker, ok := top[markerKey]; !ok || string(bytes.TrimSpace(marker)) != "1" {
		return f, malformed(markerKey, "format marker missing or not 1")
	}
	instr, ok := top["instructions"]
	if !ok || firstByte(instr) != '"' || json.Unmarshal(instr, &f.instructions) != nil {
		return f, malformed("instructions", "missing or not a string")
	}
	meta, err := decodeMeta(top["meta"])
	if err != nil {
		return f, err
	}
	f.meta = meta
	if raw, present := top["native"]; present {
		if firstByte(raw) != '{' {
			return f, malformed("native", "not an object")
		}
		ndec := json.NewDecoder(bytes.NewReader(raw))
		ndec.UseNumber()
		if err := ndec.Decode(&f.native); err != nil {
			return f, malformed("native", err.Error())
		}
		f.hasNative = true
	}
	return f, nil
}

func firstByte(raw json.RawMessage) byte {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return 0
	}
	return t[0]
}

// decodeMeta reads the ordered [key, value] pairs, rejecting duplicate keys.
func decodeMeta(raw json.RawMessage) ([]mosaic.FieldPair, error) {
	if firstByte(raw) != '[' {
		return nil, malformed("meta", "missing or not an array")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, malformed("meta", err.Error())
	}
	seen := map[string]bool{}
	pairs := make([]mosaic.FieldPair, 0, len(entries))
	for _, entry := range entries {
		p, err := decodePair(entry)
		if err != nil {
			return nil, err
		}
		if seen[p.Key] {
			return nil, malformed(p.Key, "duplicate meta key")
		}
		seen[p.Key] = true
		pairs = append(pairs, p)
	}
	return pairs, nil
}

// decodePair reads one two-element [key, value] array.
func decodePair(raw json.RawMessage) (mosaic.FieldPair, error) {
	var elems []json.RawMessage
	if firstByte(raw) != '[' || json.Unmarshal(raw, &elems) != nil || len(elems) != 2 {
		return mosaic.FieldPair{}, malformed("meta", "entry is not a [key, value] pair")
	}
	var key string
	if firstByte(elems[0]) != '"' || json.Unmarshal(elems[0], &key) != nil {
		return mosaic.FieldPair{}, malformed("meta", "entry key is not a string")
	}
	value, err := decodeValue(key, elems[1])
	if err != nil {
		return mosaic.FieldPair{}, err
	}
	return mosaic.FieldPair{Key: key, Value: value}, nil
}

// decodeValue reads a meta value: a string, {"list": [...]} or {"map": [[k, v], ...]}.
func decodeValue(key string, raw json.RawMessage) (mosaic.FieldValue, error) {
	switch firstByte(raw) {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return mosaic.FieldValue{}, malformed(key, err.Error())
		}
		return mosaic.ScalarValue(s, mosaic.QuotePlain), nil
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil || len(obj) != 1 {
			return mosaic.FieldValue{}, malformed(key, "value object must hold exactly list or map")
		}
		if list, ok := obj["list"]; ok {
			return decodeList(key, list)
		}
		if m, ok := obj["map"]; ok {
			return decodeMap(key, m)
		}
	}
	return mosaic.FieldValue{}, malformed(key, "value is not a string, list or map")
}

func decodeList(key string, raw json.RawMessage) (mosaic.FieldValue, error) {
	var elems []json.RawMessage
	if firstByte(raw) != '[' || json.Unmarshal(raw, &elems) != nil {
		return mosaic.FieldValue{}, malformed(key, "list is not an array")
	}
	items := make([]mosaic.FieldValue, 0, len(elems))
	for _, e := range elems {
		v, err := decodeValue(key, e)
		if err != nil {
			return mosaic.FieldValue{}, err
		}
		items = append(items, v)
	}
	style := mosaic.ListBlock
	if len(items) == 0 {
		style = mosaic.ListFlow
	}
	return mosaic.ListValue(items, style), nil
}

func decodeMap(key string, raw json.RawMessage) (mosaic.FieldValue, error) {
	var elems []json.RawMessage
	if firstByte(raw) != '[' || json.Unmarshal(raw, &elems) != nil {
		return mosaic.FieldValue{}, malformed(key, "map is not an array of pairs")
	}
	pairs := make([]mosaic.FieldPair, 0, len(elems))
	for _, e := range elems {
		p, err := decodePair(e)
		if err != nil {
			return mosaic.FieldValue{}, err
		}
		pairs = append(pairs, p)
	}
	return mosaic.MappingValue(pairs), nil
}

// renderCanonical builds the canonical Markdown document: YAML frontmatter from meta in
// order, then the instructions as the body, byte-exact.
func renderCanonical(f fileContent) []byte {
	fields := make([]mosaic.FrontmatterField, 0, len(f.meta))
	for _, p := range f.meta {
		fields = append(fields, mosaic.FrontmatterField{Key: p.Key, Value: yamlValue(p.Value)})
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.WriteString(docformat.RenderFrontmatter(fields, "\n"))
	buf.WriteString("---\n")
	buf.WriteString(f.instructions)
	return buf.Bytes()
}

// yamlValue prepares a value for rendering: scalars are pre-quoted when plain YAML
// could not represent them.
func yamlValue(v mosaic.FieldValue) mosaic.FieldValue {
	switch v.Kind {
	case mosaic.KindList:
		items := make([]mosaic.FieldValue, len(v.Items))
		for i, item := range v.Items {
			items[i] = yamlValue(item)
		}
		return mosaic.ListValue(items, v.List)
	case mosaic.KindMapping:
		pairs := make([]mosaic.FieldPair, len(v.Pairs))
		for i, p := range v.Pairs {
			pairs[i] = mosaic.FieldPair{Key: p.Key, Value: yamlValue(p.Value)}
		}
		return mosaic.MappingValue(pairs)
	}
	if plainSafe(v.Scalar) {
		return v
	}
	return mosaic.ScalarValue(`"`+escapeDoubleQuoted(v.Scalar)+`"`, mosaic.QuotePlain)
}

// plainSafe reports whether s can be written as an unquoted YAML scalar and re-read as s.
func plainSafe(s string) bool {
	if s == "" || s != strings.TrimSpace(s) {
		return false
	}
	if strings.ContainsAny(s[:1], "#\"'|>&*!`@%-?:,[]{}") {
		return false
	}
	if strings.Contains(s, ": ") || strings.Contains(s, " #") || strings.HasSuffix(s, ":") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7F {
			return false
		}
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err != nil
}

// escapeDoubleQuoted escapes the five sequences docformat reads inside a double-quoted scalar.
func escapeDoubleQuoted(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
