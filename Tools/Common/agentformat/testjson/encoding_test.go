package testjson_test

// encoding_test.go pins the on-disk form: a JSON object that cannot be mistaken for
// Markdown or TOML. No '<', no "---" and no region tag appear literally.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncode_OutputHasNoRegionTagAndNoFrontmatterFence(t *testing.T) {
	tr := translator(t)

	encoded := mustEncodeCreate(t, tr, canonicalDoc(sampleFrontmatter(), sampleBody))

	for _, forbidden := range []string{"---", "<", ">", "&", "<CommunicationProtocol", "type=\"project\"", "type=\"custom\"", "</"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Errorf("encoded file contains literal %q\nfile: %s", forbidden, encoded)
		}
	}
}

func TestEncode_WithoutNativeMember_HasNoLiteralHyphenAtAll(t *testing.T) {
	tr := translator(t)

	encoded := mustEncodeCreate(t, tr, canonicalDoc(sampleFrontmatter(), sampleBody))

	if bytes.ContainsRune(encoded, '-') {
		t.Errorf("encoded create file contains a literal '-'; every hyphen in a string must be escaped\nfile: %s", encoded)
	}
}

func TestEncode_EscapesAreLowerCaseSixCharacterUnicodeEscapes(t *testing.T) {
	tr := translator(t)

	encoded := string(mustEncodeCreate(t, tr, canonicalDoc("name: a\n", "x < y > z & w - v\n")))

	for _, want := range []string{uEsc("003c"), uEsc("003e"), uEsc("0026"), uEsc("002d")} {
		if !strings.Contains(encoded, want) {
			t.Errorf("encoded file lacks escape %s\nfile: %s", want, encoded)
		}
	}
	for _, upper := range []string{uEsc("003C"), uEsc("003E"), uEsc("002D")} {
		if strings.Contains(encoded, upper) {
			t.Errorf("encoded file contains upper-case escape %s; lower-case only\nfile: %s", upper, encoded)
		}
	}
}

// uEsc builds the six-character JSON escape (backslash, 'u', four hex digits). It is
// assembled at run time so the test source never holds a literal escape sequence.
func uEsc(hex string) string {
	return "\\" + "u" + hex
}

func TestEncode_EscapesApplyToKeysAndValuesToo(t *testing.T) {
	tr := translator(t)

	encoded := string(mustEncodeCreate(t, tr, canonicalDoc("my-key: a-b\n", "body\n")))

	hyphen := uEsc("002d")
	for _, want := range []string{"my" + hyphen + "key", "a" + hyphen + "b"} {
		if !strings.Contains(encoded, want) {
			t.Errorf("hyphen in a key and a value must be escaped; missing %s\nfile: %s", want, encoded)
		}
	}
	if strings.ContainsRune(encoded, '-') {
		t.Errorf("encoded file contains a literal '-'\nfile: %s", encoded)
	}
}

func TestEncode_IsOneJSONObjectWithMembersInFixedOrderAndOneTrailingNewline(t *testing.T) {
	tr := translator(t)

	encoded := mustEncodeCreate(t, tr, canonicalDoc(sampleFrontmatter(), sampleBody))

	obj := wireObject(t, encoded)
	if string(obj["mosaic_test_agent"]) != "1" {
		t.Errorf("mosaic_test_agent = %s, want 1", obj["mosaic_test_agent"])
	}
	if _, ok := obj["native"]; ok {
		t.Errorf("create output has a native member; it must have none")
	}
	if len(obj) != 3 {
		t.Errorf("create output has %d members, want exactly mosaic_test_agent, instructions, meta", len(obj))
	}
	s := string(encoded)
	iMarker := strings.Index(s, `"mosaic_test_agent"`)
	iInstr := strings.Index(s, `"instructions"`)
	iMeta := strings.Index(s, `"meta"`)
	if !(iMarker >= 0 && iMarker < iInstr && iInstr < iMeta) {
		t.Errorf("members out of order (marker %d, instructions %d, meta %d)", iMarker, iInstr, iMeta)
	}
	if !strings.HasSuffix(s, "}\n") || strings.HasSuffix(s, "}\n\n") {
		t.Errorf("file must end with the closing brace and exactly one LF; got tail %q", s[max(0, len(s)-4):])
	}
	if strings.Contains(s, "\r") {
		t.Errorf("file contains CR; LF line endings only")
	}
}

func TestEncode_InstructionsHoldTheCanonicalBodyByteExact(t *testing.T) {
	tr := translator(t)

	encoded := mustEncodeCreate(t, tr, canonicalDoc(sampleFrontmatter(), sampleBody))

	var instructions string
	if err := json.Unmarshal(wireObject(t, encoded)["instructions"], &instructions); err != nil {
		t.Fatalf("instructions is not a JSON string: %v", err)
	}
	if instructions != sampleBody {
		t.Errorf("instructions != canonical body\nwant: %q\ngot:  %q", sampleBody, instructions)
	}
}

func TestEncode_MetaIsOrderedPairsWithTaggedListAndMapValues(t *testing.T) {
	tr := translator(t)
	canonical := canonicalDoc("name: reviewer\ntools:\n  - read\n  - grep\nmetadata:\n  tier: gold\n", "b\n")

	encoded := mustEncodeCreate(t, tr, canonical)

	var meta []json.RawMessage
	if err := json.Unmarshal(wireObject(t, encoded)["meta"], &meta); err != nil {
		t.Fatalf("meta is not a JSON array: %v", err)
	}
	if len(meta) != 3 {
		t.Fatalf("meta has %d entries, want 3", len(meta))
	}
	want := []string{
		`["name","reviewer"]`,
		`["tools",{"list":["read","grep"]}]`,
		`["metadata",{"map":[["tier","gold"]]}]`,
	}
	for i, w := range want {
		if got := compactJSON(t, meta[i]); got != w {
			t.Errorf("meta[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestEncode_StampsAreOrdinaryMetaEntries(t *testing.T) {
	tr := translator(t)

	encoded := mustEncodeCreate(t, tr, canonicalDoc(stampKey()+": abc-123\n", "b\n"))

	var meta [][]json.RawMessage
	if err := json.Unmarshal(wireObject(t, encoded)["meta"], &meta); err != nil {
		t.Fatalf("meta is not an array of pairs: %v", err)
	}
	if len(meta) != 1 || compactJSON(t, meta[0][0]) != `"`+stampKey()+`"` {
		t.Errorf("stamp %q is not carried as an ordinary meta entry: %s", stampKey(), encoded)
	}
}

// compactJSON re-marshals raw JSON in compact form for comparison.
func compactJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("compact: %v", err)
	}
	return buf.String()
}
