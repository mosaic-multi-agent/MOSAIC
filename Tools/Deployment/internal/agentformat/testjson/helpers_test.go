package testjson_test

// helpers_test.go holds the shared fixtures and assertions of the testjson contract
// tests. The translator is reached only through the runtime lookup, as production
// code reaches any translator.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-common/mosaic"

	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/agentformat/testjson"
	"mosaic-deploy/internal/domain"
)

// sampleBody holds a Deploy-owned region, a project region, a custom region, and
// characters the encoding must escape ('<', '>', '&', '-' and a "---" run).
const sampleBody = "\n<CommunicationProtocol type=\"managed\" version=\"1\">\n" +
	"Protocol text - with dashes & ampersands <b>bold</b>.\n" +
	"</CommunicationProtocol>\n\n" +
	"<IdentityExtension type=\"project\">\nproject notes\n</IdentityExtension>\n\n" +
	"<LocalNotes type=\"custom\">\nmine\n</LocalNotes>\n\n" +
	"Trailing prose with --- inside.\n"

// stampKey returns a real deployed stamp key; stamps are ordinary data in this format.
func stampKey() string { return agentfields.All()[0].Deployed }

// sampleFrontmatter covers scalar, quoted scalar, list, mapping, hyphenated keys and a stamp.
func sampleFrontmatter() string {
	return "name: reviewer\n" +
		"description: \"Reviews code: carefully\"\n" +
		"tools:\n  - read\n  - grep\n" +
		"metadata:\n  owner-team: platform\n  tier: gold\n" +
		"my-hyphen-key: value-with-dash\n" +
		stampKey() + ": abc-123\n"
}

// canonicalDoc assembles a canonical MOSAIC document.
func canonicalDoc(frontmatter, body string) []byte {
	return []byte("---\n" + frontmatter + "---\n" + body)
}

// translator registers the test format and resolves it through the runtime lookup.
func translator(t *testing.T) agentformat.Translator {
	t.Helper()
	testjson.Register()
	tr, err := agentformat.LookupString(string(testjson.ID))
	if err != nil {
		t.Fatalf("LookupString(%q) after Register: %v", testjson.ID, err)
	}
	if tr == nil {
		t.Fatalf("LookupString(%q) returned a nil translator with nil error", testjson.ID)
	}
	return tr
}

func createCtx() agentformat.ArtifactContext {
	return agentformat.ArtifactContext{AgentKey: "reviewer", Kind: domain.ArtifactAgent, Op: agentformat.OpCreate}
}

func updateCtx(prior []byte) agentformat.ArtifactContext {
	return agentformat.ArtifactContext{AgentKey: "reviewer", Kind: domain.ArtifactAgent, Op: agentformat.OpUpdate, PriorDeployed: prior}
}

func decodeCtx() agentformat.ArtifactContext {
	return agentformat.ArtifactContext{AgentKey: "reviewer", Kind: domain.ArtifactAgent}
}

// mustEncodeCreate encodes canonical with a create context and fails on error.
func mustEncodeCreate(t *testing.T, tr agentformat.Translator, canonical []byte) []byte {
	t.Helper()
	out, _, err := tr.Encode(canonical, createCtx())
	if err != nil {
		t.Fatalf("Encode(create) error: %v", err)
	}
	return out
}

// mustDecode decodes deployed bytes and fails on error.
func mustDecode(t *testing.T, tr agentformat.Translator, deployed []byte) []byte {
	t.Helper()
	out, rep, err := tr.Decode(deployed, decodeCtx())
	if err != nil {
		t.Fatalf("Decode error: %v\nfile: %s", err, deployed)
	}
	if len(rep.Entries) != 0 {
		t.Fatalf("Decode of a valid file returned report entries: %+v", rep.Entries)
	}
	return out
}

// semanticValue reduces a FieldValue to its value-level content: strings, lists and
// ordered pairs, with no quoting or list-style information.
func semanticValue(v mosaic.FieldValue) any {
	switch v.Kind {
	case mosaic.KindList:
		items := make([]any, 0, len(v.Items))
		for _, it := range v.Items {
			items = append(items, semanticValue(it))
		}
		return items
	case mosaic.KindMapping:
		pairs := make([]any, 0, len(v.Pairs))
		for _, p := range v.Pairs {
			pairs = append(pairs, []any{p.Key, semanticValue(p.Value)})
		}
		return map[string]any{"map": pairs}
	default:
		return v.Scalar
	}
}

// valueLevel splits a canonical document into ordered frontmatter pairs (value-level)
// and the exact body.
func valueLevel(t *testing.T, canonical []byte) ([]any, string) {
	t.Helper()
	doc, err := docformat.Parse(canonical)
	if err != nil {
		t.Fatalf("docformat.Parse: %v\ndocument: %s", err, canonical)
	}
	_, body, err := docformat.SplitFrontmatter(canonical)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	fm := doc.Frontmatter()
	var pairs []any
	for _, k := range fm.Keys() {
		v, _ := fm.Get(k)
		pairs = append(pairs, []any{k, semanticValue(v)})
	}
	return pairs, string(body)
}

// assertValueEqual fails unless both canonical documents are equal at value level for
// frontmatter and byte-exact for the body.
func assertValueEqual(t *testing.T, want, got []byte) {
	t.Helper()
	wantFM, wantBody := valueLevel(t, want)
	gotFM, gotBody := valueLevel(t, got)
	if !reflect.DeepEqual(wantFM, gotFM) {
		t.Errorf("frontmatter differs at value level\nwant: %v\ngot:  %v", wantFM, gotFM)
	}
	if wantBody != gotBody {
		t.Errorf("body not byte-exact\nwant: %q\ngot:  %q", wantBody, gotBody)
	}
}

// wireObject decodes an encoded file as a JSON object, failing when it is not one.
func wireObject(t *testing.T, encoded []byte) map[string]json.RawMessage {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &obj); err != nil {
		t.Fatalf("encoded file is not a JSON object: %v\nfile: %s", err, encoded)
	}
	return obj
}

// wireDocument builds a valid file with plain (unescaped) JSON, for decode and prior-bytes
// fixtures. nativeJSON is omitted when empty.
func wireDocument(instructions, metaJSON, nativeJSON string) []byte {
	instr, _ := json.Marshal(instructions)
	s := "{\"mosaic_test_agent\":1,\"instructions\":" + string(instr) + ",\"meta\":" + metaJSON
	if nativeJSON != "" {
		s += ",\"native\":" + nativeJSON
	}
	return []byte(s + "}\n")
}

func bytesContain(b []byte, sub string) bool { return strings.Contains(string(b), sub) }

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
