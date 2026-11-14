package testjson_test

// registration_test.go: in a test binary that registers the test format, the format
// resolves through the runtime lookup, and the production vocabulary is untouched.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/agentformat/formatid"
	"mosaic-deploy/internal/agentformat/testjson"
)

func TestRegister_FormatResolvesThroughLookupString(t *testing.T) {
	testjson.Register()

	tr, err := agentformat.LookupString(string(testjson.ID))

	if err != nil {
		t.Fatalf("LookupString(%q) after Register: %v", testjson.ID, err)
	}
	if tr == nil {
		t.Fatal("LookupString returned a nil translator with nil error")
	}
}

func TestRegister_IsIdempotentAndKeepsTheFirstTranslator(t *testing.T) {
	testjson.Register()
	first, err := agentformat.LookupString(string(testjson.ID))
	if err != nil {
		t.Fatalf("LookupString after first Register: %v", err)
	}

	testjson.Register()
	second, err := agentformat.LookupString(string(testjson.ID))

	if err != nil {
		t.Fatalf("LookupString after second Register: %v", err)
	}
	if first != second {
		t.Errorf("a second Register replaced the translator")
	}
}

func TestRegister_DoesNotChangeProductionVocabularyOrRegistry(t *testing.T) {
	testjson.Register()

	for _, id := range formatid.All() {
		if id == testjson.ID {
			t.Errorf("formatid.All() contains the test format %q", id)
		}
	}
	if formatid.IsKnown(testjson.ID) {
		t.Errorf("formatid.IsKnown(%q) = true", testjson.ID)
	}
	if _, err := formatid.Parse(string(testjson.ID)); !errors.Is(err, formatid.ErrUnknownFormatID) {
		t.Errorf("formatid.Parse(%q) error = %v, want ErrUnknownFormatID", testjson.ID, err)
	}
	for _, id := range agentformat.RegisteredIDs() {
		if id == testjson.ID {
			t.Errorf("RegisteredIDs() contains the test format %q", id)
		}
	}
	if _, err := agentformat.Lookup(testjson.ID); !errors.Is(err, agentformat.ErrUnknownFormat) {
		t.Errorf("Lookup(%q) error = %v, want ErrUnknownFormat (production lookup must not see it)", testjson.ID, err)
	}
}

func TestRegister_ProductionFormatsStillResolve(t *testing.T) {
	testjson.Register()

	for _, id := range []string{"", string(formatid.Markdown)} {
		if _, err := agentformat.LookupString(id); err != nil {
			t.Errorf("LookupString(%q) after Register: %v", id, err)
		}
	}
}
