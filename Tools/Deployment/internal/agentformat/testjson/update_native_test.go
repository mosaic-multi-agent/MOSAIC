package testjson_test

// update_native_test.go covers the Op contract, the prior-bytes channel that keeps
// user-owned native data across an update, and carriage-container handling.

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"mosaic-deploy/internal/agentformat"
)

const priorNative = `{"user_setting":"keep-me","count":-5,"nested":{"list":[1,2,{"deep":true}]},"ratio":1.5e-3}`

func updateCanonical() []byte {
	return canonicalDoc("name: reviewer\ndescription: refreshed\n", "new body\n")
}

func nativeOf(t *testing.T, encoded []byte) (json.RawMessage, bool) {
	t.Helper()
	raw, ok := wireObject(t, encoded)["native"]
	return raw, ok
}

func TestEncode_Update_CopiesNativeOfPriorBytesAtValueLevel(t *testing.T) {
	tr := translator(t)
	prior := wireDocument("old body\n", `[["name","reviewer"]]`, priorNative)

	encoded, _, err := tr.Encode(updateCanonical(), updateCtx(prior))
	if err != nil {
		t.Fatalf("Encode(update) error: %v", err)
	}

	raw, ok := nativeOf(t, encoded)
	if !ok {
		t.Fatalf("update output lost the native member of the prior bytes\nfile: %s", encoded)
	}
	var got, want any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("native is not valid JSON: %v", err)
	}
	_ = json.Unmarshal([]byte(priorNative), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("native changed\nwant: %v\ngot:  %v", want, got)
	}
}

// priorNativeNeedingEscapes is written unescaped in the prior file: its strings hold a
// region tag, a fence, ampersand, angle brackets and hyphens, plus number tokens.
const priorNativeNeedingEscapes = `{"tag":"<Region type=\"project\">","fence":"---","amp":"a&b","gt":"x>y","hy-key":"keep-me","count":-5,"ratio":1.5e-3}`

func TestEncode_Update_NativeMemberKeepsEncodingRulesAndNumberTokens(t *testing.T) {
	tr := translator(t)
	prior := wireDocument("old\n", `[]`, priorNativeNeedingEscapes)

	encoded, _, err := tr.Encode(updateCanonical(), updateCtx(prior))
	if err != nil {
		t.Fatalf("Encode(update) error: %v", err)
	}

	for _, banned := range []string{"<", ">", "&", "---"} {
		if bytesContain(encoded, banned) {
			t.Errorf("update output contains literal %q\nfile: %s", banned, encoded)
		}
	}
	raw, ok := nativeOf(t, encoded)
	if !ok {
		t.Fatalf("update output lost the native member\nfile: %s", encoded)
	}
	s := string(raw)
	if !containsAll(s, uEsc("003c"), uEsc("003e"), uEsc("0026"), "keep"+uEsc("002d")+"me", "hy"+uEsc("002d")+"key") {
		t.Errorf("native strings must be written with lower-case escapes; got %s", s)
	}
	if !containsAll(s, `-5`, `1.5e-3`) {
		t.Errorf("number tokens of native must be re-emitted as read; got %s", s)
	}
	var got, want any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("native is not valid JSON: %v", err)
	}
	_ = json.Unmarshal([]byte(priorNativeNeedingEscapes), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("native value changed\nwant: %v\ngot:  %v", want, got)
	}
}

func TestEncode_Update_NewMetaAndBodyWinOverPriorValues(t *testing.T) {
	tr := translator(t)
	prior := wireDocument("old body\n", `[["name","stale"],["description","stale"]]`, priorNative)

	encoded, _, err := tr.Encode(updateCanonical(), updateCtx(prior))
	if err != nil {
		t.Fatalf("Encode(update) error: %v", err)
	}

	assertValueEqual(t, updateCanonical(), mustDecode(t, tr, encoded))
}

func TestDecode_DoesNotRepresentNativeInCanonicalForm(t *testing.T) {
	tr := translator(t)
	file := wireDocument("body\n", `[["name","reviewer"]]`, priorNative)

	decoded := mustDecode(t, tr, file)

	if bytesContain(decoded, "user_setting") || bytesContain(decoded, "keep-me") {
		t.Errorf("native data appears in the canonical form:\n%s", decoded)
	}
}

func TestEncode_Update_PriorWithoutNativeYieldsNoNative(t *testing.T) {
	tr := translator(t)
	prior := wireDocument("old\n", `[["name","reviewer"]]`, "")

	encoded, _, err := tr.Encode(updateCanonical(), updateCtx(prior))
	if err != nil {
		t.Fatalf("Encode(update) error: %v", err)
	}

	if _, ok := nativeOf(t, encoded); ok {
		t.Errorf("update output has a native member although the prior had none")
	}
}

func TestEncode_Update_UndecodablePriorSucceedsWithoutNativeOrReport(t *testing.T) {
	tr := translator(t)
	prior := []byte("{ this is not json at all")

	encoded, rep, err := tr.Encode(updateCanonical(), updateCtx(prior))

	if err != nil {
		t.Fatalf("Encode(update) with a corrupt prior returned an error: %v", err)
	}
	if len(rep.Entries) != 0 {
		t.Errorf("report entries for a corrupt prior: %+v; want none", rep.Entries)
	}
	if _, ok := nativeOf(t, encoded); ok {
		t.Errorf("native member present although the prior did not decode")
	}
	assertValueEqual(t, updateCanonical(), mustDecode(t, tr, encoded))
}

func TestEncode_Update_WithoutPriorBytes_ReturnsErrMissingPriorBytes(t *testing.T) {
	tr := translator(t)

	_, _, err := tr.Encode(updateCanonical(), updateCtx(nil))

	assertArtifactError(t, err, agentformat.ErrMissingPriorBytes)
}

func TestEncode_UnspecifiedOperation_ReturnsErrUnspecifiedOperation(t *testing.T) {
	tr := translator(t)
	ctx := createCtx()
	ctx.Op = agentformat.OpUnspecified

	_, _, err := tr.Encode(updateCanonical(), ctx)

	assertArtifactError(t, err, agentformat.ErrUnspecifiedOperation)
}

func TestEncode_CreateWithPriorBytes_ReturnsErrUnspecifiedOperation(t *testing.T) {
	tr := translator(t)
	ctx := createCtx()
	ctx.PriorDeployed = wireDocument("old\n", `[]`, "")

	_, _, err := tr.Encode(updateCanonical(), ctx)

	assertArtifactError(t, err, agentformat.ErrUnspecifiedOperation)
}

func TestEncode_CarriageKeys_AreNotWrittenAndEachHeldKeyIsReportedStripped(t *testing.T) {
	tr := translator(t)
	fm := "name: reviewer\n" +
		agentformat.CarriageValuesKey + ":\n  user_setting: custom\n" +
		agentformat.CarriageMarkersKey + ": [\"marked_key\"]\n"

	encoded, rep, err := tr.Encode(canonicalDoc(fm, "b\n"), createCtx())

	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	decoded := mustDecode(t, tr, encoded)
	for _, key := range agentformat.CarriageKeys() {
		if bytesContain(decoded, key) || bytesContain(encoded, key) {
			t.Errorf("carriage key %q was written to the file", key)
		}
	}
	stripped := map[string]bool{}
	for _, e := range rep.Entries {
		if e.Kind != agentformat.EntryStrippedContainer {
			t.Errorf("unexpected report entry kind %q for key %q; only stripped_container is allowed", e.Kind, e.Key)
		}
		stripped[e.Key] = true
	}
	if !reflect.DeepEqual(stripped, map[string]bool{"user_setting": true, "marked_key": true}) {
		t.Errorf("stripped keys = %v, want user_setting and marked_key", stripped)
	}
	assertValueEqual(t, canonicalDoc("name: reviewer\n", "b\n"), decoded)
}

func assertArtifactError(t *testing.T, err error, sentinel error) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want one wrapping %v", sentinel)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap %v", err, sentinel)
	}
	var ae *agentformat.ArtifactError
	if !errors.As(err, &ae) {
		t.Errorf("error %v is not an *agentformat.ArtifactError", err)
	}
}
