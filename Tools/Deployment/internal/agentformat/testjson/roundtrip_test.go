package testjson_test

// roundtrip_test.go: Decode(Encode(d)) equals d at value level for the frontmatter
// (stamps included) and byte-exactly for the body.

import (
	"testing"
)

func TestRoundTrip_FullAgent_EqualsOriginalAtValueLevel(t *testing.T) {
	tr := translator(t)
	canonical := canonicalDoc(sampleFrontmatter(), sampleBody)

	encoded := mustEncodeCreate(t, tr, canonical)
	decoded := mustDecode(t, tr, encoded)

	assertValueEqual(t, canonical, decoded)
}

func TestRoundTrip_FrontmatterOrderIsPreserved(t *testing.T) {
	tr := translator(t)
	canonical := canonicalDoc("zeta: 1x\nalpha: 2x\nmid: 3x\n", "body\n")

	decoded := mustDecode(t, tr, mustEncodeCreate(t, tr, canonical))

	assertValueEqual(t, canonical, decoded)
}

func TestRoundTrip_EmptyFrontmatterAndEmptyBody(t *testing.T) {
	tr := translator(t)
	canonical := canonicalDoc("", "")

	decoded := mustDecode(t, tr, mustEncodeCreate(t, tr, canonical))

	assertValueEqual(t, canonical, decoded)
}

func TestRoundTrip_BodyIsByteExact(t *testing.T) {
	bodies := map[string]string{
		"no trailing newline":      "text without newline",
		"crlf line endings":        "line one\r\nline two\r\n",
		"leading blank lines":      "\n\n\nafter blanks\n",
		"fence-like lines":         "above\n---\nbelow\n---\n",
		"html comment region tag":  "<!-- note -->\n<Region type=\"project\">\nx\n</Region>\n",
		"unicode":                  "café 中文 \U0001F600\n",
		"escape-heavy":             "quote \" backslash \\ tab\t amp & lt < gt > dash - dashes --- end\n",
		"control characters":       "bell\x07 and nul\x00 inside\n",
		"literal escape sequences": "text \\u003c and \\u002d stay literal\n",
	}
	for name, body := range bodies {
		body := body
		t.Run(name, func(t *testing.T) {
			tr := translator(t)
			canonical := canonicalDoc("name: reviewer\n", body)

			decoded := mustDecode(t, tr, mustEncodeCreate(t, tr, canonical))

			_, gotBody := valueLevel(t, decoded)
			if gotBody != body {
				t.Errorf("body not byte-exact\nwant: %q\ngot:  %q", body, gotBody)
			}
		})
	}
}

func TestRoundTrip_StableOverRepeatedRoundTrips(t *testing.T) {
	tr := translator(t)
	canonical := canonicalDoc(sampleFrontmatter(), sampleBody)

	once := mustDecode(t, tr, mustEncodeCreate(t, tr, canonical))
	twice := mustDecode(t, tr, mustEncodeCreate(t, tr, once))

	assertValueEqual(t, canonical, twice)
}

func TestDecode_ReturnsCanonicalMarkdownWithFrontmatterThenBody(t *testing.T) {
	tr := translator(t)
	file := wireDocument("Do the work.\n", `[["name","reviewer"],["tools",{"list":["read","grep"]}],["meta",{"map":[["a","1x"]]}]]`, "")

	decoded := mustDecode(t, tr, file)

	want := canonicalDoc("name: reviewer\ntools:\n  - read\n  - grep\nmeta:\n  a: 1x\n", "Do the work.\n")
	assertValueEqual(t, want, decoded)
}

func TestDecode_AcceptsAnyValidJSONEscapeAndUnescapedCharacters(t *testing.T) {
	tr := translator(t)
	// Raw '<' and '-', and an upper-case hex escape, are valid JSON the decoder must accept.
	file := []byte("{\"mosaic_test_agent\":1,\"instructions\":\"a <b> - \\u003C\\u002D\",\"meta\":[[\"k-1\",\"v\"]]}\n")

	decoded := mustDecode(t, tr, file)

	_, body := valueLevel(t, decoded)
	if body != "a <b> - <-" {
		t.Errorf("body = %q, want %q", body, "a <b> - <-")
	}
}

func TestEncode_IgnoresAgentKeyAndRefreshMode(t *testing.T) {
	tr := translator(t)
	canonical := canonicalDoc("name: alpha\ndescription: d\n", "body\n")
	ctx := createCtx()
	ctx.AgentKey = "beta"
	ctx.RefreshMode = true

	encoded, rep, err := tr.Encode(canonical, ctx)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if len(rep.Entries) != 0 {
		t.Errorf("Encode returned report entries %+v; want none (no name forcing, no normalisation)", rep.Entries)
	}

	assertValueEqual(t, canonical, mustDecode(t, tr, encoded))
}
