package codextoml

// capability_settings.go owns the two Codex capability settings MOSAIC writes into an
// agent file: the top-level web_search level and the [sandbox_workspace_write] table.
// Both are MOSAIC-owned keys. Decode returns them as ordinary canonical frontmatter
// (web_search as a scalar, the table as a mapping of leaves); encode writes them back
// as native TOML. Table leaves may be booleans, integers, strings or string arrays.

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"mosaic-common/agentformat"
	"mosaic-common/mosaic"
)

const (
	webSearchKey     = "web_search"
	sandboxTableKey  = "sandbox_workspace_write"
	capabilityDecode = "decode"
)

// capabilityMalformed builds the malformed-file error naming the offending key.
func capabilityMalformed(key, format string, args ...interface{}) error {
	return &agentformat.ArtifactError{
		Phase: capabilityDecode,
		Key:   key,
		Err:   fmt.Errorf("%w: %s", agentformat.ErrMalformedDeployed, fmt.Sprintf(format, args...)),
	}
}

// decodeCapabilityFields returns the canonical frontmatter fields for web_search and
// sandbox_workspace_write, in that order, each only when the deployed file carries it.
func decodeCapabilityFields(raw map[string]interface{}) ([]mosaic.FrontmatterField, error) {
	var fields []mosaic.FrontmatterField

	if v, ok := raw[webSearchKey]; ok {
		s, isString := v.(string)
		if !isString {
			return nil, capabilityMalformed(webSearchKey, "key %q must be a string, got %T", webSearchKey, v)
		}
		fv, err := applyQuotingPolicy(s, webSearchKey)
		if err != nil {
			return nil, err
		}
		fields = append(fields, mosaic.FrontmatterField{Key: webSearchKey, Value: fv})
	}

	if v, ok := raw[sandboxTableKey]; ok {
		table, isTable := v.(map[string]interface{})
		if !isTable {
			return nil, capabilityMalformed(sandboxTableKey, "key %q must be a table, got %T", sandboxTableKey, v)
		}
		fv, err := decodeSandboxTable(table)
		if err != nil {
			return nil, err
		}
		fields = append(fields, mosaic.FrontmatterField{Key: sandboxTableKey, Value: fv})
	}
	return fields, nil
}

// decodeSandboxTable converts the table's leaves to a canonical mapping, sorted by key.
func decodeSandboxTable(table map[string]interface{}) (mosaic.FieldValue, error) {
	keys := make([]string, 0, len(table))
	for k := range table {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]mosaic.FieldPair, 0, len(keys))
	for _, k := range keys {
		fv, err := decodeSandboxLeaf(k, table[k])
		if err != nil {
			return mosaic.FieldValue{}, err
		}
		pairs = append(pairs, mosaic.FieldPair{Key: k, Value: fv})
	}
	return mosaic.MappingValue(pairs), nil
}

func decodeSandboxLeaf(key string, v interface{}) (mosaic.FieldValue, error) {
	switch vv := v.(type) {
	case bool:
		return mosaic.ScalarValue(strconv.FormatBool(vv), mosaic.QuotePlain), nil
	case int64:
		return mosaic.ScalarValue(strconv.FormatInt(vv, 10), mosaic.QuotePlain), nil
	case string:
		if hasUnrepresentableControlChar(vv) {
			return mosaic.FieldValue{}, &agentformat.ArtifactError{
				Phase: capabilityDecode, Key: sandboxTableKey, Err: agentformat.ErrUnrepresentableValue,
			}
		}
		return makeStringCarriageFV(vv), nil
	case []interface{}:
		items := make([]mosaic.FieldValue, 0, len(vv))
		for _, el := range vv {
			s, ok := el.(string)
			if !ok || hasUnrepresentableControlChar(s) {
				return mosaic.FieldValue{}, capabilityMalformed(sandboxTableKey,
					"leaf %q of table %q must be a string array", key, sandboxTableKey)
			}
			items = append(items, makeStringCarriageFV(s))
		}
		return mosaic.ListValue(items, mosaic.ListFlow), nil
	default:
		return mosaic.FieldValue{}, capabilityMalformed(sandboxTableKey,
			"leaf %q of table %q has unsupported type %T", key, sandboxTableKey, v)
	}
}

// writeWebSearchLine emits the web_search key when the canonical frontmatter carries it.
func writeWebSearchLine(buf *bytes.Buffer, v interface{}) {
	if v == nil {
		return
	}
	buf.WriteString(webSearchKey + " = ")
	writeTomlBasicString(buf, fmValStr(v))
	buf.WriteByte('\n')
}

// writeSandboxTable emits the [sandbox_workspace_write] table when the canonical
// frontmatter carries a non-empty mapping. Leaves are written in sorted key order.
func writeSandboxTable(buf *bytes.Buffer, v interface{}) error {
	table, ok := v.(map[string]interface{})
	if !ok || len(table) == 0 {
		return nil
	}
	keys := make([]string, 0, len(table))
	for k := range table {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf.WriteString("\n[" + sandboxTableKey + "]\n")
	for _, k := range keys {
		if isBareKey(k) {
			buf.WriteString(k)
		} else {
			writeTomlBasicString(buf, k)
		}
		buf.WriteString(" = ")
		if err := writeSandboxLeaf(buf, table[k]); err != nil {
			return &agentformat.ArtifactError{Phase: "encode", Key: sandboxTableKey, Err: err}
		}
		buf.WriteByte('\n')
	}
	return nil
}

func writeSandboxLeaf(buf *bytes.Buffer, v interface{}) error {
	switch vv := v.(type) {
	case []interface{}:
		buf.WriteByte('[')
		for i, el := range vv {
			if i > 0 {
				buf.WriteString(", ")
			}
			s, ok := el.(string)
			if !ok {
				return fmt.Errorf("unsupported array element type %T", el)
			}
			writeTomlBasicString(buf, s)
		}
		buf.WriteByte(']')
		return nil
	case string, bool, int, int64, uint64:
		return writeForeignKeyValue(buf, v)
	default:
		return fmt.Errorf("unsupported table leaf type %T", v)
	}
}
